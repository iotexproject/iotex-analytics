// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mimo

import (
	"context"
	"math/big"
	"time"

	mimoprotocol "github.com/iotexproject/iotex-analytics/indexprotocol/mimo"
	"github.com/pkg/errors"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.
const (
	// HexPrefix is the prefix of ERC20 address in hex string
	HexPrefix = "0x"
	// DefaultPageSize is the size of page when pagination parameters are not set
	DefaultPageSize = 20
	// MaximumPageSize is the maximum size of page
	MaximumPageSize = 256
)

var (
	// ErrPaginationNotFound is the error indicating that pagination is not specified
	ErrPaginationNotFound = errors.New("pagination information is not found")
	// ErrPaginationInvalidOffset is the error indicating that pagination's offset parameter is invalid
	ErrPaginationInvalidOffset = errors.New("invalid pagination offset number")
	// ErrPaginationInvalidSize is the error indicating that pagination's size parameter is invalid
	ErrPaginationInvalidSize = errors.New("invalid pagination size number")
	// ErrInvalidParameter is the error indicating that invalid size
	ErrInvalidParameter = errors.New("invalid parameter number")
)

type queryResolver struct {
	service *mimoService
}

func (r *queryResolver) exchanges(ctx context.Context, height uint64, pairs []AddressPair) ([]*Exchange, error) {
	exchanges := make([]string, len(pairs))
	tokens := make([]string, len(pairs))
	reversePairs := make([]AddressPair, len(pairs))
	for i, pair := range pairs {
		exchanges[i] = pair.Address1
		tokens[i] = pair.Address2
		reversePairs[i] = AddressPair{
			Address1: pair.Address2,
			Address2: pair.Address1,
		}
	}
	balances, err := r.service.balances(height, exchanges)
	if err != nil {
		return nil, err
	}
	balances24HoursAgo, err := r.service.balances(height-720*24, exchanges)
	if err != nil {
		return nil, err
	}
	supplies, err := r.service.supplies(height, exchanges)
	if err != nil {
		return nil, err
	}
	tokenInfos, err := r.service.tokens(height, tokens)
	if err != nil {
		return nil, err
	}
	tokenBalances, err := r.service.tokenBalances(height, exchanges)
	if err != nil {
		return nil, err
	}
	tokenBalances24HoursAgo, err := r.service.tokenBalances(height-720*24, exchanges)
	if err != nil {
		return nil, err
	}
	volumesInPast24Hours, err := r.service.volumes(exchanges, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	volumesInPast48Hours, err := r.service.volumes(exchanges, 48*time.Hour)
	if err != nil {
		return nil, err
	}
	volumesInPast7Days, err := r.service.volumes(exchanges, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	ret := make([]*Exchange, 0, len(exchanges))
	for _, pair := range reversePairs {
		token := pair.Address1
		exchange := pair.Address2
		balance, ok := balances[exchange]
		if !ok {
			balance = big.NewInt(0)
		}
		tokenBalance, ok := tokenBalances[exchange]
		if !ok {
			tokenBalance = big.NewInt(0)
		}
		balance24HoursAgo, ok := balances24HoursAgo[exchange]
		if !ok {
			balance24HoursAgo = big.NewInt(0)
		}
		tokenBalance24HoursAgo, ok := tokenBalances24HoursAgo[exchange]
		if !ok {
			tokenBalance24HoursAgo = big.NewInt(0)
		}
		supply, ok := supplies[exchange]
		if !ok {
			supply = big.NewInt(0)
		}
		volumeInPast24Hours, ok := volumesInPast24Hours[exchange]
		if !ok {
			volumeInPast24Hours = big.NewInt(0)
		}
		volumeInPast48Hours, ok := volumesInPast48Hours[exchange]
		if !ok {
			volumeInPast48Hours = big.NewInt(0)
		}
		volumeInPast7Days, ok := volumesInPast7Days[exchange]
		if !ok {
			volumeInPast7Days = big.NewInt(0)
		}
		info, ok := tokenInfos[token]
		if !ok {
			info = Token{Address: token}
		}
		ret = append(ret, &Exchange{
			Address:                  exchange,
			Token:                    info,
			VolumeInPast24Hours:      volumeInPast24Hours.String(),
			VolumeInPast48Hours:      volumeInPast48Hours.String(),
			VolumeInPast7Days:        volumeInPast7Days.String(),
			Supply:                   supply.String(),
			BalanceOfIotx:            balance.String(),
			BalanceOfIOTX24HoursAgo:  balance24HoursAgo.String(),
			BalanceOfToken:           tokenBalance.String(),
			BalanceOfToken24HoursAgo: tokenBalance24HoursAgo.String(),
		})
	}
	return ret, nil
}

// Exchange returns an exchange given address
func (r *queryResolver) Exchange(ctx context.Context, exchange string) (*Exchange, error) {
	tip, err := r.service.latestHeight()
	if err != nil {
		return nil, err
	}
	pair, err := r.service.exchange(tip.Uint64(), exchange)
	if err != nil {
		return nil, err
	}
	values, err := r.exchanges(ctx, tip.Uint64(), []AddressPair{pair})
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.Errorf("failed to extract features of exchange %s", exchange)
	}
	return values[0], nil
}

// Exchanges returns all exchanges
func (r *queryResolver) Exchanges(ctx context.Context, pagination Pagination) ([]*Exchange, error) {
	if pagination.Skip < 0 {
		return nil, ErrPaginationInvalidOffset
	}
	if pagination.First <= 0 || pagination.First > MaximumPageSize {
		return nil, ErrPaginationInvalidSize
	}
	h, err := r.service.latestHeight()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tip height")
	}
	pairs, err := r.service.exchanges(h.Uint64(), uint32(pagination.Skip), uint8(pagination.First))
	if err != nil {
		return nil, err
	}
	return r.exchanges(ctx, h.Uint64(), pairs)
}

func (r *queryResolver) TipHeight(ctx context.Context) (string, error) {
	tip, err := r.service.latestHeight()
	if err != nil {
		return "", err
	}
	return tip.String(), nil
}

func (r *queryResolver) NumOfPairs(ctx context.Context) (int, error) {
	return r.service.numOfPairs()
}

func (r *queryResolver) Stats(ctx context.Context, hours int) (*Stats, error) {
	if hours < 0 {
		hours = 24
	}
	duration := time.Duration(hours) * time.Hour
	numOfTransactions, err := r.service.numOfTransactions(duration)
	if err != nil {
		return nil, err
	}
	volumes, err := r.service.volumes([]string{}, duration)
	if err != nil {
		return nil, err
	}
	totalVolume := big.NewInt(0)
	for _, volume := range volumes {
		totalVolume.Add(totalVolume, volume)
	}
	return &Stats{
		NumOfTransations: numOfTransactions,
		Volume:           totalVolume.String(),
	}, nil
}

func (r *queryResolver) Volumes(ctx context.Context, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.volumesInPastNDays([]string{}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) VolumesOfExchange(ctx context.Context, exchange string, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.volumesInPastNDays([]string{exchange}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) Liquidities(ctx context.Context, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.liquiditiesInPastNDays([]string{}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) LiquiditiesOfExchange(ctx context.Context, exchange string, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.liquiditiesInPastNDays([]string{exchange}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) TokenBalances(ctx context.Context, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.tokenBalancesInPastNDays([]string{}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) TokenBalancesOfExchange(ctx context.Context, exchange string, days int) ([]*AmountInOneDay, error) {
	if days < 0 {
		days = 30
	}
	if days > 256 {
		days = 256
	}
	dates, volumes, err := r.service.tokenBalancesInPastNDays([]string{exchange}, uint8(days))
	if err != nil {
		return nil, err
	}
	ret := []*AmountInOneDay{}
	for i, date := range dates {
		ret = append(ret, &AmountInOneDay{
			Amount: volumes[i].String(),
			Date:   date.UTC().String(),
		})
	}
	return ret, nil
}

func (r *queryResolver) Actions(ctx context.Context, actionType ActionType, pagination Pagination) ([]*Action, error) {
	if pagination.Skip < 0 {
		return nil, ErrPaginationInvalidOffset
	}
	if pagination.First <= 0 || pagination.First > MaximumPageSize {
		return nil, ErrPaginationInvalidSize
	}

	return r.service.actions([]string{}, actionTypeToTopics(actionType), pagination.Skip, pagination.First)
}

func actionTypeToTopics(actionType ActionType) []mimoprotocol.EventTopic {
	topics := []mimoprotocol.EventTopic{}
	switch actionType {
	case ActionTypeAll:
		topics = append(topics, mimoprotocol.AddLiquidity, mimoprotocol.RemoveLiquidity, mimoprotocol.TokenPurchase, mimoprotocol.CoinPurchase)
	case ActionTypeAdd:
		topics = append(topics, mimoprotocol.AddLiquidity)
	case ActionTypeRemove:
		topics = append(topics, mimoprotocol.RemoveLiquidity)
	case ActionTypeBuyCoin:
		topics = append(topics, mimoprotocol.CoinPurchase)
	case ActionTypeBuyToken:
		topics = append(topics, mimoprotocol.TokenPurchase)
	}
	return topics
}

func (r *queryResolver) ActionsOfExchange(ctx context.Context, exchange string, actionType ActionType, pagination Pagination) ([]*Action, error) {
	if pagination.Skip < 0 {
		return nil, ErrPaginationInvalidOffset
	}
	if pagination.First <= 0 || pagination.First > MaximumPageSize {
		return nil, ErrPaginationInvalidSize
	}

	return r.service.actions([]string{exchange}, actionTypeToTopics(actionType), pagination.Skip, pagination.First)
}
