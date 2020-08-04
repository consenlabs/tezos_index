// Copyright (c) 2020 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package models

import (
	"sync"
	"tezos_index/chain"
)

var rightPool = &sync.Pool{
	New: func() interface{} { return new(Right) },
}

type Right struct {
	RowId          uint64          `gorm:"primary_key;column:row_id"   json:"row_id"`            // unique id
	Type           chain.RightType `gorm:"column:type"      json:"type"`                         // default accounts
	Height         int64           `gorm:"column:height"      json:"height"`                     // bc: block height (also for orphans)
	Cycle          int64           `gorm:"column:cycle"      json:"cycle"`                       // bc: block cycle (tezos specific)
	Priority       int             `gorm:"column:priority"      json:"priority"`                 // baking prio or endorsing slot
	AccountId      AccountID       `gorm:"column:account_id"      json:"account_id"`             // original rights holder
	IsLost         bool            `gorm:"column:is_lost"      json:"is_lost"`                   // owner lost this baking right
	IsStolen       bool            `gorm:"column:is_stolen"      json:"is_stolen"`               // owner stole this baking right
	IsMissed       bool            `gorm:"column:is_missed"      json:"is_missed"`               // owner missed using this endorsement right
	IsSeedRequired bool            `gorm:"column:is_seed_required"      json:"is_seed_required"` // seed nonce must be revealed (height%32==0)
	IsSeedRevealed bool            `gorm:"column:is_seed_revealed"      json:"is_seed_revealed"` // seed nonce has been revealed in next cycle
}

func (r *Right) ID() uint64 {
	return r.RowId
}

func (r *Right) SetID(id uint64) {
	r.RowId = id
}

func AllocRight() *Right {
	return rightPool.Get().(*Right)
}

func (r *Right) Free() {
	r.Reset()
	rightPool.Put(r)
}

func (r *Right) Reset() {
	r.RowId = 0
	r.Type = 0
	r.Height = 0
	r.Cycle = 0
	r.Priority = 0
	r.AccountId = 0
	r.IsLost = false
	r.IsStolen = false
	r.IsMissed = false
	r.IsSeedRequired = false
	r.IsSeedRevealed = false
}
