// Copyright (c) 2020 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package index

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/zyjblockchain/sandy_log/log"
	"io"
	"tezos_index/chain"
	"tezos_index/puller/models"
	"tezos_index/rpc"
	"time"
)

const (
	firstVoteBlock int64 = 2 // chain is initialized at block 1 !
)

var (
	// ErrNoElectionEntry is an error that indicates a requested entry does
	// not exist in the election table.
	ErrNoElectionEntry = errors.New("election not found")

	// ErrNoProposalEntry is an error that indicates a requested entry does
	// not exist in the proposal table.
	ErrNoProposalEntry = errors.New("proposal not found")

	// ErrNoVoteEntry is an error that indicates a requested entry does
	// not exist in the vote table.
	ErrNoVoteEntry = errors.New("vote not found")

	// ErrNoBallotEntry is an error that indicates a requested entry does
	// not exist in the ballot table.
	ErrNoBallotEntry = errors.New("ballot not found")
)

type GovIndex struct {
	db *gorm.DB
}

func NewGovIndex(db *gorm.DB) *GovIndex {
	return &GovIndex{db}
}

func (idx *GovIndex) DB() *gorm.DB {
	return idx.db
}

func (idx *GovIndex) ConnectBlock(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// skip genesis and bootstrap blocks
	if block.Height < firstVoteBlock {
		return nil
	}

	// detect first and last block of a voting period
	isPeriodStart := block.Height == firstVoteBlock || block.Params.IsVoteStart(block.Height)
	isPeriodEnd := block.Height > firstVoteBlock && block.Params.IsVoteEnd(block.Height)

	// open a new election or vote on first block
	if isPeriodStart {
		if block.VotingPeriodKind == chain.VotingPeriodProposal {
			if err := idx.openElection(ctx, block, builder); err != nil {
				log.Errorf("Open election at block %d %s: %v", block.Height, block.VotingPeriodKind, err)
				return err
			}
		}
		if err := idx.openVote(ctx, block, builder); err != nil {
			log.Errorf("Open vote at block %d %s: %v", block.Height, block.VotingPeriodKind, err)
			return err
		}
	}

	// process proposals (1) or ballots (2, 4)
	var err error
	switch block.VotingPeriodKind {
	case chain.VotingPeriodProposal:
		err = idx.processProposals(ctx, block, builder)
	case chain.VotingPeriodTestingVote:
		err = idx.processBallots(ctx, block, builder)
	case chain.VotingPeriodTesting:
		// nothing to do here
	case chain.VotingPeriodPromotionVote:
		err = idx.processBallots(ctx, block, builder)
	}
	if err != nil {
		return err
	}

	// close any previous period after last block
	if isPeriodEnd {
		// close the last vote
		success, err := idx.closeVote(ctx, block, builder)
		if err != nil {
			log.Errorf("Close %s vote at block %d: %v", block.VotingPeriodKind, block.Height, err)
			return err
		}

		// on failure or on end, close last election
		if !success || block.VotingPeriodKind == chain.VotingPeriodProposal {
			if err := idx.closeElection(ctx, block, builder); err != nil {
				log.Errorf("Close election at block %d: %v", block.Height, err)
				return err
			}
		}
	}
	return nil
}

func (idx *GovIndex) DisconnectBlock(ctx context.Context, block *models.Block, _ models.BlockBuilder) error {
	return idx.DeleteBlock(ctx, block.Height)
}

func (idx *GovIndex) DeleteBlock(ctx context.Context, height int64) error {
	// delete ballots by height
	if err := idx.DB().Where("height = ?", height).Delete(&models.Ballot{}).Error; err != nil {
		return nil
	}

	// delete proposals by height
	if err := idx.DB().Where("height = ?", height).Delete(&models.Proposal{}).Error; err != nil {
		return nil
	}

	// on vote period start, delete vote by start height
	if err := idx.DB().Where("period_start_height = ?", height).Delete(&models.Vote{}).Error; err != nil {
		return nil
	}

	// on election start, delete election (maybe not because we use row id as counter)
	if err := idx.DB().Where("start_height = ?", height).Delete(&models.Election{}).Error; err != nil {
		return nil
	}
	return nil
}

