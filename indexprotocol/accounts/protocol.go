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
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "accounts"
	// AccountHistoryTableName is the table name of account history
	AccountHistoryTableName = "account_history"
	// AccountBalanceViewName is the view name of account balance
	AccountBalanceViewName = "account_balance"
	// EpochAddressIndexName is the index name of epoch number and address on account history table
	EpochAddressIndexName = "epoch_address_index"
)

var specialActionHash = hash.ZeroHash256

type (
	// AccountHistory defines the base schema of "account history" table
	AccountHistory struct {
		EpochNumber uint64
		BlockHeight uint64
		ActionHash  string
		Address     string
		In          string
		Out         string
	}

	// AccountBalance defines the base schema of "account balance" view
	AccountBalance struct {
		EpochNumber   uint64
		Address       string
		BalanceChange string
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
		"address VARCHAR(41) NOT NULL, `in` DECIMAL(65, 0) DEFAULT 0, `out` DECIMAL(65, 0) DEFAULT 0)", AccountHistoryTableName)); err != nil {
		return err
	}

	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf("SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = "+
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", AccountHistoryTableName, EpochAddressIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE INDEX %s ON %s (epoch_number, address)", EpochAddressIndexName, AccountHistoryTableName)); err != nil {
			return err
		}
	}

	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE OR REPLACE VIEW %s AS SELECT epoch_number, address, (SUM(`in`)-SUM(`out`)) AS balance_change "+
		"FROM %s GROUP BY epoch_number, address", AccountBalanceViewName, AccountHistoryTableName)); err != nil {
		return err
	}

	return nil
}

// Initialize initializes actions protocol
func (p *Protocol) Initialize(ctx context.Context, tx *sql.Tx, genesis *indexprotocol.Genesis) error {
	for addr, amount := range genesis.InitBalanceMap {
		insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (epoch_number, block_height, action_hash, address, `in`) VALUES (?, ?, ?, ?, ?)",
			AccountHistoryTableName)
		if _, err := tx.Exec(insertQuery, uint64(0), uint64(0), hex.EncodeToString(specialActionHash[:]), addr, amount); err != nil {
			return errors.Wrapf(err, "failed to update account history for address %s", addr)
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
			if act.Amount().Sign() > 0 {
				if err := p.updateAccountHistory(tx, epochNumber, height, actionHash, dst, src, act.Amount().String()); err != nil {
					return errors.Wrapf(err, "failed to update account history on height %d", height)
				}
			}
		case *action.Execution:
			if dst != "" && act.Amount().Sign() > 0 {
				if err := p.updateAccountHistory(tx, epochNumber, height, actionHash, dst, src, act.Amount().String()); err != nil {
					return errors.Wrapf(err, "failed to update account history on height %d", height)
				}
			}
		case *action.DepositToRewardingFund:
			if act.Amount().Sign() > 0 {
				if err := p.updateAccountHistory(tx, epochNumber, height, actionHash, "", src, act.Amount().String()); err != nil {
					return errors.Wrapf(err, "failed to update account history on height %d", height)
				}
			}
		case *action.ClaimFromRewardingFund:
			if act.Amount().Sign() > 0 {
				if err := p.updateAccountHistory(tx, epochNumber, height, actionHash, src, "", act.Amount().String()); err != nil {
					return errors.Wrapf(err, "failed to update account history on height %d", height)
				}
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

		if err := p.updateAccountHistory(tx, epochNumber, height, actHash, "", srcAddr, gasFee.String()); err != nil {
			return errors.Wrapf(err, "failed to update account history with address %s", srcAddr)
		}
	}

	return nil
}

func (p *Protocol) getAccountHistory(address string) ([]*AccountHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE address=?",
		AccountHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var accountHistory AccountHistory
	parsedRows, err := s.ParseSQLRows(rows, &accountHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	var accountHistoryList []*AccountHistory
	for _, parsedRow := range parsedRows {
		acctChange := parsedRow.(*AccountHistory)
		accountHistoryList = append(accountHistoryList, acctChange)
	}
	return accountHistoryList, nil
}

func (p *Protocol) getAccountBalanceChange(epochNumber uint64, address string) (*AccountBalance, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND address=?",
		AccountBalanceViewName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber, address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var accountBalance AccountBalance
	parsedRows, err := s.ParseSQLRows(rows, &accountBalance)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	return parsedRows[0].(*AccountBalance), nil
}

func (p *Protocol) updateAccountHistory(
	tx *sql.Tx,
	epochNumber uint64,
	blockHeight uint64,
	actionHash hash.Hash256,
	inAddr string,
	outAddr string,
	amount string,
) error {
	if inAddr != "" {
		insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (epoch_number, block_height, action_hash, address, `in`) VALUES (?, ?, ?, ?, ?)",
			AccountHistoryTableName)
		if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(actionHash[:]), inAddr, amount); err != nil {
			return errors.Wrapf(err, "failed to update account history for address %s", inAddr)
		}
	}
	if outAddr != "" {
		insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (epoch_number, block_height, action_hash, address, `out`) VALUES (?, ?, ?, ?, ?)",
			AccountHistoryTableName)
		if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(actionHash[:]), outAddr, amount); err != nil {
			return errors.Wrapf(err, "failed to update account history for address %s", outAddr)
		}
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
