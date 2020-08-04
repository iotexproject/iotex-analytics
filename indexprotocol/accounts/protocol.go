package accounts

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
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

	createBalanceHistory = "CREATE TABLE IF NOT EXISTS %s " +
		"(epoch_number DECIMAL(65, 0) NOT NULL, block_height DECIMAL(65, 0) NOT NULL, action_hash VARCHAR(64) NOT NULL, " +
		"action_type TEXT NOT NULL, `from` VARCHAR(41) NOT NULL, `to` VARCHAR(41) NOT NULL, amount DECIMAL(65, 0) NOT NULL)"
	createAccountInflow = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"address VARCHAR(41) NOT NULL, inflow DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))"
	createAccountOutflow = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"address VARCHAR(41) NOT NULL, outflow DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))"
	createAccountIncome = "CREATE TABLE IF NOT EXISTS %s (epoch_number DECIMAL(65, 0) NOT NULL, " +
		"address VARCHAR(41) NOT NULL, income DECIMAL(65, 0) NOT NULL, UNIQUE KEY %s (epoch_number, address))"
	rowExists            = "SELECT * FROM %s WHERE action_hash = ?"
	insertBalanceHistory = "INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)"
	selectBalanceHistory = "SELECT * FROM %s WHERE `from`=? OR `to`=?"
	selectAccountIncome  = "SELECT * FROM %s WHERE epoch_number = ? AND address = ?"
	insertAccountInflow  = "INSERT IGNORE INTO %s SELECT epoch_number, `to` AS address, " +
		"SUM(amount) AS inflow FROM %s GROUP BY epoch_number, `to`"
	insertAccountOutflow = "INSERT IGNORE INTO %s SELECT epoch_number, `from` AS address, " +
		"SUM(amount) AS outflow FROM %s GROUP BY epoch_number, `from`"
	insertAccountIncome = "INSERT IGNORE INTO %s SELECT t1.epoch_number, t1.address, " +
		"CAST(IFNULL(inflow, 0) AS DECIMAL(65, 0)) - CAST(IFNULL(outflow, 0) AS DECIMAL(65, 0)) AS income " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.epoch_number = t2.epoch_number AND t1.address=t2.address UNION " +
		"SELECT t2.epoch_number, t2.address, CAST(IFNULL(inflow, 0) AS DECIMAL(65, 0)) - CAST(IFNULL(outflow, 0) AS DECIMAL(65, 0)) AS income " +
		"FROM %s AS t1 RIGHT JOIN %s AS t2 ON t1.epoch_number = t2.epoch_number AND t1.address=t2.address"

	selectIndexInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	createIndex         = "CREATE INDEX %s ON %s (action_hash)"
	actionHashIndexName = "action_hash_index"
)

const (
	transfer                   = "transfer"
	execution                  = "execution"
	depositToRewardingFund     = "depositToRewardingFund"
	claimFromRewardingFund     = "claimFromRewardingFund"
	stakeCreate                = "stakeCreate"
	stakeWithdraw              = "stakeWithdraw"
	stakeAddDeposit            = "stakeAddDeposit"
	candidateRegisterFee       = "candidateRegisterFee"
	candidateRegisterSelfStake = "candidateRegisterSelfStake"
	gasFee                     = "gasFee"
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
	Store    s.Store
	epochCtx *epochctx.EpochCtx
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, epochctx *epochctx.EpochCtx) *Protocol {
	return &Protocol{
		Store:    store,
		epochCtx: epochctx,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createBalanceHistory, BalanceHistoryTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createAccountInflow, AccountInflowTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createAccountOutflow, AccountOutflowTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createAccountIncome, AccountIncomeTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectIndexInfo, BalanceHistoryTableName, actionHashIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createIndex, actionHashIndexName, BalanceHistoryTableName)); err != nil {
			return err
		}
	}
	return nil
}

