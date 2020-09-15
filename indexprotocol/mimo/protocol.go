// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mino

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "mino"

	// ExchangeCreationTableName is the table storing exchange creation records
	ExchangeCreationTableName = "mimo_exchange_creations"

	// ExchangeMonitorViewName is the table storing all the exchange addresses
	ExchangeMonitorViewName = "mimo_exchange_to_monitor"

	// TokenMonitorViewName is the table storing all the <token,account> to monitor
	TokenMonitorViewName = "mimo_token_to_monitor"

	createTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeCreationTableName + "` (" +
		"`id` int(11) NOT NULL AUTO_INCREMENT," +
		"`exchange` varchar(41) NOT NULL," +
		"`token` varchar(41) NOT NULL," +
		"`block_height` decimal(65, 0) NOT NULL," +
		"`action_hash` varchar(40) NOT NULL," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `exchange_UNIQUE` (`exchange`)," +
		"UNIQUE KEY `token_UNIQUE` (`token`)," +
		"KEY `i_block_height` (`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createExchangeViewQuery = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + ExchangeMonitorViewName + "` AS select `exchange` AS `account` from `" + ExchangeCreationTableName + "`"
	createTokenViewQuery    = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + TokenMonitorViewName + "` AS select `token`,`exchange` AS `account` from `" + ExchangeCreationTableName + "` union all select `exchange` AS `token`,'*' from `" + ExchangeCreationTableName + "`"

	insertExchangeQuery = "INSERT INTO `" + ExchangeCreationTableName + "` (`exchange`, `token`, `block_height`, `action_hash`) VALUES %s"
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	factoryAddr address.Address
}

// NewProtocol creates a new protocol
func NewProtocol(factoryAddr address.Address) *Protocol {
	return &Protocol{
		factoryAddr: factoryAddr,
	}
}

// Initialize creates the tables in the protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.Exec(createTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange base table")
	}
	if _, err := tx.Exec(createExchangeViewQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange view")
	}
	if _, err := tx.Exec(createTokenViewQuery); err != nil {
		return errors.Wrap(err, "failed to create token view")
	}

	return nil
}

// HandleBlockData handles blocks
func (p *Protocol) HandleBlockData(ctx context.Context, tx *sql.Tx, data *indexprotocol.BlockData) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	if p.factoryAddr == nil {
		return nil
	}
	for _, receipt := range data.Block.Receipts {
		if receipt.Status != uint64(1) {
			continue
		}
		for _, l := range receipt.Logs() {
			if l.Address != p.factoryAddr.String() {
				continue
			}
			if len(l.Topics) == 0 {
				continue
			}
			if hex.EncodeToString(l.Topics[0][:]) != "9d42cb017eb05bd8944ab536a8b35bc68085931dd5f4356489801453923953f9" {
				continue
			}
			token, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
			if err != nil {
				return err
			}
			exchange, err := indexprotocol.ConvertTopicToAddress(l.Topics[2])
			if err != nil {
				return err
			}
			valStrs = append(valStrs, "(?, ?, ?, ?)")
			valArgs = append(
				valArgs,
				exchange.String(),
				token.String(),
				l.BlockHeight,
				hex.EncodeToString(l.ActionHash[:]),
			)
		}
	}
	if len(valStrs) == 0 {
		return nil
	}
	_, err := tx.Exec(fmt.Sprintf(insertExchangeQuery, strings.Join(valStrs, ",")), valArgs...)

	return err
}
