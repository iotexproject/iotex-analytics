// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockinfo

import (
	"context"
	"database/sql"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
)

const (
	// TableName is the name of the table in this protocol
	TableName = "blockinfo"
)

var (
	createTable = "CREATE TABLE IF NOT EXISTS `" + TableName + "` (" +
		"`block_height` decimal(65,0) NOT NULL," +
		"`timestamp` datetime DEFAULT NULL," +
		"PRIMARY KEY (`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	addBlock = "INSERT IGNORE INTO `" + TableName + "` (`block_height`, `timestamp`) VALUES (?,?)"
)

// Protocol defines the account balance protocol
type Protocol struct {
}

// NewProtocol creates a new protocol
func NewProtocol() *Protocol {
	return &Protocol{}
}

// Initialize creates tables and insert initial records
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(createTable)

	return err
}

// HandleBlockData handles block data
func (p *Protocol) HandleBlockData(ctx context.Context, tx *sql.Tx, data *indexprotocol.BlockData) error {
	_, err := tx.Exec(addBlock, data.Block.Height(), data.Block.Timestamp())

	return err
}
