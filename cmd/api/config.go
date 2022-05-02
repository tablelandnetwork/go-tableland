package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/omeid/uconfig"
	"github.com/textileio/go-tableland/internal/tableland"
)

// configFilename is the filename of the config file automatically loaded.
var configFilename = "config.json"

type config struct {
	Impl string `default:"mesa"` // service implementation (mock or mesa)
	HTTP struct {
		Port string `default:"8080"` // HTTP port (e.g. 8080)

		RateLimInterval       string `default:"1s"`
		MaxRequestPerInterval uint64 `default:"10"`
	}
	Gateway struct {
		ExternalURIPrefix string `default:"http://testnet.tableland.network"`
	}
	DB struct {
		Host string `default:"database"`
		Port string `default:"5432"`
		User string `default:"dev_user"`
		Pass string `default:"dev_password"`
		Name string `default:"dev_database"`
	}
	TableConstraints struct {
		MaxRowCount   int `default:"100_000"`
		MaxColumns    int `default:"24"`
		MaxTextLength int `default:"1024"`
	}
	Throttling struct {
		ReadQueryDelay string `default:"0ms"`
	}
	Metrics struct {
		Port string `default:"9090"`
	}
	Log struct {
		Human bool `default:"false"`
		Debug bool `default:"false"`
	}
	AdminAPI struct {
		Username string `default:""`
		Password string `default:""`
	}
	Chains []ChainConfig
}

// ChainConfig contains all the chain execution stack configuration for a particular EVM chain.
type ChainConfig struct {
	Name     string            `default:""`
	ChainID  tableland.ChainID `default:"0"`
	Registry struct {
		EthEndpoint     string `default:"eth_endpoint"`
		ContractAddress string `default:"contract_address"`
	}
	Signer struct {
		PrivateKey string `default:""`
	}
	EventFeed struct {
		ChainAPIBackoff    string `default:"15s"`
		MaxBlocksFetchSize int    `default:"10000"`
		MinBlockDepth      int    `default:"5"`
		NewBlockTimeout    string `default:"30s"`
	}
	EventProcessor struct {
		BlockFailedExecutionBackoff string `default:"10s"`
	}
	NonceTracker struct {
		CheckInterval string `default:"10s"`
		StuckInterval string `default:"10m"`
		MinBlockDepth int    `default:"5"`
	}
}

func setupConfig() *config {
	conf := &config{}
	confFiles := uconfig.Files{
		{configFilename, json.Unmarshal},
	}

	c, err := uconfig.Classic(&conf, confFiles)
	if err != nil {
		c.Usage()
		os.Exit(1)
	}

	buf, err := json.MarshalIndent(&conf, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", buf)
	os.Exit(1)

	return conf
}
