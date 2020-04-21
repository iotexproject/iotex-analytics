// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocks

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "blocks"
	// BlockHistoryTableName is the table name of block history
	BlockHistoryTableName = "block_history"
	// ProductivityTableName is the table name of block producers' productivity
	ProductivityTableName = "productivity_history"
	// ExpectedProducerTableName is a table required by productivity table
	ExpectedProducerTableName = "expected_producer_history"
	// ProducerTableName is a table required by productivity table
	ProducerTableName = "producer_history"
	// EpochProducerIndexName is the index name of epoch number and producer's name on block history table
	EpochProducerIndexName = "epoch_producer_index"
	// TimestampIndexName is the 'timestamp' index name of BlockHistory table
	TimestampIndexName      = "timestamp_index"
	createBlockHistoryIndex = "CREATE INDEX %s ON %s (timestamp)"
	createBlockHistory      = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"block_height DECIMAL(65, 0) NOT NULL, block_hash VARCHAR(64) NOT NULL, transfer DECIMAL(65, 0) NOT NULL, execution DECIMAL(65, 0) NOT NULL, " +
		"depositToRewardingFund DECIMAL(65, 0) NOT NULL, claimFromRewardingFund DECIMAL(65, 0) NOT NULL, grantReward DECIMAL(65, 0) NOT NULL, " +
		"putPollResult DECIMAL(65, 0) NOT NULL, gas_consumed DECIMAL(65, 0) NOT NULL, producer_address VARCHAR(41) NOT NULL, " +
		"producer_name VARCHAR(24) NOT NULL, expected_producer_address VARCHAR(41) NOT NULL, " +
		"expected_producer_name VARCHAR(24) NOT NULL, timestamp DECIMAL(65, 0) NOT NULL, PRIMARY KEY (block_height))"
	selectBlockHistoryInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	createIndex    = "CREATE INDEX %s ON %s (epoch_number, producer_name, expected_producer_name)"
	createProducer = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"producer_name VARCHAR(24) NOT NULL, production DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, producer_name))"
	createExpectedProducer = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"expected_producer_name VARCHAR(24) NOT NULL, expected_production DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, expected_producer_name))"
	createProductivity = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"delegate_name VARCHAR(24) NOT NULL, production DECIMAL(65, 0) NOT NULL, expected_production DECIMAL(65, 0) " +
		"NOT NULL, UNIQUE KEY %s (epoch_number, delegate_name))"
	selectBlockHistory = "SELECT * FROM %s WHERE block_height=?"
	selectProductivity = "SELECT * FROM %s WHERE epoch_number=? AND delegate_name=?"
	insertBlockHistory = "INSERT INTO %s (epoch_number, block_height, block_hash, transfer, execution, " +
		"depositToRewardingFund, claimFromRewardingFund, grantReward, putPollResult, gas_consumed, producer_address, " +
		"producer_name, expected_producer_address, expected_producer_name, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	insertExpectedProducer = "INSERT IGNORE INTO %s SELECT epoch_number, expected_producer_name, " +
		"COUNT(expected_producer_address) AS expected_production FROM %s GROUP BY epoch_number, expected_producer_name"
	insertProducer = "INSERT IGNORE INTO %s SELECT epoch_number, producer_name, " +
		"COUNT(producer_address) AS production FROM %s GROUP BY epoch_number, producer_name"
	insertProductivity = "INSERT IGNORE INTO %s SELECT t1.epoch_number, t1.expected_producer_name AS delegate_name, " +
		"CAST(IFNULL(production, 0) AS DECIMAL(65, 0)) AS production, CAST(expected_production AS DECIMAL(65, 0)) AS expected_production " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.epoch_number = t2.epoch_number AND t1.expected_producer_name=t2.producer_name"
	backoffInterval = 1
	numOfRetry      = 1000
)

