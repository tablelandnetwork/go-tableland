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
	tbleth "github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
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
	TxnEvents   []TxnEvents
}

// TxnEvents contains all events in a transaction.
type TxnEvents struct {
	TxnHash common.Hash
	Events  []interface{}
}

// EventType is an event type.
type EventType string

const (
	// RunSQL is a RunSQL event fired by the SC.
	RunSQL EventType = "RunSQL"
	// CreateTable is a CreateTable event fired by the SC.
	CreateTable = "CreateTable"
	// SetController is a SetController event fired by the SC.
	SetController = "SetController"
	// TransferTable is a TransferTable event fired by the SC.
	TransferTable = "TransferTable"
)

// SupportedEvents contains a map from **all** EventType values
// to the corresponding struct that will be used for unmarshaling.
// Note that tbleth.Contract*** is automatically generated by
// `make ethereum`, so keeping this mapping is easy since these
// structs are generated from the contract ABI.
//
// IMPORTANT: we should *always* have a mapping for all EventType
// values.
var SupportedEvents = map[EventType]reflect.Type{
	RunSQL:        reflect.TypeOf(tbleth.ContractRunSQL{}),
	CreateTable:   reflect.TypeOf(tbleth.ContractCreateTable{}),
	SetController: reflect.TypeOf(tbleth.ContractSetController{}),
	TransferTable: reflect.TypeOf(tbleth.ContractTransferTable{}),
}

// Config contains configuration parameters for an event feed.
type Config struct {
	MinBlockChainDepth int
	ChainAPIBackoff    time.Duration
	NewBlockTimeout    time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MinBlockChainDepth: 5,
		ChainAPIBackoff:    time.Second * 15,
		NewBlockTimeout:    time.Second * 30,
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

// WithNewBlockTimeout is the maximum duration to wait for a new expected block.
// If we don't receive a new block from the chain after this time, the underlying
// system will repair the faulty subscription. An arbitrary safe value would be
// ~5*avg_block_time of the underlying chain.
func WithNewBlockTimeout(timeout time.Duration) Option {
	return func(c *Config) error {
		if timeout < time.Second {
			return fmt.Errorf("new head timeout is too low (<1s)")
		}
		c.NewBlockTimeout = timeout
		return nil
	}
}
