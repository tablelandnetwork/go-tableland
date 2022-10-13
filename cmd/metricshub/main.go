package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

func main() {
	log.Info().Msg("starting the server...")
	config, err := initConfig()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("could not init config")
	}

	logging.SetupLogger(buildinfo.GitCommit, false, false)

	bq := newBigQueryStore(config.project, config.dataset, config.table)
	http.HandleFunc("/", makeHandler(bq, config))

	log.Info().Str("port", config.port).Msg("listening...")
	if err := http.ListenAndServe(":"+config.port, nil); err != nil {
		log.Fatal().
			Err(err).
			Msg("starting http server")
	}
}

type request struct {
	NodeID  string             `json:"node_id"`
	Metrics []telemetry.Metric `json:"metrics"`
}

func (r *request) check() error {
	if len(r.Metrics) == 0 {
		return errors.New("empty metrics")
	}

	if _, err := uuid.Parse(r.NodeID); err != nil {
		return errors.New("node is not uuid")
	}

	return nil
}

type store interface {
	insert(context.Context, request) error
}

func isAuthorized(headerKey string, allowedKeys []string) bool {
	for _, key := range allowedKeys {
		if headerKey == key {
			return true
		}
	}
	return false
}

func makeHandler(store store, c *config) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Api-Key")
		if !isAuthorized(apiKey, c.apiKeys) {
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			log.Error().Msg("request is not POST")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error().Err(err).Msg("decoding the request")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := req.check(); err != nil {
			log.Error().Err(err).Msg("request is invalid")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		ip, err := extractClientIP(r)
		if err != nil {
			log.Warn().Msg("could not get ip address")
		}

		log.Debug().Str("node_id", req.NodeID).Str("ip_address", ip).Msg("got metric from node")

		if err := store.insert(r.Context(), req); err != nil {
			log.Error().Err(err).Msg("inserting")
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
	}
}

func extractClientIP(r *http.Request) (string, error) {
	// Use X-Forwarded-For IP if present.
	// i.g: https://cloud.google.com/load-balancing/docs/https#x-forwarded-for_header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.Split(xff, ",")[0]
		return ip, nil
	}

	// Use the request remote address.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("getting ip from remote addr: %s", err)
	}
	return ip, nil
}
