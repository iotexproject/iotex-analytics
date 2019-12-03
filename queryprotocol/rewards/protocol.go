// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

import (
	"database/sql"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/util"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	rowExist               = "SELECT * FROM %s WHERE epoch_number >= ? and epoch_number <= ? and candidate_name = ?"
	selectAccountRewardSum = "SELECT SUM(block_reward), SUM(epoch_reward), SUM(foundation_bonus) FROM %s " +
		"WHERE epoch_number >= %d  AND epoch_number <= %d AND candidate_name=?"
	selectVotingResult  = "SELECT epoch_number, total_weighted_votes FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND delegate_name = ?"
	selectAccountReward = "SELECT epoch_number, epoch_reward, foundation_bonus FROM %s " +
		"WHERE epoch_number >= ?  AND epoch_number <= ? AND candidate_name= ? "
	selectAggregateVoting    = "SELECT * FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND candidate_name=?"
	selectAccountRewardIn    = "SELECT * FROM %s WHERE (epoch_number, candidate_name) IN (%s)"
	selectVotingResultAll    = "SELECT * FROM %s WHERE epoch_number >= ?  AND epoch_number <= ? AND reward_address= ? "
	selectAggregateVotingIn  = "SELECT * FROM %s WHERE (epoch_number, candidate_name) IN (%s)"
	selectAggregateVotingAll = "SELECT * FROM %s WHERE epoch_number >= ?  AND epoch_number <= ? AND voter_address= ? "
	selectVotingResultIn     = "SELECT * FROM %s WHERE (epoch_number, delegate_name) IN (%s)"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// VotingInfo defines voting info
type VotingInfo struct {
	EpochNumber   uint64
	VoterAddress  string
	WeightedVotes string
}

// RewardDistribution defines reward distribute info
type RewardDistribution struct {
	VoterEthAddress   string
	VoterIotexAddress string
	Amount            string
}

// DelegateHermesDistribution defines the Hermes reward distributions for each delegate
type DelegateHermesDistribution struct {
	DelegateName        string
	Distributions       []*RewardDistribution
	StakingIotexAddress string
	VoterCount          uint64
	WaiveServiceFee     bool
	Refund              string
}

// DelegateAmount defines delegate associated with an amount
type DelegateAmount struct {
	DelegateName string
	Amount       string
}

// TotalWeight defines a delegate's total weighted votes
type TotalWeight struct {
	EpochNumber uint64
	TotalWeight string
}

// EpochFoundationReward defines a delegate's epoch reward and foundation bonus
type EpochFoundationReward struct {
	EpochNumber     uint64
	EpochReward     string
	FoundationBonus string
}

// HermesDistributionPlan defines the distribution plan of delegates registering in Hermes
type HermesDistributionPlan struct {
	TotalWeightedVotes        *big.Int
	StakingAddress            string
	BlockRewardPercentage     uint64
	EpochRewardPercentage     uint64
	FoundationBonusPercentage uint64
}

// HermesDistributionSource defines the distribution source of delegates registering in Hermes
type HermesDistributionSource struct {
	BlockReward     *big.Int
	EpochReward     *big.Int
	FoundationBonus *big.Int
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetAccountReward gets account reward
func (p *Protocol) GetAccountReward(startEpoch uint64, epochCount uint64, candidateName string) (string, string, string, error) {
	if _, ok := p.indexer.Registry.Find(rewards.ProtocolID); !ok {
		return "", "", "", errors.New("rewards protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	endEpoch := startEpoch + epochCount - 1
	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf(rowExist,
		rewards.AccountRewardTableName), startEpoch, endEpoch, candidateName)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return "", "", "", indexprotocol.ErrNotExist
	}

	getQuery := fmt.Sprintf(selectAccountRewardSum, rewards.AccountRewardTableName, startEpoch, endEpoch)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	var blockReward, epochReward, foundationBonus string
	if err = stmt.QueryRow(candidateName).Scan(&blockReward, &epochReward, &foundationBonus); err != nil {
		return "", "", "", errors.Wrap(err, "failed to execute get query")
	}
	return blockReward, epochReward, foundationBonus, nil
}

// GetBookkeeping gets reward distribution info
func (p *Protocol) GetBookkeeping(startEpoch uint64, epochCount uint64, delegateName string, percentage int, includeFoundationBonus bool) ([]*RewardDistribution, error) {
	endEpoch := startEpoch + epochCount - 1

	distrRewardMap, err := p.rewardsToSplit(startEpoch, endEpoch, delegateName, percentage, includeFoundationBonus)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reward distribution map")
	}
	delegateTotalVotesMap, err := p.totalWeightedVotes(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get delegate total weighted votes")
	}
	epochToVotersMap, err := p.voterVotes(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get voters map")
	}

	voterAddrToReward := make(map[string]*big.Int)
	for epoch, distrReward := range distrRewardMap {
		totalWeightedVotes, ok := delegateTotalVotesMap[epoch]
		if !ok {
			return nil, errors.Errorf("Missing delegate total weighted votes information on epoch %d", epoch)
		}
		if totalWeightedVotes.Sign() == 0 {
			continue
		}
		votersInfo, ok := epochToVotersMap[epoch]
		if !ok {
			return nil, errors.Errorf("Missing voters' weighted votes information on epoch %d", epoch)
		}
		for voterAddr, weightedVotes := range votersInfo {
			amount := new(big.Int).Set(distrReward)
			amount = amount.Mul(amount, weightedVotes).Div(amount, totalWeightedVotes)
			if _, ok := voterAddrToReward[voterAddr]; !ok {
				voterAddrToReward[voterAddr] = big.NewInt(0)
			}
			voterAddrToReward[voterAddr].Add(voterAddrToReward[voterAddr], amount)
		}
	}
	return convertVoterDistributionMapToList(voterAddrToReward)
}

// GetHermesBookkeeping gets reward distribution info and delegate metadata for all delegates who register Hermes
func (p *Protocol) GetHermesBookkeeping(startEpoch uint64, epochCount uint64, rewardAddress string, waiverThreshold uint64) ([]*DelegateHermesDistribution, error) {
	endEpoch := startEpoch + epochCount - 1

	distributePlanMap, err := p.distributionPlanByRewardAddress(startEpoch, endEpoch, rewardAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reward distribution plan")
	}

	// Form search column pairs
	searchPairs := make([]string, 0)
	for delegateName, planMap := range distributePlanMap {
		for epochNumber := range planMap {
			searchPairs = append(searchPairs, fmt.Sprintf("(%d, '%s')", epochNumber, delegateName))
		}
	}
	accountRewardsMap, err := p.accountRewards(searchPairs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account rewards")
	}
	voterVotesMap, err := p.weightedVotesBySearchPairs(searchPairs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get voter votes")
	}

	hermesDistributions := make([]*DelegateHermesDistribution, 0, len(accountRewardsMap))
	for delegate, rewardsMap := range accountRewardsMap {
		planMap := distributePlanMap[delegate]
		epochVoterMap := voterVotesMap[delegate]

		voterAddrToReward := make(map[string]*big.Int)
		balanceAfterDistribution := big.NewInt(0)
		voterCountMap := make(map[string]bool)
		feeWaiver := true
		var stakingAddress string

		for epoch, rewards := range rewardsMap {
			distributePlan := planMap[epoch]
			voterMap := epochVoterMap[epoch]

			if stakingAddress == "" {
				ethStakingAddress := common.HexToAddress(distributePlan.StakingAddress)
				ioStakingAddress, err := address.FromBytes(ethStakingAddress.Bytes())
				if err != nil {
					return nil, errors.New("failed to form IoTeX address from ETH address")
				}
				stakingAddress = ioStakingAddress.String()
			}

			totalRewards := new(big.Int).Set(rewards.BlockReward)
			totalRewards.Add(totalRewards, rewards.EpochReward).Add(totalRewards, rewards.FoundationBonus)
			balanceAfterDistribution.Add(balanceAfterDistribution, totalRewards)

			distrReward := big.NewInt(0)
			if distributePlan.BlockRewardPercentage > 0 {
				distrBlockReward := new(big.Int).Set(rewards.BlockReward)
				distrBlockReward.Mul(distrBlockReward, big.NewInt(int64(distributePlan.BlockRewardPercentage))).Div(distrBlockReward, big.NewInt(100))
				distrReward.Add(distrReward, distrBlockReward)
			}
			if distributePlan.EpochRewardPercentage > 0 {
				distrEpochReward := new(big.Int).Set(rewards.EpochReward)
				distrEpochReward.Mul(distrEpochReward, big.NewInt(int64(distributePlan.EpochRewardPercentage))).Div(distrEpochReward, big.NewInt(100))
				distrReward.Add(distrReward, distrEpochReward)
			}
			if distributePlan.FoundationBonusPercentage > 0 {
				distrFoundationBonus := new(big.Int).Set(rewards.FoundationBonus)
				distrFoundationBonus.Mul(distrFoundationBonus, big.NewInt(int64(distributePlan.FoundationBonusPercentage))).Div(distrFoundationBonus, big.NewInt(100))
				distrReward.Add(distrReward, distrFoundationBonus)
			}

			if distributePlan.BlockRewardPercentage < waiverThreshold || distributePlan.EpochRewardPercentage < waiverThreshold ||
				distributePlan.FoundationBonusPercentage < waiverThreshold {
				feeWaiver = false
			}

			for voterAddr, weightedVotes := range voterMap {
				amount := new(big.Int).Set(distrReward)
				amount = amount.Mul(amount, weightedVotes).Div(amount, distributePlan.TotalWeightedVotes)
				if _, ok := voterAddrToReward[voterAddr]; !ok {
					voterAddrToReward[voterAddr] = big.NewInt(0)
				}
				voterAddrToReward[voterAddr].Add(voterAddrToReward[voterAddr], amount)
				balanceAfterDistribution.Sub(balanceAfterDistribution, amount)
				voterCountMap[voterAddr] = true
			}
		}
		rewardDistribution, err := convertVoterDistributionMapToList(voterAddrToReward)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert voter distribution map to list")
		}
		hermesDistributions = append(hermesDistributions, &DelegateHermesDistribution{
			DelegateName:        delegate,
			Distributions:       rewardDistribution,
			StakingIotexAddress: stakingAddress,
			VoterCount:          uint64(len(voterCountMap)),
			WaiveServiceFee:     feeWaiver,
			Refund:              balanceAfterDistribution.String(),
		})
	}
	return hermesDistributions, nil
}

