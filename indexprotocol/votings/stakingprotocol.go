// Copyright (c) 2020 IoTeX
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
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-election/db"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/contract"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	topicProfileUpdated     = "217aa5ef0b78f028d51fd573433bdbe2daf6f8505e6a71f3af1393c8440b341b"
	blockRewardPortion      = "blockRewardPortion"
	epochRewardPortion      = "epochRewardPortion"
	foundationRewardPortion = "foundationRewardPortion"
)

func (p *Protocol) processStaking(tx *sql.Tx, chainClient iotexapi.APIServiceClient, epochStartheight, epochNumber uint64, probationList *iotextypes.ProbationCandidateList) (err error) {
	voteBucketList, err := indexprotocol.GetAllStakingBuckets(chainClient, epochStartheight)
	if err != nil {
		return errors.Wrap(err, "failed to get buckets count")
	}
	candidateList, err := indexprotocol.GetAllStakingCandidates(chainClient, epochStartheight)
	if err != nil {
		return errors.Wrap(err, "failed to get buckets count")
	}
	// after get and clean data,the following code is for writing mysql
	// update staking_bucket and height_to_staking_bucket table
	if err = p.stakingBucketTableOperator.Put(epochStartheight, voteBucketList, tx); err != nil {
		return
	}
	// update staking_candidate and height_to_staking_candidate table
	if err = p.stakingCandidateTableOperator.Put(epochStartheight, candidateList, tx); err != nil {
		return
	}
	if probationList != nil {
		candidateList, err = filterStakingCandidates(candidateList, probationList, epochStartheight)
		if err != nil {
			return errors.Wrap(err, "failed to filter candidate with probation list")
		}
	}
	// update voting_result table
	if err = p.updateStakingResult(tx, candidateList, epochNumber, epochStartheight, chainClient); err != nil {
		return
	}
	// update aggregate_voting and voting_meta table
	if err = p.updateAggregateStaking(tx, voteBucketList, candidateList, epochNumber, probationList); err != nil {
		return
	}
	return
}

