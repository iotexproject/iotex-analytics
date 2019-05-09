package blocks

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/protocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "blocks"
	// BlockHistoryTableName is the table name of block history
	BlockHistoryTableName = "block_history"
)

type (
	// BlockHistory defines the schema of "block history" table
	BlockHistory struct {
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
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store                 s.Store
	NumDelegates          uint64
	NumCandidateDelegates uint64
	NumSubEpochs          uint64
	ActiveBlockProducers  []string
	OperatorAddrToName    map[string]string
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numCandidateDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{
		Store:                 store,
		NumDelegates:          numDelegates,
		NumCandidateDelegates: numCandidateDelegates,
		NumSubEpochs:          numSubEpochs,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create reward history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ([block_height] INT NOT NULL, "+
		"[block_hash] TEXT NOT NULL, [transfer] INT NOT NULL, [execution] INT NOT NULL, "+
		"[depositToRewardingFund] INT NOT NULL, [claimFromRewardingFund] INT NOT NULL, [grantReward] INT NOT NULL, "+
		"[putPollResult] INT NOT NULL, [gas_consumed] INT NOT NULL, [producer_address] TEXT NOT NULL, "+
		"[producer_name] TEXT NOT NULL, [expected_producer_address] TEXT NOT NULL, "+
		"[expected_producer_name] TEXT NOT NULL)", BlockHistoryTableName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes rewards protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx, genesisCfg *protocol.GenesisConfig) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := protocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, height)
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient
	electionClient := indexCtx.ElectionClient
	// Special handling for epoch start height
	if height == protocol.GetEpochHeight(epochNumber, p.NumDelegates, p.NumSubEpochs) || p.OperatorAddrToName == nil {
		if err := p.updateDelegates(chainClient, electionClient, height, epochNumber); err != nil {
			return errors.Wrapf(err, "failed to update delegates in epoch %d", epochNumber)
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
	expectedProducerAddr := p.ActiveBlockProducers[int(height)%len(p.ActiveBlockProducers)]
	expectedProducerName := p.OperatorAddrToName[expectedProducerAddr]
	return p.updateBlockHistory(tx, height, hex.EncodeToString(hash[:]), transferCount, executionCount,
		depositToRewardingFundCount, claimFromRewardingFundCount, grantRewardCount, putPollResultCount, gasConsumed,
		producerAddr, producerName, expectedProducerAddr, expectedProducerName)
}

// GetBlockHistory gets block history
func (p *Protocol) GetBlockHistory(blockHeight uint64) (*BlockHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE block_height=?", BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

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
		return nil, protocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	blockInfo := parsedRows[0].(*BlockHistory)
	return blockInfo, nil
}

// updateBlockHistory stores reward information into reward history table
func (p *Protocol) updateBlockHistory(
	tx *sql.Tx,
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
) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (block_height, block_hash, transfer, execution, "+
		"depositToRewardingFund, claimFromRewardingFund, grantReward, putPollResult, gas_consumed, producer_address, "+
		"producer_name, expected_producer_address, expected_producer_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		BlockHistoryTableName)
	if _, err := tx.Exec(insertQuery, height, hash, transfers, executions, depositToRewardingFunds,
		claimFromRewardingFunds, grantRewards, putPollResults, gasConsumed, producerAddress, producerName,
		expectedProducerAddress, expectedProducerName); err != nil {
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
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(height)},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get gravity chain start height")
	}
	gravityChainStartHeight := byteutil.BytesToUint64(readStateRes.Data)

	getCandidatesRequest := &api.GetCandidatesRequest{
		Height: strconv.Itoa(int(gravityChainStartHeight)),
		Offset: uint32(0),
		Limit:  uint32(p.NumCandidateDelegates),
	}

	getCanidatesResponse, err := electionClient.GetCandidates(context.Background(), getCandidatesRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get candidates from election service")
	}

	p.OperatorAddrToName = make(map[string]string)
	var candidateAddrList []string
	for _, candidate := range getCanidatesResponse.Candidates {
		candidateAddrList = append(candidateAddrList, candidate.OperatorAddress)
		p.OperatorAddrToName[candidate.OperatorAddress] = candidate.Name
	}

	crypto.SortCandidates(candidateAddrList, epochNumber, crypto.CryptoSeed)
	p.ActiveBlockProducers = candidateAddrList
	if len(p.ActiveBlockProducers) > int(p.NumDelegates) {
		p.ActiveBlockProducers = p.ActiveBlockProducers[:p.NumDelegates]
	}

	return nil
}