// GetRewardSources gets reward sources given a voter's IoTeX address
func (p *Protocol) GetRewardSources(startEpoch uint64, epochCount uint64, voterIotexAddress string) ([]*DelegateAmount, error) {
	voterEthAddress, err := util.IoAddrToEvmAddr(voterIotexAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert IoTeX address to ETH address")
	}
	hexAddress := voterEthAddress.String()
	endEpoch := startEpoch + epochCount - 1

	weightedVotesMap, err := p.weightedVotesByVoterAddress(startEpoch, endEpoch, hexAddress[2:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to get voter's weighted votes")
	}

	// Form search column pairs
	searchPairs := make([]string, 0)
	for delegateName, epochMap := range weightedVotesMap {
		for epochNumber := range epochMap {
			searchPairs = append(searchPairs, fmt.Sprintf("(%d, '%s')", epochNumber, delegateName))
		}
	}
	accountRewardsMap, err := p.accountRewards(searchPairs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account rewards")
	}
	distributePlanMap, err := p.distributionPlanBySearchPairs(searchPairs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reward distribution plan")
	}

	delegateDistributionMap := make(map[string]*big.Int)
	for delegate, rewardsMap := range accountRewardsMap {
		planMap := distributePlanMap[delegate]
		delegateMap := weightedVotesMap[delegate]
		delegateDistributionMap[delegate] = big.NewInt(0)

		for epoch, rewards := range rewardsMap {
			distributePlan := planMap[epoch]

			distrReward := big.NewInt(0)
			if distributePlan.BlockRewardPercentage > 0 {
				distrBlockReward := new(big.Int).Set(rewards.BlockReward)
				distrBlockReward.Mul(distrBlockReward, big.NewInt(int64(distributePlan.BlockRewardPercentage))).Div(distrBlockReward, big.NewInt(100))
				distrReward.Add(distrReward, distrBlockReward)
			}
			if distributePlan.EpochRewardPercentage > 0 {
				distrEpochReward := new(big.Int).Set(rewards.EpochReward)
				distrEpochReward.Mul(distrEpochReward, big.NewInt(int64(distributePlan.EpochRewardPercentage))).Div(distrEpochReward, big.NewInt(100))
				distrReward.Add(distrReward, distrEpochReward)
			}
			if distributePlan.FoundationBonusPercentage > 0 {
				distrFoundationBonus := new(big.Int).Set(rewards.FoundationBonus)
				distrFoundationBonus.Mul(distrFoundationBonus, big.NewInt(int64(distributePlan.FoundationBonusPercentage))).Div(distrFoundationBonus, big.NewInt(100))
				distrReward.Add(distrReward, distrFoundationBonus)
			}

			weightedVotes := delegateMap[epoch]
			amount := distrReward.Mul(distrReward, weightedVotes).Div(distrReward, distributePlan.TotalWeightedVotes)

			delegateDistributionMap[delegate].Add(delegateDistributionMap[delegate], amount)
		}
	}

	delegateDistributions := make([]*DelegateAmount, 0)
	for delegateName, amount := range delegateDistributionMap {
		delegateDistributions = append(delegateDistributions, &DelegateAmount{
			DelegateName: delegateName,
			Amount:       amount.String(),
		})
	}
	return delegateDistributions, nil
}

// totalWeightedVotes gets the given delegate's total weighted votes from start epoch to end epoch
func (p *Protocol) totalWeightedVotes(startEpoch uint64, endEpoch uint64, delegateName string) (map[uint64]*big.Int, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectVotingResult,
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var tw TotalWeight
	parsedRows, err := s.ParseSQLRows(rows, &tw)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	totalVotesMap := make(map[uint64]*big.Int)
	for _, parsedRow := range parsedRows {
		votes := parsedRow.(*TotalWeight)
		bigIntVotes, err := stringToBigInt(votes.TotalWeight)
		if err != nil {
			return nil, errors.New("failed to covert string to big int")
		}
		totalVotesMap[votes.EpochNumber] = bigIntVotes
	}
	return totalVotesMap, nil
}

// rewardToSplit gets the reward to split from the given delegate from start epoch to end epoch
func (p *Protocol) rewardsToSplit(startEpoch uint64, endEpoch uint64, delegateName string, percentage int, includeFoundationBonus bool) (map[uint64]*big.Int, error) {
	if _, ok := p.indexer.Registry.Find(rewards.ProtocolID); !ok {
		return nil, errors.New("rewards protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectAccountReward, rewards.AccountRewardTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var efr EpochFoundationReward
	parsedRows, err := s.ParseSQLRows(rows, &efr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	distrRewardMap := make(map[uint64]*big.Int)
	for _, parsedRow := range parsedRows {
		rewards := parsedRow.(*EpochFoundationReward)
		distrReward, err := stringToBigInt(rewards.EpochReward)
		if err != nil {
			return nil, errors.New("failed to covert string to big int")
		}
		if includeFoundationBonus {
			foundationBonus, err := stringToBigInt(rewards.FoundationBonus)
			if err != nil {
				return nil, errors.New("failed to covert string to big int")
			}
			distrReward.Add(distrReward, foundationBonus)
		}
		distrRewardMap[rewards.EpochNumber] = distrReward.Mul(distrReward, big.NewInt(int64(percentage))).Div(distrReward, big.NewInt(100))
	}
	return distrRewardMap, nil
}

// voterVotes gets voters' address and weighted votes for the given delegate from start epoch to end epoch
func (p *Protocol) voterVotes(startEpoch uint64, endEpoch uint64, delegateName string) (map[uint64]map[string]*big.Int, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectAggregateVoting,
		votings.AggregateVotingTable)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var aggregateVoting votings.AggregateVoting
	parsedRows, err := s.ParseSQLRows(rows, &aggregateVoting)

	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	epochToVoters := make(map[uint64]map[string]*big.Int)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*votings.AggregateVoting)

		if _, ok := epochToVoters[voting.EpochNumber]; !ok {
			epochToVoters[voting.EpochNumber] = make(map[string]*big.Int)
		}
		weightedVotesInt, errs := stringToBigInt(voting.AggregateVotes)
		if errs != nil {
			return nil, errors.Wrap(errs, "failed to convert to big int")

		}
		if val, ok := epochToVoters[voting.EpochNumber][voting.VoterAddress]; !ok {
			epochToVoters[voting.EpochNumber][voting.VoterAddress] = weightedVotesInt
		} else {
			val.Add(val, weightedVotesInt)
		}
	}

	return epochToVoters, nil
}

// accountRewards gets the reward information for the delegates in the search pairs
func (p *Protocol) accountRewards(searchPairs []string) (map[string]map[uint64]*HermesDistributionSource, error) {
	if _, ok := p.indexer.Registry.Find(rewards.ProtocolID); !ok {
		return nil, errors.New("rewards protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectAccountRewardIn,
		rewards.AccountRewardTableName, strings.Join(searchPairs, ","))
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var accountReward rewards.AccountReward
	parsedRows, err := s.ParseSQLRows(rows, &accountReward)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	accountRewardsMap := make(map[string]map[uint64]*HermesDistributionSource)
	for _, parsedRow := range parsedRows {
		rewards := parsedRow.(*rewards.AccountReward)
		if _, ok := accountRewardsMap[rewards.CandidateName]; !ok {
			accountRewardsMap[rewards.CandidateName] = make(map[uint64]*HermesDistributionSource, 0)
		}
		rewardsMap := accountRewardsMap[rewards.CandidateName]
		blockReward, err := stringToBigInt(rewards.BlockReward)
		if err != nil {
			return nil, errors.New("failed to covert string to big int")
		}
		epochReward, err := stringToBigInt(rewards.EpochReward)
		if err != nil {
			return nil, errors.New("failed to covert string to big int")
		}
		foundationBonus, err := stringToBigInt(rewards.FoundationBonus)
		if err != nil {
			return nil, errors.New("failed to covert string to big int")
		}
		rewardsMap[rewards.EpochNumber] = &HermesDistributionSource{
			BlockReward:     blockReward,
			EpochReward:     epochReward,
			FoundationBonus: foundationBonus,
		}
	}
	return accountRewardsMap, nil
}

// distributionPlanByRewardAddress gets delegates' reward distribution plan by reward address
func (p *Protocol) distributionPlanByRewardAddress(startEpoch uint64, endEpoch uint64, rewardAddress string) (map[string]map[uint64]*HermesDistributionPlan, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("rewards protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectVotingResultAll,
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, rewardAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}
	return parseDistributionPlanFromVotingResult(rows)
}

// weightedVotesBySearchPairs gets voters' address and weighted votes for delegates in the search pairs
func (p *Protocol) weightedVotesBySearchPairs(searchPairs []string) (map[string]map[uint64]map[string]*big.Int, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectAggregateVotingIn,
		votings.AggregateVotingTable, strings.Join(searchPairs, ","))
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var aggregateVoting votings.AggregateVoting
	parsedRows, err := s.ParseSQLRows(rows, &aggregateVoting)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	voterVotesMap := make(map[string]map[uint64]map[string]*big.Int)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*votings.AggregateVoting)
		if _, ok := voterVotesMap[voting.CandidateName]; !ok {
			voterVotesMap[voting.CandidateName] = make(map[uint64]map[string]*big.Int)
		}
		epochVoterMap := voterVotesMap[voting.CandidateName]
		if _, ok := epochVoterMap[voting.EpochNumber]; !ok {
			epochVoterMap[voting.EpochNumber] = make(map[string]*big.Int)
		}
		voterMap := epochVoterMap[voting.EpochNumber]

		weightedVotesInt, errs := stringToBigInt(voting.AggregateVotes)
		if errs != nil {
			return nil, errors.Wrap(errs, "failed to convert to big int")
		}
		if val, ok := voterMap[voting.VoterAddress]; !ok {
			voterMap[voting.VoterAddress] = weightedVotesInt
		} else {
			val.Add(val, weightedVotesInt)
		}
	}
	return voterVotesMap, nil
}

