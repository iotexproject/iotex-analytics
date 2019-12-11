// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package graphql

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/ast"
	"golang.org/x/sync/errgroup"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/queryprotocol/actions"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta"
	"github.com/iotexproject/iotex-analytics/queryprotocol/productivity"
	"github.com/iotexproject/iotex-analytics/queryprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/queryprotocol/votings"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.
const (
	// HexPrefix is the prefix of ERC20 address in hex string
	HexPrefix = "0x"
	// DefaultPageSize is the size of page when pagination parameters are not set
	DefaultPageSize = 200
	// MaximumPageSize is the maximum size of page
	MaximumPageSize = 500
)

var (
	// ErrPaginationNotFound is the error indicating that pagination is not specified
	ErrPaginationNotFound = errors.New("pagination information is not found")
	// ErrPaginationInvalidOffset is the error indicating that pagination's offset parameter is invalid
	ErrPaginationInvalidOffset = errors.New("invalid pagination offset number")
	// ErrPaginationInvalidSize is the error indicating that pagination's size parameter is invalid
	ErrPaginationInvalidSize = errors.New("invalid pagination size number")
)

// EncodeDelegateName converts a delegate name input to an internal format
func EncodeDelegateName(name string) (string, error) {
	l := len(name)
	switch {
	case l == 24:
		return name, nil
	case l <= 12:
		prefixZeros := []byte{}
		for i := 0; i < 12-len(name); i++ {
			prefixZeros = append(prefixZeros, byte(0))
		}
		suffixZeros := []byte{}
		for strings.HasSuffix(name, "#") {
			name = strings.TrimSuffix(name, "#")
			suffixZeros = append(suffixZeros, byte(0))
		}
		return hex.EncodeToString(append(append(prefixZeros, []byte(name)...), suffixZeros...)), nil
	}
	return "", errors.Errorf("invalid length %d", l)
}

// DecodeDelegateName converts format to readable delegate name
func DecodeDelegateName(name string) (string, error) {
	suffix := ""
	for strings.HasSuffix(name, "00") {
		name = strings.TrimSuffix(name, "00")
		suffix += "#"
	}
	aliasBytes, err := hex.DecodeString(strings.TrimLeft(name, "0"))
	if err != nil {
		return "", err
	}
	aliasString := string(aliasBytes) + suffix
	return aliasString, nil
}

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
	if containField(requestedFields, "alias") {
		g.Go(func() error { return r.getAlias(ctx, accountResponse) })
	}
	if containField(requestedFields, "operatorAddress") {
		g.Go(func() error { return r.getOperatorAddress(ctx, accountResponse) })
	}

	return accountResponse, g.Wait()
}

func (r *queryResolver) Action(ctx context.Context) (*Action, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	actionResponse := &Action{}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "byDates") {
		g.Go(func() error { return r.getActionsByDates(ctx, actionResponse) })
	}
	if containField(requestedFields, "byHash") {
		g.Go(func() error { return r.getActionByHash(ctx, actionResponse) })
	}
	if containField(requestedFields, "byAddress") {
		g.Go(func() error { return r.getActionsByAddress(ctx, actionResponse) })
	}
	if containField(requestedFields, "evmTransfersByAddress") {
		g.Go(func() error { return r.getEvmTransfersByAddress(ctx, actionResponse) })
	}
	return actionResponse, g.Wait()
}

// Chain handles chain requests
func (r *queryResolver) Chain(ctx context.Context) (*Chain, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	chainResponse := &Chain{}

	if err := r.getLastEpochAndHeight(chainResponse); err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "votingResultMeta") {
		g.Go(func() error { return r.getVotingResultMeta(chainResponse) })
	}
	if containField(requestedFields, "mostRecentTPS") {
		g.Go(func() error { return r.getTPS(ctx, chainResponse) })
	}
	if containField(requestedFields, "numberOfActions") {
		g.Go(func() error { return r.getNumberOfActions(ctx, chainResponse) })
	}

	return chainResponse, g.Wait()
}

