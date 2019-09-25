package accounts

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "accounts"
	// BalanceHistoryTableName is the table name of balance history
	BalanceHistoryTableName = "balance_history"
	// AccountInflowTableName is the table name of account inflow
	AccountInflowTableName = "account_inflow"
	// AccountOutflowTableName is the table name of account outflow
	AccountOutflowTableName = "account_outflow"
	// AccountIncomeTableName is the table name of account income
	AccountIncomeTableName = "account_income"
	// EpochAddressIndexName is the index name of epoch number and account address
	EpochAddressIndexName = "epoch_address_index"
)

var specialActionHash = hash.ZeroHash256

type (
	// BalanceHistory defines the base schema of "balance history" table
	BalanceHistory struct {
		EpochNumber uint64
		BlockHeight uint64
		ActionHash  string
		ActionType  string
		From        string
		To          string
		Amount      string
	}

	// AccountIncome defines the base schema of "account income" table
	AccountIncome struct {
		EpochNumber uint64
		Address     string
		Income      string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store        s.Store
	NumDelegates uint64
	NumSubEpochs uint64
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{
		Store:        store,
		NumDelegates: numDelegates,
		NumSubEpochs: numSubEpochs,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"(epoch_number DECIMAL(65, 0) NOT NULL, block_height DECIMAL(65, 0) NOT NULL, action_hash VARCHAR(64) NOT NULL, "+
		"action_type TEXT NOT NULL, `from` VARCHAR(41) NOT NULL, `to` VARCHAR(41) NOT NULL, amount DECIMAL(65, 0) NOT NULL)", BalanceHistoryTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"address VARCHAR(41) NOT NULL, inflow DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))", AccountInflowTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"address VARCHAR(41) NOT NULL, outflow DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))", AccountOutflowTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, "+
		"address VARCHAR(41) NOT NULL, income DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))", AccountIncomeTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	return nil
}

// Initialize initializes actions protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx, genesis *indexprotocol.Genesis) error {
	db := p.Store.GetDB()
	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE action_hash = ?",
		BalanceHistoryTableName), hex.EncodeToString(specialActionHash[:]))
	if err != nil {
		return errors.Wrap(err, "failed to check if the row exists")
	}
	if exist {
		return nil
	}
	for addr, amount := range genesis.InitBalanceMap {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
			BalanceHistoryTableName)
		if _, err := tx.Exec(insertQuery, uint64(0), uint64(0), hex.EncodeToString(specialActionHash[:]), "genesis", "", addr, amount); err != nil {
			return errors.Wrapf(err, "failed to update balance history for address %s", addr)
		}
	}
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := indexprotocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, height)
	// log action index
	hashToGasPrice := make(map[string]*big.Int)
	hashToSrcAddr := make(map[string]string)
	// Special handling for epoch start height
	epochHeight := indexprotocol.GetEpochHeight(epochNumber, p.NumDelegates, p.NumSubEpochs)
	if height == epochHeight {
		if err := p.rebuildAccountIncomeTable(tx); err != nil {
			return errors.Wrap(err, "failed to rebuild account income table")
		}
	}

	for _, selp := range blk.Actions {
		actionHash := selp.Hash()
		src, dst, err := getsrcAndDst(selp)
		if err != nil {
			return errors.Wrap(err, "failed to get source address and destination address")
		}
		hashToSrcAddr[hex.EncodeToString(actionHash[:])] = src
		hashToGasPrice[hex.EncodeToString(actionHash[:])] = selp.GasPrice()
		act := selp.Action()
		switch act := act.(type) {
		case *action.Transfer:
			actionType := "transfer"
			if err := p.updateBalanceHistory(tx, epochNumber, height, actionHash, actionType, dst, src, act.Amount().String()); err != nil {
				return errors.Wrapf(err, "failed to update balance history on height %d", height)
			}
		case *action.DepositToRewardingFund:
			actionType := "depositToRewardingFund"
			if err := p.updateBalanceHistory(tx, epochNumber, height, actionHash, actionType, "", src, act.Amount().String()); err != nil {
				return errors.Wrapf(err, "failed to update balance history on height %d", height)
			}
		case *action.ClaimFromRewardingFund:
			actionType := "claimFromRewardingFund"
			if err := p.updateBalanceHistory(tx, epochNumber, height, actionHash, actionType, src, "", act.Amount().String()); err != nil {
				return errors.Wrapf(err, "failed to update balance history on height %d", height)
			}
		}
	}
	for _, receipt := range blk.Receipts {
		actHash := receipt.ActionHash
		srcAddr, ok := hashToSrcAddr[hex.EncodeToString(actHash[:])]
		if !ok {
			return errors.New("failed to find the corresponding action from receipt")
		}
		gasPrice, ok := hashToGasPrice[hex.EncodeToString(actHash[:])]
		if !ok {
			return errors.New("failed to find the corresponding action from receipt")
		}
		gasFee := gasPrice.Mul(gasPrice, big.NewInt(0).SetUint64(receipt.GasConsumed))

		if gasFee.String() == "0" {
			continue
		}
		if err := p.updateBalanceHistory(tx, epochNumber, height, actHash, "gasFee", "", srcAddr, gasFee.String()); err != nil {
			return errors.Wrapf(err, "failed to update balance history with address %s", srcAddr)
		}
	}

	return nil
}

