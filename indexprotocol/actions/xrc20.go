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
	"strings"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

const (
	// Xrc20HistoryTableName is the table name of xrc20 history
	Xrc20HistoryTableName = "xrc20_history"
	// transferSha3 is sha3 of xrc20's transfer,keccak('Transfer(address,address,uint256)')
	transferSha3 = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

type (
	// Xrc20History defines the base schema of "xrc20 history" table
	Xrc20History struct {
		ActionHash  string
		ReceiptHash string
		Address     string
		Data        []byte
		BlockHeight uint64
		Index       uint64
		Timestamp   uint64
	}
)

// CreateTables creates tables
func (p *Protocol) CreateXrc20Tables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (action_hash VARCHAR(64) NOT NULL, receipt_hash VARCHAR(64) NOT NULL UNIQUE, address VARCHAR(41) NOT NULL,`topics` VARCHAR(192),`data` VARCHAR(192),block_height DECIMAL(65, 0), `index` DECIMAL(65, 0),`timestamp` DECIMAL(65, 0),status VARCHAR(7) NOT NULL, PRIMARY KEY (action_hash,receipt_hash,topics))",
		Xrc20HistoryTableName)); err != nil {
		return err
	}
	return nil
}

// updateXrc20History stores Xrc20 information into Xrc20 history table
func (p *Protocol) updateXrc20History(
	tx *sql.Tx,
	blk *block.Block,
) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	for _, receipt := range blk.Receipts {
		receiptStatus := "failure"
		if receipt.Status == uint64(1) {
			receiptStatus = "success"
		}
		for _, l := range receipt.Logs {
			data := hex.EncodeToString(l.Data)
			var topics string
			for _, t := range l.Topics {
				topics += hex.EncodeToString(t[:])
			}
			if topics == "" || len(topics) > 64*3 || len(data) > 64*3 {
				continue
			}
			if !strings.Contains(topics, transferSha3) {
				continue
			}
			ah := hex.EncodeToString(l.ActionHash[:])
			receiptHash := receipt.Hash()

			rh := hex.EncodeToString(receiptHash[:])
			valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valArgs = append(valArgs, ah, rh, l.Address, topics, data, l.BlockHeight, l.Index, blk.Timestamp().Unix(), receiptStatus)
		}
	}
	if len(valArgs) == 0 {
		return nil
	}
	insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (action_hash, receipt_hash, address,topics,`data`,block_height, `index`,`timestamp`,status) VALUES %s", Xrc20HistoryTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}