func (p *Protocol) updateStakingResult(tx *sql.Tx, candidates *iotextypes.CandidateListV2, epochNumber, epochStartheight uint64, chainClient iotexapi.APIServiceClient) (err error) {
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
	blockRewardPortionMap, epochRewardPortionMap, foundationBonusPortionMap, err := p.getAllStakingDelegateRewardPortions(epochStartheight, epochNumber, chainClient)
	if err != nil {
		return errors.Errorf("get delegate reward portions:%d,%s", epochStartheight, err.Error())
	}

	blockRewardPortion, epochRewardPortion, foundationBonusPortion := 0.0, 0.0, 0.0
	for _, candidate := range candidates.Candidates {
		stakingAddress, err := util.IoAddrToEvmAddr(candidate.OwnerAddress)
		if err != nil {
			return errors.Wrap(err, "failed to convert IoTeX address to ETH address")
		}
		blockRewardPortion = blockRewardPortionMap[strings.ToLower(stakingAddress.String()[2:])]
		epochRewardPortion = epochRewardPortionMap[strings.ToLower(stakingAddress.String()[2:])]
		foundationBonusPortion = foundationBonusPortionMap[strings.ToLower(stakingAddress.String()[2:])]
		encodedName, err := indexprotocol.EncodeDelegateName(candidate.Name)
		if err != nil {
			return errors.Wrap(err, "encode delegate name error")
		}
		if _, err = voteResultStmt.Exec(
			epochNumber,
			encodedName,
			candidate.OperatorAddress,
			candidate.RewardAddress,
			candidate.TotalWeightedVotes,
			candidate.SelfStakingTokens,
			fmt.Sprintf("%0.2f", blockRewardPortion),
			fmt.Sprintf("%0.2f", epochRewardPortion),
			fmt.Sprintf("%0.2f", foundationBonusPortion),
			hex.EncodeToString(stakingAddress.Bytes()),
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) updateAggregateStaking(tx *sql.Tx, votes *iotextypes.VoteBucketList, delegates *iotextypes.CandidateListV2, epochNumber uint64, probationList *iotextypes.ProbationCandidateList) (err error) {
	nameMap, err := ownerAddressToNameMap(delegates)
	if err != nil {
		return errors.Wrap(err, "owner address to name map error")
	}
	pb := convertProbationListToLocal(probationList)
	intensityRate, probationMap := stakingProbationListToMap(delegates, pb)
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
		weightedAmount, err := calculateVoteWeight(p.voteCfg, vote, selfStake)
		if err != nil {
			return errors.Wrap(err, "failed to calculate vote weight")
		}
		stakeAmount, ok := big.NewInt(0).SetString(vote.StakedAmount, 10)
		if !ok {
			return errors.New("failed to convert string to big int")
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
		if _, ok := nameMap[key.candidateName]; !ok {
			return errors.New("candidate cannot find name through owner address")
		}
		voterAddress, err := util.IoAddrToEvmAddr(key.voterAddress)
		if err != nil {
			return errors.Wrap(err, "failed to convert IoTeX address to ETH address")
		}
		if _, err = aggregateStmt.Exec(
			key.epochNumber,
			nameMap[key.candidateName],
			hex.EncodeToString(voterAddress.Bytes()),
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

func (p *Protocol) getStakingBucketInfoByEpoch(height, epochNum uint64, delegateName string) ([]*VotingInfo, error) {
	ret, err := p.stakingBucketTableOperator.Get(height, p.Store.GetDB(), nil)
	if errors.Cause(err) == db.ErrNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get staking buckets")
	}
	bucketList, ok := ret.(*iotextypes.VoteBucketList)
	if !ok {
		return nil, errors.Errorf("Unexpected type %s", reflect.TypeOf(ret))
	}
	can, err := p.stakingCandidateTableOperator.Get(height, p.Store.GetDB(), nil)
	if errors.Cause(err) == db.ErrNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get staking candidates")
	}
	candidateList, ok := can.(*iotextypes.CandidateListV2)
	if !ok {
		return nil, errors.Errorf("Unexpected type %s", reflect.TypeOf(can))
	}
	var candidateAddress string
	for _, cand := range candidateList.Candidates {
		encodedName, err := indexprotocol.EncodeDelegateName(cand.Name)
		if err != nil {
			return nil, errors.Wrap(err, "error when encode delegate name")
		}
		if encodedName == delegateName {
			candidateAddress = cand.OwnerAddress
			break
		}
	}
	// update weighted votes based on probation
	pblist, err := p.getProbationList(epochNum)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get probation list from table")
	}
	intensityRate, probationMap := stakingProbationListToMap(candidateList, pblist)
	var votinginfoList []*VotingInfo
	selfStakeIndex := selfStakeIndexMap(candidateList)
	for _, vote := range bucketList.Buckets {
		if vote.CandidateAddress == candidateAddress {
			selfStake := false
			if _, ok := selfStakeIndex[vote.Index]; ok {
				selfStake = true
			}
			weightedVotes, err := calculateVoteWeight(p.voteCfg, vote, selfStake)
			if err != nil {
				return nil, errors.Wrap(err, "calculate vote weight error")
			}
			if _, ok := probationMap[vote.CandidateAddress]; ok {
				// filter based on probation
				votingPower := new(big.Float).SetInt(weightedVotes)
				weightedVotes, _ = votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
			}
			voteOwnerAddress, err := util.IoAddrToEvmAddr(vote.Owner)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert IoTeX address to ETH address")
			}
			votinginfo := &VotingInfo{
				EpochNumber:       epochNum,
				VoterAddress:      hex.EncodeToString(voteOwnerAddress.Bytes()),
				IsNative:          true,
				Votes:             vote.StakedAmount,
				WeightedVotes:     weightedVotes.Text(10),
				RemainingDuration: remainingTime(vote).String(),
				StartTime:         time.Unix(vote.StakeStartTime.Seconds, int64(vote.StakeStartTime.Nanos)).String(),
				Decay:             !vote.AutoStake,
			}
			votinginfoList = append(votinginfoList, votinginfo)
		}
	}
	return votinginfoList, nil
}

func (p *Protocol) getAllStakingDelegateRewardPortions(epochStartHeight, epochNumber uint64, chainClient iotexapi.APIServiceClient) (blockRewardPercentage, epochRewardPercentage, foundationBonusPercentage map[string]float64, err error) {
	blockRewardPercentage = make(map[string]float64)
	epochRewardPercentage = make(map[string]float64)
	foundationBonusPercentage = make(map[string]float64)
	if epochStartHeight == p.epochCtx.FairbankHeight() {
		// init from contract,from contract deployed height to epochStartheight-1,get latest portion
		if p.rewardPortionContract == "" {
			// todo make sure if ignore this error
			//err = errors.New("portion contract address is empty")
			return
		}
		if p.rewardPortionContractDeployHeight > epochStartHeight {
			// todo make sure if ignore this error
			//err = errors.New("portion contract deploy height should less than fairbank height")
			return
		}
		count := epochStartHeight - p.rewardPortionContractDeployHeight
		blockRewardPercentage, epochRewardPercentage, foundationBonusPercentage, err = getlog(p.rewardPortionContract, p.rewardPortionContractDeployHeight, count, chainClient, p.abi)
		if err != nil {
			err = errors.Wrap(err, "get log from chain error")
			return
		}
	} else {
		// get from mysql first
		blockRewardPercentage, epochRewardPercentage, foundationBonusPercentage, err = getLastEpochPortion(p.Store.GetDB(), epochNumber-1)
		if err != nil && errors.Cause(err) != indexprotocol.ErrNotExist {
			// todo make sure if ignore this error
			err = errors.Wrap(err, "get last epoch portion error")
			return
		}

		//and then update from contract from last epochstartHeight to this epochStartheight-1
		lastEpochStartHeight := p.epochCtx.GetEpochHeight(epochNumber - 1)
		if epochStartHeight < lastEpochStartHeight {
			err = errors.Wrap(err, "epoch start height less than last epoch start height")
			return
		}
		count := epochStartHeight - lastEpochStartHeight
		var blockRewardFromLog, epochRewardFromLog, foundationBonusFromLog map[string]float64
		blockRewardFromLog, epochRewardFromLog, foundationBonusFromLog, err = getlog(p.rewardPortionContract, lastEpochStartHeight, count, chainClient, p.abi)
		if err != nil {
			err = errors.Wrap(err, "get log from chain error")
			return
		}
		// update to mysql's portion
		for k, v := range blockRewardFromLog {
			blockRewardPercentage[k] = v
		}
		for k, v := range epochRewardFromLog {
			epochRewardPercentage[k] = v
		}
		for k, v := range foundationBonusFromLog {
			foundationBonusPercentage[k] = v
		}
	}
	return
}

func calculateVoteWeight(cfg indexprotocol.VoteWeightCalConsts, v *iotextypes.VoteBucket, selfStake bool) (*big.Int, error) {
	remainingTime := float64(v.StakedDuration * 86400)
	weight := float64(1)
	var m float64
	if v.AutoStake {
		m = cfg.AutoStake
	}
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)*(1+m)) / math.Log(cfg.DurationLg) / 100
	}
	if selfStake && v.AutoStake && v.StakedDuration >= 91 {
		// self-stake extra bonus requires enable auto-stake for at least 3 months
		weight *= cfg.SelfStake
	}

	amountInt, ok := big.NewInt(0).SetString(v.StakedAmount, 10)
	if !ok {
		return nil, errors.New("failed to convert string to big int")
	}
	amount := new(big.Float).SetInt(amountInt)
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)
	return weightedAmount, nil
}

