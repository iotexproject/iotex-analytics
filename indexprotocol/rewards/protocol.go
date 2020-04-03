// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "rewards"
	// RewardHistoryTableName is the table name of reward history
	RewardHistoryTableName = "reward_history"
	// AccountRewardTableName is the table name of account rewards
	AccountRewardTableName = "account_reward"
	// EpochCandidateIndexName is the index name of epoch number and candidate name on account reward view
	EpochCandidateIndexName = "epoch_candidate_index"

	createRewardHistory = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"action_hash VARCHAR(64) NOT NULL, reward_address VARCHAR(41) NOT NULL, candidate_name VARCHAR(24) NOT NULL, " +
		"block_reward DECIMAL(65, 0) NOT NULL, epoch_reward DECIMAL(65, 0) NOT NULL, foundation_bonus DECIMAL(65, 0) NOT NULL)"
	createAccountReward = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"candidate_name VARCHAR(24) NOT NULL, block_reward DECIMAL(65, 0) NOT NULL, epoch_reward DECIMAL(65, 0) NOT NULL, " +
		"foundation_bonus DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, candidate_name))"
	selectRewardHistory = "SELECT * FROM %s WHERE action_hash=?"
	selectAccountReward = "SELECT * FROM %s WHERE epoch_number=? AND candidate_name=?"
	insertRewardHistory = "INSERT INTO %s (epoch_number, action_hash,reward_address,candidate_name,block_reward,epoch_reward," +
		"foundation_bonus) VALUES %s"
	selectRewardHistoryGroup = "SELECT epoch_number, reward_address, SUM(block_reward), SUM(epoch_reward), SUM(foundation_bonus) " +
		"FROM %s WHERE epoch_number = ? GROUP BY epoch_number, reward_address"
	insertAccountReward = "INSERT IGNORE INTO %s (epoch_number,candidate_name,block_reward,epoch_reward," +
		"foundation_bonus) VALUES %s"
	selectVotingResult = "SELECT * FROM %s WHERE epoch_number = ?"
	selectBlockHistory = "SELECT t1.epoch_number, t1.expected_producer_name AS delegate_name, " +
		"CAST(IFNULL(production, 0) AS DECIMAL(65, 0)) AS production, CAST(expected_production AS DECIMAL(65, 0)) AS expected_production " +
		"FROM (SELECT epoch_number, expected_producer_name, COUNT(expected_producer_address) AS expected_production FROM %s WHERE epoch_number = ? GROUP BY epoch_number, expected_producer_name) " +
		"AS t1 LEFT JOIN (SELECT epoch_number, producer_name, COUNT(producer_address) AS production FROM %s WHERE epoch_number = ? GROUP BY epoch_number, producer_name) " +
		"AS t2 ON t1.epoch_number = t2.epoch_number AND t1.expected_producer_name=t2.producer_name"
)

