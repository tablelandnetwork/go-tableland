package eventprocessor

import (
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/tables"
)

type webhook struct {
	Enabled      bool
	URL          string
	EndpointType string
}

// Config contains configuration attributes for an event processor.
type Config struct {
	BlockFailedExecutionBackoff time.Duration
	DedupExecutedTxns           bool
	HashCalcStep                int64
	Webhook                     webhook
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		BlockFailedExecutionBackoff: time.Second * 10,
		DedupExecutedTxns:           false,
		HashCalcStep:                100,
	}
}

// Option modifies a configuration attribute.
type Option func(*Config) error

// WithBlockFailedExecutionBackoff provides a sleep duration between retryiable
// executions. e.g: if execution block events fails due to the underlying database
// being unavailable, we'll wait this time before retrying.
func WithBlockFailedExecutionBackoff(backoff time.Duration) Option {
	return func(c *Config) error {
		if backoff.Seconds() < 1 {
			return fmt.Errorf("backoff is too low (<1s)")
		}
		c.BlockFailedExecutionBackoff = backoff
		return nil
	}
}

// WithDedupExecutedTxns makes the event processor skip executing txn hashes that have
// already been executed before.
// **IMPORTANT NOTE**: This is an unsafe flag that should only be enabled in test environments.
// A txn hash should never appear again after it was executed since that indicates
// there was a reorg in the chain.
func WithDedupExecutedTxns(dedupExecutedTxns bool) Option {
	return func(c *Config) error {
		c.DedupExecutedTxns = dedupExecutedTxns
		return nil
	}
}

// WithHashCalcStep determines the pace of state hash calculations.
// The hash will be calculated for every block equal or greater to the next multiple of the step.
func WithHashCalcStep(step int64) Option {
	return func(c *Config) error {
		if step < 1 {
			return fmt.Errorf("step cannot be less than 1")
		}
		c.HashCalcStep = step
		return nil
	}
}

// WithWebhook is set when we want send table update notifications
// to an external webhook.
func WithWebhook(endpointType string, url string) Option {
	return func(c *Config) error {
		c.Webhook.Enabled = true
		c.Webhook.URL = url
		c.Webhook.EndpointType = endpointType
		return nil
	}
}

// EventProcessor processes events from a smart-contract.
type EventProcessor interface {
	GetLastExecutedBlockNumber() int64
	Start() error
	Stop()
}

// Receipt is an event receipt.
type Receipt struct {
	ChainID      tableland.ChainID
	BlockNumber  int64
	IndexInBlock int64
	TxnHash      string

	TableIDs      tables.TableIDs
	Error         *string
	ErrorEventIdx *int

	// Deprecated
	TableID *tables.TableID
}
