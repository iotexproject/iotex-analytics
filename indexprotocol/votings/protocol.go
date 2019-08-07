// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-election/carrier"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/contract"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "voting"
	// VotingHistoryTableName is the table name of voting history
	VotingHistoryTableName = "voting_history"
	// VotingResultTableName is the table name of voting result
	VotingResultTableName = "voting_result"
	//VotingMetaTableName is the voting meta table
	VotingMetaTableName = "voting_meta"
	// AggregateVotingTable is the table name of voters' aggregate voting
	AggregateVotingTable = "aggregate_voting"
	// EpochIndexName is the index name of epoch number on voting meta table
	EpochIndexName = "epoch_index"
	// EpochVoterIndexName is the index name of epoch number and voter address on voting history table
	EpochVoterIndexName = "epoch_voter_index"
	// CandidateVoterIndexName is the index name of candidate name and voter address on voting history table
	CandidateVoterIndexName = "candidate_voter_index"
	// EpochCandidateIndexName is the index name of epoch number and candidate name on voting history/result table
	EpochCandidateIndexName = "epoch_candidate_index"
	// EpochCandidateVoterIndexName is the index name of epoch number, candidate name, and voter address on aggregate voting table
	EpochCandidateVoterIndexName = "epoch_candidate_voter_index"
	// DefaultStakingAddress is the default staking address for delegates
	DefaultStakingAddress = "0000000000000000000000000000000000000000"
)

type (
	// VotingHistory defines the schema of "voting history" table
	VotingHistory struct {
		EpochNumber       uint64
		CandidateName     string
		VoterAddress      string
		Votes             string
		WeightedVotes     string
		RemainingDuration string
	}

	// VotingResult defines the schema of "voting result" table
	VotingResult struct {
		EpochNumber               uint64
		DelegateName              string
		OperatorAddress           string
		RewardAddress             string
		TotalWeightedVotes        string
		SelfStaking               string
		BlockRewardPercentage     uint64
		EpochRewardPercentage     uint64
		FoundationBonusPercentage uint64
		StakingAddress            string
	}

	// AggregateVoting defines the schema of "aggregate voting" table
	AggregateVoting struct {
		EpochNumber    uint64
		CandidateName  string
		VoterAddress   string
		AggregateVotes string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store           s.Store
	NumDelegates    uint64
	NumSubEpochs    uint64
	GravityChainCfg indexprotocol.GravityChain
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numSubEpochs uint64, gravityChainCfg indexprotocol.GravityChain) *Protocol {
	return &Protocol{Store: store, NumDelegates: numDelegates, NumSubEpochs: numSubEpochs, GravityChainCfg: gravityChainCfg}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create voting history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"(epoch_number DECIMAL(65, 0) NOT NULL, candidate_name VARCHAR(24) NOT NULL, voter_address VARCHAR(40) NOT NULL, votes DECIMAL(65, 0) NOT NULL, "+
		"weighted_votes DECIMAL(65, 0) NOT NULL, remaining_duration TEXT NOT NULL)",
		VotingHistoryTableName)); err != nil {
		return err
	}

	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf("SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = "+
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", VotingHistoryTableName, EpochCandidateIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE INDEX %s ON %s (epoch_number, candidate_name)", EpochCandidateIndexName, VotingHistoryTableName)); err != nil {
			return err
		}
	}
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf("SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = "+
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", VotingHistoryTableName, EpochVoterIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE INDEX %s ON %s (epoch_number, voter_address)", EpochVoterIndexName, VotingHistoryTableName)); err != nil {
			return err
		}
	}
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf("SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = "+
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", VotingHistoryTableName, CandidateVoterIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE INDEX %s ON %s (candidate_name, voter_address)", CandidateVoterIndexName, VotingHistoryTableName)); err != nil {
			return err
		}
	}

	// create voting result table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"(epoch_number DECIMAL(65, 0) NOT NULL, delegate_name VARCHAR(255) NOT NULL, operator_address VARCHAR(41) NOT NULL, "+
		"reward_address VARCHAR(41) NOT NULL, total_weighted_votes DECIMAL(65, 0) NOT NULL, self_staking DECIMAL(65,0) NOT NULL, "+
		"block_reward_percentage INT DEFAULT 100, epoch_reward_percentage INT DEFAULT 100, foundation_bonus_percentage INT DEFAULT 100, "+
		"staking_address VARCHAR(40) DEFAULT %s)",
		VotingResultTableName, DefaultStakingAddress)); err != nil {
		return err
	}

	if err := p.Store.GetDB().QueryRow(fmt.Sprintf("SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = "+
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", VotingResultTableName, EpochCandidateIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (epoch_number, delegate_name)", EpochCandidateIndexName, VotingResultTableName)); err != nil {
			return err
		}
	}

	return nil
}