type (
	// RewardHistory defines the schema of "reward history" table
	RewardHistory struct {
		EpochNumber     uint64
		ActionHash      string
		RewardAddress   string
		CandidateName   string
		BlockReward     string
		EpochReward     string
		FoundationBonus string
	}

	// AccountReward defines the schema of "account reward" view
	AccountReward struct {
		EpochNumber     uint64
		CandidateName   string
		BlockReward     string
		EpochReward     string
		FoundationBonus string
	}

	// AggregateReward defines aggregate reward records
	AggregateReward struct {
		EpochNumber     uint64
		RewardAddress   string
		BlockReward     string
		EpochReward     string
		FoundationBonus string
	}

	// CandidateVote defines candidate vote structure
	CandidateVote struct {
		CandidateName      string
		TotalWeightedVotes *big.Int
	}

	// Productivity defines block producers' productivity in an epoch
	Productivity struct {
		Production         uint64
		ExpectedProduction uint64
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store            s.Store
	RewardAddrToName map[string][]string
	RewardConfig     indexprotocol.Rewarding
	epochCtx         *epochctx.EpochCtx
	gravityChainCfg  indexprotocol.GravityChain
}

// RewardInfo indicates the amount of different reward types
type RewardInfo struct {
	BlockReward     *big.Int
	EpochReward     *big.Int
	FoundationBonus *big.Int
}

// NewProtocol creates a new protocol
func NewProtocol(
	store s.Store,
	epochCtx *epochctx.EpochCtx,
	rewardingConfig indexprotocol.Rewarding,
	gravityChainCfg indexprotocol.GravityChain,
) *Protocol {
	return &Protocol{
		Store:           store,
		RewardConfig:    rewardingConfig,
		epochCtx:        epochCtx,
		gravityChainCfg: gravityChainCfg,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create reward history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createRewardHistory,
		RewardHistoryTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createAccountReward, AccountRewardTableName, EpochCandidateIndexName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes rewards protocol
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
	if indexCtx.ConsensusScheme == "ROLLDPOS" && (height == epochHeight || p.RewardAddrToName == nil) {
		if err := p.updateCandidateRewardAddress(chainClient, electionClient, height, epochNumber); err != nil {
			return errors.Wrapf(err, "failed to update candidates in epoch %d", epochNumber)
		}
	}
	if indexCtx.ConsensusScheme == "ROLLDPOS" && height == epochHeight {
		if err := p.rebuildAccountRewardTable(tx, epochNumber-1); err != nil {
			return errors.Wrap(err, "failed to rebuild account reward table")
		}
	}

	grantRewardActs := make(map[hash.Hash256]bool)
	// log action index
	for _, selp := range blk.Actions {
		if _, ok := selp.Action().(*action.GrantReward); ok {
			grantRewardActs[selp.Hash()] = true
		}
	}
	// log receipt index
	for _, receipt := range blk.Receipts {
		if _, ok := grantRewardActs[receipt.ActionHash]; ok {
			// Parse receipt of grant reward
			rewardInfoMap, err := p.getRewardInfoFromReceipt(receipt)
			if err != nil {
				return errors.Wrap(err, "failed to get reward info from receipt")
			}
			if len(rewardInfoMap) == 0 {
				continue
			}
			// Update reward info in DB
			actionHash := hex.EncodeToString(receipt.ActionHash[:])
			if err := p.updateRewardHistory(tx, epochNumber, actionHash, rewardInfoMap); err != nil {
				return errors.Wrap(err, "failed to update epoch number and reward address to reward history table")
			}
		}
	}
	return nil
}

// getRewardHistory reads reward history
func (p *Protocol) getRewardHistory(actionHash string) ([]*RewardHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectRewardHistory,
		RewardHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(actionHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var rewardHistory RewardHistory
	parsedRows, err := s.ParseSQLRows(rows, &rewardHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	var rewardHistoryList []*RewardHistory
	for _, parsedRow := range parsedRows {
		rewards := parsedRow.(*RewardHistory)
		rewardHistoryList = append(rewardHistoryList, rewards)
	}
	return rewardHistoryList, nil
}

// getAccountReward reads account reward details
func (p *Protocol) getAccountReward(epochNumber uint64, candidateName string) (*AccountReward, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectAccountReward,
		AccountRewardTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, candidateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var accountReward AccountReward
	parsedRows, err := s.ParseSQLRows(rows, &accountReward)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	return parsedRows[0].(*AccountReward), nil
}

// updateRewardHistory stores reward information into reward history table
func (p *Protocol) updateRewardHistory(tx *sql.Tx, epochNumber uint64, actionHash string, rewardInfoMap map[string]*RewardInfo) error {
	valStrs := make([]string, 0, len(rewardInfoMap))
	valArgs := make([]interface{}, 0, len(rewardInfoMap)*7)
	for rewardAddress, rewards := range rewardInfoMap {
		blockReward := rewards.BlockReward.String()
		epochReward := rewards.EpochReward.String()
		foundationBonus := rewards.FoundationBonus.String()

		var candidateName string
		// If more than one candidates share the same reward address, just use the first candidate as their delegate
		if len(p.RewardAddrToName[rewardAddress]) > 0 {
			candidateName = p.RewardAddrToName[rewardAddress][0]
		}

		valStrs = append(valStrs, "(?, ?, ?, ?, CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)))")
		valArgs = append(valArgs, epochNumber, actionHash, rewardAddress, candidateName, blockReward, epochReward, foundationBonus)
	}
	insertQuery := fmt.Sprintf(insertRewardHistory, RewardHistoryTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) getRewardInfoFromReceipt(receipt *action.Receipt) (map[string]*RewardInfo, error) {
	rewardInfoMap := make(map[string]*RewardInfo)
	for _, l := range receipt.Logs {
		rewardLog := &rewardingpb.RewardLog{}
		if err := proto.Unmarshal(l.Data, rewardLog); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal receipt data into reward log")
		}
		rewards, ok := rewardInfoMap[rewardLog.Addr]
		if !ok {
			rewardInfoMap[rewardLog.Addr] = &RewardInfo{
				BlockReward:     big.NewInt(0),
				EpochReward:     big.NewInt(0),
				FoundationBonus: big.NewInt(0),
			}
			rewards = rewardInfoMap[rewardLog.Addr]
		}
		amount, ok := big.NewInt(0).SetString(rewardLog.Amount, 10)
		if !ok {
			log.L().Fatal("Failed to convert reward amount from string to big int")
			return nil, errors.New("failed to convert reward amount from string to big int")
		}
		switch rewardLog.Type {
		case rewardingpb.RewardLog_BLOCK_REWARD:
			rewards.BlockReward.Add(rewards.BlockReward, amount)
		case rewardingpb.RewardLog_EPOCH_REWARD:
			rewards.EpochReward.Add(rewards.EpochReward, amount)
		case rewardingpb.RewardLog_FOUNDATION_BONUS:
			rewards.FoundationBonus.Add(rewards.FoundationBonus, amount)
		default:
			log.L().Fatal("Unknown type of reward")
		}
	}
	return rewardInfoMap, nil
}

func (p *Protocol) updateCandidateRewardAddress(
	chainClient iotexapi.APIServiceClient,
	electionClient api.APIServiceClient,
	height uint64,
	epochNumber uint64,
) (err error) {
	if height >= p.epochCtx.FairbankEffectiveHeight() {
		return p.updateStakingCandidateRewardAddress(chainClient, height)
	}
	var gravityChainStartHeight uint64
	if epochNumber == 1 {
		gravityChainStartHeight = p.gravityChainCfg.GravityChainStartHeight
	} else {
		prevEpochHeight := p.epochCtx.GetEpochHeight(epochNumber - 1)
		gravityChainStartHeight, err = indexprotocol.GetGravityChainStartHeight(chainClient, prevEpochHeight)
		if err != nil {
			return errors.Wrapf(err, "failed to get gravity height from chain service in epoch %d", epochNumber-1)
		}
	}
	getCandidatesRequest := &api.GetCandidatesRequest{
		Height: strconv.Itoa(int(gravityChainStartHeight)),
		Offset: uint32(0),
		Limit:  math.MaxUint32,
	}

	getCandidatesResponse, err := electionClient.GetCandidates(context.Background(), getCandidatesRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get candidates from election service")
	}

	p.RewardAddrToName = make(map[string][]string)
	for _, candidate := range getCandidatesResponse.GetCandidates() {
		if _, ok := p.RewardAddrToName[candidate.GetRewardAddress()]; !ok {
			p.RewardAddrToName[candidate.GetRewardAddress()] = make([]string, 0)
		}
		p.RewardAddrToName[candidate.GetRewardAddress()] = append(p.RewardAddrToName[candidate.GetRewardAddress()], candidate.GetName())
	}
	return nil
}

func (p *Protocol) updateStakingCandidateRewardAddress(
	chainClient iotexapi.APIServiceClient,
	height uint64,
) error {
	candidateList, err := indexprotocol.GetAllStakingCandidates(chainClient, height)
	if err != nil {
		return errors.Wrap(err, "get candidate error")
	}
	p.RewardAddrToName = make(map[string][]string)
	for _, candidate := range candidateList.Candidates {
		if _, ok := p.RewardAddrToName[candidate.RewardAddress]; !ok {
			p.RewardAddrToName[candidate.RewardAddress] = make([]string, 0)
		}
		encodedDelegateName, err := indexprotocol.EncodeDelegateName(candidate.Name)
		if err != nil {
			return errors.Wrap(err, "encode delegate name error")
		}
		p.RewardAddrToName[candidate.RewardAddress] = append(p.RewardAddrToName[candidate.RewardAddress], encodedDelegateName)
	}
	return nil
}

func (p *Protocol) rebuildAccountRewardTable(tx *sql.Tx, lastEpoch uint64) error {
	if lastEpoch == 0 {
		return nil
	}
	// Get voting result from last epoch
	rewardAddrToNameMapping, weightedVotesMapping, err := p.getVotingInfo(tx, lastEpoch)
	if err != nil {
		return errors.Wrap(err, "failed to get voting info")
	}
	// Get aggregate reward	records from last epoch
	getQuery := fmt.Sprintf(selectRewardHistoryGroup, RewardHistoryTableName)
	rows, err := tx.Query(getQuery, lastEpoch)
	if err != nil {
		return errors.Wrap(err, "failed to get reward history query")
	}

	var aggregateReward AggregateReward
	parsedRows, err := s.ParseSQLRows(rows, &aggregateReward)
	if err != nil {
		return errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		log.S().Warnf("No reward history for epoch %s", lastEpoch)
		return nil
	}

	exemptionMap := make(map[string]bool)
	for _, name := range p.RewardConfig.ExemptCandidatesFromEpochReward {
		exemptionMap[name] = true
	}

	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	for _, parsedRow := range parsedRows {
		record := parsedRow.(*AggregateReward)
		epochNumber := record.EpochNumber
		rewardAddress := record.RewardAddress
		candidateNames := rewardAddrToNameMapping[rewardAddress]
		if len(candidateNames) == 1 {
			candidateName := candidateNames[0]
			valStrs = append(valStrs, "(?, ?, CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)))")
			valArgs = append(valArgs, epochNumber, candidateName, record.BlockReward, record.EpochReward, record.FoundationBonus)
			continue
		}

		// Multiple delegates share reward address
		totalBlockReward, ok := big.NewInt(0).SetString(record.BlockReward, 10)
		if !ok {
			return errors.New("failed to convert string to big int")
		}
		totalEpochReward, ok := big.NewInt(0).SetString(record.EpochReward, 10)
		if !ok {
			return errors.New("failed to convert string to big int")
		}
		totalFoundationBonus, ok := big.NewInt(0).SetString(record.FoundationBonus, 10)
		if !ok {
			return errors.New("failed to convert string to big int")
		}
		candidateRewardsMap, err := p.breakdownRewards(epochNumber, candidateNames, weightedVotesMapping,
			exemptionMap, totalBlockReward, totalEpochReward, totalFoundationBonus)
		if err != nil {
			return errors.Wrap(err, "failed to get candidate rewards map")
		}
		for candidateName, rewards := range candidateRewardsMap {
			valStrs = append(valStrs, "(?, ?, CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)), CAST(? as DECIMAL(65, 0)))")
			valArgs = append(valArgs, epochNumber, candidateName, rewards[0], rewards[1], rewards[2])
		}
	}

	if len(valStrs) == 0 {
		return nil
	}
	insertQuery := fmt.Sprintf(insertAccountReward, AccountRewardTableName, strings.Join(valStrs, ","))
	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) getVotingInfo(tx *sql.Tx, lastEpoch uint64) (map[string][]string, map[string]*big.Int, error) {
	// get voting results
	getQuery := fmt.Sprintf(selectVotingResult, votings.VotingResultTableName)
	rows, err := tx.Query(getQuery, lastEpoch)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get voting result query")
	}

	var votingResult votings.VotingResult
	parsedRows, err := s.ParseSQLRows(rows, &votingResult)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, nil, indexprotocol.ErrNotExist
	}
	rewardAddrToNameMapping := make(map[string][]string)
	weightedVotesMapping := make(map[string]*big.Int)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*votings.VotingResult)
		if _, ok := rewardAddrToNameMapping[voting.RewardAddress]; !ok {
			rewardAddrToNameMapping[voting.RewardAddress] = make([]string, 0)
		}
		rewardAddrToNameMapping[voting.RewardAddress] = append(rewardAddrToNameMapping[voting.RewardAddress], voting.DelegateName)

		totalWeightedVotes, err := stringToBigInt(voting.TotalWeightedVotes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to covert string to big int")
		}
		weightedVotesMapping[voting.DelegateName] = totalWeightedVotes
	}
	return rewardAddrToNameMapping, weightedVotesMapping, nil
}

