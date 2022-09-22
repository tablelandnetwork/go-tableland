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
	Txns        []TxnEvents
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
	MinBlockChainDepth  int
	ChainAPIBackoff     time.Duration
	NewHeadPollFreq     time.Duration
	PersistEvents       bool
	FetchExtraBlockInfo bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MinBlockChainDepth:  5,
		ChainAPIBackoff:     time.Second * 15,
		NewHeadPollFreq:     time.Second * 10,
		PersistEvents:       false,
		FetchExtraBlockInfo: false,
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

// WithNewHeadPollFreq is the rate at which we poll the chain to detect new blocks.
// This should be configured close to the expected block time of the chain.
//
// If set to lower, the validator is more reactive to new blocks as soon as they get miner paying an efficiency cost.
// For example, Ethereum has an expected block time of 12s. If we set this value to 6s, on average half of the polls
// will detect a new block.
//
// If set to greater, the validator will have some delay to execute new events on new blocks. But, it would be more
// efficient in Ethereum node APIs usage, since the next detected block might be N (N>1) blocks further than the last
// detected one, making the `eth_getLogs(...)` query block range wider. This means that with less Ethereum node APIs
// we would detect more events on average (again, paying the cost of having a bit more delay on event execution).
//
// Tunning this setting has a direct impact on potential Ethereum node API as a service cost, since bigger values
// have a direct impact in total API calls per day. Operators can also use this configuration to adjust to their budget.
func WithNewHeadPollFreq(pollFreq time.Duration) Option {
	return func(c *Config) error {
		c.NewHeadPollFreq = pollFreq
		return nil
	}
}

// WithEventPersistence indicates that all events should be persisted.
func WithEventPersistence(enabled bool) Option {
	return func(c *Config) error {
		c.PersistEvents = enabled
		return nil
	}
}

// WithFetchExtraBlockInformation indicates that we'll persist extra block information
// from persisted events.
func WithFetchExtraBlockInformation(enabled bool) Option {
	return func(c *Config) error {
		c.FetchExtraBlockInfo = enabled
		return nil
	}
}
