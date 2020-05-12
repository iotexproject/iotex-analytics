// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
)

func (p *Protocol) stakingV2(chainClient iotexapi.APIServiceClient, epochStartheight, epochNumber uint64, probationList *iotextypes.ProbationCandidateList) (err error) {
	voteBucketList, err := indexprotocol.GetBucketsAllV2(chainClient, epochStartheight)
	if err != nil {
		return errors.Wrap(err, "failed to get buckets count")
	}
	candidateList, err := indexprotocol.GetCandidatesAllV2(chainClient, epochStartheight)
	if err != nil {
		return errors.Wrap(err, "failed to get buckets count")
	}
	if probationList != nil {
		candidateList, err = filterCandidatesV2(candidateList, probationList, epochStartheight)
		if err != nil {
			return errors.Wrap(err, "failed to filter candidate with probation list")
		}
	}
	if len(voteBucketList.Buckets) == 0 || len(candidateList.Candidates) == 0 {
		log.S().Errorf("buckets len:%d, candidates len:%d", len(voteBucketList.Buckets), len(candidateList.Candidates))
		return nil
	}
	// after get and clean data,the following code is for writing mysql
	tx, err := p.Store.GetDB().Begin()
	// update stakingV2_bucket and height_to_stakingV2_bucket table
	if err = p.nativeV2BucketTableOperator.Put(epochStartheight, voteBucketList, tx); err != nil {
		return
	}
	// update stakingV2_candidate and height_to_stakingV2_candidate table
	if err = p.nativeV2CandidateTableOperator.Put(epochStartheight, candidateList, tx); err != nil {
		return
	}
	// update voting_result table
	if err = p.updateVotingResultV2(tx, candidateList, epochNumber); err != nil {
		return
	}
	// update aggregate_voting and voting_meta table
	if err = p.updateAggregateVotingV2(tx, voteBucketList, candidateList, epochNumber, probationList); err != nil {
		return
	}
	tx.Commit()
	return
}

