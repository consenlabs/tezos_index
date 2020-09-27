package index

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"tezos_index/puller/models"
	"time"
)
import _ "github.com/jinzhu/gorm/dialects/mysql"

func TestAccountIndex_DB(t *testing.T) {
	dsn := "root:WcGsHDMBmcv7mc#QWkuR@tcp(127.0.0.1:3306)/tezos_index?charset=utf8mb4&parseTime=True&loc=Local"
	db := InitDB(dsn)
	db.AutoMigrate(&models.Right{})
	startTime := time.Now()
	rights := make([]*models.Right, 0, 10000)
	for i := 0; i < 1; i++ {
		num := rand.Intn(99999)
		right := &models.Right{
			Type:           1,
			Height:         int64(2 * num),
			Cycle:          int64(num),
			Priority:       num,
			AccountId:      models.AccountID(num / 2),
			IsLost:         true,
			IsStolen:       false,
			IsSeedRevealed: true,
		}
		rights = append(rights, right)
	}
	batch := 500
	err := BatchInsertRights(rights, batch, db)
	assert.NoError(t, err)

	spendTime := time.Since(startTime).Seconds()
	t.Log(spendTime)
}
