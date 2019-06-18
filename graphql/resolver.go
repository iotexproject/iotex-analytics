// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package graphql

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/ast"
	"golang.org/x/sync/errgroup"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/queryprotocol/actions"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta"
	"github.com/iotexproject/iotex-analytics/queryprotocol/productivity"
	"github.com/iotexproject/iotex-analytics/queryprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/queryprotocol/votings"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

// HexPrefix is the prefix of ERC20 address in hex string
const HexPrefix = "0x"

// ErrPaginationNotFound is the error indicating that pagination is not specified
var ErrPaginationNotFound = errors.New("Pagination information is not found")

// Resolver is hte resolver that handles GraphQL request
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

// Account handles account requests
func (r *queryResolver) Account(ctx context.Context) (*Account, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	accountResponse := &Account{}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "activeAccounts") {
		g.Go(func() error { return r.getActiveAccounts(ctx, accountResponse) })
	}

	return accountResponse, g.Wait()
}

// Chain handles chain requests
func (r *queryResolver) Chain(ctx context.Context) (*Chain, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	chainResponse := &Chain{}

	if err := r.getLastEpochAndHeight(chainResponse); err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "mostRecentTPS") {
		g.Go(func() error { return r.getTPS(ctx, chainResponse) })
	}
	if containField(requestedFields, "numberOfActions") {
		g.Go(func() error { return r.getNumberOfActions(ctx, chainResponse) })
	}

	return chainResponse, g.Wait()
}

// Voting handles voting requests
func (r *queryResolver) Voting(ctx context.Context, startEpoch int, epochCount int) (*Voting, error) {
	cl, numConsensusDelegates, err := r.VP.GetCandidateMeta(uint64(startEpoch), uint64(epochCount))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		return &Voting{Exist: false}, nil
	case err != nil:
		return nil, errors.Wrap(err, "failed to get reward information")
	}
	candidateMetaList := make([]*CandidateMeta, 0)
	for _, candidateMeta := range cl {
		candidateMetaList = append(candidateMetaList, &CandidateMeta{
			EpochNumber:        int(candidateMeta.EpochNumber),
			TotalCandidates:    int(candidateMeta.NumberOfCandidates),
			ConsensusDelegates: int(numConsensusDelegates),
			TotalWeightedVotes: candidateMeta.TotalWeightedVotes,
			TotalTokens:        candidateMeta.TotalTokens,
		})
	}
	return &Voting{Exist: true, CandidateMeta: candidateMetaList}, nil
}

// Delegate handles delegate requests
func (r *queryResolver) Delegate(ctx context.Context, startEpoch int, epochCount int, delegateName string) (*Delegate, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	delegateResponse := &Delegate{}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "reward") {
		g.Go(func() error { return r.getRewards(delegateResponse, startEpoch, epochCount, delegateName) })
	}
	if containField(requestedFields, "productivity") {
		g.Go(func() error { return r.getProductivity(delegateResponse, startEpoch, epochCount, delegateName) })
	}
	if containField(requestedFields, "bookkeeping") {
		g.Go(func() error { return r.getBookkeeping(ctx, delegateResponse, startEpoch, epochCount, delegateName) })
	}
	if containField(requestedFields, "bucketInfo") {
		g.Go(func() error { return r.getBucketInfo(delegateResponse, startEpoch, epochCount, delegateName) })
	}
	return delegateResponse, g.Wait()
}

func (r *queryResolver) getActiveAccounts(ctx context.Context, accountResponse *Account) error {
	argsMap := parseFieldArguments(ctx, "activeAccounts", "")
	count, err := getIntArg(argsMap, "count")
	if err != nil {
		return errors.Wrap(err, "failed to get count for active accounts")
	}
	if count < 1 {
		return errors.New("invalid count number")
	}
	accounts, err := r.AP.GetActiveAccount(count)
	if err != nil {
		return errors.Wrap(err, "failed to get active accounts information")
	}
	accountResponse.ActiveAccounts = accounts
	return nil
}

func (r *queryResolver) getLastEpochAndHeight(chainResponse *Chain) error {
	epoch, tipHeight, err := r.CP.GetLastEpochAndHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get last epoch number and tip block height")
	}
	chainResponse.MostRecentEpoch = int(epoch)
	chainResponse.MostRecentBlockHeight = int(tipHeight)
	return nil
}