func (idx *GovIndex) openElection(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	election := &models.Election{
		NumPeriods:   1,
		VotingPeriod: block.TZ.Block.Metadata.Level.VotingPeriod,
		StartTime:    block.Timestamp,
		EndTime:      time.Time{}.UTC(), // set on close
		StartHeight:  block.Height,
		EndHeight:    0, // set on close
		IsEmpty:      true,
		IsOpen:       true,
		IsFailed:     false,
		NoQuorum:     false,
		NoMajority:   false,
	}

	return idx.DB().Create(election).Error
}

func (idx *GovIndex) closeElection(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// load current election
	election, err := idx.electionByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return err
	}
	// check its open
	if !election.IsOpen {
		return fmt.Errorf("closing election: election %d already closed", election.RowId)
	}
	// load current vote
	vote, err := idx.voteByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return err
	}
	// check its closed
	if vote.IsOpen {
		return fmt.Errorf("closing election: vote %d/%d is not closed", vote.ElectionId, vote.VotingPeriod)
	}
	election.EndHeight = vote.EndHeight
	election.EndTime = vote.EndTime
	election.IsOpen = false
	election.IsEmpty = election.NumProposals == 0 && vote.IsFailed
	election.IsFailed = vote.IsFailed
	election.NoQuorum = vote.NoQuorum
	election.NoMajority = vote.NoMajority

	return idx.DB().Model(&models.Election{}).Updates(election).Error
}

func (idx *GovIndex) openVote(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// load current election, must exist
	election, err := idx.electionByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return err
	}
	if !election.IsOpen {
		return fmt.Errorf("opening vote: election %d already closed", election.RowId)
	}

	// update election
	switch block.VotingPeriodKind {
	case chain.VotingPeriodTestingVote:
		election.NumPeriods = 2
	case chain.VotingPeriodTesting:
		election.NumPeriods = 3
	case chain.VotingPeriodPromotionVote:
		election.NumPeriods = 4
	}

	// Note: this adjusts end height of first cycle (we run this func at height 2 instead of 0)
	//       otherwise the formula could be simpler
	p := block.Params
	endHeight := (block.Height - block.Height%p.BlocksPerVotingPeriod) + p.BlocksPerVotingPeriod

	vote := &models.Vote{
		ElectionId:       election.RowId,
		ProposalId:       election.ProposalId, // voted proposal, zero in first voting period
		VotingPeriod:     block.TZ.Block.Metadata.Level.VotingPeriod,
		VotingPeriodKind: block.VotingPeriodKind,
		StartTime:        block.Timestamp,
		EndTime:          time.Time{}.UTC(), // set on close
		StartHeight:      block.Height,
		EndHeight:        endHeight,
		IsOpen:           true,
	}

	// add rolls and calculate quorum
	// - use current (cycle resp. vote start block) for roll snapshot
	// - at genesis there is no parent block, we use defaults here
	cd := block.Chain
	vote.EligibleRolls = cd.Rolls
	vote.EligibleVoters = cd.RollOwners
	switch vote.VotingPeriodKind {
	case chain.VotingPeriodProposal:
		// fixed min proposal quorum as defined by protocol
		vote.QuorumPct = p.MinProposalQuorum
	case chain.VotingPeriodTesting:
		// no quorum
		vote.QuorumPct = 0
	case chain.VotingPeriodTestingVote, chain.VotingPeriodPromotionVote:
		// from most recent (testing_vote or promotion_vote) period
		quorumPct, turnoutEma, err := idx.quorumByHeight(ctx, block.Height, p)
		if err != nil {
			return err
		}
		// quorum adjusts at the end of each voting period (exploration & promotion)
		vote.QuorumPct = quorumPct
		vote.TurnoutEma = turnoutEma
	}
	vote.QuorumRolls = vote.EligibleRolls * vote.QuorumPct / 10000

	// insert vote
	if err := idx.DB().Create(vote).Error; err != nil {
		return err
	}

	// update election
	return idx.DB().Model(&models.Election{}).Updates(election).Error
}

