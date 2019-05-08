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
