package indexer

import (
	"context"
	"encoding/hex"

	"github.com/iotexproject/iotex-analytics/model"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type blockIndexer struct {
	iht   *IndexHeightTable
	log   *zap.Logger
	model model.Schema
}

// NewBlockIndexer returns the
func NewBlockIndexer(store s.Store) AsyncIndexer {
	name := "blockindexer"
	return &blockIndexer{
		iht: &IndexHeightTable{
			Name: name,
		},
		log:   log.Logger(name),
		model: model.NewBlock(store),
	}
}

func (bi *blockIndexer) Start(ctx context.Context) error {

	if err := bi.iht.Init(ctx); err != nil {
		return err
	}
	return bi.model.Initialize()
}

func (bi *blockIndexer) Stop(ctx context.Context) error {
	return nil
}

func (bi *blockIndexer) NextHeight(ctx context.Context) (uint64, error) {
	height, err := bi.iht.Height(ctx)
	if err != nil {
		return 0, err
	}
	nextHeight := height + 1
	return nextHeight, nil
}

func (bi *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	blkHash := blk.HashBlock()

	fields := model.Fields{
		"block_height":     blk.Height(),
		"block_hash":       hex.EncodeToString(blkHash[:]),
		"producer_address": blk.ProducerAddress(),
		"timestamp":        blk.Timestamp().Unix(),
	}
	if err := bi.model.Insert(fields); err != nil {
		return errors.Wrap(err, "failed to insert data in blockIndexer")
	}
	bi.iht.Upsert(ctx, blk.Height())
	return nil
}
