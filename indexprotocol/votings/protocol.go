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
	"time"

	// require sqlite 3
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/carrier"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-election/types"
	electiondb "github.com/iotexproject/iotex-election/db"
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

	// VotingInfo defines voting info
	VotingInfo struct {
		EpochNumber   uint64
		VoterAddress  string
		WeightedVotes string
	}

	rawData struct {
		mintTime          time.Time
		buckets           []*types.Bucket
		registrations     []*types.Registration
	}
	aggregateKey struct {
		epochNumber 		uint64
		candidateName 		string
		voterAddress 		string
	}

)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store           		s.Store
	PollArchive				committee.PollArchive
	NumDelegates    		uint64
	NumSubEpochs    		uint64
	GravityChainCfg 		indexprotocol.GravityChain
	SkipManifiedCandidate 	bool
	VoteThreshold         	*big.Int
	ScoreThreshold        	*big.Int
	SelfStakingThreshold  	*big.Int
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numSubEpochs uint64, gravityChainCfg indexprotocol.GravityChain, pollCfg indexprotocol.Poll) *Protocol {
	sqldb, err := sql.Open("sqlite3", "sqlite.db")
	if err != nil {
		zap.L().Error("failed to make sqliteDB")
		return nil
	}
	pollArchive, err := committee.NewArchive(sqldb, 0, 0, nil)
	if err != nil {
		zap.L().Error("failed to make pollArchive")
		return nil
	}
	voteThreshold, ok := new(big.Int).SetString(pollCfg.VoteThreshold, 10)
	if !ok {
		zap.L().Error("Invalid vote threshold")
		return nil
	}
	scoreThreshold, ok := new(big.Int).SetString(pollCfg.ScoreThreshold, 10)
	if !ok {
		zap.L().Error("Invalid score threshold")
		return nil
	}
	selfStakingThreshold, ok := new(big.Int).SetString(pollCfg.SelfStakingThreshold, 10)
	if !ok {
		zap.L().Error("Invalid self staking threshold")
		return nil
	}
	return &Protocol{
		Store: 					store, 
		PollArchive: 			pollArchive, 
		NumDelegates: 			numDelegates, 
		NumSubEpochs: 			numSubEpochs, 
		GravityChainCfg: 		gravityChainCfg,
		VoteThreshold:			voteThreshold,
		ScoreThreshold:			scoreThreshold,
		SelfStakingThreshold:	selfStakingThreshold,
		SkipManifiedCandidate:	pollCfg.SkipManifiedCandidate,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	if err := p.PollArchive.Start(ctx); err != nil{
		return errors.Wrap(err, "failed to start pollArchive")
	}
	// create voting result table
	var exist uint64
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
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"candidate_name VARCHAR(255) NOT NULL, voter_address VARCHAR(40) NOT NULL, aggregate_votes DECIMAL(65, 0) NOT NULL, "+
		"UNIQUE KEY %s (epoch_number, candidate_name, voter_address))", AggregateVotingTable, EpochCandidateVoterIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"voted_token DECIMAL(65,0) NOT NULL, delegate_count DECIMAL(65,0) NOT NULL, total_weighted DECIMAL(65, 0) NOT NULL, "+
		"UNIQUE KEY %s (epoch_number))", VotingMetaTableName, EpochIndexName)); err != nil {
		return err
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
		if err := p.rebuildAggregateVotingTable(tx, epochNumber - 1); err != nil {
			return errors.Wrap(err, "failed to rebuild aggregate voting table")
		}

		indexCtx := indexcontext.MustGetIndexCtx(ctx)
		chainClient := indexCtx.ChainClient
		electionClient := indexCtx.ElectionClient

		candidates, gravityHeight, err := p.getCandidates(chainClient, electionClient, height)
		if err != nil {
			return errors.Wrapf(err, "failed to get candidates from election service in epoch %d", epochNumber)
		}
		//getRawData from iotex-election
		getRawDataRequest := &api.GetRawDataRequest{
			Height: strconv.Itoa(int(gravityHeight)),
		}
		getRawDataResponse, err := electionClient.GetRawData(context.Background(), getRawDataRequest)
		if err != nil {
			return errors.Wrapf(err, "failed to get rawdata from iotex-election in epoch %d", epochNumber)
		}
		var buckets []*types.Bucket
		var regs []*types.Registration

		for _, bucketPb := range getRawDataResponse.Buckets {
			var bucket *types.Bucket
			bucket.FromProtoMsg(bucketPb)
			buckets = append(buckets, bucket)
		}

		for _, regPb := range getRawDataResponse.Registrations {
			var reg *types.Registration
			reg.FromProtoMsg(regPb)
			regs = append(regs, reg)
		}

		mintTime, err := ptypes.Timestamp(getRawDataResponse.Timestamp)
		if err != nil {
			return err
		}

		if err := p.PollArchive.PutPoll(height, mintTime, regs, buckets); err != nil { // iotex-height
			return errors.Wrapf(err, "failed to put poll in epoch %d", epochNumber)
		}

		if err := p.updateVotingResult(tx, candidates, epochNumber, gravityHeight); err != nil {
			return errors.Wrapf(err, "failed to update voting result in epoch %d", epochNumber)
		}
	}
	return nil
}

func (p *Protocol) bucketFilter(v *types.Bucket) bool {
	return p.VoteThreshold.Cmp(v.Amount()) > 0
}

