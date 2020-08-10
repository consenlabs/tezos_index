package puller

import (
	"flag"
	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"
	"github.com/zyjblockchain/sandy_log/log"
	log2 "github.com/zyjblockchain/sandy_log/log"
	"net/http"
	"net/url"
	"tezos_index/common"
	"tezos_index/puller/index"
	"tezos_index/puller/models"
	"tezos_index/rpc"
)

type Configuration struct {
	Mysql         string
	Redis         string
	ApiUrl        string
	ProxyUrl      string
	Start         uint
	End           uint
	Fix           bool
	Verbose       int
	Listen        string
	Chain         string
	Network       string
	Sentry        string
	Kafka         string
	OnlyBlock     bool
	GasStationUrl string
}

type Environment struct {
	Conf        Configuration
	Engine      *gorm.DB
	Client      *rpc.Client
	RedisClient *redis.Client
	// RiskClient *risk.Client
}

func NewEnvironment() *Environment {
	flag.String("mysql", common.DefaultString, "tezos mysql uri like 'mysql://tcp(ip:[port]/database'")
	flag.String("chain", common.DefaultString, "tezos json-rpc like 'http://faq:faq@localhost:18332/0'")
	flag.String("api-url", common.DefaultString, "api url like 'http://:111")
	flag.String("redis", common.DefaultString, "redis url like ")
	flag.Int("start", common.DefaultInt, "tezos start with special block number")
	flag.String("network", common.DefaultString, "tezos network mainnet or kovan")
	flag.String("listen", common.DefaultString, "tezos biz api listen ")
	flag.Int("verbose", common.DefaultInt, "tezos print verbose message")
	flag.Bool("fix", false, "tezos fix blocks")
	flag.Int("end", common.DefaultInt, "tezos fix end special blocks")
	flag.String("sentry", common.DefaultString, "sentry url")
	flag.String("kafka", common.DefaultString, "kafka broker")
	flag.String("gas-station-url", common.DefaultString, "gas station url")
	flag.Bool("only-block", false, "only sync blocks")
	flag.String("node-type", common.DefaultString, "node-type")

	viperConfig := common.NewViperConfig()

	domain := "tezos"

	conf := Configuration{}

	conf.Verbose = viperConfig.GetInt("", "verbose")

	conf.Redis = viperConfig.GetString(domain, "redis")
	if conf.Redis == "" {
		log2.Crit("please set redis connection info")
		panic("system fail")
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     conf.Redis,
		Password: "",
		DB:       2,
	})

	conf.Mysql = viperConfig.GetString(domain, "mysql")
	if conf.Mysql == "" {
		log2.Crit("please set mysql connection info")
		panic("system fail")
	}
	engine := index.InitDB(conf.Mysql)

	conf.Chain = viperConfig.GetString(domain, "chain")

	httpClient := http.DefaultClient
	pUrl := viperConfig.GetString(domain, "proxy")
	if pUrl != "" {
		proxyUrl, err := url.Parse(pUrl)
		if err != nil {
			log.Errorf("url parse error: %v", err)
			panic(err)
		}
		tr := &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		httpClient = &http.Client{Transport: tr}
	}

	client, err := rpc.NewClient(httpClient, conf.Chain)
	if err != nil {
		log.Errorf("connect tezos node client error: %v", err)
		panic("connet tezos node error")
	}
	conf.Start = uint(viperConfig.GetInt(domain, "start"))
	conf.End = uint(viperConfig.GetInt(domain, "end"))
	conf.Fix = viperConfig.GetBool(domain, "fix")
	conf.Network = viperConfig.GetString(domain, "network")
	conf.Listen = viperConfig.GetString("", "listen")
	conf.Sentry = viperConfig.GetString("", "sentry")
	conf.Kafka = viperConfig.GetString(domain, "kafka")
	conf.GasStationUrl = viperConfig.GetString(domain, "gas-station-url")
	conf.OnlyBlock = viperConfig.GetBool(domain, "only-block")

	return &Environment{Conf: conf, Engine: engine, Client: client, RedisClient: redisClient}
}

func (e *Environment) NewPuller() *Crawler {
	indexer := NewIndexer(IndexerConfig{
		StateDB: e.Engine,
		CacheDB: e.RedisClient,
		Indexes: []models.BlockIndexer{
			index.NewAccountIndex(e.Engine),
			index.NewBigMapIndex(e.Engine),
			index.NewBlockIndex(e.Engine),
			index.NewChainIndex(e.Engine),
			index.NewContractIndex(e.Engine),
			index.NewFlowIndex(e.Engine),
			index.NewGovIndex(e.Engine),
			index.NewIncomeIndex(e.Engine),
			index.NewOpIndex(e.Engine),
			index.NewRightsIndex(e.Engine),
			index.NewSnapshotIndex(e.Engine),
			index.NewSupplyIndex(e.Engine),
		},
	})

	cf := CrawlerConfig{
		DB:            e.Engine,
		Indexer:       indexer,
		Client:        e.Client,
		Queue:         4,
		StopBlock:     0,
		EnableMonitor: false,
	}
	return NewCrawler(cf)
}

//
// func (e *Environment) NewWalletService() *WalletService {
// 	return NewWalletService(e)
// }
//
// func (e *Environment) NewContractQuery() *ContractQuery {
// 	return NewContractQuery(e)
// }
//
// func (e *Environment) UpgradeSchema() {
// 	if err := common.UpgradeSchema(e.Conf.Mysql); err != nil {
// 		log2.Crit("upgrade database", "uri", e.Conf.Mysql, "err", err)
// 		panic("system fail")
// 	}
// }
