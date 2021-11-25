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
}

func SetupConfig() *config {
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
