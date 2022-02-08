package impl

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/parsing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// InstrumentedSQLValidator implements an instrumented Parsing interface.
type InstrumentedSQLValidator struct {
	parser           parsing.SQLValidator
	callCount        metric.Int64Counter
	latencyHistogram metric.Int64Histogram
}

// NewInstrumentedSQLValidator returns creates a wrapped QueryValidator for registering metrics.
func NewInstrumentedSQLValidator(p parsing.SQLValidator) *InstrumentedSQLValidator {
	meter := metric.Must(global.Meter("tableland"))
	return &InstrumentedSQLValidator{
		parser:           p,
		callCount:        meter.NewInt64Counter("tableland.sqlvalidator.call.count"),
		latencyHistogram: meter.NewInt64Histogram("tableland.sqlvalidator.call.latency"),
	}
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
	[]parsing.SugaredWriteStmt,
	error) {
	log.Debug().Str("query", query).Msg("call ValidateRunSQL")
	start := time.Now()
	readStmt, writeStmts, err := ip.parser.ValidateRunSQL(query)
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

	return readStmt, writeStmts, err
}