func remainingTime(bucket *iotextypes.VoteBucket) time.Duration {
	now := time.Now()
	startTime := time.Unix(bucket.StakeStartTime.Seconds, int64(bucket.StakeStartTime.Nanos))
	if now.Before(startTime) {
		return 0
	}
	duration := time.Duration(bucket.StakedDuration) * 24 * time.Hour
	if !bucket.AutoStake {
		endTime := startTime.Add(duration)
		if endTime.After(now) {
			return endTime.Sub(now)
		}
		return 0
	}
	return duration
}

func selfStakeIndexMap(candidates *iotextypes.CandidateListV2) map[uint64]struct{} {
	ret := make(map[uint64]struct{})
	for _, can := range candidates.Candidates {
		ret[can.SelfStakeBucketIdx] = struct{}{}
	}
	return ret
}

func ownerAddressToNameMap(candidates *iotextypes.CandidateListV2) (ret map[string]string, err error) {
	ret = make(map[string]string)
	for _, can := range candidates.Candidates {
		var name string
		name, err = indexprotocol.EncodeDelegateName(can.Name)
		if err != nil {
			return
		}
		ret[can.OwnerAddress] = name
	}
	return
}

func getlog(contractAddress string, from, count uint64, chainClient iotexapi.APIServiceClient, delegateABI abi.ABI) (blockReward, epochReward, foundationReward map[string]float64, err error) {
	blockReward = make(map[string]float64)
	epochReward = make(map[string]float64)
	foundationReward = make(map[string]float64)
	topics := make([][]byte, 0)
	tp, err := hex.DecodeString(topicProfileUpdated)
	if err != nil {
		return
	}
	topics = append(topics, tp)

	response, err := chainClient.GetLogs(context.Background(), &iotexapi.GetLogsRequest{
		Filter: &iotexapi.LogsFilter{
			Address: []string{contractAddress},
			Topics:  []*iotexapi.Topics{&iotexapi.Topics{Topic: topics}},
		},
		Lookup: &iotexapi.GetLogsRequest_ByRange{
			ByRange: &iotexapi.GetLogsByRange{
				FromBlock: from,
				Count:     count,
			},
		},
	})
	if err != nil {
		return
	}
	for _, l := range response.Logs {
		for _, topic := range l.Topics {
			switch hex.EncodeToString(topic) {
			case topicProfileUpdated:
				event := new(contract.DelegateProfileProfileUpdated)
				if err := delegateABI.Unpack(event, "ProfileUpdated", l.Data); err != nil {
					continue
				}
				switch event.Name {
				case blockRewardPortion:
					blockReward[strings.ToLower(event.Delegate.String()[2:])] = float64(big.NewInt(0).SetBytes(event.Value).Uint64()) / 100
				case epochRewardPortion:
					epochReward[strings.ToLower(event.Delegate.String()[2:])] = float64(big.NewInt(0).SetBytes(event.Value).Uint64()) / 100
				case foundationRewardPortion:
					foundationReward[strings.ToLower(event.Delegate.String()[2:])] = float64(big.NewInt(0).SetBytes(event.Value).Uint64()) / 100
				}
			}
		}
	}
	return
}

func getLastEpochPortion(db *sql.DB, epochNumber uint64) (blockReward, epochReward, foundationReward map[string]float64, err error) {
	blockReward = make(map[string]float64)
	epochReward = make(map[string]float64)
	foundationReward = make(map[string]float64)
	getQuery := fmt.Sprintf(selectVotingResultFromStakingAddress,
		VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var votingResult VotingResult
	parsedRows, err := s.ParseSQLRows(rows, &votingResult)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}

	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}

	for _, parsedRow := range parsedRows {
		vr := parsedRow.(*VotingResult)
		blockReward[vr.StakingAddress] = vr.BlockRewardPercentage
		epochReward[vr.StakingAddress] = vr.EpochRewardPercentage
		foundationReward[vr.StakingAddress] = vr.FoundationBonusPercentage
	}
	return
}
