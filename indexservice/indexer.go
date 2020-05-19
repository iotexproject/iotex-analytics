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

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
)

var (
	blockHeightMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "block_height",
			Help: "block height",
		},
		[]string{},
	)
)

// Indexer handles the index build for blocks
type Indexer struct {
	Store          s.Store
	Registry       *indexprotocol.Registry
	IndexProtocols []indexprotocol.Protocol
	Config         Config
	lastHeight     uint64
	terminate      chan bool
	epochCtx       *epochctx.EpochCtx
	hermesConfig   indexprotocol.HermesConfig
}

// Config contains indexer configs
type Config struct {
	NumDelegates            uint64                            `yaml:"numDelegates"`
	NumCandidateDelegates   uint64                            `yaml:"numCandidateDelegates"`
	NumSubEpochs            uint64                            `yaml:"numSubEpochs"`
	NumSubEpochsDardanelles uint64                            `yaml:"numSubEpochsDardanelles"`
	DardanellesHeight       uint64                            `yaml:"dardanellesHeight"`
	DardanellesOn           bool                              `yaml:"dardanellesOn"`
	FairbankHeight          uint64                            `yaml:"fairbankHeight"`
	ConsensusScheme         string                            `yaml:"consensusScheme"`
	RangeQueryLimit         uint64                            `yaml:"rangeQueryLimit"`
	Genesis                 indexprotocol.Genesis             `yaml:"genesis"`
	GravityChain            indexprotocol.GravityChain        `yaml:"gravityChain"`
	Rewarding               indexprotocol.Rewarding           `yaml:"rewarding"`
	Poll                    indexprotocol.Poll                `yaml:"poll"`
	HermesConfig            indexprotocol.HermesConfig        `yaml:"hermesConfig"`
	VoteWeightCalConsts     indexprotocol.VoteWeightCalConsts `yaml:"voteWeightCalConsts"`
}

// NewIndexer creates a new indexer
func NewIndexer(store s.Store, cfg Config) *Indexer {
	return &Indexer{
		Store:          store,
		Registry:       &indexprotocol.Registry{},
		Config:         cfg,
		IndexProtocols: make([]indexprotocol.Protocol, 0),
		epochCtx: epochctx.NewEpochCtx(
			cfg.NumCandidateDelegates,
			cfg.NumDelegates,
			cfg.NumSubEpochs,
			epochctx.EnableDardanellesSubEpoch(cfg.DardanellesHeight, cfg.NumSubEpochsDardanelles),
			epochctx.FairbankHeight(cfg.FairbankHeight),
		),
		hermesConfig: indexprotocol.HermesConfig{
			HermesContractAddress:    cfg.HermesConfig.HermesContractAddress,
			MultiSendContractAddress: cfg.HermesConfig.MultiSendContractAddress,
		},
	}
}

// Start starts the indexer
func (idx *Indexer) Start(ctx context.Context) error {
	prometheus.MustRegister(blockHeightMtc)
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	chainClient := indexCtx.ChainClient

	if err := idx.Store.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start db")
	}

	if err := idx.CreateTablesIfNotExist(); err != nil {
		return errors.Wrap(err, "failed to create tables")
	}

	if err := idx.Initialize(ctx, &idx.Config.Genesis); err != nil {
		return errors.Wrap(err, "failed to initialize the index protocols")
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
	tipHeight := getChainMetaRes.GetChainMeta().GetHeight()

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

// Initialize initialize the registered protocols
func (idx *Indexer) Initialize(ctx context.Context, genesis *indexprotocol.Genesis) error {
	if err := idx.Store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.IndexProtocols {
			if err := p.Initialize(ctx, tx, genesis); err != nil {
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
	for _, p := range idx.IndexProtocols {
		if err := p.CreateTables(context.Background()); err != nil {
			return errors.Wrap(err, "failed to create a table")
		}
	}
	return nil
}

// RegisterProtocol registers a protocol to the indexer
func (idx *Indexer) RegisterProtocol(protocolID string, protocol indexprotocol.Protocol) error {
	if err := idx.Registry.Register(protocolID, protocol); err != nil {
		return errors.Wrapf(err, "failed to register index protocol with protocol ID %s", protocolID)
	}
	idx.IndexProtocols = append(idx.IndexProtocols, protocol)

	return nil
}

// RegisterDefaultProtocols registers default protocols to the indexer
func (idx *Indexer) RegisterDefaultProtocols() error {
	actionsProtocol := actions.NewProtocol(idx.Store, idx.hermesConfig, idx.epochCtx)
	blocksProtocol := blocks.NewProtocol(idx.Store, idx.epochCtx)
	rewardsProtocol := rewards.NewProtocol(idx.Store, idx.epochCtx, idx.Config.Rewarding)
	accountsProtocol := accounts.NewProtocol(idx.Store, idx.epochCtx)
	votingsProtocol, err := votings.NewProtocol(idx.Store, idx.epochCtx, idx.Config.GravityChain, idx.Config.Poll, idx.Config.VoteWeightCalConsts)
	if err != nil {
		log.L().Error("failed to make new voting protocol", zap.Error(err))
	}
	if err := idx.RegisterProtocol(blocks.ProtocolID, blocksProtocol); err != nil {
		return errors.Wrap(err, "failed to register blocks protocol")
	}
	if err := idx.RegisterProtocol(actions.ProtocolID, actionsProtocol); err != nil {
		return errors.Wrap(err, "failed to register actions protocol")
	}
	if err := idx.RegisterProtocol(rewards.ProtocolID, rewardsProtocol); err != nil {
		return errors.Wrap(err, "failed to register rewards protocol")
	}
	if err := idx.RegisterProtocol(votings.ProtocolID, votingsProtocol); err != nil {
		return errors.Wrap(err, "failed to register votings protocol")
	}
	return idx.RegisterProtocol(accounts.ProtocolID, accountsProtocol)
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
		for _, blkInfo := range getRawBlocksRes.GetBlocks() {
			blk := &block.Block{}
			if err := blk.ConvertFromBlockPb(blkInfo.GetBlock()); err != nil {
				return errors.Wrap(err, "failed to convert block protobuf to raw block")
			}

			for _, receiptPb := range blkInfo.GetReceipts() {
				receipt := &action.Receipt{}
				receipt.ConvertFromReceiptPb(receiptPb)
				blk.Receipts = append(blk.Receipts, receipt)
			}

			if err := idx.buildIndex(ctx, blk); err != nil {
				return errors.Wrap(err, "failed to build index for the block")
			}
			// Update lastHeight tracker
			idx.lastHeight = blk.Height()
			blockHeightMtc.With(prometheus.Labels{}).Set(float64(idx.lastHeight))
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
		for _, p := range idx.IndexProtocols {
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