func (idx *GovIndex) closeVote(ctx context.Context, block *models.Block, builder models.BlockBuilder) (bool, error) {
	// load current vote
	vote, err := idx.voteByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return false, err
	}
	// check its open
	if !vote.IsOpen {
		return false, fmt.Errorf("closing vote: vote %d/%d is already closed", vote.ElectionId, vote.VotingPeriod)
	}
	params := block.Params

	// determine result
	switch vote.VotingPeriodKind {
	case chain.VotingPeriodProposal:
		// select the winning proposal if any and update election
		var isDraw bool
		if vote.TurnoutRolls > 0 {
			proposals, err := idx.proposalsByElection(ctx, vote.ElectionId)
			if err != nil {
				return false, err
			}

			// select the winner
			var (
				winner models.ProposalID
				count  int64
			)
			for _, v := range proposals {
				if v.Rolls < count {
					continue
				}
				if v.Rolls > count {
					isDraw = false
					count = v.Rolls
					winner = v.RowId
				} else {
					isDraw = true
				}
			}

			if !isDraw {
				// load current election, must exist
				election, err := idx.electionByHeight(ctx, block.Height, block.Params)
				if err != nil {
					return false, err
				}
				// store winner and update election
				election.ProposalId = winner
				if err := idx.DB().Model(&models.Election{}).Updates(election).Error; err != nil {
					return false, err
				}
				vote.ProposalId = winner
			}
		}
		vote.NoProposal = vote.TurnoutRolls == 0
		vote.NoQuorum = params.MinProposalQuorum > 0 && vote.TurnoutRolls < vote.QuorumRolls
		vote.IsDraw = isDraw
		vote.IsFailed = vote.NoProposal || vote.NoQuorum || vote.IsDraw

	case chain.VotingPeriodTestingVote, chain.VotingPeriodPromotionVote:
		vote.NoQuorum = vote.TurnoutRolls < vote.QuorumRolls
		vote.NoMajority = vote.YayRolls < (vote.YayRolls+vote.NayRolls)*8/10
		vote.IsFailed = vote.NoQuorum || vote.NoMajority

	case chain.VotingPeriodTesting:
		// empty, cannot fail
	}

	vote.EndTime = block.Timestamp
	vote.IsOpen = false

	if err := idx.DB().Model(&models.Vote{}).Updates(vote).Error; err != nil {
		return false, err
	}

	return !vote.IsFailed, nil
}

