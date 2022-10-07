package chainscollector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

// ChainsCollector captures metrics from chains stacks with a defined frequency.
type ChainsCollector struct {
	log              zerolog.Logger
	chainStacks      map[tableland.ChainID]chains.ChainStack
	collectFrequency time.Duration
}

// New returns a new *ChainsCollector.
func New(
	chainStacks map[tableland.ChainID]chains.ChainStack,
	collectFrequency time.Duration,
) (*ChainsCollector, error) {
	if collectFrequency <= time.Second {
		return nil, fmt.Errorf("collect frequency should be greater than one second")
	}
	return &ChainsCollector{
		log:              logger.With().Str("component", "chainscollector").Logger(),
		chainStacks:      chainStacks,
		collectFrequency: collectFrequency,
	}, nil
}

// Start starts collecting chains stack telemetry metrics until the context is canceled.
func (cc *ChainsCollector) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			cc.log.Info().Msg("gracefully closed")
			return
		case <-time.After(cc.collectFrequency):
			metric := make(chainIDBlockNumbers, len(cc.chainStacks))
			for chainID, chainStack := range cc.chainStacks {
				blockNumber, err := chainStack.EventProcessor.GetLastExecutedBlockNumber(ctx)
				if err != nil {
					cc.log.Error().Err(err).Msg("get last executed block number")
					continue
				}
				metric[chainID] = blockNumber
			}
			if err := telemetry.Collect(ctx, metric); err != nil {
				cc.log.Error().Err(err).Msg("collecting chain stack metric")
			}
		}
	}
}

type chainIDBlockNumbers map[tableland.ChainID]int64

func (cbn chainIDBlockNumbers) GetLastProcessedBlockNumber() map[tableland.ChainID]int64 {
	return cbn
}