// Initialize initializes votings protocol
func (p *Protocol) Initialize(context.Context, *sql.Tx, *indexprotocol.Genesis) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := indexprotocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, height)

	if height == indexprotocol.GetEpochHeight(epochNumber, p.NumDelegates, p.NumSubEpochs) {
		if err := p.rebuildAggregateVotingTable(tx); err != nil {
			return errors.Wrap(err, "failed to rebuild aggregate voting table")
		}

		indexCtx := indexcontext.MustGetIndexCtx(ctx)
		chainClient := indexCtx.ChainClient
		electionClient := indexCtx.ElectionClient

		candidates, gravityHeight, err := p.getCandidates(chainClient, electionClient, height)
		if err != nil {
			return errors.Wrapf(err, "failed to get candidates from election service in epoch %d", epochNumber)
		}

		candidateToBuckets := make(map[string][]*api.Bucket)
		for _, candidate := range candidates {
			getBucketsRequest := &api.GetBucketsByCandidateRequest{
				Name:   candidate.Name,
				Height: strconv.Itoa(int(gravityHeight)),
				Offset: uint32(0),
				Limit:  math.MaxUint32,
			}
			getBucketsResponse, err := electionClient.GetBucketsByCandidate(ctx, getBucketsRequest)
			if err != nil {
				// TODO: Need a better way to handle the case that a candidate has no votes (len(voters) == 0)
				if strings.Contains(err.Error(), "offset is out of range") {
					continue
				}
				return errors.Wrapf(err, "failed to get buckets by candidate in epoch %d", epochNumber)
			}
			buckets := getBucketsResponse.Buckets
			candidateToBuckets[candidate.Name] = buckets
		}
		if err := p.updateVotingHistory(tx, candidateToBuckets, epochNumber); err != nil {
			return errors.Wrapf(err, "failed to update voting history in epoch %d", epochNumber)
		}
		if err := p.updateVotingResult(tx, candidates, epochNumber, gravityHeight); err != nil {
			return errors.Wrapf(err, "failed to update voting result in epoch %d", epochNumber)
		}
	}
	return nil
}

// getVotingHistory gets voting history
func (p *Protocol) getVotingHistory(epochNumber uint64, candidateName string) ([]*VotingHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND candidate_name=?",
		VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, candidateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var votingHistory VotingHistory
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	var votingHistoryList []*VotingHistory
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*VotingHistory)
		votingHistoryList = append(votingHistoryList, voting)
	}
	return votingHistoryList, nil
}

// getVotingResult gets voting result
func (p *Protocol) getVotingResult(epochNumber uint64, delegateName string) (*VotingResult, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND delegate_name=?",
		VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var votingResult VotingResult
	parsedRows, err := s.ParseSQLRows(rows, &votingResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	return parsedRows[0].(*VotingResult), nil
}

func (p *Protocol) getCandidates(
	chainClient iotexapi.APIServiceClient,
	electionClient api.APIServiceClient,
	height uint64,
) ([]*api.Candidate, uint64, error) {
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(height)},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to get gravity chain start height")
	}
	gravityChainStartHeight := byteutil.BytesToUint64(readStateRes.Data)

	getCandidatesRequest := &api.GetCandidatesRequest{
		Height: strconv.Itoa(int(gravityChainStartHeight)),
		Offset: uint32(0),
		Limit:  math.MaxUint32,
	}

	getCandidatesResponse, err := electionClient.GetCandidates(context.Background(), getCandidatesRequest)
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to get candidates from election service")
	}
	return getCandidatesResponse.Candidates, gravityChainStartHeight, nil
}

