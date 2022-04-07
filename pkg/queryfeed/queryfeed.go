package queryfeed

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	tbleth "github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

const (
	maxLogsBatchSize = 1000
	minChainDepth    = 5
)

type BlockEvents struct {
	BlockNumber int64
	Events      []interface{}
}

type EthClient interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
}

type QueryFeed struct {
	ethClient   EthClient
	scAddress   common.Address
	contractAbi abi.ABI
}

type MutStatement struct {
	Height    uint64
	Statement string
}

func New(ethClient EthClient, scAddress common.Address) (*QueryFeed, error) {
	contractAbi, err := abi.JSON(strings.NewReader(tbleth.ContractMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("get contract-abi: %s", err)
	}
	return &QueryFeed{
		ethClient:   ethClient,
		scAddress:   scAddress,
		contractAbi: contractAbi,
	}, nil
}

func (qf *QueryFeed) Start(ctx context.Context, fromHeight int64, ch chan<- BlockEvents, filterEventTypes ...common.Hash) error {
	ctx, abort := context.WithCancel(ctx)
	defer abort()

	// TODO(jsign): add mechanism to fire with the current head to avoid waiting and refactor.
	chHeader := make(chan *types.Header)
	subHeader, err := qf.ethClient.SubscribeNewHead(ctx, chHeader)
	if err != nil {
		return fmt.Errorf("subscribing to new heads: %s", err)
	}
	go func() {
		defer subHeader.Unsubscribe()
		defer close(chHeader)
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("gracefully closing new heads subscription")
				return
			case err := <-subHeader.Err():
				log.Error().Err(err).Msg("new heads subscription")
				return
			}
		}
	}()

	for {
		select {
		case h, ok := <-chHeader:
			if !ok {
				log.Info().Msg("new head channel was closed, closing gracefully")
				return nil
			}

			// Only make a new filter logs query if the next intended height to
			// process is at least minChainDepth behind the reported head. This is
			// done to avoid reorg problems.
			toHeight := h.Number.Int64() - minChainDepth
			if toHeight < fromHeight {
				continue
			}

			// Put a cap on how big the query will be. This is important if we are
			// doing a cold syncing or have fall behind after a long stop.
			if toHeight-fromHeight > maxLogsBatchSize {
				toHeight = fromHeight + maxLogsBatchSize
			}

			var topics [][]common.Hash
			if len(filterEventTypes) > 0 {
				topics = [][]common.Hash{filterEventTypes}
			}
			query := ethereum.FilterQuery{
				FromBlock: big.NewInt(fromHeight),
				ToBlock:   big.NewInt(toHeight),
				Addresses: []common.Address{qf.scAddress},
				Topics:    topics,
			}
			logs, err := qf.ethClient.FilterLogs(ctx, query)
			if err != nil {
				log.Warn().Err(err).Msgf("filter logs from %d to %d", fromHeight, toHeight)
				continue
			}

			if len(logs) == 0 {
				continue
			}

			bq := BlockEvents{
				BlockNumber: int64(logs[0].BlockNumber),
			}
			for _, l := range logs {
				if bq.BlockNumber != int64(l.BlockNumber) {
					ch <- bq
					bq = BlockEvents{
						BlockNumber: int64(l.BlockNumber),
					}
				}

				event, err := qf.parseEvent(l.Topics[0], l.Data)
				if err != nil {
					return fmt.Errorf("couldn't parse event: %s", err)
				}
				bq.Events = append(bq.Events, event)
			}
			// Sent last block events construction of the loop.
			ch <- bq

			// Update our fromHeight to the latest processed height plus one.
			fromHeight = bq.BlockNumber + 1
		}
	}
}

func (qf *QueryFeed) parseEvent(eventSignature common.Hash, eventData []byte) (interface{}, error) {
	eventDescr, err := qf.contractAbi.EventByID(eventSignature)
	if err != nil {
		return nil, fmt.Errorf("detecting event type: %s", err)
	}

	e, err := qf.contractAbi.Unpack(eventDescr.Name, eventData)
	if err != nil {
		return nil, fmt.Errorf("unpacking event: %s", err)
	}

	return e, nil
}
