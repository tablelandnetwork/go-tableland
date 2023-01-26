package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/omeid/uconfig"
	"github.com/omeid/uconfig/plugins"
	"github.com/omeid/uconfig/plugins/file"
)

// configFilename is the filename of the config file automatically loaded.
var configFilename = "config.json"

type config struct {
	Metrics struct {
		Port string `default:"9090"`
	}
	Log struct {
		Human bool `default:"false"`
		Debug bool `default:"false"`
	}
	Chains []ChainConfig
}

// ChainConfig contains probe configuration for a particular chain.
type ChainConfig struct {
	ChainID          int
	WalletPrivateKey string
	AlchemyAPIKey    string
	Probe            struct {
		CheckInterval  string `default:"15s"`
		ReceiptTimeout string `default:"20s"`
		Tablename      string `default:""`
	}
	OverrideClient struct {
		GatewayEndpoint             string
		ContractAddr                string
		SuggestedGasPriceMultiplier float64 `default:"1.0"`
	}
}

func setupConfig() *config {
	fileBytes, err := os.ReadFile(configFilename)
	fileStr := string(fileBytes)
	var plugins []plugins.Plugin
	if err != os.ErrNotExist {
		fileStr = os.ExpandEnv(fileStr)
		plugins = append(plugins, file.NewReader(strings.NewReader(fileStr), json.Unmarshal))
	}

	conf := &config{}
	c, err := uconfig.Classic(&conf, file.Files{}, plugins...)
	if err != nil {
		fmt.Printf("invalid configuration: %s", err)
		c.Usage()
		os.Exit(1)
	}

	return conf
}
