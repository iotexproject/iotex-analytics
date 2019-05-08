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
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/protocol"
	"github.com/iotexproject/iotex-analytics/protocol/actions"
	"github.com/iotexproject/iotex-analytics/protocol/producers"
	"github.com/iotexproject/iotex-analytics/protocol/rewards"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Indexer handles the index build for blocks
type Indexer struct {
	store      s.Store
	config     Config
	registry   *protocol.Registry
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
		store:    store,
		config:   cfg,
		registry: &protocol.Registry{},
	}
}

// Start starts the indexer
func (idx *Indexer) Start(ctx context.Context) error {
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient

	if err := idx.store.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start db")
	}

	lastHeight, err := idx.GetLastHeight()
	if err != nil {
		if err := idx.CreateTablesIfNotExist(); err != nil {
			return errors.Wrap(err, "failed to create tables")
		}

		readStateRequest := &iotexapi.ReadStateRequest{
			ProtocolID: []byte(poll.ProtocolID),
			MethodName: []byte("DelegatesByEpoch"),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(uint64(1))},
		}
		res, err := chainClient.ReadState(ctx, readStateRequest)
		if err != nil {
			return errors.Wrap(err, "failed to read genesis delegates from blockchain")
		}
		var genesisDelegates state.CandidateList
		if err := genesisDelegates.Deserialize(res.Data); err != nil {
			return errors.Wrap(err, "failed to deserialize gensisDelegates")
		}
		gensisConfig := &protocol.GenesisConfig{InitCandidates: genesisDelegates}

		// Initialize indexer
		if err := idx.Initialize(gensisConfig); err != nil {
			return errors.Wrap(err, "failed to initialize the indexer")
		}
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
	return idx.store.Stop(ctx)
}

// Initialize initialize the registered protocols
func (idx *Indexer) Initialize(genesisCfg *protocol.GenesisConfig) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.registry.All() {
			if err := p.Initialize(context.Background(), tx, genesisCfg); err != nil {
				return errors.Wrap(err, "failed to initialize the protocol")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	for _, p := range idx.registry.All() {
		if err := p.CreateTables(context.Background()); err != nil {
			return errors.Wrap(err, "failed to create a table")
		}
	}
	return nil
}

// RegisterProtocol registers a protocol to the indexer
func (idx *Indexer) RegisterProtocol(protocolID string, protocol protocol.Protocol) error {
	return idx.registry.Register(protocolID, protocol)
}

// RegisterDefaultProtocols registers default protocols to hte indexer
func (idx *Indexer) RegisterDefaultProtocols() error {
	actionsProtocol := actions.NewProtocol(idx.store)
	rewardProtocol := rewards.NewProtocol(idx.store, idx.config.NumDelegates, idx.config.NumSubEpochs)
	producersProtocol := producers.NewProtocol(idx.store, idx.config.NumDelegates, idx.config.NumCandidateDelegates,
		idx.config.NumSubEpochs)

	if err := idx.RegisterProtocol(actions.ProtocolID, actionsProtocol); err != nil {
		return errors.Wrap(err, "failed to register actions protocol")
	}
	if err := idx.RegisterProtocol(rewards.ProtocolID, rewardProtocol); err != nil {
		return errors.Wrap(err, "failed to register rewards protocol")
	}
	return idx.RegisterProtocol(producers.ProtocolID, producersProtocol)
}

// IndexInBatch indexes blocks in batch
func (idx *Indexer) IndexInBatch(ctx context.Context, tipHeight uint64) error {
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient

	startHeight := idx.lastHeight + 1
	for startHeight <= tipHeight {
		count := idx.config.RangeQueryLimit
		if idx.config.RangeQueryLimit > tipHeight-startHeight+1 {
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

// GetLastHeight gets last height stored in the underlying db
func (idx *Indexer) GetLastHeight() (uint64, error) {
	p, ok := idx.registry.Find(producers.ProtocolID)
	if !ok {
		return uint64(0), errors.New("producers protocol is unregistered")
	}
	pp, ok := p.(*producers.Protocol)
	if !ok {
		return uint64(0), errors.New("failed to cast protocol interface to producers protocol")
	}
	return pp.GetLastHeight()
}

// GetRewardHistory gets reward history
func (idx *Indexer) GetRewardHistory(startEpoch uint64, epochCount uint64, rewardAddr string) (*rewards.RewardInfo, error) {
	p, ok := idx.registry.Find(rewards.ProtocolID)
	if !ok {
		return nil, errors.New("rewards protocol is unregistered")
	}
	rp, ok := p.(*rewards.Protocol)
	if !ok {
		return nil, errors.New("failed to cast protocol interface to rewards protocol")
	}
	return rp.GetRewardHistory(startEpoch, epochCount, rewardAddr)
}

// GetProductivityHistory gets productivity history
func (idx *Indexer) GetProductivityHistory(startEpoch uint64, epochCount uint64, address string) (uint64, uint64, error) {
	p, ok := idx.registry.Find(producers.ProtocolID)
	if !ok {
		return uint64(0), uint64(0), errors.New("producers protocol is unregistered")
	}
	pp, ok := p.(*producers.Protocol)
	if !ok {
		return uint64(0), uint64(0), errors.New("failed to cast protocol interface to rewards protocol")
	}
	return pp.GetProductivityHistory(startEpoch, epochCount, address)
}

// buildIndex builds the index for a block
func (idx *Indexer) buildIndex(ctx context.Context, blk *block.Block) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.registry.All() {
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
