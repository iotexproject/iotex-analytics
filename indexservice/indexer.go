// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"database/sql"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/protocol"
	"github.com/iotexproject/iotex-analytics/protocol/actions"
	"github.com/iotexproject/iotex-analytics/protocol/producers"
	"github.com/iotexproject/iotex-analytics/protocol/rewards"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Indexer handles the index build for blocks
type Indexer struct {
	store    s.Store
	registry *protocol.Registry
}

// NewIndexer creates a new indexer
func NewIndexer(store s.Store) *Indexer {
	return &Indexer{
		store:    store,
		registry: &protocol.Registry{},
	}
}

// HandleBlock is an implementation of interface BlockCreationSubscriber
func (idx *Indexer) HandleBlock(blk *block.Block) error {
	return idx.BuildIndex(blk)
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

// BuildIndex builds the index for a block
func (idx *Indexer) BuildIndex(blk *block.Block) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.registry.All() {
			if err := p.HandleBlock(context.Background(), tx, blk); err != nil {
				return errors.Wrapf(err, "failed to build index for block on height %d", blk.Height())
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
func (idx *Indexer) RegisterDefaultProtocols(cfg config.Config) error {
	actionsProtocol := actions.NewProtocol(idx.store)
	rewardProtocol := rewards.NewProtocol(idx.store, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	producersProtocol := producers.NewProtocol(idx.store, cfg.Genesis.NumDelegates, cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumSubEpochs)

	if err := idx.RegisterProtocol(actions.ProtocolID, actionsProtocol); err != nil {
		return errors.Wrap(err, "failed to register actions protocol")
	}
	if err := idx.RegisterProtocol(rewards.ProtocolID, rewardProtocol); err != nil {
		return errors.Wrap(err, "failed to register rewards protocol")
	}
	return idx.RegisterProtocol(producers.ProtocolID, producersProtocol)
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
