package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
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
func (ip *InstrumentedSQLValidator) ValidateCreateTable(query string) (parsing.CreateStmt, error) {
	log.Debug().Str("query", query).Msg("call ValidateCreateTable")
	start := time.Now()
	cs, err := ip.parser.ValidateCreateTable(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateCreateTable")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return cs, err
}

// ValidateRunSQL register metrics for its corresponding wrapped parser.
func (ip *InstrumentedSQLValidator) ValidateRunSQL(query string) (
	parsing.SugaredReadStmt,
	[]parsing.SugaredMutatingStmt,
	error) {
	log.Debug().Str("query", query).Msg("call ValidateRunSQL")
	start := time.Now()
	readStmt, mutatingStmts, err := ip.parser.ValidateRunSQL(query)
	latency := time.Since(start).Milliseconds()

	queryType := "write"
	if readStmt != nil {
		queryType = "read"
	}
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateRunSQL")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "type", Value: attribute.StringValue(queryType)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return readStmt, mutatingStmts, err
}
