package eventprocessor

import (
	"fmt"
	"time"
)

type Config struct {
	BlockFailedExecutionBackoff time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		BlockFailedExecutionBackoff: time.Second * 10,
	}
}

type Option func(*Config) error

func WithBLockFailedExecutionBackoff(backoff time.Duration) Option {
	return func(c *Config) error {
		if backoff.Seconds() < 1 {
			return fmt.Errorf("backoff is too low (<1s)")
		}
		c.BlockFailedExecutionBackoff = backoff
		return nil
	}
}

type EventProcessor interface {
	StartSync() error
	StopSync()
}
