// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mimo

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

	// ExchangeCreationTableName is the table storing exchange creation records
	ExchangeCreationTableName = "mimo_exchange_creations"

	// ExchangeMonitorViewName is the table storing all the exchange addresses
	ExchangeMonitorViewName = "mimo_exchange_to_monitor"

	// TokenMonitorViewName is the table storing all the <token,account> to monitor
	TokenMonitorViewName = "mimo_token_to_monitor"

	// ExchangeActionTableName is the table storing the actions of exchanges
	ExchangeActionTableName = "mimo_exchange_actions"

	// TokenActionTableName is the table storing the actions of token from/to exchanges
	TokenActionTableName = "mimo_token_actions"

	// ExchangeTokenActionTableName is the table storing the actions of exchange tokens
	ExchangeTokenActionTableName = "mimo_exchange_token_actions"

	// CoinBalanceTableName is the table storing the coin balances of exchanges
	CoinBalanceTableName = "mimo_coin_balances"

	// TokenBalanceTableName is the table stroing the token balances of exchanges
	TokenBalanceTableName = "mimo_token_balances"

	// ProviderBalanceTableName is the table storing the exchange holders
	ProviderBalanceTableName = "mimo_holder_balances"

	// SupplyTableName is the table storing the supplies of exchanges
	SupplyTableName = "mimo_supplies"
)