type (
	// BlockHistory defines the schema of "block history" table
	BlockHistory struct {
		EpochNumber             uint64
		BlockHeight             uint64
		BlockHash               string
		Transfer                uint64
		Execution               uint64
		DepositToRewaringFund   uint64
		ClaimFromRewardingFund  uint64
		GrantReward             uint64
		PutPollResult           uint64
		GasConsumed             uint64
		ProducerAddress         string
		ProducerName            string
		ExpectedProducerAddress string
		ExpectedProducerName    string
		Timestamp               uint64
	}

	// ProductivityHistory defines the schema of "productivity history" view
	ProductivityHistory struct {
		EpochNumber        uint64
		ProducerName       string
		Production         uint64
		ExpectedProduction uint64
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store                s.Store
	ActiveBlockProducers []string
	OperatorAddrToName   map[string]string
	epochCtx             *epochctx.EpochCtx
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, epochctx *epochctx.EpochCtx) *Protocol {
	return &Protocol{
		Store:    store,
		epochCtx: epochctx,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createBlockHistory, BlockHistoryTableName)); err != nil {
		return err
	}

	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectBlockHistoryInfo, BlockHistoryTableName, EpochProducerIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createIndex, EpochProducerIndexName, BlockHistoryTableName)); err != nil {
			return err
		}
	}
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectBlockHistoryInfo, BlockHistoryTableName, TimestampIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createBlockHistoryIndex, TimestampIndexName, BlockHistoryTableName)); err != nil {
			return err
		}
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createProducer,
		ProducerTableName, EpochProducerIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createExpectedProducer,
		ExpectedProducerTableName, EpochProducerIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createProductivity,
		ProductivityTableName, EpochProducerIndexName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes blocks index protocol
func (p *Protocol) Initialize(context.Context, *sql.Tx, *indexprotocol.Genesis) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := p.epochCtx.GetEpochNumber(height)
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient
	electionClient := indexCtx.ElectionClient
	// Special handling for epoch start height
	epochHeight := p.epochCtx.GetEpochHeight(epochNumber)
	if indexCtx.ConsensusScheme == "ROLLDPOS" && (height == epochHeight || p.OperatorAddrToName == nil) {
		if err := p.updateDelegates(chainClient, electionClient, height, epochNumber); err != nil {
			return errors.Wrapf(err, "failed to update delegates in epoch %d", epochNumber)
		}
	}
	if height == epochHeight {
		if err := p.rebuildProductivityTable(tx); err != nil {
			return errors.Wrap(err, "failed to rebuild productivity table")
		}
	}

	// log action index
	var transferCount uint64
	var executionCount uint64
	var depositToRewardingFundCount uint64
	var claimFromRewardingFundCount uint64
	var grantRewardCount uint64
	var putPollResultCount uint64
	for _, selp := range blk.Actions {
		act := selp.Action()
		if _, ok := act.(*action.Transfer); ok {
			transferCount++
		} else if _, ok := act.(*action.Execution); ok {
			executionCount++
		} else if _, ok := act.(*action.DepositToRewardingFund); ok {
			depositToRewardingFundCount++
		} else if _, ok := act.(*action.ClaimFromRewardingFund); ok {
			claimFromRewardingFundCount++
		} else if _, ok := act.(*action.GrantReward); ok {
			grantRewardCount++
		} else if _, ok := act.(*action.PutPollResult); ok {
			putPollResultCount++
		}
	}
	var gasConsumed uint64
	// log receipt index
	for _, receipt := range blk.Receipts {
		gasConsumed += receipt.GasConsumed
	}
	hash := blk.HashBlock()
	producerAddr := blk.ProducerAddress()
	producerName := p.OperatorAddrToName[producerAddr]
	var expectedProducerAddr string
	if len(p.ActiveBlockProducers) > 0 {
		expectedProducerAddr = p.ActiveBlockProducers[int(height)%len(p.ActiveBlockProducers)]
	}
	expectedProducerName := p.OperatorAddrToName[expectedProducerAddr]
	return p.updateBlockHistory(tx, epochNumber, height, hex.EncodeToString(hash[:]), transferCount, executionCount,
		depositToRewardingFundCount, claimFromRewardingFundCount, grantRewardCount, putPollResultCount, gasConsumed,
		producerAddr, producerName, expectedProducerAddr, expectedProducerName, blk.Timestamp())
}

