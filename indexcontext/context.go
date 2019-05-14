// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexcontext

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

type indexCtxKey struct{}

// IndexCtx provides the indexer with auxiliary information
type IndexCtx struct {
	ChainClient    iotexapi.APIServiceClient
	ElectionClient api.APIServiceClient
}

// WithIndexCtx adds IndexCtx into context
func WithIndexCtx(ctx context.Context, indexCtx IndexCtx) context.Context {
	return context.WithValue(ctx, indexCtxKey{}, indexCtx)
}

// MustGetIndexCtx must get index context
func MustGetIndexCtx(ctx context.Context) IndexCtx {
	indexCtx, ok := ctx.Value(indexCtxKey{}).(IndexCtx)
	if !ok {
		log.S().Panic("Miss index context")
	}
	return indexCtx
}
