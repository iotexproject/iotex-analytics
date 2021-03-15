package indexer

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type blockIndexer struct {
	log         *zap.Logger
	tablename   string
	indexname   string
	startHeight uint64
	startTime   time.Time
}

// NewBlockIndexer returns the
func NewBlockIndexer() AsyncIndexer {
	name := "blockindexer"
	return &blockIndexer{
		log:       log.Logger(name),
		tablename: "block",
		indexname: name,
	}
}

func (bi *blockIndexer) Start(ctx context.Context) error {
	createSql := "CREATE TABLE IF NOT EXISTS `" + bi.tablename + "` (" +
		"`block_height` bigint(20) NOT NULL," +
		"`block_hash` varchar(64) NOT NULL," +
		"`producer_address` varchar(42) NOT NULL," +
		"`num_actions` int(6) unsigned NOT NULL DEFAULT '0'," +
		"`timestamp` int(11) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`) USING BTREE," +
		"KEY `producer_address` (`producer_address`) USING BTREE" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"
	if _, err := getDB(ctx).Exec(createSql); err != nil {
		return errors.Wrap(err, "failed to start blockindexer")
	}
	height, err := getIndexHeight(getDB(ctx), bi.indexname)
	if err != nil {
		return err
	}
	bi.startHeight = height
	bi.startTime = time.Now()
	bi.log.Info("blockIndexer start", zap.Uint64("height", height))
	return nil
}

func (bi *blockIndexer) Stop(ctx context.Context) error {
	height, err := getIndexHeight(getDB(ctx), bi.indexname)
	if err != nil {
		return err
	}
	elapsed := time.Since(bi.startTime)
	bi.log.Info("blockIndexer stop",
		zap.Uint64("height", height),
		zap.Uint64("qps", (height-bi.startHeight)/uint64(elapsed.Seconds())),
		zap.String("elapsedTime", elapsed.String()))
	return nil
}

func (bi *blockIndexer) NextHeight(ctx context.Context) (uint64, error) {
	height, err := getIndexHeight(getDB(ctx), bi.indexname)
	if err != nil {
		return 0, err
	}
	nextHeight := height + 1
	return nextHeight, nil
}

func (bi *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	blkHash := blk.HashBlock()

	insertData := H{
		"block_height":     blk.Height(),
		"block_hash":       hex.EncodeToString(blkHash[:]),
		"producer_address": blk.ProducerAddress(),
		"timestamp":        blk.Timestamp().Unix(),
	}

	err := withTransaction(getDB(ctx), func(tx Transaction) error {
		if err := insertTableData(tx, bi.tablename, insertData); err != nil {
			return err
		}
		return updateIndexHeight(tx, bi.indexname, blk.Height())
	})

	return err
}
