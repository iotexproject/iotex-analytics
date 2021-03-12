package indexer

import (
	"context"

	"github.com/iotexproject/iotex-analytics/model"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"
)

type blockMetaIndexer struct {
	iht   *IndexHeightTable
	log   *zap.Logger
	model model.Schema
}

// NewBlockIndexer returns the
func NewBlockMetaIndexer(store s.Store) AsyncIndexer {
	name := "blockMetaIndexer"
	return &blockMetaIndexer{
		iht: &IndexHeightTable{
			Name: name,
		},
		log:   log.Logger(name),
		model: model.NewBlockMeta(store),
	}
}

func (bi *blockMetaIndexer) Start(ctx context.Context) error {

	if err := bi.iht.Init(ctx); err != nil {
		return err
	}
	return bi.model.Initialize()
}

func (bi *blockMetaIndexer) Stop(ctx context.Context) error {
	return nil
}

func (bi *blockMetaIndexer) NextHeight(ctx context.Context) (uint64, error) {
	height, err := bi.iht.Height(ctx)
	if err != nil {
		return 0, err
	}
	nextHeight := height + 1
	return nextHeight, nil
}

func (bi *blockMetaIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	var gasConsumed uint64
	for _, receipt := range blk.Receipts {
		gasConsumed += receipt.GasConsumed
	}

	insertData := model.Fields{
		"block_height":  blk.Height(),
		"gas_consumed":  gasConsumed,
		"producer_name": "",
	}

	if err := bi.model.Insert(insertData); err != nil {
		return err
	}
	bi.iht.Upsert(ctx, blk.Height())
	return nil
}
