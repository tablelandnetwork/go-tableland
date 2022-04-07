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
	tbleth "github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

type QueryFeed struct {
	ethClient bind.ContractFilterer
	scAddress common.Address
}

type MutStatement struct {
	Height    uint64
	Statement string
}

func New(ethClient bind.ContractFilterer, scAddress common.Address) *QueryFeed {
	return &QueryFeed{
		ethClient: ethClient,
		scAddress: scAddress,
	}
}

func (qf *QueryFeed) Start(ctx context.Context, fromHeight int64, ch chan<- MutStatement, filterTables ...string) error {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromHeight),
		Addresses: []common.Address{qf.scAddress},
	}
	contractAbi, err := abi.JSON(strings.NewReader(tbleth.ContractMetaData.ABI))
	if err != nil {
		return fmt.Errorf("get contract-abi: %s", err)
	}

	sink := make(chan types.Log)
	sub, err := qf.ethClient.SubscribeFilterLogs(ctx, query, sink)
	if err != nil {
		return fmt.Errorf("subscribing to filtered logs: %s", err)
	}
	for {
		select {
		case event := <-sink:
			fmt.Printf("RECEIVED at height %d\n", event.BlockNumber)
			e := struct {
				Table      string
				Controller common.Address
				Statement  string
			}{}
			err = contractAbi.UnpackIntoInterface(&e, "RunSQL", event.Data)
			if err != nil {
				return fmt.Errorf("unpacking event into interface: %s", err)
			}

			ch <- MutStatement{
				Height:    event.BlockNumber,
				Statement: e.Statement,
			}
		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %s", err)
		}
	}
	return nil
}