func (p *Protocol) getBalanceHistory(address string) ([]*BalanceHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE `from`=? OR `to`=?",
		BalanceHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(address, address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var balanceHistory BalanceHistory
	parsedRows, err := s.ParseSQLRows(rows, &balanceHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	var balanceHistoryList []*BalanceHistory
	for _, parsedRow := range parsedRows {
		balChange := parsedRow.(*BalanceHistory)
		balanceHistoryList = append(balanceHistoryList, balChange)
	}
	return balanceHistoryList, nil
}

func (p *Protocol) getAccountIncome(epochNumber uint64, address string) (*AccountIncome, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number = ? AND address = ?",
		AccountIncomeTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var accountIncome AccountIncome
	parsedRows, err := s.ParseSQLRows(rows, &accountIncome)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	return parsedRows[0].(*AccountIncome), nil
}

func (p *Protocol) updateBalanceHistory(
	tx *sql.Tx,
	epochNumber uint64,
	blockHeight uint64,
	actionHash hash.Hash256,
	actionType string,
	to string,
	from string,
	amount string,
) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
		BalanceHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(actionHash[:]), actionType, from, to, amount); err != nil {
		return errors.Wrap(err, "failed to update balance history")
	}
	return nil
}

func (p *Protocol) rebuildAccountIncomeTable(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf("INSERT IGNORE INTO %s SELECT epoch_number, `to` AS address, "+
		"SUM(amount) AS inflow FROM %s GROUP BY epoch_number, `to`", AccountInflowTableName, BalanceHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf("INSERT IGNORE INTO %s SELECT epoch_number, `from` AS address, "+
		"SUM(amount) AS outflow FROM %s GROUP BY epoch_number, `from`", AccountOutflowTableName, BalanceHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf("INSERT IGNORE INTO %s SELECT t1.epoch_number, t1.address, "+
		"CAST(IFNULL(inflow, 0) AS DECIMAL(65, 0)) - CAST(IFNULL(outflow, 0) AS DECIMAL(65, 0)) AS income "+
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.epoch_number = t2.epoch_number AND t1.address=t2.address UNION "+
		"SELECT t2.epoch_number, t2.address, CAST(IFNULL(inflow, 0) AS DECIMAL(65, 0)) - CAST(IFNULL(outflow, 0) AS DECIMAL(65, 0)) AS income "+
		"FROM %s AS t1 RIGHT JOIN %s AS t2 ON t1.epoch_number = t2.epoch_number AND t1.address=t2.address", AccountIncomeTableName,
		AccountInflowTableName, AccountOutflowTableName, AccountInflowTableName, AccountOutflowTableName)); err != nil {
		return err
	}

	return nil
}

func getsrcAndDst(selp action.SealedEnvelope) (string, string, error) {
	callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
	if err != nil {
		return "", "", err
	}
	dst, _ := selp.Destination()
	return callerAddr.String(), dst, nil
}