// weightedVotesByVoterAddress gets voter's weighted votes for delegates by voter's address
func (p *Protocol) weightedVotesByVoterAddress(startEpoch uint64, endEpoch uint64, voterEthAddress string) (map[string]map[uint64]*big.Int, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("rewards protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectAggregateVotingAll,
		votings.AggregateVotingTable)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, voterEthAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var aggregateVoting votings.AggregateVoting
	parsedRows, err := s.ParseSQLRows(rows, &aggregateVoting)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	weightedVotesMap := make(map[string]map[uint64]*big.Int)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*votings.AggregateVoting)
		if _, ok := weightedVotesMap[voting.CandidateName]; !ok {
			weightedVotesMap[voting.CandidateName] = make(map[uint64]*big.Int)
		}
		weightedVotesInt, errs := stringToBigInt(voting.AggregateVotes)
		if errs != nil {
			return nil, errors.Wrap(errs, "failed to convert to big int")

		}
		if val, ok := weightedVotesMap[voting.CandidateName][voting.EpochNumber]; !ok {
			weightedVotesMap[voting.CandidateName][voting.EpochNumber] = weightedVotesInt
		} else {
			val.Add(val, weightedVotesInt)
		}
	}
	return weightedVotesMap, nil
}

// distributionPlanBySearchPairs gets delegates' reward distribution plan in the search pairs
func (p *Protocol) distributionPlanBySearchPairs(searchPairs []string) (map[string]map[uint64]*HermesDistributionPlan, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectVotingResultIn,
		votings.VotingResultTableName, strings.Join(searchPairs, ","))
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}
	return parseDistributionPlanFromVotingResult(rows)
}