func (idx *GovIndex) processProposals(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// skip blocks without proposals
	if block.NProposal == 0 {
		return nil
	}

	// load current vote
	vote, err := idx.voteByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return err
	}

	// load known proposals
	proposals, err := idx.proposalsByElection(ctx, vote.ElectionId)
	if err != nil {
		return err
	}
	proposalMap := make(map[string]*models.Proposal)
	for _, v := range proposals {
		proposalMap[v.Hash.String()] = v
	}

	// find unknown proposals
	insProposals := make([]*models.Proposal, 0)
	for _, op := range block.Ops {
		if op.Type != chain.OpTypeProposals {
			continue
		}
		cop, ok := block.GetRPCOp(op.OpN, op.OpC)
		if !ok {
			return fmt.Errorf("missing proposal op [%d:%d]", op.OpN, op.OpC)
		}
		pop, ok := cop.(*rpc.ProposalsOp)
		if !ok {
			return fmt.Errorf("ballot op [%d:%d]: unexpected type %T ", op.OpN, op.OpC, cop)
		}
		acc, aok := builder.AccountByAddress(pop.Source)
		if !aok {
			return fmt.Errorf("missing account %s in proposal op [%d:%d]", pop.Source, op.OpN, op.OpC)
		}
		for _, v := range pop.Proposals {
			if _, ok := proposalMap[v.String()]; ok {
				continue
			}
			prop := &models.Proposal{
				Hash:         v,
				Height:       block.Height,
				Time:         block.Timestamp,
				SourceId:     acc.RowId,
				OpId:         op.RowId,
				ElectionId:   vote.ElectionId,
				VotingPeriod: vote.VotingPeriod,
			}
			insProposals = append(insProposals, prop)
		}
	}

	// insert unknown proposals to create ids
	if len(insProposals) > 0 {
		// todo batch insert
		tx := idx.DB().Begin()
		for _, val := range insProposals {
			if err := tx.Create(val).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		tx.Commit()

		for _, p := range insProposals {
			proposalMap[p.Hash.String()] = p
		}
		// update election, counting proposals
		election, err := idx.electionByHeight(ctx, block.Height, block.Params)
		if err != nil {
			return err
		}
		// check its open
		if !election.IsOpen {
			return fmt.Errorf("update election: election %d already closed", election.RowId)
		}
		election.IsEmpty = false
		election.NumProposals += len(insProposals)
		if err := idx.DB().Model(&models.Election{}).Updates(election).Error; err != nil {
			return err
		}
	}

	// create and store ballots for each proposal
	insBallots := make([]*models.Ballot, 0)
	for _, op := range block.Ops {
		if op.Type != chain.OpTypeProposals {
			continue
		}
		cop, ok := block.GetRPCOp(op.OpN, op.OpC)
		if !ok {
			return fmt.Errorf("missing proposal op [%d:%d]", op.OpN, op.OpC)
		}
		pop, ok := cop.(*rpc.ProposalsOp)
		if !ok {
			return fmt.Errorf("proposals op [%d:%d]: unexpected type %T ", op.OpN, op.OpC, cop)
		}
		acc, aok := builder.AccountByAddress(pop.Source)
		if !aok {
			return fmt.Errorf("missing account %s in proposal op [%d:%d]", pop.Source, op.OpN, op.OpC)
		}
		// load account rolls at snapshot block (i.e. at current voting period start - 1)
		rolls, err := idx.rollsByHeight(ctx, acc.RowId, vote.StartHeight-1)
		if err != nil {
			return fmt.Errorf("missing roll snapshot for %s in vote period %d (%s) start %d",
				acc, vote.VotingPeriod, vote.VotingPeriodKind, vote.StartHeight)
		}
		// fix for missing pre-genesis snapshot
		if block.Cycle == 0 && rolls == 0 {
			rolls = acc.Rolls(block.Params)
		}

		// create ballots for all proposals
		for _, v := range pop.Proposals {
			prop, ok := proposalMap[v.String()]
			if !ok {
				return fmt.Errorf("missing proposal %s in op [%d:%d]", v, op.OpN, op.OpC)
			}

			var cnt int
			err := idx.DB().Model(&models.Ballot{}).Where("source_id = ? and voting_period = ? and proposal_id = ?",
				acc.RowId.Value(), vote.VotingPeriod, prop.RowId.Value()).Count(&cnt).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			} else if cnt > 0 {
				continue
			}

			b := &models.Ballot{
				ElectionId:       vote.ElectionId,
				ProposalId:       prop.RowId,
				VotingPeriod:     vote.VotingPeriod,
				VotingPeriodKind: vote.VotingPeriodKind,
				Height:           block.Height,
				Time:             block.Timestamp,
				SourceId:         acc.RowId,
				OpId:             op.RowId,
				Rolls:            rolls,
				Ballot:           chain.BallotVoteYay,
			}
			insBallots = append(insBallots, b)

			// update proposal too
			// log.Infof("New voter %s for proposal %s with %d rolls (add to %d voters, %d rolls)",
			// 	acc, v, rolls, prop.Voters, prop.Rolls)
			prop.Voters++
			prop.Rolls += rolls
		}

		// update vote, skip when the same account voted already
		var cnt int
		err = idx.DB().Model(&models.Ballot{}).Where("source_id = ? and voting_period = ?", acc.RowId.Value(), vote.VotingPeriod).Count(&cnt).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		} else if cnt == 0 {
			// log.Infof("Update turnout for period %d voter %s with %d rolls (add to %d voters, %d rolls)",
			// 	vote.VotingPeriod, acc, rolls, vote.TurnoutVoters, vote.TurnoutRolls)
			vote.TurnoutRolls += rolls
			vote.TurnoutVoters++
		} else {
			// log.Infof("Skipping turnout calc for period %d voter %s  with %d rolls (already voted %d times)", vote.VotingPeriod, acc, rolls, cnt)
		}
	}

	// update eligible rolls when zero (happens when vote opens on genesis)
	if vote.EligibleRolls == 0 {
		vote.EligibleRolls = block.Chain.Rolls
		vote.EligibleVoters = block.Chain.RollOwners
		vote.QuorumPct, _, _ = idx.quorumByHeight(ctx, block.Height, block.Params)
	}

	// finalize vote for this round and safe
	vote.TurnoutPct = vote.TurnoutRolls * 10000 / vote.EligibleRolls
	if err := idx.DB().Model(&models.Vote{}).Updates(vote).Error; err != nil {
		return err
	}

	// finalize proposals, reuse slice
	insProposals = insProposals[:0]
	for _, v := range proposalMap {
		insProposals = append(insProposals, v)
	}
	if err := idx.DB().Model(&models.Proposal{}).Updates(insProposals).Error; err != nil {
		return err
	}

	return idx.DB().Create(insBallots).Error
}

