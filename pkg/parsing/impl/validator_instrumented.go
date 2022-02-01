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
func (ip *InstrumentedSQLValidator) ValidateCreateTable(query string) error {
	log.Debug().Str("query", query).Msg("call ValidateCreateTable")
	start := time.Now()
	err := ip.parser.ValidateCreateTable(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateCreateTable")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return err
}

// ValidateRunSQL register metrics for its corresponding wrapped parser.
func (ip *InstrumentedSQLValidator) ValidateRunSQL(query string) (parsing.QueryType, error) {
	log.Debug().Str("query", query).Msg("call ValidateRunSQL")
	start := time.Now()
	queryType, err := ip.parser.ValidateRunSQL(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateRunSQL")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "type", Value: attribute.StringValue(string(queryType))},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return queryType, err
}

// GetWriteStatements returns a list of parsed write statements.
func (ip *InstrumentedSQLValidator) GetWriteStatements(query string) ([]parsing.WriteStmt, error) {
	log.Debug().Str("query", query).Msg("call GetWriteStatements")
	start := time.Now()
	stmts, err := ip.parser.GetWriteStatements(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetWriteStatements")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return stmts, err
}
