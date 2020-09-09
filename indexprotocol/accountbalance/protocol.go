// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package accountbalance

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
)

const (
	transactionTableName = "transactions"
	balanceTableName     = "balances"
)

var (
	createTransactionTable = "CREATE TABLE IF NOT EXISTS `" + transactionTableName + "` (" +
		"`block_height` decimal(65,0) NOT NULL," +
		"`action_hash` varchar(64) NOT NULL," +
		"`idx` INT(5) unsigned NOT NULL," +
		"`sender` varchar(41) NOT NULL," +
		"`recipient` varchar(41) NOT NULL," +
		"`amount` decimal(65, 0) unsigned NOT NULL," +
		"PRIMARY KEY (`action_hash`,`idx`)," +
		"KEY `i_block_height` (`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"
		/*
			createBalanceTable = "CREATE TABLE IF NOT EXISTS `" + balanceTableName + "` (" +
				"`account` varchar(41) NOT NULL," +
				"`block_height` decimal(65,0) unsigned NOT NULL," +
				"`income` decimal(65,0) unsigned DEFAULT NULL," +
				"`expense` decimal(65,0) unsigned DEFAULT NULL," +
				"PRIMARY KEY (`address`,`block_height`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=latin1;"
		*/
	createBalanceTable = "CREATE TABLE IF NOT EXISTS `" + balanceTableName + "` (" +
		"`account` varchar(41) NOT NULL," +
		"`block_height` decimal(65,0) unsigned NOT NULL," +
		"`balance` decimal(65,0) unsigned DEFAULT NULL," +
		"PRIMARY KEY (`account`,`block_height`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"

	insertTransactionQuery = "INSERT IGNORE INTO `" + transactionTableName + "` (`action_hash`, `idx`, `sender`, `recipient`, `block_height`, `amount`) SELECT ?,?,?,?,?,? FROM %s m WHERE m.account = ? OR m.account = ? LIMIT 1"

	/*
		updateBalances = "INSERT INTO `" + balanceTableName + "` (address, block_height, income, expense) " +
			"SELECT t1.address, %d, coalesce(SUM(t2.amount), 0), coalesce(SUM(t3.amount), 0) " +
			"FROM (SELECT `sender` address FROM `" + transactionTableName + "` WHERE block_height = %d UNION SELECT `recipient` address FROM `" + transactionTableName + "` WHERE block_height = %d) t1 " +
			"LEFT JOIN `" + transactionTableName + "` t2 ON t1.address = t2.recipient " +
			"LEFT JOIN `" + transactionTableName + "` t3 ON t1.address = t3.sender " +
			"GROUP BY t1.address"
	*/
	updateBalancesQuery = "INSERT INTO `" + balanceTableName + "` (`account`, `block_height`, `balance`) " +
		"SELECT delta_balances.account, ?, coalesce(current_balances.balance, 0) + coalesce(SUM(delta_balances.balance), 0) " +
		"FROM (" +
		"    SELECT b1.account, b1.balance " +
		"    FROM `" + balanceTableName + "` b1 INNER JOIN (" +
		"	     SELECT t.account, MAX(t.block_height) max_height " +
		"        FROM `" + balanceTableName + "` t INNER JOIN %s m ON t.account = m.account " +
		"        GROUP BY t.account" +
		"    ) h1 ON b1.account = h1.account and b1.block_height = h1.max_height" +
		") AS current_balances RIGHT JOIN (" +
		"    SELECT t.sender account, SUM(-t.amount) balance " +
		"    FROM `" + transactionTableName + "` t INNER JOIN %s m ON m.account = t.sender AND t.block_height = ? " +
		"    GROUP BY t.sender UNION ALL " +
		"    SELECT t.recipient account, SUM(t.amount) balance " +
		"    FROM `" + transactionTableName + "` t INNER JOIN %s m ON m.account = t.recipient AND t.block_height = ? " +
		"    GROUP BY t.recipient" +
		") AS delta_balances ON current_balances.account = delta_balances.account GROUP BY delta_balances.account"
)

type (
	// Transaction defines an IOTX transaction
	Transaction struct {
		Height     uint64
		ActionHash string
		Index      uint16
		Sender     string
		Recipient  string
		Amount     string
	}
)

// Protocol defines the account balance protocol
type Protocol struct {
	initBalances           map[string]*big.Int
	insertTransactionQuery string
	updateBalancesQuery    string
	withMonitorAccounts    bool
}

// NewProtocolWithMonitorTable creates a new protocol
func NewProtocolWithMonitorTable(monitorTableName string, initBalances map[string]*big.Int) *Protocol {
	return &Protocol{
		initBalances:           initBalances,
		insertTransactionQuery: fmt.Sprintf(insertTransactionQuery, monitorTableName),
		updateBalancesQuery:    fmt.Sprintf(updateBalancesQuery, monitorTableName, monitorTableName, monitorTableName),
	}
}

// Initialize creates tables and insert initial records
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.Exec(createBalanceTable); err != nil {
		return err
	}
	if _, err := tx.Exec(createTransactionTable); err != nil {
		return err
	}
	keys := make([]string, 0, len(p.initBalances))
	for key := range p.initBalances {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	count := 0
	for i, key := range keys {
		inserted, err := p.insertRecord(tx, 0, "", uint16(i), address.ZeroAddress, key, p.initBalances[key].String())
		if err != nil {
			return errors.Wrapf(err, "failed to insert initial balance for %s", key)
		}
		if inserted {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	return p.updateBalances(tx, 0)
}

// HandleBlockData handles block data
func (p *Protocol) HandleBlockData(ctx context.Context, tx *sql.Tx, data *indexprotocol.BlockData) error {
	count := 0
	height := data.Block.Height()
	for _, tl := range data.TransactionLogs {
		actionHash := hex.EncodeToString(tl.ActionHash)
		for i, l := range tl.Transactions {
			inserted, err := p.insertRecord(tx, height, actionHash, uint16(i), l.Sender, l.Recipient, l.Amount)
			if err != nil {
				return errors.Wrapf(err, "failed to process transaction %+v", l)
			}
			if inserted {
				count++
			}
		}
	}
	if count == 0 {
		return nil
	}

	return p.updateBalances(tx, height)
}

func (p *Protocol) updateBalances(tx *sql.Tx, height uint64) error {
	_, err := tx.Exec(p.updateBalancesQuery, height, height, height)

	return err
}

func (p *Protocol) insertRecord(
	tx *sql.Tx,
	height uint64,
	actionHash string,
	index uint16,
	sender string,
	recipient string,
	amount string,
) (bool, error) {
	if amount == "0" {
		return false, nil
	}
	if _, ok := new(big.Int).SetString(amount, 10); !ok {
		return false, errors.Errorf("failed to parse amount %s", amount)
	}
	_, err := tx.Exec(p.insertTransactionQuery, actionHash, index, sender, recipient, height, amount, sender, recipient)
	return true, err
}