var (
	createTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeCreationTableName + "` (" +
		"`exchange` varchar(41) NOT NULL," +
		"`token` varchar(41) NOT NULL," +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`action_hash` varchar(64) NOT NULL," +
		"`token_name` varchar(140) NOT NULL," +
		"`token_symbol` varchar(140) NOT NULL," +
		"`token_decimals` int(10) unsigned NOT NULL DEFAULT 18," +
		"PRIMARY KEY (`exchange`,`token`)," +
		"UNIQUE KEY `exchange_UNIQUE` (`exchange`)," +
		"UNIQUE KEY `token_UNIQUE` (`token`)," +
		"KEY `i_block_height` (`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createExchangeActionTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeActionTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`action_hash` varchar(64) NOT NULL," +
		"`idx` int(10) NOT NULL," +
		"`type` enum(" + JoinTopicsWithQuotes(AddLiquidity, RemoveLiquidity, CoinPurchase, TokenPurchase) + ") NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`provider` varchar(41) NOT NULL," +
		"`iotx_amount` decimal(65,0) unsigned NOT NULL," +
		"`token_amount` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`action_hash`,`idx`)," +
		"KEY `i_action_hash` (`action_hash`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"KEY `i_provider` (`provider`)," +
		"KEY `i_type` (`type`)," +
		"CONSTRAINT `fk_actions_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createTokenActionTableQuery = "CREATE TABLE IF NOT EXISTS `" + TokenActionTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`action_hash` varchar(64) NOT NULL," +
		"`idx` int(10) NOT NULL," +
		"`token` varchar(41) NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`amount` decimal(65,0) NOT NULL," +
		"PRIMARY KEY (`action_hash`,`idx`,`exchange`,`token`)," +
		"KEY `i_action_hash` (`action_hash`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"KEY `i_token` (`token`)," +
		"KEY `fk_exchange_token` (`exchange`,`token`)," +
		"CONSTRAINT `fk_exchange_token` FOREIGN KEY (`exchange`, `token`) REFERENCES `mimo_exchange_creations` (`exchange`, `token`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createExchangeTokenActionTableQuery = "CREATE TABLE IF NOT EXISTS `" + ExchangeTokenActionTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`action_hash` varchar(64) NOT NULL," +
		"`idx` int(10) NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`sender` varchar(41) NOT NULL," +
		"`recipient` varchar(41) NOT NULL," +
		"`amount` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`action_hash`,`idx`)," +
		"KEY `i_action_hash` (`action_hash`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"KEY `i_sender` (`sender`)," +
		"KEY `i_recipient` (`recipient`)," +
		"CONSTRAINT `fk_token_actions_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createCoinBalanceTableQuery = "CREATE TABLE IF NOT EXISTS `" + CoinBalanceTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`balance` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`,`exchange`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"CONSTRAINT `fk_coin_balances_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createTokenBalanceTableQuery = "CREATE TABLE IF NOT EXISTS `" + TokenBalanceTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`balance` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`,`exchange`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"CONSTRAINT `fk_token_balances_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createProviderBalanceTableQuery = "CREATE TABLE IF NOT EXISTS `" + ProviderBalanceTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`provider` varchar(41) NOT NULL," +
		"`balance` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`,`exchange`,`provider`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"KEY `i_provider` (`provider`)," +
		"CONSTRAINT `fk_holder_balances_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createSupplyTableQuery = "CREATE TABLE IF NOT EXISTS `" + SupplyTableName + "` (" +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`exchange` varchar(41) NOT NULL," +
		"`supply` decimal(65,0) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`,`exchange`)," +
		"KEY `i_block_height` (`block_height`)," +
		"KEY `i_exchange` (`exchange`)," +
		"CONSTRAINT `fk_supplies_exchange` FOREIGN KEY (`exchange`) REFERENCES `mimo_exchange_creations` (`exchange`) ON DELETE NO ACTION ON UPDATE NO ACTION" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	createExchangeViewQuery = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + ExchangeMonitorViewName + "` AS select `exchange` AS `account` from `" + ExchangeCreationTableName + "`"
	createTokenViewQuery    = "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`admin`@`%` SQL SECURITY DEFINER VIEW `" + TokenMonitorViewName + "` AS select `token`,`exchange` AS `account` from `" + ExchangeCreationTableName + "` union all select `exchange` AS `token`,'*' from `" + ExchangeCreationTableName + "`"

	insertExchangeQuery             = "INSERT INTO `" + ExchangeCreationTableName + "` (`exchange`,`token`,`block_height`,`action_hash`,`token_name`,`token_symbol`,`token_decimals`) VALUES %s"
	insertExchangeActionsQuery      = "INSERT INTO `" + ExchangeActionTableName + "` (`block_height`, `action_hash`,`idx`,`type`,`exchange`,`provider`,`iotx_amount`,`token_amount`) VALUES %s"
	insertTokenActionsQuery         = "INSERT IGNORE INTO `" + TokenActionTableName + "` (`block_height`,`action_hash`,`idx`,`token`,`exchange`,`amount`) VALUES %s"
	insertExchangeTokenActionsQuery = "INSERT IGNORE INTO `" + ExchangeTokenActionTableName + "` (`block_height`,`action_hash`,`idx`,`exchange`,`sender`,`recipient`,`amount`) VALUES %s"

	updateCoinBalancesQuery = "INSERT INTO `" + CoinBalanceTableName + "` (`block_height`,`exchange`,`balance`) " +
		"SELECT ?, delta.exchange, coalesce(curr.balance, 0)+coalesce(delta.amount, 0) " +
		"FROM (" +
		"    SELECT cb.exchange, cb.balance " +
		"    FROM `" + CoinBalanceTableName + "` cb " +
		"    INNER JOIN (" +
		"        SELECT `exchange`, MAX(`block_height`) `max_height` " +
		"        FROM `" + CoinBalanceTableName + "` " +
		"        WHERE `block_height` < ? " +
		"        GROUP BY `exchange` " +
		"    ) h1 ON cb.exchange = h1.exchange AND cb.block_height = h1.max_height" +
		") AS `curr` RIGHT JOIN (" +
		"    SELECT `exchange`, SUM(IF(`type` in (" + JoinTopicsWithQuotes(AddLiquidity, TokenPurchase) + "), `iotx_amount`, -`iotx_amount`)) `amount` " +
		"    FROM `" + ExchangeActionTableName + "` " +
		"    WHERE `block_height` = ? " +
		"    GROUP BY `exchange` " +
		") AS `delta` ON delta.exchange = curr.exchange "

	updateTokenBalancesQuery = "INSERT INTO `" + TokenBalanceTableName + "` (`block_height`,`exchange`,`balance`) " +
		"SELECT ?, delta.exchange, coalesce(curr.balance, 0)+coalesce(delta.amount, 0) " +
		"FROM (" +
		"    SELECT tb.exchange, tb.balance " +
		"    FROM `" + TokenBalanceTableName + "` tb " +
		"    INNER JOIN (" +
		"        SELECT `exchange`, MAX(`block_height`) `max_height` " +
		"        FROM `" + TokenBalanceTableName + "` " +
		"        WHERE `block_height` < ? " +
		"        GROUP BY `exchange` " +
		"    ) h1 ON tb.exchange = h1.exchange AND tb.block_height = h1.max_height" +
		") AS `curr` RIGHT JOIN (" +
		"    SELECT `exchange`, SUM(`amount`) `amount` " +
		"    FROM `" + TokenActionTableName + "` " +
		"    WHERE `block_height` = ? " +
		"    GROUP BY `exchange` " +
		") AS `delta` ON delta.exchange = curr.exchange "

	updateProviderBalancesQuery = "INSERT INTO `" + ProviderBalanceTableName + "` (`block_height`,`exchange`,`provider`,`balance`) " +
		"SELECT ?, delta.exchange, delta.provider, coalesce(curr.balance, 0)+coalesce(delta.amount, 0) " +
		"FROM (" +
		"    SELECT pb.exchange, pb.provider, pb.balance " +
		"    FROM `" + ProviderBalanceTableName + "` pb " +
		"    INNER JOIN (" +
		"        SELECT `exchange`,`provider`,MAX(`block_height`) `max_height` " +
		"        FROM `" + ProviderBalanceTableName + "` " +
		"        WHERE `block_height` < ? " +
		"        GROUP BY `exchange`,`provider`" +
		"    ) h1 ON pb.exchange = h1.exchange AND pb.provider = h1.provider AND pb.block_height = h1.max_height" +
		") AS `curr` RIGHT JOIN (SELECT `exchange`, `provider`, SUM(`amount`) `amount` FROM ((" +
		"    SELECT `exchange`, `sender` `provider`, SUM(-`amount`) `amount` " +
		"    FROM `" + ExchangeTokenActionTableName + "` " +
		"    WHERE `block_height` = ? AND `sender` != '" + address.ZeroAddress + "' " +
		"    GROUP BY `exchange`,`sender`" +
		") UNION (" +
		"    SELECT `exchange`, `recipient` `provider`, SUM(`amount`) `amount` " +
		"    FROM `" + ExchangeTokenActionTableName + "` " +
		"    WHERE `block_height` = ? AND `recipient` != '" + address.ZeroAddress + "' " +
		"    GROUP BY `exchange`,`recipient`" +
		")) delta_union GROUP BY `exchange`, `provider`) AS `delta` ON delta.exchange = curr.exchange AND delta.provider = curr.provider"

	updateSuppliesQuery = "INSERT INTO `" + SupplyTableName + "` (`block_height`,`exchange`,`supply`) " +
		"SELECT ?, delta.exchange,  coalesce(curr.supply, 0)+coalesce(delta.amount, 0) " +
		"FROM (" +
		"    SELECT s.exchange, s.supply " +
		"    FROM `" + SupplyTableName + "` s " +
		"    INNER JOIN (" +
		"        SELECT `exchange`, MAX(`block_height`) `max_height` " +
		"        FROM `" + SupplyTableName + "` " +
		"        WHERE `block_height` < ? " +
		"        GROUP BY `exchange`" +
		"    ) h1 ON s.exchange = h1.exchange AND s.block_height = h1.max_height" +
		") AS `curr` RIGHT JOIN (" +
		"    SELECT `exchange`,SUM(IF(`sender` = '" + address.ZeroAddress + "', `amount`, -`amount`)) `amount` " +
		"    FROM `" + ExchangeTokenActionTableName + "` " +
		"    WHERE `block_height` = ? AND (`sender` = '" + address.ZeroAddress + "' OR `recipient` = '" + address.ZeroAddress + "')" +
		"    GROUP BY `exchange`" +
		") AS `delta` ON delta.exchange = curr.exchange"
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
	if _, err := tx.Exec(createExchangeActionTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange action table")
	}
	if _, err := tx.Exec(createTokenActionTableQuery); err != nil {
		return errors.Wrap(err, "failed to create token action of exchanges table")
	}
	if _, err := tx.Exec(createExchangeTokenActionTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange token action table")
	}
	if _, err := tx.Exec(createCoinBalanceTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange coin balance table")
	}
	if _, err := tx.Exec(createTokenBalanceTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange token balance table")
	}
	if _, err := tx.Exec(createProviderBalanceTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange provider balance table")
	}
	if _, err := tx.Exec(createSupplyTableQuery); err != nil {
		return errors.Wrap(err, "failed to create exchange supply table")
	}
	return nil
}

// HandleBlockData handles blocks
func (p *Protocol) HandleBlockData(ctx context.Context, tx *sql.Tx, data *indexprotocol.BlockData) error {
	creationValStrs := make([]string, 0)
	creationValArgs := make([]interface{}, 0)
	actionValStrs := make([]string, 0)
	actionValArgs := make([]interface{}, 0)
	tokenTransferValStrs := make([]string, 0)
	tokenTransferValArgs := make([]interface{}, 0)
	exchangeTokenTransferValStrs := make([]string, 0)
	exchangeTokenTransferValArgs := make([]interface{}, 0)
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
					creationValStrs = append(creationValStrs, "(?,?,?,?,?,?,?)")
					creationValArgs = append(
						creationValArgs,
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
			var eventTopic EventTopic
			var iotxAmount, tokenAmount string
			switch topic {
			case "06239653922ac7bea6aa2b19dc486b9361821d37712eb796adfd38d81de278ca":
				eventTopic = AddLiquidity
				iotxAmount = new(big.Int).SetBytes(l.Topics[2][:]).String()
				tokenAmount = new(big.Int).SetBytes(l.Topics[3][:]).String()
			case "0fbf06c058b90cb038a618f8c2acbf6145f8b3570fd1fa56abb8f0f3f05b36e8":
				eventTopic = RemoveLiquidity
				iotxAmount = new(big.Int).SetBytes(l.Topics[2][:]).String()
				tokenAmount = new(big.Int).SetBytes(l.Topics[3][:]).String()
			case "cd60aa75dea3072fbc07ae6d7d856b5dc5f4eee88854f5b4abf7b680ef8bc50f":
				eventTopic = TokenPurchase
				iotxAmount = new(big.Int).SetBytes(l.Topics[2][:]).String()
				tokenAmount = new(big.Int).SetBytes(l.Topics[3][:]).String()
			case "bd5084afcc95a37b2846c5adaf2918caab943ad011b8830b1eb3f7ff81a8b24f":
				eventTopic = CoinPurchase
				tokenAmount = new(big.Int).SetBytes(l.Topics[2][:]).String()
				iotxAmount = new(big.Int).SetBytes(l.Topics[3][:]).String()
			case "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef":
				amount := new(big.Int).SetBytes(l.Data)
				if amount.Cmp(big.NewInt(0)) > 0 {
					sender, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
					if err != nil {
						return err
					}
					recipient, err := indexprotocol.ConvertTopicToAddress(l.Topics[2])
					if err != nil {
						return err
					}
					exchangeTokenTransferValStrs = append(exchangeTokenTransferValStrs, "(?,?,?,?,?,?,?)")
					exchangeTokenTransferValArgs = append(
						exchangeTokenTransferValArgs,
						l.BlockHeight,
						hex.EncodeToString(l.ActionHash[:]),
						i,
						l.Address,
						sender.String(),
						recipient.String(),
						amount.String(),
					)
					tokenTransferValStrs = append(tokenTransferValStrs, "(?,?,?,?,?,?)", "(?,?,?,?,?,?)")
					tokenTransferValArgs = append(
						tokenTransferValArgs,
						l.BlockHeight,
						hex.EncodeToString(l.ActionHash[:]),
						i,
						l.Address,
						sender.String(),
						new(big.Int).Neg(amount).String(),
						l.BlockHeight,
						hex.EncodeToString(l.ActionHash[:]),
						i,
						l.Address,
						recipient.String(),
						amount.String(),
					)
				}
			}
			if eventTopic.IsValid() {
				actor, err := indexprotocol.ConvertTopicToAddress(l.Topics[1])
				if err != nil {
					return err
				}
				actionValStrs = append(actionValStrs, "(?,?,?,?,?,?,?,?)")
				actionValArgs = append(
					actionValArgs,
					l.BlockHeight,
					hex.EncodeToString(l.ActionHash[:]),
					i,
					eventTopic,
					l.Address,
					actor.String(),
					iotxAmount,
					tokenAmount,
				)
			}
		}
	}
	if len(creationValStrs) != 0 {
		if _, err := tx.Exec(fmt.Sprintf(insertExchangeQuery, strings.Join(creationValStrs, ",")), creationValArgs...); err != nil {
			return errors.Wrap(err, "failed to insert exchange creation records")
		}
	}
	if len(actionValStrs) != 0 {
		if _, err := tx.Exec(fmt.Sprintf(insertExchangeActionsQuery, strings.Join(actionValStrs, ",")), actionValArgs...); err != nil {
			return errors.Wrap(err, "failed to insert exchange action records")
		}
	}
	if len(tokenTransferValStrs) != 0 {
		if _, err := tx.Exec(fmt.Sprintf(insertTokenActionsQuery, strings.Join(tokenTransferValStrs, ",")), tokenTransferValArgs...); err != nil {
			return errors.Wrap(err, "failed to insert token transfer records")
		}
	}
	if len(exchangeTokenTransferValStrs) != 0 {
		if _, err := tx.Exec(fmt.Sprintf(insertExchangeTokenActionsQuery, strings.Join(exchangeTokenTransferValStrs, ",")), exchangeTokenTransferValArgs...); err != nil {
			return errors.Wrap(err, "failed to insert exchange token transfer records")
		}
	}
	height := data.Block.Height()
	if _, err := tx.Exec(updateCoinBalancesQuery, height, height, height); err != nil {
		return errors.Wrap(err, "failed to update coin balances")
	}
	if _, err := tx.Exec(updateTokenBalancesQuery, height, height, height); err != nil {
		return errors.Wrap(err, "failed to update token balances")
	}
	if _, err := tx.Exec(updateProviderBalancesQuery, height, height, height, height); err != nil {
		return errors.Wrap(err, "failed to update provider balances")
	}
	if _, err := tx.Exec(updateSuppliesQuery, height, height, height); err != nil {
		return errors.Wrap(err, "failed to update supplies")
	}

	return nil
}

func decodeString(output []byte) string {
	return string(output[64 : 64+new(big.Int).SetBytes(output[32:64]).Uint64()])
}