// Delegate handles delegate requests
func (r *queryResolver) Delegate(ctx context.Context, startEpoch int, epochCount int, delegateName string) (*Delegate, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	delegateResponse := &Delegate{}

	delegateName, err := EncodeDelegateName(delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to format delegate name")
	}

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
		g.Go(func() error { return r.getBucketInfo(ctx, delegateResponse, startEpoch, epochCount, delegateName) })
	}
	if containField(requestedFields, "staking") {
		g.Go(func() error { return r.getStaking(delegateResponse, startEpoch, epochCount, delegateName) })
	}
	return delegateResponse, g.Wait()
}

// Voting handles voting requests
func (r *queryResolver) Voting(ctx context.Context, startEpoch int, epochCount int) (*Voting, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	votingResponse := &Voting{}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "candidateInfo") {
		g.Go(func() error { return r.getCandidateList(votingResponse, startEpoch, epochCount) })
	}
	if containField(requestedFields, "votingMeta") {
		g.Go(func() error { return r.getVotingMeta(votingResponse, startEpoch, epochCount) })
	}
	if containField(requestedFields, "rewardSources") {
		g.Go(func() error { return r.getRewardSources(ctx, votingResponse, startEpoch, epochCount) })
	}
	return votingResponse, g.Wait()
}

// Hermes handles Hermes bookkeeping requests
func (r *queryResolver) Hermes(ctx context.Context, startEpoch int, epochCount int, rewardAddress string, waiverThreshold int) (*Hermes, error) {
	hermes, err := r.RP.GetHermesBookkeeping(uint64(startEpoch), uint64(epochCount), rewardAddress, uint64(waiverThreshold))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		return &Hermes{Exist: false}, nil
	case err != nil:
		return nil, errors.Wrap(err, "failed to get hermes bookkeeping information")
	}

	hermesDistribution := make([]*HermesDistribution, 0, len(hermes))
	for _, ret := range hermes {
		rds := make([]*RewardDistribution, 0)
		for _, distribution := range ret.Distributions {
			v := &RewardDistribution{
				VoterEthAddress:   HexPrefix + distribution.VoterEthAddress,
				VoterIotexAddress: distribution.VoterIotexAddress,
				Amount:            distribution.Amount,
			}
			rds = append(rds, v)
		}
		sort.Slice(rds, func(i, j int) bool { return rds[i].VoterEthAddress < rds[j].VoterEthAddress })

		aliasString, err := DecodeDelegateName(ret.DelegateName)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode delegate name")
		}
		hermesDistribution = append(hermesDistribution, &HermesDistribution{
			DelegateName:        aliasString,
			RewardDistribution:  rds,
			StakingIotexAddress: ret.StakingIotexAddress,
			VoterCount:          int(ret.VoterCount),
			WaiveServiceFee:     ret.WaiveServiceFee,
			Refund:              ret.Refund,
		})
	}
	sort.Slice(hermesDistribution, func(i, j int) bool { return hermesDistribution[i].DelegateName < hermesDistribution[j].DelegateName })
	return &Hermes{Exist: true, HermesDistribution: hermesDistribution}, nil
}

// Xrc20 handles Xrc20 requests
func (r *queryResolver) Xrc20(ctx context.Context) (*Xrc20, error) {
	requestedFields := graphql.CollectAllFields(ctx)
	actionResponse := &Xrc20{}

	g, ctx := errgroup.WithContext(ctx)
	if containField(requestedFields, "byContractAddress") {
		g.Go(func() error { return r.getXrc20ByContractAddress(ctx, actionResponse) })
	}
	if containField(requestedFields, "byAddress") {
		g.Go(func() error { return r.getXrc20ByAddress(ctx, actionResponse) })
	}
	if containField(requestedFields, "byPage") {
		g.Go(func() error { return r.getXrc20ByPage(ctx, actionResponse) })
	}
	return actionResponse, g.Wait()
}

// TopHolders handles top holders requests
func (r *queryResolver) TopHolders(ctx context.Context, endEpochNumber int, pagination Pagination) ([]*TopHolder, error) {
	if pagination.Skip < 0 {
		return nil, ErrPaginationInvalidOffset
	}
	if pagination.First <= 0 || pagination.First > MaximumPageSize {
		return nil, ErrPaginationInvalidSize
	}
	holders, err := r.AP.GetTopHolders(uint64(endEpochNumber), uint64(pagination.Skip), uint64(pagination.First))
	if err != nil {
		return nil, err
	}
	ret := make([]*TopHolder, 0)
	for _, h := range holders {
		t := &TopHolder{
			Address: h.Address,
			Balance: h.Balance,
		}
		ret = append(ret, t)
	}
	return ret, nil
}

