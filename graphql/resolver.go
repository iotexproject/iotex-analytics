// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package graphql

import (
	"context"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexservice"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

// Resolver is the resolver that handles graphql request
type Resolver struct {
	Indexer *indexservice.Indexer
}

// Query returns a query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

// Rewards handles GetAccountReward request
func (r *queryResolver) Rewards(ctx context.Context, startEpoch int, epochCount int, candidateName string) (*Reward, error) {
	blockReward, epochReward, foundationBonus, err := r.Indexer.GetAccountReward(uint64(startEpoch), uint64(epochCount), candidateName)
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
	production, expectedProduction, err := r.Indexer.GetProductivityHistory(uint64(startEpoch), uint64(epochCount), producerName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get productivity information")
	}
	return &Productivity{
		Production:         int(production),
		ExpectedProduction: int(expectedProduction),
	}, nil
}