func (r *queryResolver) getTPS(ctx context.Context, chainResponse *Chain) error {
	argsMap := parseFieldArguments(ctx, "mostRecentTPS", "")
	blockWindow, err := getIntArg(argsMap, "blockWindow")
	if err != nil {
		return errors.Wrap(err, "failed to get blockWindow for TPS")
	}
	if blockWindow <= 0 {
		return errors.New("invalid block window")
	}
	tps, err := r.CP.MostRecentTPS(uint64(blockWindow))
	if err != nil {
		return errors.Wrap(err, "failed to get most recent TPS")
	}
	chainResponse.MostRecentTps = tps
	return nil
}

func (r *queryResolver) getNumberOfActions(ctx context.Context, chainResponse *Chain) error {
	argsMap := parseFieldArguments(ctx, "numberOfActions", "")
	paginationMap, err := getPaginationArgs(argsMap)
	var numberOfActions uint64
	switch {
	case err == ErrPaginationNotFound:
		numberOfActions, err = r.CP.GetNumberOfActions(uint64(1), uint64(chainResponse.MostRecentEpoch))
		if err != nil {
			return errors.Wrapf(err, "failed to get number of actions")
		}
	case err != nil:
		return errors.Wrap(err, "failed to get pagination arguments for bookkeeping")
	default:
		startEpoch := paginationMap["startEpoch"]
		epochCount := paginationMap["epochCount"]
		if startEpoch < 1 || epochCount < 0 {
			return errors.New("invalid start epoch number or epoch count for getting number of actions")
		}
		numberOfActions, err = r.CP.GetNumberOfActions(uint64(startEpoch), uint64(epochCount))
		switch {
		case errors.Cause(err) == indexprotocol.ErrNotExist:
			chainResponse.NumberOfActions = &NumberOfActions{Exist: false}
			return nil
		case err != nil:
			return errors.Wrap(err, "failed to get number of actions")
		}
	}
	chainResponse.NumberOfActions = &NumberOfActions{Exist: true, Count: int(numberOfActions)}
	return nil
}

func (r *queryResolver) getRewards(delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	blockReward, epochReward, foundationBonus, err := r.RP.GetAccountReward(uint64(startEpoch), uint64(epochCount), delegateName)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		delegateResponse.Reward = &Reward{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get reward information")
	}
	delegateResponse.Reward = &Reward{
		Exist:           true,
		BlockReward:     blockReward,
		EpochReward:     epochReward,
		FoundationBonus: foundationBonus,
	}
	return nil
}

func (r *queryResolver) getProductivity(delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	production, expectedProduction, err := r.PP.GetProductivityHistory(uint64(startEpoch), uint64(epochCount), delegateName)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		delegateResponse.Productivity = &Productivity{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get productivity information")
	}
	delegateResponse.Productivity = &Productivity{
		Exist:              true,
		Production:         production,
		ExpectedProduction: expectedProduction,
	}
	return nil
}

func (r *queryResolver) getBookkeeping(ctx context.Context, delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	argsMap := parseFieldArguments(ctx, "bookkeeping", "rewardDistribution")
	percentage, err := getIntArg(argsMap, "percentage")
	if err != nil {
		return errors.Wrap(err, "failed to get percentage for bookkeeping")
	}
	includeFoundationBonus, err := getBoolArg(argsMap, "includeFoundationBonus")
	if err != nil {
		return errors.Wrap(err, "failed to get includeFoundationBonus for bookkeeping")
	}

	if percentage < 0 || percentage > 100 {
		return errors.New("percentage should be 0-100")
	}

	rets, err := r.RP.GetBookkeeping(uint64(startEpoch), uint64(epochCount), delegateName, percentage, includeFoundationBonus)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		delegateResponse.Bookkeeping = &Bookkeeping{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get bookkeeping information")
	}

	rds := make([]*RewardDistribution, 0)
	for _, ret := range rets {
		v := &RewardDistribution{
			VoterEthAddress:   HexPrefix + ret.VoterEthAddress,
			VoterIotexAddress: ret.VoterIotexAddress,
			Amount:            ret.Amount,
		}
		rds = append(rds, v)
	}

	sort.Slice(rds, func(i, j int) bool { return rds[i].VoterEthAddress < rds[j].VoterEthAddress })

	bookkeepingOutput := &Bookkeeping{Exist: true, Count: len(rds)}
	paginationMap, err := getPaginationArgs(argsMap)
	switch {
	case err == ErrPaginationNotFound:
		bookkeepingOutput.RewardDistribution = rds
	case err != nil:
		return errors.Wrap(err, "failed to get pagination arguments for reward distributions")
	default:
		skip := paginationMap["skip"]
		first := paginationMap["first"]
		if skip < 0 || skip >= len(rds) {
			return errors.New("invalid pagination skip number for reward distributions")
		}
		if len(rds)-skip < first {
			first = len(rds) - skip
		}
		bookkeepingOutput.RewardDistribution = rds[skip : skip+first]
	}
	delegateResponse.Bookkeeping = bookkeepingOutput
	return nil
}

