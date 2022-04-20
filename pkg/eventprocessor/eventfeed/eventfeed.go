package eventfeed

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	tbleth "github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

// ChainClient provides basic apis for an EventFeed.
type ChainClient interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
	HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error)
}

// EventFeed provides a stream of on-chain events from a smart contract.
type EventFeed interface {
	Start(ctx context.Context, fromHeight int64, ch chan<- BlockEvents, filterEventTypes []EventType) error
}

// BlockEvents contains a set of events for a particular block height.
type BlockEvents struct {
	BlockNumber int64
	Events      []BlockEvent
}

type BlockEvent struct {
	TxnHash common.Hash
	Event   interface{}
}

// EventType is an event type.
type EventType string

const (
	// RunSQL is a RunSQL event fired by the SC.
	RunSQL EventType = "RunSQL"
	// Transfer is a Transfer event fired by the SC.
	Transfer = "Transfer"
)

var (
	// SupportedEvents contains a map from **all** EventType values
	// to the corresponding struct that will be used for unmarshaling.
	// Note that tbleth.Contract*** is automatically generated by
	// `make ethereum`, so keeping this mapping is easy since these
	// structs are generated from the contract ABI.
	//
	// IMPORTANT: we should *always* have a mapping for all EventType
	// values.
	SupportedEvents = map[EventType]reflect.Type{
		RunSQL:   reflect.TypeOf(tbleth.ContractRunSQL{}),
		Transfer: reflect.TypeOf(tbleth.ContractTransfer{}),
	}
)

// Config contains configuration parameters for an event feed.
type Config struct {
	MinBlockChainDepth int
	MaxBlocksFetchSize int
	ChainAPIBackoff    time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MinBlockChainDepth: 5,
		MaxBlocksFetchSize: 10000,
		ChainAPIBackoff:    time.Second * 15,
	}
}

// Option modifies a configuration attribute.
type Option func(*Config) error

// WithMinBlockDepth provides the confidence interval of block depth
// from which the event feed can safely assume block finality.
func WithMinBlockDepth(depth int) Option {
	return func(c *Config) error {
		if depth < 0 {
			return fmt.Errorf("depth must non-negative")
		}
		c.MinBlockChainDepth = depth
		return nil
	}
}

// WithMaxBlocksFetchSize provides a limit on the maximum number of blocks
// to query the node api asking for events. This allows to bound the node api
// load, and also processing cpu/memory.
func WithMaxBlocksFetchSize(batchSize int) Option {
	return func(c *Config) error {
		if batchSize <= 0 {
			return fmt.Errorf("batch size should greater than zero")
		}
		c.MaxBlocksFetchSize = batchSize
		return nil
	}
}

// WithChainAPIBackoff provides a sleep duration between failed node api
// calls to retry.
func WithChainAPIBackoff(backoff time.Duration) Option {
	return func(c *Config) error {
		if backoff < time.Second {
			return fmt.Errorf("chain api backoff is too low (<1s)")
		}
		c.ChainAPIBackoff = backoff
		return nil
	}
}
