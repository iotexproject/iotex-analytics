// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-api/protocol"
	s "github.com/iotexproject/iotex-api/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "actions"
	// BlockByActionTableName is the table name of block by action
	BlockByActionTableName = "block_by_action"
	// ActionHistoryTableName is the table name of action history
	ActionHistoryTableName = "action_history"
)

type (
	// BlockByAction defines the base schema of "action to block" table
	BlockByAction struct {
		ActionHash  []byte
		ReceiptHash []byte
		BlockHash   []byte
	}
	// ActionHistory defines the schema of "action history" table
	ActionHistory struct {
		UserAddress string
		ActionHash  string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store s.Store
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store) *Protocol {
	return &Protocol{Store: store}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([action_hash] BLOB(32) NOT NULL, [receipt_hash] BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)", BlockByActionTableName)); err != nil {
		return err
	}

	// create action history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([user_address] TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)", ActionHistoryTableName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes actions protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx, genesisCfg *protocol.GenesisConfig) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	actionToReceipt := make(map[hash.Hash256]hash.Hash256)

	// log action index
	for _, selp := range blk.Actions {
		callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		// put new action for sender
		if err := p.updateActionHistory(tx, callerAddr.String(), selp.Hash()); err != nil {
			return errors.Wrap(err, "failed to update action to action history table")
		}
		// put new transfer for recipient
		dst, ok := selp.Destination()
		if ok {
			if err := p.updateActionHistory(tx, dst, selp.Hash()); err != nil {
				return errors.Wrap(err, "failed to update action to action history table")
			}
		}
		actionToReceipt[selp.Hash()] = hash.ZeroHash256
	}

	for _, receipt := range blk.Receipts {
		// map receipt to action
		if _, ok := actionToReceipt[receipt.ActionHash]; !ok {
			return errors.New("failed to find the corresponding action from receipt")
		}
		actionToReceipt[receipt.ActionHash] = receipt.Hash()
	}

	if err := p.updateBlockByAction(tx, actionToReceipt, blk.HashBlock()); err != nil {
		return errors.Wrap(err, "failed to update action index to block")
	}

	return nil
}

// GetActionHistory returns list of action hash by user address
func (p *Protocol) GetActionHistory(userAddr string) ([]hash.Hash256, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE user_address=?",
		ActionHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(userAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var actionHistory ActionHistory
	parsedRows, err := s.ParseSQLRows(rows, &actionHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	actionHashes := make([]hash.Hash256, 0, len(parsedRows))
	for _, parsedRow := range parsedRows {
		var hash hash.Hash256
		copy(hash[:], parsedRow.(*ActionHistory).ActionHash)
		actionHashes = append(actionHashes, hash)
	}
	return actionHashes, nil
}

// GetBlockByAction returns block hash by action hash
func (p *Protocol) GetBlockByAction(actionHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE action_hash=?",
		BlockByActionTableName)
	return p.blockByIndex(getQuery, actionHash)
}

// GetBlockByReceipt returns block hash by receipt hash
func (p *Protocol) GetBlockByReceipt(receiptHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE receipt_hash=?",
		BlockByActionTableName)
	return p.blockByIndex(getQuery, receiptHash)
}

// updateBlockByAction maps action hash/receipt hash to block hash
func (p *Protocol) updateBlockByAction(tx *sql.Tx, actionToReceipt map[hash.Hash256]hash.Hash256,
	blockHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (action_hash,receipt_hash,block_hash) VALUES (?, ?, ?)",
		BlockByActionTableName)
	for actionHash, receiptHash := range actionToReceipt {
		if _, err := tx.Exec(insertQuery, hex.EncodeToString(actionHash[:]), hex.EncodeToString(receiptHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// updateActionHistory stores action information into action history table
func (p *Protocol) updateActionHistory(tx *sql.Tx, userAddr string,
	actionHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (user_address,action_hash) VALUES (?, ?)",
		ActionHistoryTableName)
	if _, err := tx.Exec(insertQuery, userAddr, actionHash[:]); err != nil {
		return err
	}
	return nil
}

// blockByIndex returns block by index hash
func (p *Protocol) blockByIndex(getQuery string, indexHash hash.Hash256) (hash.Hash256, error) {
	db := p.Store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(hex.EncodeToString(indexHash[:]))
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to execute get query")
	}

	var blockByAction BlockByAction
	parsedRows, err := s.ParseSQLRows(rows, &blockByAction)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash256, protocol.ErrNotExist
	}

	var hash hash.Hash256
	copy(hash[:], parsedRows[0].(*BlockByAction).BlockHash)
	return hash, nil
}
