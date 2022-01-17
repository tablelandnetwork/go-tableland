package impl

import (
	"context"
	"time"

	logger "github.com/textileio/go-log/v2"
	"github.com/textileio/go-tableland/pkg/parsing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

var (
	log = logger.Logger("parsing")
)

// InstrumentedParser implements an instrumented Parsing interface.
type InstrumentedParser struct {
	parser           parsing.Parser
	callCount        metric.Int64Counter
	latencyHistogram metric.Int64Histogram
}

func NewInstrumentedParser(p parsing.Parser) *InstrumentedParser {
	meter := metric.Must(global.Meter("tableland"))
	return &InstrumentedParser{
		parser:           p,
		callCount:        meter.NewInt64Counter("tableland.parser.call.count"),
		latencyHistogram: meter.NewInt64Histogram("tableland.sqlstore.call.latency"),
	}
}

func (ip *InstrumentedParser) ValidateCreateTable(query string) error {
	log.Debugf("call ValidateCreateTable with query %q", query)
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
func (ip *InstrumentedParser) ValidateRunSQL(query string) error {
	log.Debugf("call ValidateRunSQL with query %q", query)
	start := time.Now()
	err := ip.parser.ValidateRunSQL(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateRunSQL")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return err
}
func (ip *InstrumentedParser) ValidateReadQuery(query string) error {
	log.Debugf("call ValidateReadQuery with query %q", query)
	start := time.Now()
	err := ip.parser.ValidateReadQuery(query)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ValidateReadQuery")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	ip.callCount.Add(context.Background(), 1, attributes...)
	ip.latencyHistogram.Record(context.Background(), latency, attributes...)

	return err
}
