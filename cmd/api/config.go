package main

import (
	"encoding/json"
	"flag"
	"os"
	"path"
	"strings"

	"github.com/omeid/uconfig"
	"github.com/omeid/uconfig/plugins"
	"github.com/omeid/uconfig/plugins/file"
	"github.com/rs/zerolog/log"
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
		ExternalURIPrefix string `default:"https://testnet.tableland.network"`
	}
	TableConstraints TableConstraints
	QueryConstraints QueryConstraints
	Metrics          struct {
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

// TableConstraints describes contraints to be enforced for Tableland tables.
type TableConstraints struct {
	MaxRowCount   int `default:"100_000"`
	MaxColumns    int `default:"24"`
	MaxTextLength int `default:"1024"`
}

// QueryConstraints describes constraints to be enforced on queries.
type QueryConstraints struct {
	MaxWriteQuerySize int `default:"35000"`
	MaxReadQuerySize  int `default:"35000"`
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
		ChainAPIBackoff string `default:"15s"`
		MinBlockDepth   int    `default:"5"`
		NewBlockTimeout string `default:"30s"`
	}
	EventProcessor struct {
		BlockFailedExecutionBackoff string `default:"10s"`
		DedupExecutedTxns           bool   `default:"false"`
	}
	NonceTracker struct {
		CheckInterval string `default:"10s"`
		StuckInterval string `default:"10m"`
		MinBlockDepth int    `default:"5"`
	}
}

func setupConfig() (*config, string) {
	flagDirPath := flag.String("dir", "${HOME}/.tableland", "Directory where the configuration and DB exist")
	flag.Parse()
	if flagDirPath == nil {
		log.Fatal().Msg("--dir is null")
		return nil, "" // Helping the linter know the next line is safe.
	}
	dirPath := os.ExpandEnv(*flagDirPath)

	_ = os.MkdirAll(dirPath, 0755)

	var plugins []plugins.Plugin
	fullPath := path.Join(dirPath, configFilename)
	configFileBytes, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		log.Info().Str("configFilePath", fullPath).Msg("config file not found")
	} else if err != nil {
		log.Fatal().Str("configFilePath", fullPath).Err(err).Msg("opening config file")
	} else {
		fileStr := os.ExpandEnv(string(configFileBytes))
		plugins = append(plugins, file.NewReader(strings.NewReader(fileStr), json.Unmarshal))
	}

	conf := &config{}
	c, err := uconfig.Classic(&conf, file.Files{}, plugins...)
	if err != nil {
		c.Usage()
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	return conf, dirPath
}
