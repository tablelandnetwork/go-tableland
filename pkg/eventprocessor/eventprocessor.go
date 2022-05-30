package eventprocessor

import (
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
)

// Config contains configuration attributes for an event processor.
type Config struct {
	BlockFailedExecutionBackoff time.Duration
	DedupExecutedTxns           bool
	MaxWriteQuerySize           int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		BlockFailedExecutionBackoff: time.Second * 10,
		DedupExecutedTxns:           false,
		MaxWriteQuerySize:           35000,
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

// WithMaxWriteQuerySize limits the size of a write query.
func WithMaxWriteQuerySize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return fmt.Errorf("size should greater than zero")
		}
		c.MaxWriteQuerySize = size
		return nil
	}
}

// EventProcessor processes events from a smart-contract.
type EventProcessor interface {
	Start() error
	Stop()
}

// Receipt is an event receipt.
type Receipt struct {
	ChainID     tableland.ChainID
	BlockNumber int64
	TxnHash     string

	Error   *string
	TableID *tableland.TableID
}
