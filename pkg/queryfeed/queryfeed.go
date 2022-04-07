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

func (qf *QueryFeed) Start(ctx context.Context, fromHeight int64, ch chan<- interface{}) error {

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
			if fromHeight+minChainDepth >= h.Number.Int64() {
				continue
			}

			// Take care of putting a cap on how big the request will be.
			toHeight := h.Number.Int64() - minChainDepth
			if toHeight-fromHeight > maxLogsBatchSize {
				toHeight = fromHeight + maxLogsBatchSize
			}

			query := ethereum.FilterQuery{
				FromBlock: big.NewInt(fromHeight),
				ToBlock:   big.NewInt(toHeight),
				Addresses: []common.Address{qf.scAddress},
			}
			logs, err := qf.ethClient.FilterLogs(ctx, query)
			if err != nil {
				log.Error().Err(err).Msgf("filter logs from %d to %d", fromHeight, toHeight)
				continue
			}
		}
	}

	//////

	sub, err := qf.ethClient.SubscribeFilterLogs(ctx, query, sink)
	if err != nil {
		return fmt.Errorf("subscribing to filtered logs: %s", err)
	}
	for {
		select {
		case event := <-sink:
			log.Debug().Uint64("blockNumber", event.BlockNumber).Msg("received event")
			e := struct {
				Table      string
				Controller common.Address
				Statement  string
			}{}
			err = qf.contractAbi.UnpackIntoInterface(&e, "RunSQL", event.Data)
			if err != nil {
				return fmt.Errorf("unpacking event into interface: %s", err)
			}

			ch <- MutStatement{
				Height:    event.BlockNumber,
				Statement: e.Statement,
			}
		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %s", err)
		case <-ctx.Done():
			log.Debug().Msg("gracefully closing query feed monitoring")
			return nil
		}
	}
}
