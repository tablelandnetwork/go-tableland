package main

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

// bigQueryStore implements the Store interface for inserting metrics into BigQuery.
type bigQueryStore struct {
	project string
	dataset string
	table   string
}

// newBigQueryStore creates a new bigQueryStore object.
func newBigQueryStore(project, dataset, table string) *bigQueryStore {
	return &bigQueryStore{
		project: project,
		dataset: dataset,
		table:   table,
	}
}

// Insert insert payload from a Request into Bigquery.
func (s *bigQueryStore) insert(ctx context.Context, req request) error {
	client, err := bigquery.NewClient(ctx, s.project)
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	rows, err := s.toBigQueryRows(req)
	if err != nil {
		return fmt.Errorf("to bigquery rows: %s", err)
	}

	inserter := client.Dataset(s.dataset).Table(s.table).Inserter()
	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("inserter put: %s", err)
	}
	return nil
}

func (s *bigQueryStore) toBigQueryRows(req request) ([]*row, error) {
	rows := make([]*row, len(req.Metrics))

	for i, m := range req.Metrics {
		payload, err := m.Serialize()
		if err != nil {
			return []*row{}, fmt.Errorf("serialize: %s", err)
		}
		rows[i] = &row{
			Version:   m.Version,
			Timestamp: strings.TrimSuffix(m.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"), "Z"), // RFC3339 mili without Z
			Type:      int(m.Type),
			Payload:   string(payload),
			NodeID:    req.NodeID,
		}
	}
	return rows, nil
}

// row represents a row in BigQuery.
type row struct {
	Version   int
	Timestamp string
	Type      int
	Payload   string
	NodeID    string
}

// Save implements the ValueSaver interface.
func (r *row) Save() (map[string]bigquery.Value, string, error) { // nolint
	return map[string]bigquery.Value{
		"version":   r.Version,
		"timestamp": r.Timestamp,
		"type":      r.Type,
		"payload":   r.Payload,
		"node_id":   r.NodeID,
	}, bigquery.NoDedupeID, nil
}
