// Copyright (c) 2020 Blockwatch Data Inc.

package index

import (
	"context"
	"github.com/jinzhu/gorm"
	"github.com/zyjblockchain/sandy_log/log"
	"tezos_index/puller/models"
	"tezos_index/utils"
)

type AccountIndex struct {
	db *gorm.DB
}

func NewAccountIndex(db *gorm.DB) *AccountIndex {
	return &AccountIndex{db}
}

func (idx *AccountIndex) DB() *gorm.DB {
	return idx.db
}

func (idx *AccountIndex) ConnectBlock(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	upd := make([]*models.Account, 0, len(builder.Accounts()))
	// regular accounts
	for _, acc := range builder.Accounts() {
		if acc.IsDirty {
			upd = append(upd, acc)
		}
	}
	// delegate accounts
	for _, acc := range builder.Delegates() {
		if acc.IsDirty {
			upd = append(upd, acc)
		}
	}
	// todo batch update
	for _, upAcc := range upd {
		if err := idx.DB().Model(&models.Account{}).Updates(upAcc).Error; err != nil {
			log.Errorf("update account record error: %v; upAccount: %v", err, upAcc)
			return err
		}
	}
	return nil
}

func (idx *AccountIndex) DisconnectBlock(ctx context.Context, block *models.Block, builder models.BlockBuilder) error {
	// accounts to delete
	del := make([]uint64, 0)
	// accounts to update
	upd := make([]*models.Account, 0, len(builder.Accounts()))

	// regular accounts
	for _, acc := range builder.Accounts() {
		if acc.MustDelete {
			del = append(del, acc.RowId.Value())
		} else if acc.IsDirty {
			upd = append(upd, acc)
		}
	}

	// delegate accounts
	for _, acc := range builder.Delegates() {
		if acc.MustDelete {
			del = append(del, acc.RowId.Value())
		} else if acc.IsDirty {
			upd = append(upd, acc)
		}
	}

	// delete account
	if len(del) > 0 {
		// remove duplicates and sort; returns new slice
		del = utils.UniqueUint64Slice(del)
		log.Debugf("Rollback removing accounts %#v", del)
		if err := idx.DB().Where("row_id in (?)", del).Delete(&models.Account{}).Error; err != nil {
			log.Errorf("batch delete account error: %v", err)
			return err
		}
	}

	// Note on rebuild:
	// we don't rebuild last in/out counters since we assume
	// after reorg completes these counters are set properly again

	// todo batch update
	for _, upAcc := range upd {
		if err := idx.DB().Model(&models.Account{}).Updates(upAcc).Error; err != nil {
			log.Errorf("update account record error: %v; upAccount: %v", err, upAcc)
			return err
		}
	}
	return nil
}

// DeleteBlock
func (idx *AccountIndex) DeleteBlock(ctx context.Context, height int64) error {
	log.Debugf("Rollback deleting accounts at height %d", height)
	err := idx.DB().Where("first_seen = ?", height).Delete(&models.Account{}).Error
	return err
}