// convertVoterDistributionMapToList converts voter reward distribution map to list
func convertVoterDistributionMapToList(voterAddrToReward map[string]*big.Int) ([]*RewardDistribution, error) {
	rewardDistribution := make([]*RewardDistribution, 0)
	for voterAddr, rewardAmount := range voterAddrToReward {
		ethAddress := common.HexToAddress(voterAddr)
		ioAddress, err := address.FromBytes(ethAddress.Bytes())
		if err != nil {
			return nil, errors.New("failed to form IoTeX address from ETH address")
		}
		rewardDistribution = append(rewardDistribution, &RewardDistribution{
			VoterEthAddress:   voterAddr,
			VoterIotexAddress: ioAddress.String(),
			Amount:            rewardAmount.String(),
		})
	}
	return rewardDistribution, nil
}

// parseDistributionPlanFromVotingResult parses distribution plan from raw data of voting result
func parseDistributionPlanFromVotingResult(rows *sql.Rows) (map[string]map[uint64]*HermesDistributionPlan, error) {
	var votingResult votings.VotingResult
	parsedRows, err := s.ParseSQLRows(rows, &votingResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	distributePlanMap := make(map[string]map[uint64]*HermesDistributionPlan)
	for _, parsedRow := range parsedRows {
		result := parsedRow.(*votings.VotingResult)
		if _, ok := distributePlanMap[result.DelegateName]; !ok {
			distributePlanMap[result.DelegateName] = make(map[uint64]*HermesDistributionPlan)
		}
		planMap := distributePlanMap[result.DelegateName]
		totalWeightedVotes, err := stringToBigInt(result.TotalWeightedVotes)
		if err != nil {
			return nil, errors.New("failed to convert string to big int")
		}
		planMap[result.EpochNumber] = &HermesDistributionPlan{
			BlockRewardPercentage:     result.BlockRewardPercentage,
			EpochRewardPercentage:     result.EpochRewardPercentage,
			FoundationBonusPercentage: result.FoundationBonusPercentage,
			StakingAddress:            result.StakingAddress,
			TotalWeightedVotes:        totalWeightedVotes,
		}
	}
	return distributePlanMap, nil
}

// stringToBigInt transforms a string to big int
func stringToBigInt(estr string) (*big.Int, error) {
	ret, ok := big.NewInt(0).SetString(estr, 10)
	if !ok {
		return nil, errors.New("failed to parse string to big int")
	}
	return ret, nil
}
