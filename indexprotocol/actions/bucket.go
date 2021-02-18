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
	"math/big"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

const (

	// BucketActionsTableName is the table name of bucket actions
	BucketActionsTableName = "bucket_actions"

	createBucketActionTable = "CREATE TABLE IF NOT EXISTS %s (" +
		"action_hash VARCHAR(64) NOT NULL," +
		"bucket_id DECIMAL(65,0) NOT NULL," +
		"PRIMARY KEY (action_hash)," +
		"KEY `bucket_id_index` (`bucket_id`)," +
		"CONSTRAINT `fb_bucket_actions_action_hash` FOREIGN KEY (`action_hash`) REFERENCES `action_history` (`action_hash`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	insertBucketAction = "INSERT IGNORE INTO %s (action_hash, bucket_id) VALUES %s"
)

// CreateBucketActionTables creates tables
func (p *Protocol) CreateBucketActionTables(ctx context.Context) error {
	// create block by action table
	_, err := p.Store.GetDB().Exec(fmt.Sprintf(createBucketActionTable, BucketActionsTableName))
	return err
}

// updateXrc20History stores Xrc20 information into Xrc20 history table
func (p *Protocol) updateBucketActions(
	ctx context.Context,
	tx *sql.Tx,
	blk *block.Block,
) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)

	h := hash.Hash160b([]byte("staking"))
	stakingProtocolAddr, err := address.FromBytes(h[:])
	if err != nil {
		return err
	}
	for _, receipt := range blk.Receipts {
		actionHash := hex.EncodeToString(receipt.ActionHash[:])
		for _, log := range receipt.Logs() {
			if log.Address == stakingProtocolAddr.String() && len(log.Topics) > 1 {
				bucketIndex := new(big.Int).SetBytes(log.Topics[1][:])
				valStrs = append(valStrs, "(?, ?)")
				valArgs = append(valArgs, actionHash, bucketIndex.String())
			}
		}
	}
	if len(valArgs) == 0 {
		return nil
	}
	insertQuery := fmt.Sprintf(insertBucketAction, BucketActionsTableName, strings.Join(valStrs, ","))
	_, err = tx.Exec(insertQuery, valArgs...)

	return err
}
