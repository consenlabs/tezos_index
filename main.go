package main

import (
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/zyjblockchain/sandy_log/log"
	"tezos_index/models"
)

func main() {
	// 0. 初始化日志级别、格式、是否保存到文件
	log.Setup(log.LevelDebug, false, true)

	dsn := "root:WcGsHDMBmcv7mc#QWkuR@tcp(127.0.0.1:3306)/tezos_index?charset=utf8mb4&parseTime=True&loc=Local"
	models.InitDB(dsn)

}

// runServer
func runServer() {

}
