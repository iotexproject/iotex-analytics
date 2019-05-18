// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// RewardDistribution defines reward distribute info
type RewardDistribution struct {
	VoterAddress string `json:"voterAddress"`
	Amount       string `json:"amount"`
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

	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number = ? and candidate_name = ?",
		rewards.AccountRewardViewName), startEpoch, candidateName)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return "", "", "", indexprotocol.ErrNotExist
	}

	endEpoch := startEpoch + epochCount - 1

	getQuery := fmt.Sprintf("SELECT SUM(block_reward), SUM(epoch_reward), SUM(foundation_bonus) FROM %s "+
		"WHERE epoch_number >= %d  AND epoch_number <= %d AND candidate_name=?", rewards.AccountRewardViewName, startEpoch, endEpoch)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to prepare get query")
	}

	var blockReward, epochReward, foundationBonus string
	if err = stmt.QueryRow(candidateName).Scan(&blockReward, &epochReward, &foundationBonus); err != nil {
		return "", "", "", errors.Wrap(err, "failed to execute get query")
	}
	return blockReward, epochReward, foundationBonus, nil
}

// GetBookkeeping get reward distribution info
func (p *Protocol) GetBookkeeping(startEpoch int, epochCount int, delegateName string, percentage int, includeFoundationBonus bool) (rds []*RewardDistribution, err error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		err = errors.New("votings protocol is unregistered")
		return
	}
	// First get sum of reward pool from startEpoch to startEpoch+epochCount-1
	rewardToSplit, err := p.sumOfRewardPool(startEpoch, epochCount, delegateName, percentage, includeFoundationBonus)
	if err != nil {
		return
	}
	fmt.Println("reward to split:", rewardToSplit.Text(10))
	// Second get TotalWeightedVotes
	sumTotalWeightedVotes, err := p.totalWeightedVotes(startEpoch, epochCount, delegateName)
	if err != nil {
		return
	}
	fmt.Println("sum total weightedVotes:", sumTotalWeightedVotes.Text(10))
	// get voter's weighted votes
	voteSums, err := p.voterVotes(startEpoch, epochCount, delegateName)
	if err != nil {
		return
	}
	for _, voteSum := range voteSums {
		rewardInt, errs := stringToBigInt(voteSum.Amount)
		if errs != nil {
			err = errors.Wrap(errs, "reward convert to int error")
			return
		}
		fmt.Println(voteSum.VoterAddress, ":", voteSum.Amount)
		amount := new(big.Int).Set(rewardToSplit)
		amount = amount.Mul(amount, rewardInt).Div(amount, sumTotalWeightedVotes)
		v := &RewardDistribution{
			VoterAddress: voteSum.VoterAddress,
			Amount:       amount.Text(10),
		}
		fmt.Println("cal:", v)
		rds = append(rds, v)
	}
	return
}
func (p *Protocol) totalWeightedVotes(startEpoch int, epochCount int, delegateName string) (sumVotesInt *big.Int, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT SUM(total_weighted_votes) FROM %s WHERE epoch_number>=? AND epoch_number<? AND delegate_name=?",
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var sumVotes string
	if err = stmt.QueryRow(startEpoch, startEpoch+epochCount, delegateName).Scan(&sumVotes); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	sumVotesInt, err = stringToBigInt(sumVotes)
	if err != nil {
		err = errors.New("reward convert to int error")
		return
	}
	if sumVotesInt.Cmp(big.NewInt(0)) == 0 {
		err = errors.New("sum of votes is 0")
		return
	}
	return
}
func (p *Protocol) sumOfRewardPool(startEpoch int, epochCount int, delegateName string, percentage int, includeFoundationBonus bool) (rewardToSplit *big.Int, err error) {
	_, epochReward, foundationBonus, err := p.GetAccountReward(uint64(startEpoch), uint64(epochCount), delegateName)
	if err != nil {
		err = errors.Wrap(err, "GetAccountReward err")
		return
	}
	epochRewardInt, err := stringToBigInt(epochReward)
	if err != nil {
		return
	}
	foundationBonusInt, err := stringToBigInt(foundationBonus)
	if err != nil {
		return
	}
	fmt.Println("epoch reward:", epochRewardInt)
	fmt.Println("foundation bonus:", foundationBonusInt)
	rewardInt := epochRewardInt
	if includeFoundationBonus {
		rewardInt = epochRewardInt.Add(epochRewardInt, foundationBonusInt)
	}
	rewardToSplit = rewardInt.Mul(rewardInt, big.NewInt(int64(percentage))).Div(rewardInt, big.NewInt(100))
	if rewardToSplit.Cmp(big.NewInt(0)) == 0 {
		err = errors.New("reward is 0")
		return
	}
	return
}

// get voter's weighted votes
func (p *Protocol) voterVotes(startEpoch int, epochCount int, delegateName string) (votingSums []*RewardDistribution, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT reward_address,SUM(total_weighted_votes) FROM %s WHERE epoch_number>=? AND epoch_number<? AND delegate_name=?  GROUP BY reward_address",
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}

	rows, err := stmt.Query(startEpoch, startEpoch+epochCount, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var rd RewardDistribution
	parsedRows, err := s.ParseSQLRows(rows, &rd)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}

	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}

	for _, parsedRow := range parsedRows {
		rd := parsedRow.(*RewardDistribution)
		votingSums = append(votingSums, rd)
	}
	return
}
func stringToBigInt(estr string) (ret *big.Int, err error) {
	// convert string like this:2.687455198114428e+21
	retFloat, _, err := new(big.Float).Parse(estr, 10)
	if err != nil {
		err = errors.Wrap(err, "stringToBigInt err")
		return
	}
	ret = new(big.Int)
	retFloat.Int(ret)
	return
}
