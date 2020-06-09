// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainmeta

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

const (
	lockAddresses = "io1uqhmnttmv0pg8prugxxn7d8ex9angrvfjfthxa" // Separate multiple addresses with ","
	totalBalance  = "12700000000000000000000000000"             // 10B + 2.7B (due to Postmortem 1)
	nsv1Balance   = "262281303940000000000000000"
	bnfxBalance   = "3414253030000000000000000"

	selectBlockHistory     = "SELECT transfer,execution,depositToRewardingFund,claimFromRewardingFund,grantReward,putPollResult,timestamp FROM %s WHERE block_height>=? AND block_height<=?"
	selectBlockHistorySum  = "SELECT SUM(transfer)+SUM(execution)+SUM(depositToRewardingFund)+SUM(claimFromRewardingFund)+SUM(grantReward)+SUM(putPollResult)+SUM(stakeCreate)+SUM(stakeUnstake)+SUM(stakeWithdraw)+SUM(stakeAddDeposit)+SUM(stakeRestake)+SUM(stakeChangeCandidate)+SUM(stakeTransferOwnership)+SUM(candidateRegister)+SUM(candidateUpdate) FROM %s WHERE epoch_number>=? and epoch_number<=?"
	selectTotalTransferred = "select IFNULL(SUM(amount),0) from %s where epoch_number>=? and epoch_number<=?"
	selectBalanceByAddress = "SELECT SUM(income) from %s WHERE address=?"
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

// GetTotalTransferredTokens gets number of actions
func (p *Protocol) GetTotalTransferredTokens(startEpoch uint64, epochCount uint64) (total string, err error) {
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
	getQuery := fmt.Sprintf(selectTotalTransferred, accounts.BalanceHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	if err = stmt.QueryRow(startEpoch, endEpoch).Scan(&total); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}

// GetTotalSupply 10B - Balance(all zero address) + 2.7B (due to Postmortem 1) - Balance(nsv1) - Balance(bnfx)
func (p *Protocol) GetTotalSupply() (count string, err error) {
	// get zero address balance.
	zeroAddressBalance, err := p.getBalanceSumByAddress(address.ZeroAddress)
	if err != nil {
		return "0", err
	}

	zeroAddressBalanceInt, ok := new(big.Int).SetString(zeroAddressBalance, 10)
	if !ok {
		err = errors.New("failed to format to big int:" + zeroAddressBalance)
		return
	}

	// Convert string format to big.Int format
	totalBalanceInt, _ := new(big.Int).SetString(totalBalance, 10)
	nsv1BalanceInt, _ := new(big.Int).SetString(nsv1Balance, 10)
	bnfxBalanceInt, _ := new(big.Int).SetString(bnfxBalance, 10)

	// Compute 10B + 2.7B (due to Postmortem 1) - Balance(all zero address) - Balance(nsv1) - Balance(bnfx)
	return new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Sub(totalBalanceInt, zeroAddressBalanceInt), nsv1BalanceInt), bnfxBalanceInt).String(), nil
}

// GetTotalCirculatingSupply total supply - SUM(lock addresses) - reward pool fund
func (p *Protocol) GetTotalCirculatingSupply(ctx context.Context, totalSupply string) (count string, err error) {
	// Sum lock addresses balances
	lockAddressesBalanceInt, err := p.getLockAddressesBalance(strings.Split(lockAddresses, ","))
	if err != nil {
		return "0", err
	}

	// AvailableBalance == Rewards in the pool that has not been issued to anyone
	availableRewardInt, err := p.getAvailableReward(ctx)
	if err != nil {
		return "0", err
	}

	// Convert string format to big.Int format
	totalSupplyInt, ok := new(big.Int).SetString(totalSupply, 10)
	if !ok {
		err = errors.New("failed to format to big int:" + totalSupply)
		return "0", err

	}

	// Compute total supply - SUM(lock addresses) - reward pool fund
	return new(big.Int).Sub(new(big.Int).Sub(totalSupplyInt, lockAddressesBalanceInt), availableRewardInt).String(), nil
}

func (p *Protocol) getBalanceSumByAddress(address string) (balance string, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectBalanceByAddress, accounts.AccountIncomeTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}

	defer stmt.Close()

	if err = stmt.QueryRow(address).Scan(&balance); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}

func (p *Protocol) getLockAddressesBalance(addresses []string) (*big.Int, error) {
	lockAddressesBalanceInt := big.NewInt(0)
	for _, address := range addresses {
		balance, err := p.getBalanceSumByAddress(address)
		if err != nil {
			return nil, err
		}
		balanceInt, ok := new(big.Int).SetString(balance, 10)
		if !ok {
			err = errors.New("failed to format to big int:" + balance)
			return nil, err
		}

		lockAddressesBalanceInt = new(big.Int).Add(lockAddressesBalanceInt, balanceInt)
	}
	return lockAddressesBalanceInt, nil
}

func (p *Protocol) getAvailableReward(ctx context.Context) (*big.Int, error) {
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
	}

	response, err := p.indexer.ChainClient.ReadState(ctx, request)
	if err != nil {
		return nil, err
	}
	availableRewardInt, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		err = errors.New("failed to format to big int:" + string(response.Data))
		return nil, err
	}
	return availableRewardInt, nil
}