func (p *Protocol) updateVotingResultV2(tx *sql.Tx, candidates *iotextypes.CandidateListV2, epochNumber uint64) (err error) {
	var voteResultStmt *sql.Stmt
	insertQuery := fmt.Sprintf(insertVotingResult,
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
	for _, candidate := range candidates.Candidates {
		addr, err := address.FromString(candidate.OwnerAddress)
		if err != nil {
			return err
		}
		addressString := hex.EncodeToString(addr.Bytes())
		if _, err = voteResultStmt.Exec(
			epochNumber,
			candidate.Name,
			candidate.OperatorAddress,
			candidate.RewardAddress,
			candidate.TotalWeightedVotes,
			candidate.SelfStakingTokens,
			0,             // TODO wait for deploy contract or api
			0,             // TODO wait for deploy contract or api
			0,             // TODO wait for deploy contract or api
			addressString, // type is varchar 40,change to ethereum hex address
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) updateAggregateVotingV2(tx *sql.Tx, votes *iotextypes.VoteBucketList, delegates *iotextypes.CandidateListV2, epochNumber uint64, probationList *iotextypes.ProbationCandidateList) (err error) {
	pb := convertProbationListToLocal(probationList)
	intensityRate, probationMap := probationListToMapV2(delegates, pb)
	//update aggregate voting table
	sumOfWeightedVotes := make(map[aggregateKey]*big.Int)
	totalVoted := big.NewInt(0)
	selfStakeIndex := selfStakeIndexMap(delegates)
	for _, vote := range votes.Buckets {
		//for sumOfWeightedVotes
		key := aggregateKey{
			epochNumber:   epochNumber,
			candidateName: vote.CandidateAddress,
			voterAddress:  vote.Owner,
			isNative:      true,
		}
		selfStake := false
		if _, ok := selfStakeIndex[vote.Index]; ok {
			selfStake = true
		}
		weightedAmount := calculateVoteWeightV2(p.voteCfg, vote, selfStake)
		stakeAmount, ok := big.NewInt(0).SetString(vote.StakedAmount, 10)
		if !ok {
			err = errors.New("stake amount convert error")
			return
		}
		if val, ok := sumOfWeightedVotes[key]; ok {
			val.Add(val, weightedAmount)
		} else {
			sumOfWeightedVotes[key] = weightedAmount
		}
		totalVoted.Add(totalVoted, stakeAmount)
	}
	insertQuery := fmt.Sprintf(insertAggregateVoting, AggregateVotingTableName)
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
		if _, ok := probationMap[key.candidateName]; ok {
			// filter based on probation
			votingPower := new(big.Float).SetInt(val)
			val, _ = votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
		}
		if _, err = aggregateStmt.Exec(
			key.epochNumber,
			key.candidateName,
			key.voterAddress,
			key.isNative,
			val.Text(10),
		); err != nil {
			return err
		}
	}
	//update voting meta table
	totalWeighted := big.NewInt(0)
	for _, cand := range delegates.Candidates {
		totalWeightedVotes, ok := big.NewInt(0).SetString(cand.TotalWeightedVotes, 10)
		if !ok {
			err = errors.New("total weighted votes convert error")
			return
		}
		totalWeighted.Add(totalWeighted, totalWeightedVotes)
	}
	insertQuery = fmt.Sprintf(insertVotingMeta, VotingMetaTableName)
	if _, err = tx.Exec(insertQuery,
		epochNumber,
		totalVoted.Text(10),
		len(delegates.Candidates),
		totalWeighted.Text(10),
	); err != nil {
		return errors.Wrap(err, "failed to update voting meta table")
	}
	return
}

func (p *Protocol) getBucketInfoByEpochV2(height, epochNum uint64, delegateName string) ([]*VotingInfo, error) {
	ret, err := p.nativeV2BucketTableOperator.Get(height, p.Store.GetDB(), nil)
	bucketList, ok := ret.(*iotextypes.VoteBucketList)
	if !ok {
		return nil, errors.Errorf("Unexpected type %s", reflect.TypeOf(ret))
	}
	can, err := p.nativeV2CandidateTableOperator.Get(height, p.Store.GetDB(), nil)
	candidateList, ok := can.(*iotextypes.CandidateListV2)
	if !ok {
		return nil, errors.Errorf("Unexpected type %s", reflect.TypeOf(can))
	}
	var candidateAddress string
	for _, cand := range candidateList.Candidates {
		if cand.Name == delegateName {
			candidateAddress = cand.OwnerAddress
			break
		}
	}
	// update weighted votes based on probation
	pblist, err := p.getProbationList(epochNum)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get probation list from table")
	}
	intensityRate, probationMap := probationListToMapV2(candidateList, pblist)
	var votinginfoList []*VotingInfo
	selfStakeIndex := selfStakeIndexMap(candidateList)
	for _, vote := range bucketList.Buckets {
		if vote.CandidateAddress == candidateAddress {
			selfStake := false
			if _, ok := selfStakeIndex[vote.Index]; ok {
				selfStake = true
			}
			weightedVotes := calculateVoteWeightV2(p.voteCfg, vote, selfStake)
			if _, ok := probationMap[vote.CandidateAddress]; ok {
				// filter based on probation
				votingPower := new(big.Float).SetInt(weightedVotes)
				weightedVotes, _ = votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
			}
			votinginfo := &VotingInfo{
				EpochNumber:       epochNum,
				VoterAddress:      vote.Owner,
				IsNative:          true,
				Votes:             vote.StakedAmount,
				WeightedVotes:     weightedVotes.Text(10),
				RemainingDuration: fmt.Sprintf("%0.2f", remainingTime(vote).Seconds()),
				StartTime:         fmt.Sprintf("%d", vote.StakeStartTime.Seconds),
				Decay:             !vote.AutoStake,
			}
			votinginfoList = append(votinginfoList, votinginfo)
		}
	}
	return votinginfoList, nil
}

func calculateVoteWeightV2(cfg indexprotocol.VoteWeightCalConsts, v *iotextypes.VoteBucket, selfStake bool) *big.Int {
	remainingTime := float64(v.StakedDuration)
	weight := float64(1)
	var m float64
	if v.AutoStake {
		m = cfg.AutoStake
	}
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)*(1+m)) / math.Log(cfg.DurationLg) / 100
	}
	if selfStake {
		weight *= cfg.SelfStake
	}
	amount, ok := new(big.Float).SetString(v.StakedAmount)
	if !ok {
		return big.NewInt(0)
	}
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)
	return weightedAmount
}

func remainingTime(bucket *iotextypes.VoteBucket) time.Duration {
	now := time.Now()
	startTime := time.Unix(bucket.StakeStartTime.Seconds, int64(bucket.StakeStartTime.Nanos))
	if now.Before(startTime) {
		return 0
	}
	endTime := startTime.Add(time.Duration(bucket.StakedDuration) * time.Second)
	if endTime.After(now) {
		return startTime.Add(time.Duration(bucket.StakedDuration) * time.Second).Sub(now)
	}
	return 0
}

func selfStakeIndexMap(candidates *iotextypes.CandidateListV2) map[uint64]struct{} {
	ret := make(map[uint64]struct{})
	for _, can := range candidates.Candidates {
		ret[can.SelfStakeBucketIdx] = struct{}{}
	}
	return ret
}
