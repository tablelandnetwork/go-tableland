package impl

import (
	"context"
	"math/big"
	"time"

	"github.com/textileio/go-tableland/pkg/telemetry"
)

var fetchBlockExtraInfoDelay = time.Second * 10

func (ef *EventFeed) fetchExtraBlockInfo(ctx context.Context) {
	var fromHeight *int64
	for {
		time.Sleep(fetchBlockExtraInfoDelay)

		if ctx.Err() != nil {
			ef.log.Info().Msg("graceful close of extra block info fetcher")
			return
		}
		blockNumbers, err := ef.systemStore.GetBlocksMissingExtraInfo(ctx, fromHeight)
		if err != nil {
			ef.log.Error().Err(err).Msg("get blocks without extra info")
			continue
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
				newBlockMetric := telemetry.NewBlockMetric{
					Version:            telemetry.NewBlockMetricV1,
					ChainID:            int(ef.chainID),
					BlockNumber:        blockNumber,
					BlockTimestampUnix: block.Time,
				}
				if err := telemetry.Collect(ctx, newBlockMetric); err != nil {
					ef.log.Error().
						Err(err).
						Int64("block_number", blockNumber).
						Msg("capturing new block metric")
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
		if len(blockNumbers) > 0 {
			fromHeight = &blockNumbers[len(blockNumbers)-1]
		}
	}
}
