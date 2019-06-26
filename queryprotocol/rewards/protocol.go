// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	qvotings "github.com/iotexproject/iotex-analytics/queryprotocol/votings"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// RewardDistribution defines reward distribute info
type RewardDistribution struct {
	VoterEthAddress   string
	VoterIotexAddress string
	Amount            string
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
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number >= ? and epoch_number <= ? and candidate_name = ?",
		rewards.AccountRewardTableName), startEpoch, endEpoch, candidateName)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return "", "", "", indexprotocol.ErrNotExist
	}

	getQuery := fmt.Sprintf("SELECT SUM(block_reward), SUM(epoch_reward), SUM(foundation_bonus) FROM %s "+
		"WHERE epoch_number >= %d  AND epoch_number <= %d AND candidate_name=?", rewards.AccountRewardTableName, startEpoch, endEpoch)
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

// GetBookkeeping get reward distribution info
func (p *Protocol) GetBookkeeping(startEpoch uint64, epochCount uint64, delegateName string, percentage int, includeFoundationBonus bool) ([]*RewardDistribution, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")
	}

	endEpoch := startEpoch + epochCount - 1
	// Check existence
	db := p.indexer.Store.GetDB()
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number >= ? and epoch_number <= ? and delegate_name = ?",
		votings.VotingResultTableName), startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return nil, indexprotocol.ErrNotExist
	}

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

// totalWeightedVotes gets the given delegate's total weighted votes from start epoch to end epoch
func (p *Protocol) totalWeightedVotes(startEpoch uint64, endEpoch uint64, delegateName string) (map[uint64]*big.Int, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT epoch_number, total_weighted_votes FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND delegate_name = ?",
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
		if bigIntVotes.Sign() == 0 {
			return nil, errors.New("total votes is 0")
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

	getQuery := fmt.Sprintf("SELECT epoch_number, epoch_reward, foundation_bonus FROM %s "+
		"WHERE epoch_number >= ?  AND epoch_number <= ? AND candidate_name= ? ", rewards.AccountRewardTableName)
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
	getQuery := fmt.Sprintf("SELECT epoch_number, voter_address, aggregate_votes FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND candidate_name=?",
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

	var votingHistory qvotings.VotingInfo
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	epochToVoters := make(map[uint64]map[string]*big.Int)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*qvotings.VotingInfo)
		if _, ok := epochToVoters[voting.EpochNumber]; !ok {
			epochToVoters[voting.EpochNumber] = make(map[string]*big.Int)
		}
		weightedVotesInt, errs := stringToBigInt(voting.WeightedVotes)
		if errs != nil {
			return nil, errors.Wrap(errs, "failed to convert to big int")

		}
		epochToVoters[voting.EpochNumber][voting.VoterAddress] = weightedVotesInt
	}

	return epochToVoters, nil
}

func stringToBigInt(estr string) (ret *big.Int, err error) {
	// convert string like this:2.687455198114428e+21
	retFloat, _, err := new(big.Float).Parse(estr, 10)
	if err != nil {
		err = errors.Wrap(err, "failed to parse string to big float")
		return
	}
	ret = new(big.Int)
	retFloat.Int(ret)
	return
}
