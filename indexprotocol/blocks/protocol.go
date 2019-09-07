// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocks

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "blocks"
	// BlockHistoryTableName is the table name of block history
	BlockHistoryTableName = "temp_block_history"
)

type (
	// BlockHistory defines the schema of "block history" table
	BlockHistory struct {
		EpochNumber uint64
		BlockHeight uint64
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store                 s.Store
	NumDelegates          uint64
	NumCandidateDelegates uint64
	NumSubEpochs          uint64
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numCandidateDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{
		Store:                 store,
		NumDelegates:          numDelegates,
		NumCandidateDelegates: numCandidateDelegates,
		NumSubEpochs:          numSubEpochs,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"block_height DECIMAL(65, 0) NOT NULL, PRIMARY KEY (block_height))", BlockHistoryTableName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes blocks index protocol
func (p *Protocol) Initialize(context.Context, *sql.Tx, *indexprotocol.Genesis) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := indexprotocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, height)
	return p.updateBlockHistory(tx, epochNumber, height)
}

// getBlockHistory gets block history
func (p *Protocol) getBlockHistory(blockHeight uint64) (*BlockHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE block_height=?", BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(blockHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var blockHistory BlockHistory
	parsedRows, err := s.ParseSQLRows(rows, &blockHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	blockInfo := parsedRows[0].(*BlockHistory)
	return blockInfo, nil
}

// updateBlockHistory stores reward information into reward history table
func (p *Protocol) updateBlockHistory(tx *sql.Tx, epochNumber uint64, height uint64) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height) VALUES (?, ?)",
		BlockHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, height); err != nil {
		return err
	}
	return nil
}
