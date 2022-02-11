package main

import (
	"encoding/json"
	"os"

	"github.com/omeid/uconfig"
)

// configFilename is the filename of the config file automatically loaded.
var configFilename = "config.json"

type config struct {
	Impl string `default:"mesa"` // service implementation (mock or mesa)
	HTTP struct {
		Port string `default:"8080"` // HTTP port (e.g. 8080)
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
		WriteQueryDelay string `default:"500ms"`
		ReadQueryDelay  string `default:"0ms"`
	}
	Registry struct {
		EthEndpoint     string `default:"eth_endpoint"`
		ContractAddress string `default:"contract_address"`
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

	return conf
}
