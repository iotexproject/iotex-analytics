// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	s "github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// BlockByIndex defines the base schema of "index to block" table
	BlockByAction struct {
		ActionHash  []byte
		ReceiptHash []byte
		BlockHash   []byte
	}
	// IndexHistory defines the schema of "index history" table
	ActionHistory struct {
		UserAddress string
		ActionHash  string
	}
)

// Indexer handles the index build for blocks
type Indexer struct {
	store s.Store
}

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// HandleBlock is an implementation of interface BlockCreationSubscriber
func (idx *Indexer) HandleBlock(blk *block.Block) error {
	return idx.BuildIndex(blk)
}

// BuildIndex builds the index for a block
func (idx *Indexer) BuildIndex(blk *block.Block) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		actionToReceipt := make(map[hash.Hash256]hash.Hash256)
		// log action index
		for _, selp := range blk.Actions {
			callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new action for sender
			if err := idx.UpdateActionHistory(tx, callerAddr.String(), selp.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update action to action history table")
			}
			// put new transfer for recipient
			dst, ok := selp.Destination()
			if ok {
				if err := idx.UpdateActionHistory(tx, dst, selp.Hash()); err != nil {
					return errors.Wrapf(err, "failed to update action to action history table")
				}
			}
			actionToReceipt[selp.Hash()] = hash.ZeroHash256
		}

		// log receipt index
		for _, receipt := range blk.Receipts {
			// map receipt to action
			if _, ok := actionToReceipt[receipt.ActionHash]; !ok {
				return errors.New("failed to find the corresponding action from receipt")
			}
			actionToReceipt[receipt.ActionHash] = receipt.Hash()
		}
		if err := idx.UpdateBlockByAction(tx, actionToReceipt, blk.HashBlock()); err != nil {
			return errors.Wrapf(err, "failed to update action index to block")
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// UpdateBlockByIndex maps index hash to block hash
func (idx *Indexer) UpdateBlockByAction(tx *sql.Tx, actionToReceipt map[hash.Hash256]hash.Hash256,
	blockHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (action_hash,receipt_hash,block_hash) VALUES (?, ?, ?)",
		idx.getBlockByActionTableName())
	for actionHash, receiptHash := range actionToReceipt {
		if _, err := tx.Exec(insertQuery, hex.EncodeToString(actionHash[:]), hex.EncodeToString(receiptHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateActionHistory stores index information into index history table
func (idx *Indexer) UpdateActionHistory(tx *sql.Tx, userAddr string,
	actionHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (user_address,action_hash) VALUES (?, ?)",
		idx.getActionHistoryTableName())
	if _, err := tx.Exec(insertQuery, userAddr, actionHash[:]); err != nil {
		return err
	}
	return nil
}

// GetIndexHistory gets index history
func (idx *Indexer) GetActionHistory(userAddr string) ([]hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE user_address=?",
		idx.getActionHistoryTableName())
	indexHashes, err := idx.getActionHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get index history")
	}
	return indexHashes, nil
}

// GetBlockByAction returns block hash by action hash
func (idx *Indexer) GetBlockByAction(actionHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE action_hash=?",
		idx.getBlockByActionTableName())
	blkHash, err := idx.blockByIndex(getQuery, actionHash)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to get block hash by action hash")
	}
	return blkHash, nil
}

// GetBlockByReceipt returns block hash by receipt hash
func (idx *Indexer) GetBlockByReceipt(receiptHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE receipt_hash=?",
		idx.getBlockByActionTableName())
	blkHash, err := idx.blockByIndex(getQuery, receiptHash)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to get block hash by receipt hash")
	}
	return blkHash, nil
}

// blockByIndex returns block by index hash
func (idx *Indexer) blockByIndex(getQuery string, indexHash hash.Hash256) (hash.Hash256, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(hex.EncodeToString(indexHash[:]))
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to execute get query")
	}

	var blockByAction BlockByAction
	parsedRows, err := s.ParseSQLRows(rows, &blockByAction)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash256, ErrNotExist
	}

	var hash hash.Hash256
	copy(hash[:], parsedRows[0].(*BlockByAction).BlockHash)
	return hash, nil
}

// getIndexHistory gets index history
func (idx *Indexer) getActionHistory(getQuery string, userAddr string) ([]hash.Hash256, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var actionHistory ActionHistory
	parsedRows, err := s.ParseSQLRows(rows, &actionHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	actionHashes := make([]hash.Hash256, 0, len(parsedRows))
	for _, parsedRow := range parsedRows {
		var hash hash.Hash256
		copy(hash[:], parsedRow.(*ActionHistory).ActionHash)
		actionHashes = append(actionHashes, hash)
	}
	return actionHashes, nil
}

func (idx *Indexer) getBlockByActionTableName() string {
	return fmt.Sprintf("block_by_action")
}

func (idx *Indexer) getActionHistoryTableName() string {
	return fmt.Sprintf("action_history")
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	// create block by index tables
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([action_hash] BLOB(32) NOT NULL, [receipt_hash] BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)", idx.getBlockByActionTableName())); err != nil {
		return err
	}

	// create index history tables
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([user_address] TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)", idx.getActionHistoryTableName())); err != nil {
		return err
	}

	return nil
}