func (p *Protocol) updateVotingHistory(tx *sql.Tx, candidateToBuckets map[string][]*api.Bucket, epochNumber uint64) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	for candidateName, buckets := range candidateToBuckets {
		for _, bucket := range buckets {
			valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?)")
			valArgs = append(valArgs, epochNumber, candidateName, bucket.Voter, bucket.Votes, bucket.WeightedVotes, bucket.RemainingDuration)
		}
	}
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number,candidate_name,voter_address,votes,weighted_votes,"+
		"remaining_duration) VALUES %s", VotingHistoryTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) updateVotingResult(tx *sql.Tx, candidates []*api.Candidate, epochNumber uint64, gravityHeight uint64) error {
	valStrs := make([]string, 0, len(candidates))
	valArgs := make([]interface{}, 0, len(candidates)*10)
	for _, candidate := range candidates {
		valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		stakingAddress := common.HexToAddress(candidate.Address)
		blockRewardPortion, epochRewardPortion, foundationBonusPortion, err := p.getDelegateRewardPortions(stakingAddress, gravityHeight)
		if err != nil {
			return err
		}
		valArgs = append(valArgs, epochNumber, candidate.Name, candidate.OperatorAddress, candidate.RewardAddress,
			candidate.TotalWeightedVotes, candidate.SelfStakingTokens, blockRewardPortion, epochRewardPortion, foundationBonusPortion, candidate.Address)
	}

	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number,delegate_name,operator_address,reward_address,"+
		"total_weighted_votes, self_staking, block_reward_percentage, epoch_reward_percentage, foundation_bonus_percentage, staking_address) VALUES %s", VotingResultTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) rebuildAggregateVotingTable(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"candidate_name VARCHAR(255) NOT NULL, voter_address VARCHAR(40) NOT NULL, aggregate_votes DECIMAL(65, 0) NOT NULL, "+
		"UNIQUE KEY %s (epoch_number, candidate_name, voter_address))", AggregateVotingTable, EpochCandidateVoterIndexName)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("INSERT IGNORE INTO %s SELECT epoch_number, candidate_name, "+
		"voter_address, SUM(weighted_votes) FROM %s GROUP BY epoch_number, candidate_name, voter_address",
		AggregateVotingTable, VotingHistoryTableName)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"voted_token DECIMAL(65,0) NOT NULL, delegate_count DECIMAL(65,0) NOT NULL, total_weighted DECIMAL(65, 0) NOT NULL, "+
		"UNIQUE KEY %s (epoch_number))", VotingMetaTableName, EpochIndexName)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("INSERT IGNORE INTO %s SELECT history_t.epoch_number, voted_token, "+
		"delegate_count, total_weighted FROM (SELECT epoch_number,SUM(votes) AS voted_token FROM %s GROUP BY epoch_number) AS history_t INNER JOIN (SELECT epoch_number,COUNT(delegate_name) AS delegate_count,SUM(total_weighted_votes) AS total_weighted FROM %s GROUP BY epoch_number) AS result_t ON history_t.epoch_number=result_t.epoch_number ",
		VotingMetaTableName, VotingHistoryTableName, VotingResultTableName)); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) getDelegateRewardPortions(stakingAddress common.Address, gravityChainHeight uint64) (blockRewardPercentage, epochRewardPercentage, foundationBonusPercentage int64, err error) {
	if p.GravityChainCfg.GravityChainAPIs == nil || gravityChainHeight < p.GravityChainCfg.RewardPercentageStartHeight {
		blockRewardPercentage = 100
		epochRewardPercentage = 100
		foundationBonusPercentage = 100
		return
	}
	clientPool := carrier.NewEthClientPool(p.GravityChainCfg.GravityChainAPIs)

	if err = clientPool.Execute(func(client *ethclient.Client) error {
		if caller, err := contract.NewDelegateProfileCaller(common.HexToAddress(p.GravityChainCfg.RegisterContractAddress), client); err == nil {
			opts := &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(gravityChainHeight)}
			blockRewardPortion, err := caller.GetProfileByField(opts, stakingAddress, "blockRewardPortion")
			if err != nil {
				return err
			}
			epochRewardPortion, err := caller.GetProfileByField(opts, stakingAddress, "epochRewardPortion")
			if err != nil {
				return err
			}
			foundationRewardPortion, err := caller.GetProfileByField(opts, stakingAddress, "foundationRewardPortion")
			if err != nil {
				return err
			}

			if len(blockRewardPortion) > 0 {
				blockPortion, err := strconv.ParseInt(hex.EncodeToString(blockRewardPortion), 16, 64)
				if err != nil {
					return err
				}
				blockRewardPercentage = blockPortion / 100
			}
			if len(epochRewardPortion) > 0 {
				epochPortion, err := strconv.ParseInt(hex.EncodeToString(epochRewardPortion), 16, 64)
				if err != nil {
					return err
				}
				epochRewardPercentage = epochPortion / 100
			}
			if len(foundationRewardPortion) > 0 {
				foundationPortion, err := strconv.ParseInt(hex.EncodeToString(foundationRewardPortion), 16, 64)
				if err != nil {
					return err
				}
				foundationBonusPercentage = foundationPortion / 100
			}
		}
		return nil
	}); err != nil {
		err = errors.Wrap(err, "failed to get delegate reward portions")
	}
	return
}
