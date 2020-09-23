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
	"math/big"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/services"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "mino"

	// AddLiquidity is an event of adding liquidity
	AddLiquidity = "AddLiquidity"

	// RemoveLiquidity is an event of removing liquidity
	RemoveLiquidity = "RemoveLiquidity"

	// TokenPurchase is an event of purchasing token
	TokenPurchase = "TokenPurchase"

	// CoinPurchase is an event of purchasing coin
	CoinPurchase = "CoinPurchase"

	// ExchangeCreationTableName is the table storing exchange creation records
	ExchangeCreationTableName = "mimo_exchange_creations"

	// ExchangeMonitorViewName is the table storing all the exchange addresses
	ExchangeMonitorViewName = "mimo_exchange_to_monitor"

	// TokenMonitorViewName is the table storing all the <token,account> to monitor
	TokenMonitorViewName = "mimo_token_to_monitor"

	// ExchangeActionTableName is the table storing the exchange actions
	ExchangeActionTableName = "mimo_exchange_actions"

	createTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeCreationTableName + "` (" +
		"`id` int(11) NOT NULL AUTO_INCREMENT," +
		"`exchange` varchar(41) NOT NULL," +
		"`token` varchar(41) NOT NULL," +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`action_hash` varchar(40) NOT NULL," +
		"`token_name` varchar(140) NOT NULL," +
		"`token_symbol` varchar(140) NOT NULL," +
		"`token_decimals` int(10) unsigned NOT NULL DEFAULT 18," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `exchange_UNIQUE` (`exchange`)," +
		"UNIQUE KEY `token_UNIQUE` (`token`)," +
		"KEY `i_block_height` (`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createActionTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeActionTableName + "` (" +
		"`action_hash` varchar(40) NOT NULL," +
		"`idx` int(10) NOT NULL," +
		"`type` enum('" + AddLiquidity + "','" + RemoveLiquidity + "','" + TokenPurchase + "','" + CoinPurchase + "') NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`actor` varchar(41) NOT NULL," +
		"`iotx_amount` decimal(65,0) NOT NULL," +
		"`token_amount` decimal(65,0) NOT NULL," +
		"PRIMARY KEY (`action_hash`,`idx`)," +
		"KEY `i_action_hash` (`action_hash`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"KEY `i_actor` (`actor`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createExchangeViewQuery = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + ExchangeMonitorViewName + "` AS select `exchange` AS `account` from `" + ExchangeCreationTableName + "`"
	createTokenViewQuery    = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + TokenMonitorViewName + "` AS select `token`,`exchange` AS `account` from `" + ExchangeCreationTableName + "` union all select `exchange` AS `token`,'*' from `" + ExchangeCreationTableName + "`"

	insertExchangeQuery = "INSERT INTO `" + ExchangeCreationTableName + "` (`exchange`,`token`,`block_height`,`action_hash`,`token_name`,`token_symbol`,`token_decimals`) VALUES %s"
	insertActionsQuery  = "INSERT INTO `" + ExchangeActionTableName + "` (`action_hash`,`idx`,`type`,`exchange`,`block_height`,`actor`,`iotx_amount`,`token_amount`) VALUES %s"
)

var (
	tokenSymbol, _   = hex.DecodeString("95d89b41")
	tokenName, _     = hex.DecodeString("06fdde03")
	tokenDecimals, _ = hex.DecodeString("313ce567")
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
	if _, err := tx.Exec(createActionTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange action table")
	}

	return nil
}