func (r *queryResolver) getBucketInfo(delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	bucketMap, err := r.VP.GetBucketInformation(uint64(startEpoch), uint64(epochCount), delegateName)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		delegateResponse.BucketInfo = &BucketInfoOutput{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get voting bucket information")
	}

	bucketInfoLists := make([]*BucketInfoList, 0)
	for epoch, bucketList := range bucketMap {
		bucketInfoList := &BucketInfoList{EpochNumber: int(epoch), Count: len(bucketList)}
		bucketInfo := make([]*BucketInfo, 0)
		for _, bucket := range bucketList {
			bucketInfo = append(bucketInfo, &BucketInfo{
				VoterEthAddress: bucket.VoterAddress,
				WeightedVotes:   bucket.WeightedVotes,
			})
		}
		bucketInfoList.BucketInfo = bucketInfo
		bucketInfoLists = append(bucketInfoLists, bucketInfoList)
	}
	sort.Slice(bucketInfoLists, func(i, j int) bool { return bucketInfoLists[i].EpochNumber < bucketInfoLists[j].EpochNumber })
	delegateResponse.BucketInfo = &BucketInfoOutput{Exist: true, BucketInfoList: bucketInfoLists}
	return nil
}

func (r *queryResolver) getNumberOfCandidates(delegateResponse *Delegate, startEpoch int, epochCount int) error {
	return nil
}

func containField(requestedFields []string, field string) bool {
	for _, f := range requestedFields {
		if f == field {
			return true
		}
	}
	return false
}

func parseFieldArguments(ctx context.Context, fieldName string, selectedFieldName string) map[string]*ast.Value {
	fields := graphql.CollectFieldsCtx(ctx, nil)
	var field graphql.CollectedField
	for _, f := range fields {
		if f.Name == fieldName {
			field = f
		}
	}
	arguments := field.Arguments
	if selectedFieldName != "" {
		fields = graphql.CollectFields(ctx, field.Selections, nil)
		for _, f := range fields {
			if f.Name == selectedFieldName {
				field = f
			}
		}
		arguments = append(arguments, field.Arguments...)
	}
	argsMap := make(map[string]*ast.Value)
	for _, arg := range arguments {
		argsMap[arg.Name] = arg.Value
	}
	return argsMap
}

func getIntArg(argsMap map[string]*ast.Value, argName string) (int, error) {
	val, ok := argsMap[argName]
	if !ok {
		return 0, fmt.Errorf("%s is required", argName)
	}
	intVal, err := strconv.Atoi(val.Raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", argName)
	}
	return intVal, nil
}

func getBoolArg(argsMap map[string]*ast.Value, argName string) (bool, error) {
	val, ok := argsMap[argName]
	if !ok {
		return false, fmt.Errorf("%s is required", argName)
	}
	boolVal, err := strconv.ParseBool(val.Raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean value", argName)
	}
	return boolVal, nil
}

func getPaginationArgs(argsMap map[string]*ast.Value) (map[string]int, error) {
	pagination, ok := argsMap["pagination"]
	if !ok {
		return nil, ErrPaginationNotFound
	}
	childValueList := pagination.Children
	paginationMap := make(map[string]int)
	for _, childValue := range childValueList {
		intVal, err := strconv.Atoi(childValue.Value.Raw)
		if err != nil {
			return nil, errors.Wrap(err, "pagination value must be an integer")
		}
		paginationMap[childValue.Name] = intVal
	}
	return paginationMap, nil
}
