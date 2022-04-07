package queryfeed

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	tbleth "github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

type QueryFeed struct {
	ethClient   bind.ContractFilterer
	scAddress   common.Address
	contractAbi abi.ABI
}

type MutStatement struct {
	Height    uint64
	Statement string
}

func New(ethClient bind.ContractFilterer, scAddress common.Address) (*QueryFeed, error) {
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

func (qf *QueryFeed) Start(ctx context.Context, fromHeight int64, ch chan<- interface{}) {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromHeight),
		Addresses: []common.Address{qf.scAddress},
	}

	sink := make(chan types.Log)
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