func (r *queryResolver) getOperatorAddress(ctx context.Context, accountResponse *Account) error {
	argsMap := parseFieldArguments(ctx, "operatorAddress", "")
	val, err := getStringArg(argsMap, "aliasName")
	if err != nil {
		return errors.Wrap(err, "aliasName is required")
	}
	aName, err := EncodeDelegateName(val)
	if err != nil {
		return err
	}
	opAddress, err := r.VP.GetOperatorAddress(aName)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		accountResponse.OperatorAddress = &OperatorAddress{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get operator address")
	}
	accountResponse.OperatorAddress = &OperatorAddress{
		Exist:           true,
		OperatorAddress: opAddress,
	}
	return nil
}

func (r *queryResolver) getAlias(ctx context.Context, accountResponse *Account) error {
	argsMap := parseFieldArguments(ctx, "alias", "")
	opAddress, err := getStringArg(argsMap, "operatorAddress")
	if err != nil {
		return errors.Wrap(err, "operatorAddress is required")
	}
	aliasName, err := r.VP.GetAlias(opAddress)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		accountResponse.Alias = &Alias{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get alias name")
	}
	aliasString, err := DecodeDelegateName(aliasName)
	if err != nil {
		return err
	}
	accountResponse.Alias = &Alias{
		Exist:     true,
		AliasName: aliasString,
	}
	return nil
}

func (r *queryResolver) getCandidateList(votingResponse *Voting, startEpoch int, epochCount int) error {
	candidatesInfoMap, err := r.VP.GetCandidates(uint64(startEpoch), uint64(epochCount))
	if err != nil {
		return errors.Wrap(err, "failed to get candidate info")
	}
	candidateInfoLists := make([]*CandidateInfoList, 0)
	for epoch, candidateList := range candidatesInfoMap {
		candidateInfoByEpoch := &CandidateInfoList{
			EpochNumber: int(epoch),
		}
		candidates := make([]*CandidateInfo, 0)
		for _, cand := range candidateList {
			candidates = append(candidates, &CandidateInfo{
				Name:               cand.Name,
				Address:            cand.Address,
				TotalWeightedVotes: cand.TotalWeightedVotes,
				SelfStakingTokens:  cand.SelfStakingTokens,
				OperatorAddress:    cand.OperatorAddress,
				RewardAddress:      cand.RewardAddress,
			})
		}
		candidateInfoByEpoch.Candidates = candidates
		candidateInfoLists = append(candidateInfoLists, candidateInfoByEpoch)
	}

	votingResponse.CandidateInfo = candidateInfoLists
	return nil
}

func (r *queryResolver) getVotingMeta(votingResponse *Voting, startEpoch int, epochCount int) error {
	cl, numConsensusDelegates, err := r.VP.GetCandidateMeta(uint64(startEpoch), uint64(epochCount))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		votingResponse.VotingMeta = &VotingMeta{Exist: false}
	case err != nil:
		return errors.Wrap(err, "failed to get candidate metadata")
	}
	candidateMetaList := make([]*CandidateMeta, 0)
	for _, candidateMeta := range cl {
		candidateMetaList = append(candidateMetaList, &CandidateMeta{
			EpochNumber:        int(candidateMeta.EpochNumber),
			TotalCandidates:    int(candidateMeta.NumberOfCandidates),
			ConsensusDelegates: int(numConsensusDelegates),
			TotalWeightedVotes: candidateMeta.TotalWeightedVotes,
			VotedTokens:        candidateMeta.VotedTokens,
		})
	}
	votingResponse.VotingMeta = &VotingMeta{
		Exist:         true,
		CandidateMeta: candidateMetaList,
	}
	return nil
}

func (r *queryResolver) getVotingResultMeta(chainResponse *Chain) error {
	targetEpoch := uint64(chainResponse.MostRecentEpoch)
	cl, _, err := r.VP.GetCandidateMeta(targetEpoch, 1)
	if err != nil {
		return errors.Wrap(err, "failed to get candidate metadata")
	}
	if len(cl) != 1 {
		return errors.Wrap(err, "the output length should be 1")
	}
	candidateMeta := cl[0]
	chainResponse.VotingResultMeta = &VotingResultMeta{
		TotalCandidates:    int(candidateMeta.NumberOfCandidates),
		TotalWeightedVotes: candidateMeta.TotalWeightedVotes,
		VotedTokens:        candidateMeta.VotedTokens,
	}
	return nil
}

