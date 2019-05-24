// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"database/sql"
	"time"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Indexer handles the index build for blocks
type Indexer struct {
	Store      s.Store
	Registry   *indexprotocol.Registry
	Config     Config
	lastHeight uint64
	terminate  chan bool
}

// Config contains indexer configs
type Config struct {
	DBPath                string `yaml:"dbPath"`
	NumDelegates          uint64 `yaml:"numDelegates"`
	NumCandidateDelegates uint64 `yaml:"numCandidateDelegates"`
	NumSubEpochs          uint64 `yaml:"numSubEpochs"`
	RangeQueryLimit       uint64 `yaml:"rangeQueryLimit"`
}

// NewIndexer creates a new indexer
func NewIndexer(store s.Store, cfg Config) *Indexer {
	return &Indexer{
		Store:    store,
		Registry: &indexprotocol.Registry{},
		Config:   cfg,
	}
}

// Start starts the indexer
func (idx *Indexer) Start(ctx context.Context) error {
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient

	if err := idx.Store.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start db")
	}

	if err := idx.CreateTablesIfNotExist(); err != nil {
		return errors.Wrap(err, "failed to create tables")
	}
	_, lastHeight, err := chainmetautil.GetCurrentEpochAndHeight(idx.Registry, idx.Store)
	if err != nil && err != indexprotocol.ErrNotExist {
		return errors.Wrap(err, "failed to get current epoch and tip height")
	}
	idx.lastHeight = lastHeight

	log.L().Info("Catching up via network")
	getChainMetaRes, err := chainClient.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to get chain metadata")
	}
	tipHeight := getChainMetaRes.ChainMeta.Height

	if err := idx.IndexInBatch(ctx, tipHeight); err != nil {
		return errors.Wrap(err, "failed to index blocks in batch")
	}

	log.L().Info("Subscribing to new coming blocks")
	heightChan := make(chan uint64)
	reportChan := make(chan error)
	go func() {
		for {
			select {
			case <-idx.terminate:
				idx.terminate <- true
				return
			case tipHeight := <-heightChan:
				// index blocks up to this height
				if err := idx.IndexInBatch(ctx, tipHeight); err != nil {
					log.L().Error("failed to index blocks in batch", zap.Error(err))
				}
			case err := <-reportChan:
				log.L().Error("something goes wrong", zap.Error(err))
			}
		}
	}()
	idx.SubscribeNewBlock(chainClient, heightChan, reportChan, idx.terminate)
	return nil
}

// Stop stops the indexer
func (idx *Indexer) Stop(ctx context.Context) error {
	idx.terminate <- true
	return idx.Store.Stop(ctx)
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	for _, p := range idx.Registry.All() {
		if err := p.CreateTables(context.Background()); err != nil {
			return errors.Wrap(err, "failed to create a table")
		}
	}
	return nil
}

// RegisterProtocol registers a protocol to the indexer
func (idx *Indexer) RegisterProtocol(protocolID string, protocol indexprotocol.Protocol) error {
	return idx.Registry.Register(protocolID, protocol)
}

// RegisterDefaultProtocols registers default protocols to hte indexer
func (idx *Indexer) RegisterDefaultProtocols() error {
	actionsProtocol := actions.NewProtocol(idx.Store)
	blocksProtocol := blocks.NewProtocol(idx.Store, idx.Config.NumDelegates, idx.Config.NumCandidateDelegates, idx.Config.NumSubEpochs)
	rewardsProtocol := rewards.NewProtocol(idx.Store, idx.Config.NumDelegates, idx.Config.NumSubEpochs)
	votingsProtocol := votings.NewProtocol(idx.Store, idx.Config.NumDelegates, idx.Config.NumSubEpochs)
	if err := idx.RegisterProtocol(actions.ProtocolID, actionsProtocol); err != nil {
		return errors.Wrap(err, "failed to register actions protocol")
	}
	if err := idx.RegisterProtocol(blocks.ProtocolID, blocksProtocol); err != nil {
		return errors.Wrap(err, "failed to register blocks protocol")
	}
	if err := idx.RegisterProtocol(rewards.ProtocolID, rewardsProtocol); err != nil {
		return errors.Wrap(err, "failed to register rewards protocol")
	}
	return idx.RegisterProtocol(votings.ProtocolID, votingsProtocol)
}

// IndexInBatch indexes blocks in batch
func (idx *Indexer) IndexInBatch(ctx context.Context, tipHeight uint64) error {
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient

	startHeight := idx.lastHeight + 1
	for startHeight <= tipHeight {
		count := idx.Config.RangeQueryLimit
		if idx.Config.RangeQueryLimit > tipHeight-startHeight+1 {
			count = tipHeight - startHeight + 1
		}
		getRawBlocksRes, err := chainClient.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
			StartHeight:  startHeight,
			Count:        count,
			WithReceipts: true,
		})
		if err != nil {
			return errors.Wrap(err, "failed to get raw blocks from the chain")
		}
		for _, blkInfo := range getRawBlocksRes.Blocks {
			blk := &block.Block{}
			if err := blk.ConvertFromBlockPb(blkInfo.Block); err != nil {
				return errors.Wrap(err, "failed to convert block protobuf to raw block")
			}

			for _, receiptPb := range blkInfo.Receipts {
				receipt := &action.Receipt{}
				receipt.ConvertFromReceiptPb(receiptPb)
				blk.Receipts = append(blk.Receipts, receipt)
			}

			if err := idx.buildIndex(ctx, blk); err != nil {
				return errors.Wrap(err, "failed to build index the block")
			}
			// Update lastHeight tracker
			idx.lastHeight = blk.Height()
		}
		startHeight += count
	}
	return nil
}

// SubscribeNewBlock polls the new block height from the chain
func (idx *Indexer) SubscribeNewBlock(
	client iotexapi.APIServiceClient,
	height chan uint64,
	report chan error,
	unsubscribe chan bool,
) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-unsubscribe:
				unsubscribe <- true
				return
			case <-ticker.C:
				if res, err := client.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{}); err != nil {
					report <- err
				} else {
					height <- res.ChainMeta.Height
				}
			}
		}
	}()
}

// buildIndex builds the index for a block
func (idx *Indexer) buildIndex(ctx context.Context, blk *block.Block) error {
	if err := idx.Store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.Registry.All() {
			if err := p.HandleBlock(ctx, tx, blk); err != nil {
				return errors.Wrapf(err, "failed to build index for block on height %d", blk.Height())
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
