package main

import (
	"encoding/json"
	"os"

	"github.com/omeid/uconfig"
)

// configFilename is the filename of the config file automatically loaded
var configFilename = "config.json"

type config struct {
	Impl string `default:"mesa"` // service implementation (mock or mesa)
	HTTP struct {
		Port string `default:"8080"` // HTTP port (e.g. 8080)
	}
	DB struct {
		Host string `default:"database"`
		Port string `default:"5432"`
		User string `default:"dev_user"`
		Pass string `default:"dev_password"`
		Name string `default:"dev_database"`
	}
	Registry struct {
		EthEndpoint     string `default:"node_endpoint"`
		ContractAddress string `default:"contract_address"`
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