func (r *queryResolver) getRewardSources(ctx context.Context, votingResponse *Voting, startEpoch int, epochCount int) error {
	argsMap := parseFieldArguments(ctx, "rewardSources", "")
	voterIotexAddress, err := getStringArg(argsMap, "voterIotexAddress")
	if err != nil {
		return errors.Wrap(err, "voter's IoTeX address is required")
	}
	delegateDistributions, err := r.RP.GetRewardSources(uint64(startEpoch), uint64(epochCount), voterIotexAddress)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		votingResponse.RewardSources = &RewardSources{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get reward sources for the voter")
	}

	delegateAmount := make([]*DelegateAmount, 0)
	for _, ret := range delegateDistributions {
		aliasString, err := DecodeDelegateName(ret.DelegateName)
		if err != nil {
			return errors.Wrap(err, "failed to decode delegate name")
		}
		v := &DelegateAmount{
			DelegateName: aliasString,
			Amount:       ret.Amount,
		}
		delegateAmount = append(delegateAmount, v)
	}
	sort.Slice(delegateAmount, func(i, j int) bool { return delegateAmount[i].DelegateName < delegateAmount[j].DelegateName })
	votingResponse.RewardSources = &RewardSources{
		Exist:                 true,
		DelegateDistributions: delegateAmount,
	}
	return nil
}

