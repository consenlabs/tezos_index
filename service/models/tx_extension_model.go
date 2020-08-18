package models

import (
	"github.com/jinzhu/gorm"
	"tezos_index/chain"
	"time"
)

type TransactionExtension struct {
	TxHash          chain.StrOpHash `gorm:"index" json:"txHash"`
	From            string          `json:"from"`
	To              string          `json:"to"`
	Memo            *string         `json:"memo"`
	ContractAddress string          `json:"contractAddress"`
	Amount          uint64          `json:"amount"`
	DeviceToken     string          `json:"deviceToken"`
	Nonce           uint64          `json:"nonce"`
	GasPrice        uint64          `json:"gasPrice"`
	CreatedAt       time.Time       `json:"createdAt"`
	Referrer        string          `json:"referrer"`
	Speed           *string         `json:"speed"`
}

func (t *TransactionExtension) Create(db *gorm.DB) error {
	return db.Create(t).Error
}

func (t *TransactionExtension) Update(db *gorm.DB, updTx *TransactionExtension) error {
	return db.Model(t).Where("tx_hash = ?", updTx.TxHash).Updates(updTx).Error
}

func (t *TransactionExtension) Exist(db *gorm.DB, txHash chain.StrOpHash) (bool, error) {
	var tt TransactionExtension
	err := db.Model(t).Where("tx_hash = ?", txHash).First(&tt).Error
	if err == gorm.ErrRecordNotFound {
		return true, nil
	}
	return false, err
}