// getBlockHistory gets block history
func (p *Protocol) getBlockHistory(blockHeight uint64) (*BlockHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectBlockHistory, BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(blockHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var blockHistory BlockHistory
	parsedRows, err := s.ParseSQLRows(rows, &blockHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	blockInfo := parsedRows[0].(*BlockHistory)
	return blockInfo, nil
}

// getProductivityHistory gets productivity history
func (p *Protocol) getProductivityHistory(epochNumber uint64, producerName string) (*ProductivityHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectProductivity, ProductivityTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, producerName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var productivityHistory ProductivityHistory
	parsedRows, err := s.ParseSQLRows(rows, &productivityHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	productivityInfo := parsedRows[0].(*ProductivityHistory)
	return productivityInfo, nil
}

// updateBlockHistory stores reward information into reward history table
func (p *Protocol) updateBlockHistory(
	tx *sql.Tx,
	epochNumber uint64,
	height uint64,
	hash string,
	transfers uint64,
	executions uint64,
	depositToRewardingFunds uint64,
	claimFromRewardingFunds uint64,
	grantRewards uint64,
	putPollResults uint64,
	gasConsumed uint64,
	producerAddress string,
	producerName string,
	expectedProducerAddress string,
	expectedProducerName string,
	timestamp time.Time,
) error {
	insertQuery := fmt.Sprintf(insertBlockHistory,
		BlockHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, height, hash, transfers, executions, depositToRewardingFunds,
		claimFromRewardingFunds, grantRewards, putPollResults, gasConsumed, producerAddress, producerName,
		expectedProducerAddress, expectedProducerName, timestamp.Unix()); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) updateDelegates(
	chainClient iotexapi.APIServiceClient,
	electionClient api.APIServiceClient,
	height uint64,
	epochNumber uint64,
) error {
	var gravityChainStartHeight uint64
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(indexprotocol.PollProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(height, 10))},
	}
	retryInterval := time.Duration(backoffInterval) * time.Minute
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(retryInterval), numOfRetry)
	nerr := backoff.Retry(func() error {
		readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
		if err != nil {
			return err
		}
		gravityChainStartHeight, err = strconv.ParseUint(string(readStateRes.GetData()), 10, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse gravityChainStartHeight")
		}
		if gravityChainStartHeight == 0 {
			//retry to get chain start height again
			return errors.New("waiting for fetching next timestamp in election service")
		}
		return nil
	}, bo)
	if nerr != nil {
		return errors.Wrap(nerr, "failed to get gravity chain start height by backoff")
	}
	getCandidatesRequest := &api.GetCandidatesRequest{
		Height: strconv.Itoa(int(gravityChainStartHeight)),
		Offset: uint32(0),
		Limit:  uint32(p.epochCtx.NumCandidateDelegates()),
	}
	getCandidatesResponse, err := electionClient.GetCandidates(context.Background(), getCandidatesRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get candidates from election service")
	}

	p.OperatorAddrToName = make(map[string]string)
	for _, candidate := range getCandidatesResponse.GetCandidates() {
		p.OperatorAddrToName[candidate.GetOperatorAddress()] = candidate.GetName()
	}

	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(indexprotocol.PollProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNumber, 10))},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get active block producers")
	}

	var activeBlockProducers state.CandidateList
	if err := activeBlockProducers.Deserialize(readStateRes.GetData()); err != nil {
		return errors.Wrap(err, "failed to deserialize active block producers")
	}
	p.ActiveBlockProducers = []string{}
	for _, activeBlockProducer := range activeBlockProducers {
		p.ActiveBlockProducers = append(p.ActiveBlockProducers, activeBlockProducer.Address)
	}

	return nil
}

func (p *Protocol) rebuildProductivityTable(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf(insertExpectedProducer,
		ExpectedProducerTableName, BlockHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(insertProducer,
		ProducerTableName, BlockHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(insertProductivity, ProductivityTableName,
		ExpectedProducerTableName, ProducerTableName)); err != nil {
		return err
	}

	return nil
}
