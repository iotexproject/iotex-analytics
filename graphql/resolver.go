// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package graphql

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/queryprotocol/actions"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta"
	"github.com/iotexproject/iotex-analytics/queryprotocol/productivity"
	"github.com/iotexproject/iotex-analytics/queryprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/queryprotocol/votings"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

// Resolver is the resolver that handles graphql request
type Resolver struct {
	PP *productivity.Protocol
	RP *rewards.Protocol
	AP *actions.Protocol
	VP *votings.Protocol
	CP *chainmeta.Protocol
}

// Query returns a query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

// Rewards handles GetAccountReward request
func (r *queryResolver) Rewards(ctx context.Context, startEpoch int, epochCount int, candidateName string) (*Reward, error) {
	blockReward, epochReward, foundationBonus, err := r.RP.GetAccountReward(uint64(startEpoch), uint64(epochCount), candidateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reward information")
	}
	return &Reward{
		BlockReward:     blockReward,
		EpochReward:     epochReward,
		FoundationBonus: foundationBonus,
	}, nil
}

// Productivity handles GetProductivityHistory request
func (r *queryResolver) Productivity(ctx context.Context, startEpoch int, epochCount int, producerName string) (*Productivity, error) {
	production, expectedProduction, err := r.PP.GetProductivityHistory(uint64(startEpoch), uint64(epochCount), producerName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get productivity information")
	}
	return &Productivity{
		Production:         production,
		ExpectedProduction: expectedProduction,
	}, nil
}

// ActiveAccount handles GetActiveAccount request
func (r *queryResolver) ActiveAccount(ctx context.Context, count int) ([]string, error) {
	return r.AP.GetActiveAccount(count)
}

// VotingInformation handles GetVotingInformation request
func (r *queryResolver) VotingInformation(ctx context.Context, epochNum int, delegateName string) (votingInfos []*VotingInfo, err error) {
	votingHistorys, err := r.VP.GetVotingInformation(epochNum, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to get voting information")
		return
	}
	for _, votingHistory := range votingHistorys {
		v := &VotingInfo{
			WeightedVotes: votingHistory.WeightedVotes,
			VoterAddress:  votingHistory.VoterAddress,
		}
		votingInfos = append(votingInfos, v)
	}
	return
}

// Bookkeeping handles GetBookkeeping request
func (r *queryResolver) Bookkeeping(ctx context.Context, startEpoch int, epochCount int, delegateName string, percentage int, includeFoundationBonus bool) (rds []*RewardDistribution, err error) {
	if percentage < 0 || percentage > 100 {
		err = errors.New("percentage should be 0-100")
		return
	}
	rets, err := r.RP.GetBookkeeping(startEpoch, epochCount, delegateName, percentage, includeFoundationBonus)
	if err != nil {
		err = errors.Wrap(err, "failed to get bookkeeping information")
		return
	}
	for _, ret := range rets {
		v := &RewardDistribution{
			VoterAddress: ret.VoterAddress,
			Amount:       ret.Amount,
		}
		rds = append(rds, v)
	}
	return
}

// AverageProductivity handles AverageProductivity request
func (r *queryResolver) AverageProductivity(ctx context.Context, startEpochNumber int, epochCount int) (averageProcucitvity string, err error) {
	if startEpochNumber <= 0 || epochCount <= 0 {
		err = errors.New("epoch num and count should be greater than 0")
		return
	}
	ap, err := r.PP.AverageProductivity(startEpochNumber, epochCount)
	if err != nil {
		return
	}
	ap *= 100
	averageProcucitvity = fmt.Sprintf("%.2f", ap)
	return
}

// ChainMeta handles ChainMeta request
func (r *queryResolver) ChainMeta(ctx context.Context, tpsBlockWindow int) (rets *ChainMeta, err error) {
	if tpsBlockWindow <= 0 {
		err = errors.New("TPS block window should be greater than 0")
		return
	}
	ret, err := r.CP.GetChainMeta(tpsBlockWindow)
	if err != nil {
		err = errors.Wrap(err, "failed to get chain meta")
		return
	}
	rets = &ChainMeta{
		ret.MostRecentEpoch,
		ret.MostRecentBlockHeight,
		ret.MostRecentTps,
	}
	return
}
