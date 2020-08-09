// Copyright (c) 2020 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package index

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/zyjblockchain/sandy_log/log"
	"tezos_index/puller/models"
)

const BlockIndexKey = "block"

type BlockIndex struct {
	db *gorm.DB
}

var (
	// ErrNoBlockEntry is an error that indicates a requested entry does
	// not exist in the block bucket.
	ErrNoBlockEntry = errors.New("block not indexed")

	// ErrInvalidBlockHeight
	ErrInvalidBlockHeight = errors.New("invalid block height")

	// ErrInvalidBlockHash
	ErrInvalidBlockHash = errors.New("invalid block hash")
)

func NewBlockIndex(db *gorm.DB) *BlockIndex {
	return &BlockIndex{db}
}

func (idx *BlockIndex) DB() *gorm.DB {
	return idx.db
}

func (idx *BlockIndex) Key() string {
	return BlockIndexKey
}

func (idx *BlockIndex) ConnectBlock(ctx context.Context, block *models.Block, b models.BlockBuilder) error {
	// update parent block to write blocks endorsed bitmap
	if block.Parent != nil && block.Parent.Height > 0 {
		// 更新 parent block after build
		if err := idx.DB().Model(&models.Block{}).Updates(block.Parent).Error; err != nil {
			return fmt.Errorf("parent update: %v", err)
		}
	}

	// fetch and update snapshot block
	if snap := block.TZ.Snapshot; snap != nil {
		snapHeight := block.Params.SnapshotBlock(snap.Cycle, snap.RollSnapshot)
		log.Debugf("Marking block %d [%d] index %d as roll snapshot for cycle %d",
			snapHeight, block.Params.CycleFromHeight(snapHeight), snap.RollSnapshot, snap.Cycle)

		snapBlock := &models.Block{}
		err := idx.DB().Where("height = ?", snapHeight).First(snapBlock).Error
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("missing snapshot index block %d for cycle %d", snapHeight, snap.Cycle)
		}
		if err != nil {
			return fmt.Errorf("snapshot index block %d for cycle %d: %v", snapHeight, snap.Cycle, err)
		}

		snapBlock.IsCycleSnapshot = true

		if err := idx.DB().Model(&models.Block{}).Updates(snapBlock).Error; err != nil {
			return fmt.Errorf("snapshot index block %d: %v", snapHeight, err)
		}
	}

	// Note: during reorg some blocks may already exist (have a valid row id)
	// we assume insert will update such rows instead of creating new rows
	return idx.DB().Create(block).Error
}

func (idx *BlockIndex) DisconnectBlock(ctx context.Context, block *models.Block, _ models.BlockBuilder) error {
	// parent update will be done on next connect
	return idx.DB().Model(&models.Block{}).Updates(block).Error
}

func (idx *BlockIndex) DeleteBlock(ctx context.Context, height int64) error {
	log.Debugf("Rollback deleting block at height %d", height)
	return idx.DB().Where("height = ?", height).Delete(&models.Block{}).Error
}
