package migration

import (
	"database/sql"
	"github.com/jinzhu/gorm"
	"github.com/pressly/goose"
	"tezos_index/puller/models"
)

func init() {
	goose.AddMigration(Up20210308174740, Down20210308174740)
}

func Up20210308174740(tx *sql.Tx) error {
	// This code is executed when the migration is applied.
	db, err := gorm.Open("mysql", tx)
	if err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).AddIndex("idx01", "bigmap_id").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).AddIndex("idx02", "action").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).AddIndex("idx03", "key_hash").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).AddIndex("idx04", "is_replaced").Error; err != nil {
		return err
	}
	return nil
}

func Down20210308174740(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	db, err := gorm.Open("mysql", tx)
	if err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).RemoveIndex("idx01").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).RemoveIndex("idx02").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).RemoveIndex("idx03").Error; err != nil {
		return err
	}
	if err := db.Model(&models.BigMapItem{}).RemoveIndex("idx04").Error; err != nil {
		return err
	}
	return nil
}
