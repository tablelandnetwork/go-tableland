package eventprocessor

import (
	"fmt"
	"time"
)

// Config contains configuration attributes for an event processor.
type Config struct {
	BlockFailedExecutionBackoff time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		BlockFailedExecutionBackoff: time.Second * 10,
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

// EventProcessor processes events from a smart-contract.
type EventProcessor interface {
	Start() error
	Stop()
}
