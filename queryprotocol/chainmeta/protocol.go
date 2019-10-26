// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainmeta

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	selectBlockHistory    = "SELECT transfer,execution,depositToRewardingFund,claimFromRewardingFund,grantReward,putPollResult,timestamp FROM %s WHERE block_height>=? AND block_height<=?"
	selectBlockHistorySum = "SELECT SUM(transfer)+SUM(execution)+SUM(depositToRewardingFund)+SUM(claimFromRewardingFund)+SUM(grantReward)+SUM(putPollResult) FROM %s WHERE epoch_number>=? and epoch_number<=?"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// ChainMeta defines chain meta
type ChainMeta struct {
	MostRecentEpoch       string
	MostRecentBlockHeight string
	MostRecentTps         string
}

type blkInfo struct {
	Transfer               int
	Execution              int
	DepositToRewardingFund int
	ClaimFromRewardingFund int
	GrantReward            int
	PutPollResult          int
	Timestamp              int
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// MostRecentTPS get most tps
func (p *Protocol) MostRecentTPS(ranges uint64) (tps float64, err error) {
	_, ok := p.indexer.Registry.Find(blocks.ProtocolID)
	if !ok {
		err = errors.New("blocks protocol is unregistered")
		return
	}
	if ranges == uint64(0) {
		err = errors.New("TPS block window should be greater than 0")
		return
	}
	db := p.indexer.Store.GetDB()
	_, tipHeight, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get most recent block height")
		return
	}
	blockLimit := ranges
	if tipHeight < ranges {
		blockLimit = tipHeight
	}
	start := tipHeight - blockLimit + 1
	end := tipHeight
	getQuery := fmt.Sprintf(selectBlockHistory,
		blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	rows, err := stmt.Query(start, end)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	var blk blkInfo
	parsedRows, err := s.ParseSQLRows(rows, &blk)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}
	var numActions int
	startTime := parsedRows[0].(*blkInfo).Timestamp
	endTime := parsedRows[0].(*blkInfo).Timestamp
	for _, parsedRow := range parsedRows {
		blk := parsedRow.(*blkInfo)
		numActions += blk.Transfer + blk.Execution + blk.ClaimFromRewardingFund + blk.DepositToRewardingFund + blk.GrantReward + blk.PutPollResult
		if blk.Timestamp > startTime {
			startTime = blk.Timestamp
		}
		if blk.Timestamp < endTime {
			endTime = blk.Timestamp
		}
	}
	t1 := time.Unix(int64(startTime), 0)
	t2 := time.Unix(int64(endTime), 0)
	timeDiff := (t1.Sub(t2) + 10*time.Second) / time.Millisecond
	tps = float64(numActions*1000) / float64(timeDiff)
	return
}

// GetLastEpochAndHeight gets last epoch number and block height
func (p *Protocol) GetLastEpochAndHeight() (uint64, uint64, error) {
	return chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
}

// GetNumberOfActions gets number of actions
func (p *Protocol) GetNumberOfActions(startEpoch uint64, epochCount uint64) (numberOfActions uint64, err error) {
	db := p.indexer.Store.GetDB()

	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get current epoch")
		return
	}
	if startEpoch > currentEpoch {
		err = indexprotocol.ErrNotExist
		return
	}

	endEpoch := startEpoch + epochCount - 1
	getQuery := fmt.Sprintf(selectBlockHistorySum, blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	if err = stmt.QueryRow(startEpoch, endEpoch).Scan(&numberOfActions); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}
