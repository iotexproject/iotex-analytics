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
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-api/protocol"
	s "github.com/iotexproject/iotex-api/sql"
)

// Indexer handles the index build for blocks
type Indexer struct {
	store     s.Store
	protocols []protocol.Protocol
}

// HandleBlock is an implementation of interface BlockCreationSubscriber
func (idx *Indexer) HandleBlock(blk *block.Block) error {
	return idx.BuildIndex(blk)
}

// BuildIndex builds the index for a block
func (idx *Indexer) BuildIndex(blk *block.Block) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		for _, p := range idx.protocols {
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
	for _, p := range idx.protocols {
		if err := p.CreateTables(context.Background()); err != nil {
			return errors.Wrap(err, "failed to create a table")
		}
	}
	return nil
}

// AddProtocols adds protocols to the indexer
func (idx *Indexer) AddProtocols(protocols ...protocol.Protocol) {
	idx.protocols = append(idx.protocols, protocols...)
}
