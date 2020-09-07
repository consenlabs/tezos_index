package models

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHarvesterStatus_TableName(t *testing.T) {
	dsn := "root:WcGsHDMBmcv7mc#QWkuR@tcp(127.0.0.1:3306)/tezos_index?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open("mysql", dsn)
	defer db.Close()
	assert.NoError(t, err)
	key := "AAAaaa"
	val := "13eed"
	err = UpdateHarvesterStatus(db, key, val)
	assert.NoError(t, err)
}