func (idx *GovIndex) processBallots(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// skip blocks without ballots
	if block.NBallot == 0 {
		return nil
	}

	// load current vote
	vote, err := idx.voteByHeight(ctx, block.Height, block.Params)
	if err != nil {
		return err
	}

	insBallots := make([]*models.Ballot, 0)
	for _, op := range block.Ops {
		if op.Type != chain.OpTypeBallot {
			continue
		}
		cop, ok := block.GetRPCOp(op.OpN, op.OpC)
		if !ok {
			return fmt.Errorf("missing ballot op [%d:%d]", op.OpN, op.OpC)
		}
		bop, ok := cop.(*rpc.BallotOp)
		if !ok {
			return fmt.Errorf("ballot op [%d:%d]: unexpected type %T ", op.OpN, op.OpC, cop)
		}
		acc, aok := builder.AccountByAddress(bop.Source)
		if !aok {
			return fmt.Errorf("missing account %s in proposal op [%d:%d]", bop.Source, op.OpN, op.OpC)
		}
		// load account rolls at snapshot block (i.e. at current voting period start - 1)
		rolls, err := idx.rollsByHeight(ctx, acc.RowId, vote.StartHeight-1)
		if err != nil {
			return fmt.Errorf("missing roll snapshot for %s in vote period %d (%s) start %d",
				acc, vote.VotingPeriod, vote.VotingPeriodKind, vote.StartHeight)
		}
		// fix for missing pre-genesis snapshot
		if block.Cycle == 0 && rolls == 0 {
			rolls = acc.Rolls(block.Params)
		}

		// update vote
		vote.TurnoutRolls += rolls
		vote.TurnoutVoters++
		switch bop.Ballot {
		case chain.BallotVoteYay:
			vote.YayRolls += rolls
			vote.YayVoters++
		case chain.BallotVoteNay:
			vote.NayRolls += rolls
			vote.NayVoters++
		case chain.BallotVotePass:
			vote.PassRolls += rolls
			vote.PassVoters++
		}

		b := &models.Ballot{
			ElectionId:       vote.ElectionId,
			ProposalId:       vote.ProposalId,
			VotingPeriod:     vote.VotingPeriod,
			VotingPeriodKind: vote.VotingPeriodKind,
			Height:           block.Height,
			Time:             block.Timestamp,
			SourceId:         acc.RowId,
			OpId:             op.RowId,
			Rolls:            rolls,
			Ballot:           bop.Ballot,
		}
		insBallots = append(insBallots, b)
	}

	// update eligible rolls when zero (happens when vote opens on genesis)
	if vote.EligibleRolls == 0 {
		vote.EligibleRolls = block.Chain.Rolls
		vote.EligibleVoters = block.Chain.RollOwners
		vote.QuorumPct, _, _ = idx.quorumByHeight(ctx, block.Height, block.Params)
	}

	// finalize vote for this round and safe
	vote.TurnoutPct = vote.TurnoutRolls * 10000 / vote.EligibleRolls
	if err := idx.DB().Model(&models.Vote{}).Updates(vote).Error; err != nil {
		return err
	}

	return idx.DB().Create(insBallots).Error
}

