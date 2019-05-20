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
	qvotings "github.com/iotexproject/iotex-analytics/queryprotocol/votings"
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

type totalWeight struct {
	TotalWeight string
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
	currentEpoch, err := p.getMostRecentEpoch()
	if err != nil {
		err = errors.New("failed to get most recent epoch")
		return
	}
	endEpoch := startEpoch + epochCount - 1
	if endEpoch > currentEpoch {
		endEpoch = currentEpoch
	}
	votersSum := make(map[string]*big.Int, 0)
	for i := startEpoch; i <= endEpoch; i++ {
		rd, errs := p.getBookkeeping(i, delegateName, percentage, includeFoundationBonus)
		if errs != nil {
			err = errors.Wrap(errs, "failed to get bookkeeping")
			return
		}
		for k, v := range rd {
			existBig, ok := votersSum[k]
			if ok {
				existBig.Add(existBig, v)
			} else {
				votersSum[k] = v
			}
		}
	}
	for k, v := range votersSum {
		rd := &RewardDistribution{
			VoterAddress: k,
			Amount:       v.Text(10),
		}
		rds = append(rds, rd)
	}
	return
}

func (p *Protocol) getMostRecentEpoch() (epoch int, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT epoch_number FROM %s ORDER BY epoch_number desc LIMIT 1", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	if err = stmt.QueryRow().Scan(&epoch); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}

func (p *Protocol) getBookkeeping(epoch int, delegateName string, percentage int, includeFoundationBonus bool) (rds map[string]*big.Int, err error) {
	// First get sum of reward pool of epoch
	rewardToSplit, err := p.rewardToSplit(epoch, delegateName, percentage, includeFoundationBonus)
	if err != nil {
		err = errors.Wrap(err, "failed to get reward to split")
		return
	}
	// Second get TotalWeightedVotes
	totalWeightedVotes, err := p.totalWeightedVotes(epoch, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to get total weighted votes")
		return
	}
	// get voter's weighted votes
	voteSums, err := p.voterVotes(epoch, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to get voters' weighted votes")
		return
	}
	rds = make(map[string]*big.Int, 0)
	for k, v := range voteSums {
		amount := new(big.Int).Set(rewardToSplit)
		amount = amount.Mul(amount, v).Div(amount, totalWeightedVotes)
		rds[k] = amount
	}
	return
}
func (p *Protocol) totalWeightedVotes(epoch int, delegateName string) (sumVotesInt *big.Int, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT total_weighted_votes FROM %s WHERE epoch_number=? AND delegate_name=?",
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	rows, err := stmt.Query(epoch, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var tw totalWeight
	parsedRows, err := s.ParseSQLRows(rows, &tw)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}
	if len(parsedRows) != 1 {
		err = errors.Wrap(err, "failed to get total weighted vote")
		return
	}

	t := parsedRows[0].(*totalWeight)
	sumVotesInt, err = stringToBigInt(t.TotalWeight)
	if err != nil {
		err = errors.New("reward convert to int error")
		return
	}
	if sumVotesInt.Sign() == 0 {
		err = errors.New("sum of votes is 0")
		return
	}
	return
}
func (p *Protocol) rewardToSplit(epoch int, delegateName string, percentage int, includeFoundationBonus bool) (rewardToSplit *big.Int, err error) {
	_, epochReward, foundationBonus, err := p.GetAccountReward(uint64(epoch), uint64(1), delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to get account reward")
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
	rewardInt := epochRewardInt
	if includeFoundationBonus {
		rewardInt.Add(rewardInt, foundationBonusInt)
	}
	rewardToSplit = rewardInt.Mul(rewardInt, big.NewInt(int64(percentage))).Div(rewardInt, big.NewInt(100))
	return
}

// get voter's weighted votes
func (p *Protocol) voterVotes(epoch int, delegateName string) (votingSums map[string]*big.Int, err error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT voter_address,SUM(weighted_votes) FROM %s WHERE epoch_number=? AND candidate_name=?  GROUP BY voter_address",
		votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	rows, err := stmt.Query(epoch, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var votingHistory qvotings.VotingInfo
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}

	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}
	votingSums = make(map[string]*big.Int, 0)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*qvotings.VotingInfo)
		epochRewardInt, errs := stringToBigInt(voting.WeightedVotes)
		if errs != nil {
			err = errors.Wrap(errs, "failed to convert to big int")
			return
		}
		votingSums[voting.VoterAddress] = epochRewardInt
	}
	return
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