func (p *Protocol) candidateFilter(c *types.Candidate) bool {
	return p.SelfStakingThreshold.Cmp(c.SelfStakingTokens()) > 0 ||
		p.ScoreThreshold.Cmp(c.Score()) > 0
} 

func (p *Protocol) calcWeightedVotes(v *types.Bucket, now time.Time) *big.Int {
	if now.Before(v.StartTime()) {
		return big.NewInt(0)
	}
	remainingTime := v.RemainingTime(now).Seconds()
	weight := float64(1)
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)) / math.Log(1.2) / 100
	}
	amount := new(big.Float).SetInt(v.Amount())
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)

	return weightedAmount
}

func (p *Protocol) resultByHeight(height uint64) (*types.ElectionResult, error) {
	timestamp, err := p.PollArchive.MintTime(height)
	if err != nil {
		return nil, err
	}
	calculator := types.NewResultCalculator(timestamp,
		p.SkipManifiedCandidate,
		p.bucketFilter,
		p.calcWeightedVotes,
		p.candidateFilter,
	)
	regs, err := p.PollArchive.Registrations(height)
	if err != nil {
		return nil, err
	}
	if err := calculator.AddRegistrations(regs); err != nil {
		return nil, err
	}
	buckets, err := p.PollArchive.Buckets(height)
	if err != nil {
		return nil, err
	}
	if err := calculator.AddBuckets(buckets); err != nil {
		return nil, err
	}
	result, err := calculator.Calculate()
	if err != nil {
		return nil, err
	}
	return result, nil 
}

func (p *Protocol) GetBucketInfoByEpoch(epochNum uint64, delegateName string) ([]*VotingInfo, error) {
	height := indexprotocol.GetEpochHeight(epochNum, p.NumDelegates, p.NumSubEpochs)
	result, err := p.resultByHeight(height)
	if err != nil {
		return nil, err
	}
	votes := result.VotesByDelegate([]byte(delegateName))
	votinginfoList := make([]*VotingInfo, len(votes))
	for i, vote := range votes {
		votinginfoList[i] = &VotingInfo {
			EpochNumber: epochNum,
			VoterAddress: hex.EncodeToString(vote.Voter()),
			WeightedVotes: vote.WeightedAmount().Text(10),
		}
	}
	return votinginfoList, nil
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

func (p *Protocol) updateVotingResult(tx *sql.Tx, candidates []*api.Candidate, epochNumber uint64, gravityHeight uint64) (err error) {
	var voteResultStmt *sql.Stmt
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, delegate_name,operator_address, reward_address, "+
		"total_weighted_votes, self_staking, block_reward_percentage, epoch_reward_percentage, foundation_bonus_percentage, staking_address) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		VotingResultTableName)
	if voteResultStmt, err = tx.Prepare(insertQuery); err != nil {
		return err
	}
	defer func() {
		closeErr := voteResultStmt.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for _, candidate := range candidates {
		stakingAddress := common.HexToAddress(candidate.Address)
		blockRewardPortion, epochRewardPortion, foundationBonusPortion, err := p.getDelegateRewardPortions(stakingAddress, gravityHeight)
		if err != nil {
			return err
		}
		if _, err = voteResultStmt.Exec(
			epochNumber,
			candidate.Name,
			candidate.OperatorAddress,
			candidate.RewardAddress,
			candidate.TotalWeightedVotes,
			candidate.SelfStakingTokens,
			blockRewardPortion,
			epochRewardPortion,
			foundationBonusPortion,
			candidate.Address,
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) rebuildAggregateVotingTable(tx *sql.Tx, lastEpoch uint64) (err error) {
	if lastEpoch == 0 {
		return nil
	}
	height := indexprotocol.GetEpochHeight(lastEpoch, p.NumDelegates, p.NumSubEpochs)
	result, err := p.resultByHeight(height)
	if err != nil {
		if errors.Cause(err) == electiondb.ErrNotExist {
			return nil
		}
		return err
	}
	delegates := result.Delegates()
	votes := result.Votes()
	total_weighted := result.TotalVotes()
	var sumOfVotes *big.Int
	var sumOfWeightedVotes map[aggregateKey] *big.Int

	for _, vote := range votes {
		//for sumOfWeightedVotes
		key := aggregateKey {
			epochNumber:	lastEpoch,
			candidateName:	string(vote.Candidate()),
			voterAddress:	hex.EncodeToString(vote.Voter()),
		}
		if val, ok := sumOfWeightedVotes[key]; ok {
			val.Add(val, vote.WeightedAmount())
		} else {
			sumOfWeightedVotes[key] = vote.WeightedAmount()
		}
		//for SumOfVotes
		sumOfVotes.Add(sumOfVotes, vote.Amount())
	}
	insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (epoch_number, candidate_name, voter_address, aggregate_votes) VALUES (?, ?, ?, ?)", AggregateVotingTable)
	var aggregateStmt *sql.Stmt
	if aggregateStmt, err = tx.Prepare(insertQuery); err != nil {
		return err
	}
	defer func() {
		closeErr := aggregateStmt.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for key, val := range sumOfWeightedVotes {
		if _, err = aggregateStmt.Exec(
			key.epochNumber,
			key.candidateName,
			key.voterAddress,
			val.Text(10),
		); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(fmt.Sprintf("INSERT INTO %s (epoch_number, voted_token, delegate_count, total_weighted) VALUES (?, ?, ?, ?)", VotingMetaTableName), 
		lastEpoch, sumOfVotes.Text(10), len(delegates), total_weighted.Text(10)); err != nil {
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