func (idx *GovIndex) electionByHeight(ctx context.Context, height int64, params *chain.Params) (*models.Election, error) {
	election := &models.Election{}
	err := idx.DB().Where("start_height <= ?", height).Last(election).Error
	if err != nil {
		return nil, err
	}
	if election.RowId == 0 {
		return nil, ErrNoElectionEntry
	}
	return election, nil
}

func (idx *GovIndex) voteByHeight(ctx context.Context, height int64, params *chain.Params) (*models.Vote, error) {
	vote := &models.Vote{}
	err := idx.DB().Where("period_start_height <= ?", height).Last(vote).Error
	if err != nil {
		return nil, err
	}
	if vote.RowId == 0 {
		return nil, ErrNoVoteEntry
	}
	return vote, nil
}

func (idx *GovIndex) proposalsByElection(ctx context.Context, eid models.ElectionID) ([]*models.Proposal, error) {
	var proposals []*models.Proposal
	err := idx.DB().Where("election_id = ?", eid.Value()).Find(&proposals).Error
	if err != nil {
		return nil, err
	}
	return proposals, nil
}

func (idx *GovIndex) ballotsByVote(ctx context.Context, period int64) ([]*models.Ballot, error) {
	var ballots []*models.Ballot
	err := idx.DB().Where("voting_period = ?", period).Find(&ballots).Error
	if err != nil {
		return nil, err
	}
	return ballots, nil
}

func (idx *GovIndex) rollsByHeight(ctx context.Context, aid models.AccountID, height int64) (int64, error) {
	xr := &models.Snapshot{}
	err := idx.DB().Select("rolls").Where("height = ? and account_id = ?", height, aid.Value()).Last(xr).Error
	if err != nil {
		return 0, err
	}
	return xr.Rolls, nil
}

// quorums adjust at the end of each exploration & promotion voting period
// starting in v005 the algorithm changes to track participation as EMA (80/20)
func (idx *GovIndex) quorumByHeight(ctx context.Context, height int64, params *chain.Params) (int64, int64, error) {
	// find most recent exploration or promotion period
	var lastQuorum, lastTurnout, lastTurnoutEma, nextQuorum, nextEma int64
	var votes []*models.Vote
	// todo 顺序可能有问题
	err := idx.DB().Where("period_start_height < ?", height).Order("row_id desc").Find(&votes).Error // stream
	if err != nil {
		return 0, 0, err
	}
	for _, vote := range votes {
		switch vote.VotingPeriodKind {
		case chain.VotingPeriodTestingVote, chain.VotingPeriodPromotionVote:
			lastQuorum = vote.QuorumPct
			lastTurnout = vote.TurnoutPct
			lastTurnoutEma = vote.TurnoutEma
			err = io.EOF
		}
	}

	if err != io.EOF {
		// initial protocol quorum
		if params.Version < 5 {
			return 8000, 0, nil
		} else {
			lastTurnoutEma = params.QuorumMax
			nextQuorum = params.QuorumMin + lastTurnoutEma*(params.QuorumMax-params.QuorumMin)/10000
			return nextQuorum, lastTurnoutEma, nil
		}
	}

	// calculate next quorum
	switch true {
	case params.Version >= 5:
		// Babylon v005 changed this to participation EMA and min/max caps
		if lastTurnoutEma == 0 {
			if lastTurnout == 0 {
				// init from upper bound on chains that never had Athens votes
				// nextEma = params.QuorumMax
				lastTurnoutEma = params.QuorumMax
			} else {
				// init from last Athens quorum
				lastTurnoutEma = (8*lastQuorum + 2*lastTurnout) / 10
				// nextEma = lastTurnoutEma
			}
			nextEma = lastTurnoutEma
		} else {
			// update using actual turnout
			nextEma = (8*lastTurnoutEma + 2*lastTurnout) / 10
		}

		// q = q_min + participation_ema * (q_max - q_min)
		nextQuorum = params.QuorumMin + nextEma*(params.QuorumMax-params.QuorumMin)/10000

	default:
		// 80/20 until Athens v004
		nextQuorum = (8*lastQuorum + 2*lastTurnout) / 10
	}

	return nextQuorum, nextEma, nil
}
