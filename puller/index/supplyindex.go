// Copyright (c) 2020 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package index

import (
	"context"
	"github.com/jinzhu/gorm"
	"tezos_index/puller/models"
)

type SupplyIndex struct {
	db *gorm.DB
}

func NewSupplyIndex(db *gorm.DB) *SupplyIndex {
	return &SupplyIndex{db}
}

func (idx *SupplyIndex) DB() *gorm.DB {
	return idx.db
}

func (idx *SupplyIndex) ConnectBlock(ctx context.Context, block *models.Block, _ models.BlockBuilder) error {
	return idx.DB().Model(&models.Supply{}).Create(block.Supply).Error
}

func (idx *SupplyIndex) DisconnectBlock(ctx context.Context, block *models.Block, _ models.BlockBuilder) error {
	return idx.DeleteBlock(ctx, block.Height)
}

func (idx *SupplyIndex) DeleteBlock(ctx context.Context, height int64) error {
	log.Debugf("Rollback deleting supply state at height %d", height)
	return idx.DB().Where("height = ?", height).Delete(&models.Supply{}).Error
}
