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
	Dir                string // This will default to "", NOT the default dir value set via the flag package
	BootstrapBackupURL string `default:"" env:"BOOTSTRAP_BACKUP_URL"`

	HTTP             HTTPConfig
	Gateway          GatewayConfig
	TableConstraints TableConstraints
	QueryConstraints QueryConstraints

	Metrics struct {
		Port string `default:"9090"`
	}
	Log struct {
		Human bool `default:"false"`
		Debug bool `default:"false"`
	}
	Analytics struct {
		FetchExtraBlockInfo bool `default:"false"`
	}
	Backup             BackupConfig
	TelemetryPublisher TelemetryPublisherConfig

	Chains []ChainConfig
}

// HTTPConfig contains configuration for the HTTP server serving APIs.
type HTTPConfig struct {
	Port string `default:"8080"` // HTTP port (e.g. 8080)

	TLSCert string `default:""`
	TLSKey  string `default:""`

	RateLimInterval       string `default:"1s"`
	MaxRequestPerInterval uint64 `default:"10"`
	APIKey                string `default:""` // if client passes the key it will not be affected by rate limiter
}

// GatewayConfig contains configuration for the Gateway.
type GatewayConfig struct {
	ExternalURIPrefix    string `default:"https://testnets.tableland.network"`
	MetadataRendererURI  string `default:""`
	AnimationRendererURI string `default:""`
}

// BackupConfig contains configuration for automatic database backups.
type BackupConfig struct {
	Enabled           bool   `default:"true"`
	Dir               string `default:"backups"` // relative to dir path config (e.g. ${HOME}/.tableland/backups )
	Frequency         int    `default:"120"`     // in minutes
	EnableVacuum      bool   `default:"true"`
	EnableCompression bool   `default:"true"`
	Pruning           struct {
		Enabled   bool `default:"true"`
		KeepFiles int  `default:"5"` // number of files to keep
	}
}

// TelemetryPublisherConfig contains configuration attributes for the telemetry module.
type TelemetryPublisherConfig struct {
	Enabled            bool   `default:"false"`
	MetricsHubURL      string `default:""`
	MetricsHubAPIKey   string `default:""`
	PublishingInterval string `default:"10s"`

	ChainStackCollectFrequency string `default:"15m"`
}

// TableConstraints describes contraints to be enforced for Tableland tables.
type TableConstraints struct {
	MaxRowCount int `default:"100_000"`
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
		EthEndpoint       string `default:"eth_endpoint"`
		ContractAddress   string `default:"contract_address"`
		ProviderAuthToken string `default:"provider_auth_token"`
	}
	EventFeed struct {
		ChainAPIBackoff  string `default:"15s"`
		MinBlockDepth    int    `default:"5"`
		NewBlockPollFreq string `default:"10s"`
		PersistEvents    bool   `default:"true"`
	}
	EventProcessor struct {
		BlockFailedExecutionBackoff string `default:"10s"`
		DedupExecutedTxns           bool   `default:"false"`
		WebhookURL                  string `default:""`
	}
	HashCalculationStep int64 `default:"1000"`
}

func setupConfig() (*config, string) {
	flagDirPath := flag.String("dir", "${HOME}/.tableland", "Directory where the configuration and DB exist")
	flag.Parse()
	if flagDirPath == nil {
		log.Fatal().Msg("--dir is null")
		return nil, "" // Helping the linter know the next line is safe.
	}
	dirPath := os.ExpandEnv(*flagDirPath)

	_ = os.MkdirAll(dirPath, 0o755)

	var plugins []plugins.Plugin
	fullPath := path.Join(dirPath, configFilename)
	configFileBytes, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		log.Info().Str("config_file_path", fullPath).Msg("config file not found")
	} else if err != nil {
		log.Fatal().Str("config_file_path", fullPath).Err(err).Msg("opening config file")
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