func (r *queryResolver) getStaking(delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	rl, err := r.VP.GetStaking(uint64(startEpoch), uint64(epochCount), delegateName)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		delegateResponse.Staking = &StakingOutput{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get reward information")
	}
	stakingInfoList := make([]*StakingInformation, 0)
	for _, stakingInfo := range rl {
		stakingInfoList = append(stakingInfoList, &StakingInformation{
			EpochNumber:  int(stakingInfo.EpochNumber),
			TotalStaking: stakingInfo.TotalStaking,
			SelfStaking:  stakingInfo.SelfStaking,
		})
	}
	delegateResponse.Staking = &StakingOutput{
		Exist:       true,
		StakingInfo: stakingInfoList,
	}
	return nil
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

func (r *queryResolver) getActionsByDates(ctx context.Context, actionResponse *Action) error {
	argsMap := parseFieldArguments(ctx, "byDates", "actions")
	startDate, err := getIntArg(argsMap, "startDate")
	if err != nil {
		return errors.Wrap(err, "failed to get start date")
	}
	endDate, err := getIntArg(argsMap, "endDate")
	if err != nil {
		return errors.Wrap(err, "failed to get end date")
	}
	if startDate > endDate {
		return errors.New("invalid dates")
	}

	var offset, size int

	paginationMap, err := getPaginationArgs(argsMap)
	// TODO: put the switch part into getPaginationArgs() after pagination optimization
	switch {
	default:
		// TODO: rename skip/first into offset/size
		offset = paginationMap["skip"]
		if offset < 0 {
			return ErrPaginationInvalidOffset
		}
		size = paginationMap["first"]
		if size <= 0 || size > MaximumPageSize {
			return ErrPaginationInvalidSize
		}
	case err == ErrPaginationNotFound:
		offset = 0
		size = DefaultPageSize
	case err != nil:
		return errors.Wrap(err, "failed to get pagination arguments for actions")
	}

	actionInfoList, err := r.AP.GetActionsByDates(uint64(startDate), uint64(endDate), offset, size)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		actionResponse.ByDates = &ActionList{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get actions' information")
	}

	actInfoList := make([]*ActionInfo, 0, len(actionInfoList))
	for _, act := range actionInfoList {
		actInfoList = append(actInfoList, &ActionInfo{
			ActHash:   act.ActHash,
			BlkHash:   act.BlkHash,
			ActType:   act.ActType,
			TimeStamp: int(act.TimeStamp),
			Sender:    act.Sender,
			Recipient: act.Recipient,
			Amount:    act.Amount,
			GasFee:    act.GasFee,
		})
	}

	actionResponse.ByDates = &ActionList{Exist: true, Actions: actInfoList, Count: len(actInfoList)}

	return nil
}

func (r *queryResolver) getActionsByAddress(ctx context.Context, actionResponse *Action) error {
	argsMap := parseFieldArguments(ctx, "byAddress", "actions")
	addr, err := getStringArg(argsMap, "address")
	if err != nil {
		return errors.Wrap(err, "failed to get address")
	}

	var offset, size int

	paginationMap, err := getPaginationArgs(argsMap)
	switch {
	default:
		offset = paginationMap["skip"]
		if offset < 0 {
			return ErrPaginationInvalidOffset
		}
		size = paginationMap["first"]
		if size <= 0 || size > MaximumPageSize {
			return ErrPaginationInvalidSize
		}
	case err == ErrPaginationNotFound:
		offset = 0
		size = DefaultPageSize
	case err != nil:
		return errors.Wrap(err, "failed to get pagination arguments for actions")
	}

	actionInfoList, err := r.AP.GetActionsByAddress(addr, offset, size)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		actionResponse.ByAddress = &ActionList{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get actions' information")
	}

	actInfoList := make([]*ActionInfo, 0, len(actionInfoList))
	for _, act := range actionInfoList {
		actInfoList = append(actInfoList, &ActionInfo{
			ActHash:   act.ActHash,
			BlkHash:   act.BlkHash,
			ActType:   act.ActType,
			TimeStamp: int(act.TimeStamp),
			Sender:    act.Sender,
			Recipient: act.Recipient,
			Amount:    act.Amount,
			GasFee:    act.GasFee,
		})
	}

	actionResponse.ByAddress = &ActionList{Exist: true, Actions: actInfoList, Count: len(actInfoList)}

	return nil
}

func (r *queryResolver) getEvmTransfersByAddress(ctx context.Context, actionResponse *Action) error {
	argsMap := parseFieldArguments(ctx, "evmTransfersByAddress", "evmTransfers")
	addr, err := getStringArg(argsMap, "address")
	if err != nil {
		return errors.Wrap(err, "failed to get address")
	}

	var offset, size int

	paginationMap, err := getPaginationArgs(argsMap)
	switch {
	default:
		offset = paginationMap["skip"]
		if offset < 0 {
			return ErrPaginationInvalidOffset
		}
		size = paginationMap["first"]
		if size <= 0 || size > MaximumPageSize {
			return ErrPaginationInvalidSize
		}
	case err == ErrPaginationNotFound:
		offset = 0
		size = DefaultPageSize
	case err != nil:
		return errors.Wrap(err, "failed to get pagination arguments for actions")
	}

	evmTransferDetailList, err := r.AP.GetEvmTransferDetailListByAddress(addr, offset, size)
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		actionResponse.EvmTransfersByAddress = &EvmTransferList{Exist: false}
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get evm transfers")
	}

	evmTransfers := make([]*EvmTransferDetail, 0, len(evmTransferDetailList))
	for _, etf := range evmTransferDetailList {
		evmTransfers = append(evmTransfers, &EvmTransferDetail{
			From:      etf.From,
			To:        etf.To,
			Quantity:  etf.Quantity,
			ActHash:   etf.ActHash,
			BlkHash:   etf.BlkHash,
			TimeStamp: int(etf.TimeStamp),
		})
	}

	actionResponse.EvmTransfersByAddress = &EvmTransferList{Exist: true, EvmTransfers: evmTransfers, Count: len(evmTransfers)}

	return nil
}

func (r *queryResolver) getXrc20ByContractAddress(ctx context.Context, actionResponse *Xrc20) error {
	argsMap := parseFieldArguments(ctx, "byContractAddress", "xrc20")
	address, err := getStringArg(argsMap, "address")
	if err != nil {
		return errors.Wrap(err, "failed to get address")
	}
	numPerPage, err := getIntArg(argsMap, "numPerPage")
	if err != nil {
		return errors.Wrap(err, "failed to get numPerPage")
	}
	page, err := getIntArg(argsMap, "page")
	if err != nil {
		return errors.Wrap(err, "failed to get page")
	}
	output := &Xrc20List{Exist: false}
	actionResponse.ByContractAddress = output
	xrc20InfoList, err := r.AP.GetXrc20(address, uint64(numPerPage), uint64(page))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get contract information")
	}
	output.Exist = true
	output.Count = len(xrc20InfoList)
	output.Xrc20 = make([]*Xrc20Info, 0, len(xrc20InfoList))
	for _, c := range xrc20InfoList {
		output.Xrc20 = append(output.Xrc20, &Xrc20Info{
			Hash:      c.Hash,
			Timestamp: c.Timestamp,
			From:      c.From,
			To:        c.To,
			Quantity:  c.Quantity,
			Contract:  c.Contract,
		})
	}
	return nil
}