// HandleBlockData handles blocks
func (p *Protocol) HandleBlockData(ctx context.Context, tx *sql.Tx, data *indexprotocol.BlockData) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	actionValStrs := make([]string, 0)
	actionValArgs := make([]interface{}, 0)
	if p.factoryAddr == nil {
		return nil
	}
	client, ok := services.ServiceClient(ctx)
	if !ok {
		return errors.New("failed to service client from context")
	}
	for _, receipt := range data.Block.Receipts {
		if receipt.Status != uint64(1) {
			continue
		}
		for i, l := range receipt.Logs() {
			if len(l.Topics) == 0 {
				continue
			}
			topic := hex.EncodeToString(l.Topics[0][:])
			if l.Address == p.factoryAddr.String() {
				if topic == "9d42cb017eb05bd8944ab536a8b35bc68085931dd5f4356489801453923953f9" { // create exchange
					token, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
					if err != nil {
						return err
					}
					name, err := indexprotocol.ReadContract(client, token.String(), tokenName)
					if err != nil {
						return err
					}
					symbol, err := indexprotocol.ReadContract(client, token.String(), tokenSymbol)
					if err != nil {
						return err
					}
					decimals, err := indexprotocol.ReadContract(client, token.String(), tokenDecimals)
					if err != nil {
						return err
					}
					exchange, err := indexprotocol.ConvertTopicToAddress(l.Topics[2])
					if err != nil {
						return err
					}
					valStrs = append(valStrs, "(?,?,?,?,?,?,?)")
					valArgs = append(
						valArgs,
						exchange.String(),
						token.String(),
						l.BlockHeight,
						hex.EncodeToString(l.ActionHash[:]),
						string(decodeString(name)),
						string(decodeString(symbol)),
						new(big.Int).SetBytes(decimals).Uint64(),
					)
				}
				continue
			}
			switch topic {
			case "06239653922ac7bea6aa2b19dc486b9361821d37712eb796adfd38d81de278ca": // add liquidity
				provider, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
				if err != nil {
					return err
				}
				iotxAmount := new(big.Int).SetBytes(l.Topics[2][:])
				tokenAmount := new(big.Int).SetBytes(l.Topics[3][:])
				actionValStrs = append(actionValStrs, "(?,?,?,?,?,?,?,?)")
				actionValArgs = append(
					actionValArgs,
					hex.EncodeToString(l.ActionHash[:]),
					i,
					AddLiquidity,
					l.Address,
					l.BlockHeight,
					provider.String(),
					iotxAmount.String(),
					tokenAmount.String(),
				)
			case "0fbf06c058b90cb038a618f8c2acbf6145f8b3570fd1fa56abb8f0f3f05b36e8": // remove liquidity
				provider, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
				if err != nil {
					return err
				}
				iotxAmount := new(big.Int).SetBytes(l.Topics[2][:])
				tokenAmount := new(big.Int).SetBytes(l.Topics[3][:])
				actionValStrs = append(actionValStrs, "(?,?,?,?,?,?,?,?)")
				actionValArgs = append(
					actionValArgs,
					hex.EncodeToString(l.ActionHash[:]),
					i,
					RemoveLiquidity,
					l.Address,
					l.BlockHeight,
					provider.String(),
					iotxAmount.String(),
					tokenAmount.String(),
				)
			case "cd60aa75dea3072fbc07ae6d7d856b5dc5f4eee88854f5b4abf7b680ef8bc50f": // token purchase
				buyer, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
				if err != nil {
					return err
				}
				iotxAmount := new(big.Int).SetBytes(l.Topics[2][:])
				tokenAmount := new(big.Int).SetBytes(l.Topics[3][:])
				actionValStrs = append(actionValStrs, "(?,?,?,?,?,?,?,?)")
				actionValArgs = append(
					actionValArgs,
					hex.EncodeToString(l.ActionHash[:]),
					i,
					TokenPurchase,
					l.Address,
					l.BlockHeight,
					buyer.String(),
					iotxAmount.String(),
					tokenAmount.String(),
				)
			case "bd5084afcc95a37b2846c5adaf2918caab943ad011b8830b1eb3f7ff81a8b24f": // iotx purchase
				buyer, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
				if err != nil {
					return err
				}
				tokenAmount := new(big.Int).SetBytes(l.Topics[2][:])
				iotxAmount := new(big.Int).SetBytes(l.Topics[3][:])
				actionValStrs = append(actionValStrs, "(?,?,?,?,?,?,?,?)")
				actionValArgs = append(
					actionValArgs,
					hex.EncodeToString(l.ActionHash[:]),
					i,
					CoinPurchase,
					l.Address,
					l.BlockHeight,
					buyer.String(),
					iotxAmount.String(),
					tokenAmount.String(),
				)
			}
		}
	}
	if len(valStrs) != 0 {
		fmt.Println(fmt.Sprintf(insertExchangeQuery, strings.Join(valStrs, ",")), valArgs)
		if _, err := tx.Exec(fmt.Sprintf(insertExchangeQuery, strings.Join(valStrs, ",")), valArgs...); err != nil {
			return err
		}
	}
	if len(actionValStrs) != 0 {
		if _, err := tx.Exec(fmt.Sprintf(insertActionsQuery, strings.Join(actionValStrs, ",")), actionValArgs...); err != nil {
			return err
		}
	}
	return nil
}

func decodeString(output []byte) string {
	return string(output[64 : 64+new(big.Int).SetBytes(output[32:64]).Uint64()])
}
