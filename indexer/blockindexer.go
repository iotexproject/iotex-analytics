package indexer

import (
	"context"

	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"
)

/**
 block table
CREATE TABLE `block_history` (
  `epoch_number` decimal(65,0) NOT NULL,
  `block_height` decimal(65,0) NOT NULL,
  `block_hash` varchar(64) NOT NULL,
  `gas_consumed` decimal(65,0) NOT NULL,
  `producer_address` varchar(41) NOT NULL,
  `producer_name` varchar(24) NOT NULL,
  `expected_producer_address` varchar(41) NOT NULL,
  `expected_producer_name` varchar(24) NOT NULL,
  `timestamp` decimal(65,0) NOT NULL,
  PRIMARY KEY (`block_height`),
  KEY `block_height_index` (`block_height`),
  KEY `epoch_producer_index` (`epoch_number`,`producer_name`,`expected_producer_name`),
  KEY `timestamp_index` (`timestamp`),
  KEY `epoch_number_index` (`epoch_number`),
  KEY `expected_producer_name_index` (`expected_producer_name`),
  KEY `expected_producer_address_index` (`expected_producer_address`),
  KEY `producer_name_index` (`producer_name`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
*/

type blockIndexer struct {
	iht *IndexHeightTable
	log *zap.Logger
}

// NewBlockIndexer returns the
func NewBlockIndexer(store s.Store) AsyncIndexer {
	return &blockIndexer{
		iht: &IndexHeightTable{
			Name: "blockindexer",
		},
		log: log.Logger("blockindexer"),
	}
}

func (bi *blockIndexer) Start(ctx context.Context) error {
	bi.log.Debug("blockindex start")

	return bi.iht.Init(ctx)
}

func (bi *blockIndexer) Stop(ctx context.Context) error {
	bi.log.Debug("blockindex stop")

	return nil
}

func (bi *blockIndexer) NextHeight(ctx context.Context) (uint64, error) {
	height, err := bi.iht.Height(ctx)
	if err != nil {
		return 0, err
	}
	nextHeight := height + 1
	bi.log.Debug("blockIndexer nextHeight", zap.Uint64("nextHeight", nextHeight))
	return nextHeight, nil
}

func (bi *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	bi.iht.Upsert(ctx, blk.Height())
	bi.log.Debug("blockindexer putblock", zap.Any("block", blk), zap.Time("blockTime", blk.Header.Timestamp()))
	return nil
}