func (r *queryResolver) getXrc20ByAddress(ctx context.Context, actionResponse *Xrc20) error {
	argsMap := parseFieldArguments(ctx, "byAddress", "xrc20")
	address, err := getStringArg(argsMap, "address")
	if err != nil {
		return errors.Wrap(err, "failed to get address")
	}
	numPerPage, err := getIntArg(argsMap, "numPerPage")
	if err != nil {
		return errors.Wrap(err, "failed to get numPerPage")
	}
	page, err := getIntArg(argsMap, "page")
	if err != nil {
		return errors.Wrap(err, "failed to get page")
	}
	output := &Xrc20List{Exist: false}
	actionResponse.ByAddress = output
	xrc20InfoList, err := r.AP.GetXrc20ByAddress(address, uint64(numPerPage), uint64(page))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get contract information")
	}
	output.Exist = true
	output.Count = len(xrc20InfoList)
	output.Xrc20 = make([]*Xrc20Info, 0, len(xrc20InfoList))
	for _, c := range xrc20InfoList {
		output.Xrc20 = append(output.Xrc20, &Xrc20Info{
			Hash:      c.Hash,
			Timestamp: c.Timestamp,
			From:      c.From,
			To:        c.To,
			Quantity:  c.Quantity,
			Contract:  c.Contract,
		})
	}
	return nil
}

func (r *queryResolver) getXrc20ByPage(ctx context.Context, actionResponse *Xrc20) error {
	argsMap := parseFieldArguments(ctx, "byPage", "xrc20")
	numPerPage, err := getIntArg(argsMap, "numPerPage")
	if err != nil {
		return errors.Wrap(err, "failed to get numPerPage")
	}
	page, err := getIntArg(argsMap, "page")
	if err != nil {
		return errors.Wrap(err, "failed to get page")
	}
	output := &Xrc20List{Exist: false}
	actionResponse.ByPage = output
	xrc20InfoList, err := r.AP.GetXrc20ByPage(uint64(numPerPage), uint64(page))
	switch {
	case errors.Cause(err) == indexprotocol.ErrNotExist:
		return nil
	case err != nil:
		return errors.Wrap(err, "failed to get contract information")
	}
	output.Exist = true
	output.Count = len(xrc20InfoList)
	output.Xrc20 = make([]*Xrc20Info, 0, len(xrc20InfoList))
	for _, c := range xrc20InfoList {
		output.Xrc20 = append(output.Xrc20, &Xrc20Info{
			Hash:      c.Hash,
			Timestamp: c.Timestamp,
			From:      c.From,
			To:        c.To,
			Quantity:  c.Quantity,
			Contract:  c.Contract,
		})
	}
	return nil
}

