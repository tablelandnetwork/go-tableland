package impl

import (
	"context"
	"math/big"
	"time"
)

var fetchBlockExtraInfoDelay = time.Second * 10

func (ef *EventFeed) fetchExtraBlockInfo(ctx context.Context) {
	for {
		time.Sleep(fetchBlockExtraInfoDelay)

		if ctx.Err() != nil {
			ef.log.Info().Msg("graceful close of extra block info fetcher")
			return
		}
		blockNumbers, err := ef.systemStore.GetBlocksMissingExtraInfo(ctx)
		if err != nil {
			ef.log.Error().Err(err).Msg("get blocks without extra info")
		}

		rateLim := make(chan struct{}, 10)
		for _, blockNumber := range blockNumbers {
			rateLim <- struct{}{}
			go func(blockNumber int64) {
				defer func() { <-rateLim }()

				block, err := ef.ethClient.HeaderByNumber(ctx, big.NewInt(blockNumber))
				if err != nil {
					ef.log.Error().
						Err(err).
						Int64("block_number", blockNumber).
						Msg("get extra block info")
					return
				}
				if err := ef.systemStore.InsertBlockExtraInfo(ctx, blockNumber, block.Time); err != nil {
					ef.log.Error().
						Err(err).
						Int64("block_number", blockNumber).
						Msg("save extra block info")
					return
				}
			}(blockNumber)
		}
		for i := 0; i < cap(rateLim); i++ {
			rateLim <- struct{}{}
		}
	}
}
