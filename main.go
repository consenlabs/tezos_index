package main

import (
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/zyjblockchain/sandy_log/log"
	"tezos_index/models"
)

func main() {
	// 0. 初始化日志级别、格式、是否保存到文件
	log.Setup(log.LevelDebug, false, true)

	dsn := "tac_user:NwHJhkcTKHmDr2RZ@tcp(223.27.39.183:3306)/tezos_index?charset=utf8mb4&parseTime=True&loc=Local"
	models.InitDB(dsn)

}

// runServer
func runServer() {

}
