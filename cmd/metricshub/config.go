package main

import (
	"errors"
	"os"
)

type config struct {
	port    string
	project string
	dataset string
	table   string
}

func initConfig() (*config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default
	}

	project := os.Getenv("GCP_PROJECT")
	if project == "" {
		return nil, errors.New("empty GCP_PROJECT env")
	}

	dataset := os.Getenv("BIGQUERY_DATASET")
	if dataset == "" {
		return nil, errors.New("empty BIGQUERY_DATASET env")
	}

	table := os.Getenv("BIGQUERY_TABLE")
	if table == "" {
		return nil, errors.New("empty BIGQUERY_TABLE env")
	}

	return &config{
		port:    port,
		project: project,
		dataset: dataset,
		table:   table,
	}, nil
}