func (r *queryResolver) getActionByHash(ctx context.Context, actionResponse *Action) error {
	argsMap := parseFieldArguments(ctx, "byHash", "")
	hash, err := getStringArg(argsMap, "actHash")
	if err != nil {
		return errors.Wrap(err, "failed to get hash")
	}

	actDetail, err := r.AP.GetActionDetailByHash(hash)
	if err != nil {
		return errors.Wrap(err, "failed to get action details by hash")
	}
	actionOutput := &ActionDetail{ActionInfo: &ActionInfo{
		ActHash:   actDetail.ActionInfo.ActHash,
		BlkHash:   actDetail.ActionInfo.BlkHash,
		TimeStamp: int(actDetail.ActionInfo.TimeStamp),
		ActType:   actDetail.ActionInfo.ActType,
		Sender:    actDetail.ActionInfo.Sender,
		Recipient: actDetail.ActionInfo.Recipient,
		Amount:    actDetail.ActionInfo.Amount,
		GasFee:    actDetail.ActionInfo.GasFee,
	}}
	for _, evmTransfer := range actDetail.EvmTransfers {
		actionOutput.EvmTransfers = append(actionOutput.EvmTransfers, &EvmTransfer{
			From:     evmTransfer.From,
			To:       evmTransfer.To,
			Quantity: evmTransfer.Quantity,
		})
	}
	actionResponse.ByHash = actionOutput
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
		return errors.Wrap(err, "failed to get pagination arguments for actions")
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

func (r *queryResolver) getBucketInfo(ctx context.Context, delegateResponse *Delegate, startEpoch int, epochCount int, delegateName string) error {
	argsMap := parseFieldArguments(ctx, "bucketInfo", "bucketInfoList")
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
			voterIotexAddr, err := ethAddrToIoAddr(bucket.VoterAddress)
			if err != nil {
				return errors.Wrap(err, "failed to convert eth address to IoTeX address")
			}
			bucketInfo = append(bucketInfo, &BucketInfo{
				VoterEthAddress:   HexPrefix + bucket.VoterAddress,
				VoterIotexAddress: voterIotexAddr,
				IsNative:          bucket.IsNative,
				Votes:             bucket.Votes,
				WeightedVotes:     bucket.WeightedVotes,
				RemainingDuration: bucket.RemainingDuration,
				StartTime:         bucket.StartTime,
				Decay:             bucket.Decay,
			})
		}
		paginationMap, err := getPaginationArgs(argsMap)
		switch {
		case err == ErrPaginationNotFound:
			bucketInfoList.BucketInfo = bucketInfo
		case err != nil:
			return errors.Wrap(err, "failed to get pagination arguments for bucket info")
		default:
			skip := paginationMap["skip"]
			first := paginationMap["first"]
			if skip < 0 || skip >= len(bucketInfo) {
				return errors.New("invalid pagination skip number for bucket info")
			}
			if len(bucketInfo)-skip < first {
				first = len(bucketInfo) - skip
			}
			bucketInfoList.BucketInfo = bucketInfo[skip : skip+first]
		}
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
	parseVariables(ctx, argsMap, arguments)
	return argsMap
}
func parseVariables(ctx context.Context, argsMap map[string]*ast.Value, arguments ast.ArgumentList) {
	val := graphql.GetRequestContext(ctx)
	if val != nil {
		for _, arg := range arguments {
			if arg == nil {
				continue
			}
			switch arg.Value.ExpectedType.Name() {
			case "String":
				value, ok := val.Variables[arg.Name].(string)
				if ok {
					argsMap[arg.Name].Raw = value
				}
			case "Int":
				valueJSON, ok := val.Variables[arg.Name].(json.Number)
				if ok {
					value, err := valueJSON.Int64()
					if err != nil {
						return
					}
					argsMap[arg.Name].Raw = fmt.Sprintf("%d", value)
				}
			case "Pagination":
				value, ok := val.Variables[arg.Name].(map[string]interface{})
				if ok {
					for k, v := range value {
						valueJSON, ok := v.(json.Number)
						if ok {
							valueInt64, err := valueJSON.Int64()
							if err != nil {
								continue
							}
							child := &ast.ChildValue{Name: k, Value: &ast.Value{Raw: fmt.Sprintf("%d", valueInt64)}}
							argsMap[arg.Name].Children = append(argsMap[arg.Name].Children, child)
						}
					}
				}
			default:
				return
			}
		}
	}
}
func getIntArg(argsMap map[string]*ast.Value, argName string) (int, error) {
	getStr, err := getStringArg(argsMap, argName)
	if err != nil {
		return 0, err
	}
	intVal, err := strconv.Atoi(getStr)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", argName)
	}
	return intVal, nil
}

func getStringArg(argsMap map[string]*ast.Value, argName string) (string, error) {
	val, ok := argsMap[argName]
	if !ok {
		return "", fmt.Errorf("%s is required", argName)
	}
	return string(val.Raw), nil
}

func getBoolArg(argsMap map[string]*ast.Value, argName string) (bool, error) {
	getStr, err := getStringArg(argsMap, argName)
	if err != nil {
		return false, err
	}
	boolVal, err := strconv.ParseBool(getStr)
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

func ethAddrToIoAddr(ethAddr string) (string, error) {
	ethAddress := common.HexToAddress(ethAddr)
	ioAddress, err := address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return "", err
	}
	return ioAddress.String(), nil
}
