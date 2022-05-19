package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedSQLValidator implements an instrumented Parsing interface.
type InstrumentedSQLValidator struct {
	parser           parsing.SQLValidator
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedSQLValidator returns creates a wrapped QueryValidator for registering metrics.
func NewInstrumentedSQLValidator(p parsing.SQLValidator) (*InstrumentedSQLValidator, error) {
	meter := global.MeterProvider().Meter("tableland")

	callCount, err := meter.SyncInt64().Counter("tableland.sqlvalidator.call.count")
	if err != nil {
		return &InstrumentedSQLValidator{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.sqlvalidator.call.latency")
	if err != nil {
		return &InstrumentedSQLValidator{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedSQLValidator{
		parser:           p,
		callCount:        callCount,
		latencyHistogram: latencyHistogram,
	}, nil
}

// ValidateCreateTable register metrics for its corresponding wrapped parser.
func (ip *InstrumentedSQLValidator) ValidateCreateTable(query string, chainID tableland.ChainID) (parsing.CreateStmt, error) {
	log.Debug().Str("query", query).Msg("call ValidateCreateTable")
	start := time.Now()
	cs, err := ip.parser.ValidateCreateTable(query, chainID)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateCreateTable")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return cs, err
}

// ValidateMutatingQuery register metrics for its corresponding wrapped parser.
func (ip *InstrumentedSQLValidator) ValidateMutatingQuery(
	query string,
	chainID tableland.ChainID) ([]parsing.MutatingStmt, error) {
	log.Debug().Str("query", query).Msg("call ValidateMutatingQuery")
	start := time.Now()
	mutatingStmts, err := ip.parser.ValidateMutatingQuery(query, chainID)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateMutatingQuery")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return mutatingStmts, err
}

// ValidateReadQuery register metrics for its corresponding wrapped parser.
func (ip *InstrumentedSQLValidator) ValidateReadQuery(query string) (parsing.ReadStmt, error) {
	log.Debug().Str("query", query).Msg("call ValidateReadQuery")
	start := time.Now()
	readStmt, err := ip.parser.ValidateReadQuery(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateReadQuery")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return readStmt, err
}
