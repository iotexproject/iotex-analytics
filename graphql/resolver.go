// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package graphql

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-analytics/queryprotocol/actions"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta"
	"github.com/iotexproject/iotex-analytics/queryprotocol/productivity"
	"github.com/iotexproject/iotex-analytics/queryprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/queryprotocol/votings"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

// Resolver is the resolver that handles graphql request
type Resolver struct {
	PP *productivity.Protocol
	RP *rewards.Protocol
	AP *actions.Protocol
	VP *votings.Protocol
	CP *chainmeta.Protocol
}

var (
	//DBtimeFactory    *prometheustimer.TimerFactory
	totaltimeFactory *prometheustimer.TimerFactory
	flag             int
)

func init() {
	go func() {
		server := http.NewServeMux()
		server.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9001", server)
	}()

	// timerFactoryDB, err := prometheustimer.New(
	// 	"DB_query_time_perf",
	// 	"",
	// 	[]string{"topic"},
	// 	[]string{"default"},
	// )
	// if err != nil {
	// 	log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	// }
	timerFactorytotal, err := prometheustimer.New(
		"total_query_time_perf",
		"",
		[]string{"topic"},
		[]string{"default"},
	)
	//DBtimeFactory = timerFactoryDB
	totaltimeFactory = timerFactorytotal
	if err != nil {
		log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	}
}

// Query returns a query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct {
	*Resolver
}

// Rewards handles GetAccountReward request
func (r *queryResolver) Rewards(ctx context.Context, startEpoch int, epochCount int, candidateName string) (*Reward, error) {
	timerReward := totaltimeFactory.NewTimer("GetReward")
	defer timerReward.End()
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
	timerPro := totaltimeFactory.NewTimer("GetProductivity")
	production, expectedProduction, err := r.PP.GetProductivityHistory(uint64(startEpoch), uint64(epochCount), producerName)
	defer timerPro.End()
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
	timerActive := totaltimeFactory.NewTimer("GetActiveAccount")
	defer timerActive.End()
	str, err := r.AP.GetActiveAccount(count)
	return str, err
}

// VotingInformation handles GetVotingInformation request
func (r *queryResolver) VotingInformation(ctx context.Context, epochNum int, delegateName string) (votingInfos []*VotingInfo, err error) {
	timerVoting := totaltimeFactory.NewTimer("GetVotingInformation")
	votingHistorys, err := r.VP.GetVotingInformation(epochNum, delegateName)
	defer timerVoting.End()
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
	timerBook := totaltimeFactory.NewTimer("GetBookkeeping")
	rets, err := r.RP.GetBookkeeping(uint64(startEpoch), uint64(epochCount), delegateName, percentage, includeFoundationBonus)
	defer timerBook.End()
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
func (r *queryResolver) AverageProductivity(ctx context.Context, startEpoch int, epochCount int) (averageProcucitvity string, err error) {
	if startEpoch <= 0 || epochCount <= 0 {
		err = errors.New("epoch num and count should be greater than 0")
		return
	}
	timerAP := totaltimeFactory.NewTimer("GetAverageProductivity")
	defer timerAP.End()
	ap, err := r.PP.GetAverageProductivity(uint64(startEpoch), uint64(epochCount))
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
	timerCM := totaltimeFactory.NewTimer("GetChainMeta")
	defer timerCM.End()
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

// TotalTokens handles TotalTokens request
func (r *queryResolver) TotalTokens(ctx context.Context, startEpoch int, epochCount int) ([]string, error) {
	return r.VP.GetTotalTokenStaked(uint64(startEpoch), uint64(epochCount))
}

func (r *queryResolver) AverageTokenHolding(ctx context.Context, epochNumber int) (string, error) {
	return r.VP.GetAverageTokenHolding(uint64(epochNumber))
}

func (r *queryResolver) StakeDuration(ctx context.Context, epochNumber int) ([]string, error) {
	return r.VP.GetStakeDuration(uint64(epochNumber))
}

func (r *queryResolver) NumberOfUniqueAccounts(ctx context.Context, epochNumber int, delegateName string) (string, error) {
	return r.VP.GetNumberOfUniqueAccounts(uint64(epochNumber), delegateName)
}

func (r *queryResolver) SelfStakeAndTotalStake(ctx context.Context, epochNumber int, delegateName string) ([]string, error) {
	return r.VP.GetSelfStakeAndTotalStake(uint64(epochNumber), delegateName)

}

// NumberOfActions handles NumberOfActions request
func (r *queryResolver) NumberOfActions(ctx context.Context, startEpoch int, epochCount int) (string, error) {
	timerNA := totaltimeFactory.NewTimer("GetNumberofActions")
	defer timerNA.End()
	return r.CP.GetNumberOfActions(uint64(startEpoch), uint64(epochCount))
}

// NumberOfWeightedVotes handles NumberOfWeightedVotes request
func (r *queryResolver) NumberOfWeightedVotes(ctx context.Context, startEpoch int, epochCount int) ([]string, error) {
	timerNW := totaltimeFactory.NewTimer("GetNumberOfWeightedVotes")
	defer timerNW.End()
	return r.VP.GetNumberOfWeightedVotes(uint64(startEpoch), uint64(epochCount))
}

// NumberOfCandidates handles NumberOfCandidates request
func (r *queryResolver) NumberOfCandidates(ctx context.Context, epochNumber int) (*NumberOfCandidates, error) {
	timerNC := totaltimeFactory.NewTimer("GetNumberofCandidates")
	defer timerNC.End()
	numberOfCandidates, err := r.VP.GetNumberOfCandidates(uint64(epochNumber))
	if err != nil {
		return nil, err
	}
	return &NumberOfCandidates{numberOfCandidates.TotalCandidates, numberOfCandidates.ConsensusDelegates}, nil
}
