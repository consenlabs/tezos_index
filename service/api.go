package service

import (
	"errors"
	"github.com/jinzhu/gorm"
	"github.com/zyjblockchain/sandy_log/log"
	"net/http"
	"tezos_index/chain"
	"tezos_index/puller"
	model "tezos_index/puller/models"
	"tezos_index/service/models"
)

var (
	ErrInvalidParameter = errors.New("invalid_parameter")
)

type WalletService struct {
	*gorm.DB
	env *puller.Environment
}

func NewWalletService(env *puller.Environment) *WalletService {
	return &WalletService{
		DB:  env.Engine,
		env: env,
	}
}

// CreateTx
func (w *WalletService) CreateTx(r *http.Request, args *models.TransactionExtension, reply *string) error {
	args.DeviceToken = r.Header.Get("X-DEVICE-TOKEN")

	if len(args.TxHash) == 0 {
		return ErrInvalidParameter
	}

	// check exist
	exist, err := args.Exist(w.DB, args.TxHash)
	if err != nil {
		log.Errorf("operation db error: %v", err)
		return err
	}
	if exist {
		// update
		if err := args.Update(w.DB, args); err != nil {
			return err
		}
	} else {
		// insert
		if err := args.Create(w.DB); err != nil {
			return err
		}
	}
	*reply = "ok"
	return nil
}

type TxByHashReq struct {
	TxHash string `json:"tx_hash"`
}

// GetTxByHash
func (w *WalletService) GetTxByHash(r *http.Request, args *TxByHashReq, reply *[]*OrdinaryTxResult) error {
	_, err := chain.ParseOperationHash(args.TxHash)
	if err != nil {
		log.Errorf("parse txHash error: %v", err)
		return err
	}

	// find
	var txOps []*model.Op
	err = w.DB.Select("height, sender, receiver, volume, fee, counter,"+
		"status, is_success, gas_limit, gas_used, gas_price, "+
		"storage_limit, storage_size, storage_paid, time").Where("hash = ?", args.TxHash).Find(&txOps).Error
	if err != nil {
		log.Errorf("find tx by hash from ops table error: %v", err)
		return err
	}
	if len(txOps) == 0 {
		return gorm.ErrRecordNotFound
	}

	results := make([]*OrdinaryTxResult, 0, len(txOps))

	senderAcc := &model.Account{}
	if err := w.DB.Where("row_id = ?", txOps[0].SenderId).First(senderAcc).Error; err != nil {
		log.Errorf("get account by row_id %d error: %v", txOps[0].SenderId, err)
		return err
	}

	for _, v := range txOps {
		res := &OrdinaryTxResult{
			Height:       v.Height,
			TxHash:       args.TxHash,
			Sender:       senderAcc.String(),
			Receiver:     "",
			Amount:       v.Volume,
			Fee:          v.Fee,
			Counter:      v.Counter,
			Status:       v.Status,
			IsSuccess:    v.IsSuccess,
			GasLimit:     v.GasLimit,
			GasUsed:      v.GasUsed,
			GasPrice:     v.GasPrice,
			StorageLimit: v.StorageLimit,
			StorageSize:  v.StorageSize,
			StoragePaid:  v.StoragePaid,
			Timestamp:    v.Timestamp,
		}
		receiverAcc := &model.Account{}
		if err := w.DB.Where("row_id = ?", v.ReceiverId).First(receiverAcc).Error; err != nil {
			log.Errorf("get account by row_id %d error: %v", v.ReceiverId, err)
			return err
		}
		res.Receiver = receiverAcc.String()

		results = append(results, res)
	}
	reply = &results
	return nil
}

type TxsByAddr struct {
	Address string `json:"address"`
}

func (w *WalletService) GetTxListByAddress(r *http.Request, args *TxsByAddr, reply *[]*OrdinaryTxResult) error {
	addr, err := chain.ParseAddress(args.Address)
	if err != nil {
		log.Errorf("Input address [%s] error: %v", args.Address, err)
		return err
	}

	// get accountId from account table
	acc := model.Account{}
	if err := w.DB.Select("row_id, hash, address_type").Where("hash = ? and address_type = ?", addr.Hash, addr.Type).First(&acc).Error; err != nil {
		log.Errorf("get accountId from account table by address [%s] error: %v", args.Address, err)
		return err
	}

	// query ops by accountId
	var txOps []*model.Op
	err = w.DB.Select("height, hash, sender, receiver, volume, fee, counter,"+
		"status, is_success, gas_limit, gas_used, gas_price, "+
		"storage_limit, storage_size, storage_paid, time").Where("sender_id = ? or receiver_id = ?", acc.RowId, acc.RowId).Find(&txOps).Error
	if err != nil {
		log.Errorf("get tx list from op table by accountId %d error: %v", acc.RowId, err)
		return err
	}
	if len(txOps) == 0 {
		return nil
	}

	results := make([]*OrdinaryTxResult, 0, len(txOps))
	for _, v := range txOps {
		res := &OrdinaryTxResult{
			Height:       v.Height,
			TxHash:       v.Hash.String(),
			Sender:       "",
			Receiver:     "",
			Amount:       v.Volume,
			Fee:          v.Fee,
			Counter:      v.Counter,
			Status:       v.Status,
			IsSuccess:    v.IsSuccess,
			GasLimit:     v.GasLimit,
			GasUsed:      v.GasUsed,
			GasPrice:     v.GasPrice,
			StorageLimit: v.StorageLimit,
			StorageSize:  v.StorageSize,
			StoragePaid:  v.StoragePaid,
			Timestamp:    v.Timestamp,
		}
		if acc.RowId.Value() == v.SenderId.Value() {
			res.Sender = acc.String()

			receiverAcc := &model.Account{}
			if err := w.DB.Select("hash, address_type").Where("row_id = ?", v.ReceiverId).First(receiverAcc).Error; err != nil {
				log.Errorf("get account from account table by row_id [%d] error: %v", v.ReceiverId, err)
				return err
			}
			res.Receiver = receiverAcc.String()

		} else if acc.RowId.Value() == v.ReceiverId.Value() {
			res.Receiver = acc.String()

			senderAcc := &model.Account{}
			if err := w.DB.Select("hash, address_type").Where("row_id = ?", v.SenderId).First(senderAcc).Error; err != nil {
				log.Errorf("get account from account table by row_id [%d] error: %v", v.SenderId, err)
				return err
			}
			res.Sender = senderAcc.String()

		}

		results = append(results, res)
	}
	*reply = results

	return nil
}