func (p *Protocol) getProductivity(epochNumber uint64) (map[string]*Productivity, error) {
	// get voting results
	getQuery := fmt.Sprintf(selectBlockHistory, blocks.BlockHistoryTableName, blocks.BlockHistoryTableName)
	db := p.Store.GetDB()
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, epochNumber)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var productivity blocks.ProductivityHistory
	parsedRows, err := s.ParseSQLRows(rows, &productivity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	productivityMap := make(map[string]*Productivity)
	for _, parsedRow := range parsedRows {
		p := parsedRow.(*blocks.ProductivityHistory)
		productivityMap[p.ProducerName] = &Productivity{
			Production:         p.Production,
			ExpectedProduction: p.ExpectedProduction,
		}
	}
	return productivityMap, nil
}

func (p *Protocol) breakdownRewards(
	epochNumber uint64,
	candidateNames []string,
	weightedVotesMap map[string]*big.Int,
	exemptionMap map[string]bool,
	totalBlockReward *big.Int,
	totalEpochReward *big.Int,
	totalFoundationBonus *big.Int,
) (map[string][]string, error) {
	candidateVoteList := make([]*CandidateVote, 0, len(weightedVotesMap))
	for name, votes := range weightedVotesMap {
		candidateVoteList = append(candidateVoteList, &CandidateVote{
			CandidateName:      name,
			TotalWeightedVotes: votes,
		})
	}
	// Sort list by votes in decreasing order
	sort.Slice(candidateVoteList, func(i, j int) bool {
		return candidateVoteList[i].TotalWeightedVotes.Cmp(candidateVoteList[j].TotalWeightedVotes) == 1
	})
	candidateRank := make(map[string]uint64)
	for i, candidateVote := range candidateVoteList {
		candidateRank[candidateVote.CandidateName] = uint64(i + 1)
	}
	productivityMap, err := p.getProductivity(epochNumber)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get productivity map")
	}
	productionSum := big.NewInt(0)
	qualifiedTotalVotes := big.NewInt(0)
	foundationBonusCount := big.NewInt(0)
	earnBlockReward := make(map[string]bool)
	earnEpochReward := make(map[string]bool)
	earnFoundationBonus := make(map[string]bool)
	for _, candidateName := range candidateNames {
		productive := true
		if productivity, ok := productivityMap[candidateName]; ok {
			productionSum.Add(productionSum, big.NewInt(int64(productivity.Production)))
			earnBlockReward[candidateName] = true
			if productivity.Production*100/productivity.ExpectedProduction < p.RewardConfig.ProductivityThreshold {
				productive = false
			}
		}
		// qualify for epoch reward
		if candidateRank[candidateName] <= p.RewardConfig.NumDelegatesForEpochReward && !exemptionMap[candidateName] && productive {
			qualifiedTotalVotes.Add(qualifiedTotalVotes, weightedVotesMap[candidateName])
			earnEpochReward[candidateName] = true
		}
		// qualify for foundation bonus
		if candidateRank[candidateName] <= p.RewardConfig.NumDelegatesForFoundationBonus && !exemptionMap[candidateName] {
			foundationBonusCount.Add(foundationBonusCount, big.NewInt(1))
			earnFoundationBonus[candidateName] = true
		}
	}
	candidateRewardsMap := make(map[string][]string)
	for _, candidateName := range candidateNames {
		blockReward := big.NewInt(0)
		epochReward := big.NewInt(0)
		foundationBonus := big.NewInt(0)
		if productionSum.Sign() > 0 && earnBlockReward[candidateName] {
			production := big.NewInt(0).SetUint64(productivityMap[candidateName].Production)
			blockReward = big.NewInt(0).Div(big.NewInt(0).Mul(totalBlockReward, production), productionSum)
		}
		if qualifiedTotalVotes.Sign() > 0 && earnEpochReward[candidateName] {
			epochReward = big.NewInt(0).Div(big.NewInt(0).Mul(totalEpochReward, weightedVotesMap[candidateName]), qualifiedTotalVotes)
		}
		if totalFoundationBonus.Sign() > 0 && earnFoundationBonus[candidateName] {
			foundationBonus = big.NewInt(0).Div(totalFoundationBonus, foundationBonusCount)
		}

		if blockReward.Sign() == 0 && epochReward.Sign() == 0 && foundationBonus.Sign() == 0 {
			continue
		}
		candidateRewardsMap[candidateName] = []string{blockReward.String(), epochReward.String(), foundationBonus.String()}
	}
	return candidateRewardsMap, nil
}

// stringToBigInt transforms a string to big int
func stringToBigInt(estr string) (*big.Int, error) {
	ret, ok := big.NewInt(0).SetString(estr, 10)
	if !ok {
		return nil, errors.New("failed to parse string to big int")
	}
	return ret, nil
}
