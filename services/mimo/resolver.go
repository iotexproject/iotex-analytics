// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mimo

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/services/mimo/generated"
	"github.com/iotexproject/iotex-analytics/services/mimo/model"
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
	// ErrInvalidParameter is the error indicating that invalid size
	ErrInvalidParameter = errors.New("invalid parameter number")
)

// Resolver is hte resolver that handles GraphQL request
type Resolver struct {
}

// Query returns a query resolver
func (r *Resolver) Query() generated.QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

// Exchanges returns all exchanges
func (r *queryResolver) Exchanges(ctx context.Context, height string, pagination model.Pagination) ([]*model.Exchange, error) {
	/*if pagination.Skip < 0 {
		return nil, ErrPaginationInvalidOffset
	}
	if pagination.First <= 0 || pagination.First > MaximumPageSize {
		return nil, ErrPaginationInvalidSize
	}
	holders, err := r.AP.GetTopHolders(uint64(endEpochNumber), uint64(pagination.Skip), uint64(pagination.First))
	if err != nil {
		return nil, err
	}
	ret := make([]*model.Exchange, 0)
	for _, h := range holders {
		t := &model.Exchange{
			Address: h.Address,
			Balance: h.Balance,
		}
		ret = append(ret, t)
	}
	return ret, nil
	*/
	return nil, nil
}
