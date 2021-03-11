package indexer

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	// AsyncIndexer is the interface of an async indexer of analytics
	AsyncIndexer interface {
		Start(context.Context) error
		Stop(context.Context) error
		NextHeight(context.Context) (uint64, error)
		PutBlock(context.Context, *block.Block) error
	}
)
