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

func getsrcAndDst(selp action.SealedEnvelope) (string, string, error) {
	callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
	if err != nil {
		return "", "", err
	}
	dst, _ := selp.Destination()
	return callerAddr.String(), dst, nil
}
