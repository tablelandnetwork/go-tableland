package main

import (
	"os"

	"github.com/omeid/uconfig"
)

type config struct {
	Probe struct {
		Target        string `default:""`
		CheckInterval string `default:"15s"`
		JWT           string `default:""`
		Tablename     string `default:""`
	}
	Metrics struct {
		Port string `default:"9090"`
	}
	Log struct {
		Human bool `default:"false"`
		Debug bool `default:"false"`
	}
}

func setupConfig() *config {
	conf := &config{}

	c, err := uconfig.Classic(&conf, uconfig.Files{})
	if err != nil {
		c.Usage()
		os.Exit(1)
	}

	return conf
}