// Initialize initializes actions protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx, genesis *indexprotocol.Genesis) error {
	db := p.Store.GetDB()
	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf(rowExists,
		BalanceHistoryTableName), hex.EncodeToString(specialActionHash[:]))
	if err != nil {
		return errors.Wrap(err, "failed to check if the row exists")
	}
	if exist {
		return nil
	}
	for addr, amount := range genesis.InitBalanceMap {
		insertQuery := fmt.Sprintf(insertBalanceHistory,
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
	epochNumber := p.epochCtx.GetEpochNumber(height)
	// Special handling for epoch start height
	epochHeight := p.epochCtx.GetEpochHeight(epochNumber)
	if height == epochHeight {
		if err := p.rebuildAccountIncomeTable(tx); err != nil {
			return errors.Wrap(err, "failed to rebuild account income table")
		}
	}
	actionSuccess := make(map[hash.Hash256]bool)
	for _, receipt := range blk.Receipts {
		if receipt.Status == uint64(1) {
			actionSuccess[receipt.ActionHash] = true
		}
	}
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	transferLogMap, err := getTransactionLog(ctx, height, indexCtx.ChainClient)
	if err != nil {
		return err
	}
	for h, transactions := range transferLogMap {
		for _, transaction := range transactions {
			actionType := getActionType(transaction.Type)
			if err := p.updateBalanceHistory(tx, epochNumber, height, h, actionType, transaction.GetRecipient(), transaction.GetSender(), transaction.GetAmount()); err != nil {
				return errors.Wrapf(err, "failed to update balance history on height %d", height)
			}
		}
	}

	return nil
}

func (p *Protocol) getBalanceHistory(address string) ([]*BalanceHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectBalanceHistory,
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

	getQuery := fmt.Sprintf(selectAccountIncome,
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
	actionHash string,
	actionType string,
	to string,
	from string,
	amount string,
) error {
	insertQuery := fmt.Sprintf(insertBalanceHistory,
		BalanceHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, actionHash, actionType, from, to, amount); err != nil {
		return errors.Wrap(err, "failed to update balance history")
	}
	return nil
}

func (p *Protocol) rebuildAccountIncomeTable(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf(insertAccountInflow, AccountInflowTableName, BalanceHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(insertAccountOutflow, AccountOutflowTableName, BalanceHistoryTableName)); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(insertAccountIncome, AccountIncomeTableName,
		AccountInflowTableName, AccountOutflowTableName, AccountInflowTableName, AccountOutflowTableName)); err != nil {
		return err
	}

	return nil
}

func getTransactionLog(ctx context.Context, height uint64, client iotexapi.APIServiceClient) (
	transferLogMap map[string][]*iotextypes.TransactionLog_Transaction, err error) {
	transferLogMap = make(map[string][]*iotextypes.TransactionLog_Transaction)
	transferLog, err := client.GetTransactionLogByBlockHeight(
		ctx,
		&iotexapi.GetTransactionLogByBlockHeightRequest{BlockHeight: height},
	)

	if err == nil {
		for _, a := range transferLog.GetTransactionLogs().GetLogs() {
			h := hex.EncodeToString(a.ActionHash)
			transferLogMap[h] = a.GetTransactions()
		}
	}
	return transferLogMap, nil
}

func getActionType(t iotextypes.TransactionLogType) string {
	switch {
	case t == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER:
		return execution
	case t == iotextypes.TransactionLogType_WITHDRAW_BUCKET:
		return stakeWithdraw
	case t == iotextypes.TransactionLogType_CREATE_BUCKET:
		return stakeCreate
	case t == iotextypes.TransactionLogType_DEPOSIT_TO_BUCKET:
		return stakeAddDeposit
	case t == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND:
		return claimFromRewardingFund
	case t == iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND:
		return depositToRewardingFund
	case t == iotextypes.TransactionLogType_CANDIDATE_REGISTRATION_FEE:
		return candidateRegisterFee
	case t == iotextypes.TransactionLogType_CANDIDATE_SELF_STAKE:
		return candidateRegisterSelfStake
	case t == iotextypes.TransactionLogType_GAS_FEE:
		return gasFee
	case t == iotextypes.TransactionLogType_NATIVE_TRANSFER:
		return transfer
	}
	return ""
}
