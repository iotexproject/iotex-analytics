package indexservice

import (
	"context"

	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type indexerCtxKey struct{}

// IndexerCtx provides the indexer with auxiliary information
type IndexerCtx struct {
	Client iotexapi.APIServiceClient
}

// WithIndexerCtx adds IndexerCtx into context
func WithIndexerCtx(ctx context.Context, indexerCtx IndexerCtx) context.Context {
	return context.WithValue(ctx, indexerCtxKey{}, indexerCtx)
}

// MustGetIndexerCtx must get indexer context
func MustGetIndexerCtx(ctx context.Context) IndexerCtx {
	indexerCtx, ok := ctx.Value(indexerCtxKey{}).(IndexerCtx)
	if !ok {
		log.S().Panic("Miss indexer context")
	}
	return indexerCtx
}

