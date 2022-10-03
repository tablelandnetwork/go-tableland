package sqlitechainclient

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
)

type SQLiteChainClient struct {
	log     zerolog.Logger
	db      *sql.DB
	chainID tableland.ChainID

	onceChainTip        sync.Once
	chainTipBlockNumber int64
}

func New(dbURI string, chainID tableland.ChainID) (*SQLiteChainClient, error) {
	log := logger.With().
		Str("component", "sqlitechainclient").
		Int64("chain_id", int64(chainID)).
		Logger()

	db, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		return nil, fmt.Errorf("opening db: %s", err)
	}

	return &SQLiteChainClient{
		log:     log,
		db:      db,
		chainID: chainID,
	}, nil
}

func (scc *SQLiteChainClient) FilterLogs(ctx context.Context, filter ethereum.FilterQuery) ([]types.Log, error) {
	if len(filter.Addresses) != 1 {
		return nil, fmt.Errorf("the query filter must have a single contract address filter")
	}
	if filter.BlockHash != nil {
		return nil, fmt.Errorf("block_hash filter isn't supported")
	}

	query := `select address, topics, data, block_number, tx_hash, tx_index, block_hash, event_index
	          from system_evm_events 
			  where chain_id=?1 and 
			        block_number between ?2 and ?3 and
					address=?4
			  order by block_number asc`
	rows, err := scc.db.QueryContext(ctx, query, scc.chainID, filter.FromBlock.Int64(), filter.ToBlock.Int64(), filter.Addresses[0].Hex())
	if err != nil {
		return nil, fmt.Errorf("get filters in range: %s", err)
	}
	defer rows.Close()

	var logs []types.Log
	for rows.Next() {
		if rows.Err() != nil {
			return nil, fmt.Errorf("get row: %s", rows.Err())
		}
		var address, txHash, blockHash string
		var topicsJSON []byte
		var blockNumber uint64
		var txIndex, eventIndex uint
		var data string
		if err := rows.Scan(
			&address,
			&topicsJSON,
			&data,
			&blockNumber,
			&txHash,
			&txIndex,
			&blockHash,
			&eventIndex); err != nil {
			return nil, fmt.Errorf("scan row: %s", err)
		}

		var topicsHex []string
		if json.Unmarshal(topicsJSON, &topicsHex); err != nil {
			return nil, fmt.Errorf("unmarshal json topics: %s", err)
		}
		topics := make([]common.Hash, len(topicsHex))
		for i, topicHex := range topicsHex {
			topics[i] = common.HexToHash(topicHex)
		}
		// TODO(jsign): move to separate store?
		logs = append(logs, types.Log{
			Address:     common.HexToAddress(address),
			Topics:      topics,
			Data:        []byte(data),
			BlockNumber: blockNumber,
			TxHash:      common.HexToHash(txHash),
			TxIndex:     txIndex,
			BlockHash:   common.HexToHash(blockHash),
			Index:       uint(eventIndex),
		})
	}

	return logs, nil
}

func (scc *SQLiteChainClient) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	if block != nil {
		return nil, errors.New("the current implementation only allows returning the latest block number")
	}

	scc.onceChainTip.Do(func() {
		blockNumber, err := scc.getChainTipBlockNumber(ctx)
		if err != nil {
			scc.log.Error().Err(err).Msg("loading chain tip block number")
			scc.chainTipBlockNumber = -1
			scc.onceChainTip = sync.Once{} // Reset to retry in the next `HeaderByNumber(...)` call
			return
		}
		scc.chainTipBlockNumber = blockNumber
	})
	if scc.chainTipBlockNumber == -1 {
		return nil, fmt.Errorf("chain tip block number couldn't be loaded")
	}

	return &types.Header{
		Number: big.NewInt(scc.chainTipBlockNumber),
	}, nil
}

func (scc *SQLiteChainClient) getChainTipBlockNumber(ctx context.Context) (int64, error) {
	query := "select block_number from system_evm_events where chain_id=?1 order by block_number desc limit 1"
	row := scc.db.QueryRowContext(ctx, query, scc.chainID)
	if row.Err() == sql.ErrNoRows {
		return 0, errors.New("no blocks found")
	}
	var blockNumber int64
	if err := row.Scan(&blockNumber); err != nil {
		return 0, fmt.Errorf("reading block_number column: %s", err)
	}

	return blockNumber, nil
}
