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
	"strings"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "actions"
	// ActionHistoryTableName is the table name of action history
	ActionHistoryTableName = "action_history"
	// FromIndexName is the 'from' index name of ActionHistory table
	FromIndexName = "from_index"
	// ToIndexName is the 'to' index name of ActionHistory table
	ToIndexName             = "to_index"
	selectActionHistoryInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	createActionHistoryFromIndex = "CREATE INDEX %s ON %s (`from`)"
	createActionHistoryToIndex   = "CREATE INDEX %s ON %s (`to`)"
	createActionHistory          = "CREATE TABLE IF NOT EXISTS %s " +
		"(action_type TEXT NOT NULL, action_hash VARCHAR(64) NOT NULL, receipt_hash VARCHAR(64) NOT NULL UNIQUE, block_height DECIMAL(65, 0) NOT NULL, " +
		"`from` VARCHAR(41) NOT NULL, `to` VARCHAR(41) NOT NULL, gas_price DECIMAL(65, 0) NOT NULL, gas_consumed DECIMAL(65, 0) NOT NULL, nonce DECIMAL(65, 0) NOT NULL, " +
		"amount DECIMAL(65, 0) NOT NULL, receipt_status TEXT NOT NULL, PRIMARY KEY (action_hash), FOREIGN KEY (block_height) REFERENCES %s(block_height))"
	selectActionHistory = "SELECT * FROM %s WHERE action_hash=?"
	insertActionHistory = "INSERT INTO %s (action_type, action_hash, receipt_hash, block_height, `from`, `to`, " +
		"gas_price, gas_consumed, nonce, amount, receipt_status) VALUES %s"
)

type (
	// ActionHistory defines the base schema of "action history" table
	ActionHistory struct {
		ActionType    string
		ActionHash    string
		ReceiptHash   string
		BlockHeight   uint64
		From          string
		To            string
		GasPrice      string
		GasConsumed   uint64
		Nonce         uint64
		Amount        string
		ReceiptStatus string
	}

	// ActionInfo defines an action's information
	ActionInfo struct {
		ActionType  string
		ActionHash  string
		ReceiptHash hash.Hash256
		From        string
		To          string
		GasPrice    string
		Nonce       uint64
		Amount      string
	}

	// ReceiptInfo defines a receipt's information
	ReceiptInfo struct {
		ReceiptHash   string
		GasConsumed   uint64
		ReceiptStatus string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store        s.Store
	hermesConfig indexprotocol.HermesConfig
	epochCtx     *epochctx.EpochCtx
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, cfg indexprotocol.HermesConfig, epochCtx *epochctx.EpochCtx) *Protocol {
	return &Protocol{
		Store:        store,
		hermesConfig: cfg,
		epochCtx:     epochCtx,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createActionHistory,
		ActionHistoryTableName, blocks.BlockHistoryTableName)); err != nil {
		return err
	}
	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectActionHistoryInfo, ActionHistoryTableName, FromIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createActionHistoryFromIndex, FromIndexName, ActionHistoryTableName)); err != nil {
			return err
		}
	}
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectActionHistoryInfo, ActionHistoryTableName, ToIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createActionHistoryToIndex, ToIndexName, ActionHistoryTableName)); err != nil {
			return err
		}
	}

	// Prepare Xrc20 tables
	if err := p.CreateXrc20Tables(ctx); err != nil {
		return err
	}

	return p.CreateHermesTables(ctx)
}

// Initialize initializes actions protocol
func (p *Protocol) Initialize(context.Context, *sql.Tx, *indexprotocol.Genesis) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	hashToActionInfo := make(map[hash.Hash256]*ActionInfo)
	hermesHashes := make(map[hash.Hash256]bool)

	// log action index
	for _, selp := range blk.Actions {
		actionHash := selp.Hash()
		callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		dst, _ := selp.Destination()
		gasPrice := selp.GasPrice().String()
		nonce := selp.Nonce()

		act := selp.Action()
		var actionType string
		amount := "0"
		if tsf, ok := act.(*action.Transfer); ok {
			actionType = "transfer"
			amount = tsf.Amount().String()
		} else if exec, ok := act.(*action.Execution); ok {
			actionType = "execution"
			amount = exec.Amount().String()
		} else if df, ok := act.(*action.DepositToRewardingFund); ok {
			actionType = "depositToRewardingFund"
			amount = df.Amount().String()
		} else if cf, ok := act.(*action.ClaimFromRewardingFund); ok {
			actionType = "claimFromRewardingFund"
			amount = cf.Amount().String()
		} else if _, ok := act.(*action.GrantReward); ok {
			actionType = "grantReward"
		} else if _, ok := act.(*action.PutPollResult); ok {
			actionType = "putPollResult"
		}
		hashToActionInfo[actionHash] = &ActionInfo{
			ActionType: actionType,
			ActionHash: hex.EncodeToString(actionHash[:]),
			From:       callerAddr.String(),
			To:         dst,
			GasPrice:   gasPrice,
			Nonce:      nonce,
			Amount:     amount,
		}
		if strings.Compare(dst, p.hermesConfig.HermesContractAddress) == 0 {
			hermesHashes[actionHash] = true
		}
	}

	hashToReceiptInfo := make(map[hash.Hash256]*ReceiptInfo)
	hermesReceipts := make([]*action.Receipt, 0)
	for _, receipt := range blk.Receipts {
		// map receipt to action
		actionInfo, ok := hashToActionInfo[receipt.ActionHash]
		if !ok {
			return errors.New("failed to find the corresponding action from receipt")
		}
		receiptHash := receipt.Hash()
		actionInfo.ReceiptHash = receiptHash

		receiptStatus := "failure"
		if receipt.Status == uint64(1) {
			receiptStatus = "success"
		}

		hashToReceiptInfo[receiptHash] = &ReceiptInfo{
			ReceiptHash:   hex.EncodeToString(receiptHash[:]),
			GasConsumed:   receipt.GasConsumed,
			ReceiptStatus: receiptStatus,
		}
		if receiptStatus == "success" && hermesHashes[receipt.ActionHash] {
			hermesReceipts = append(hermesReceipts, receipt)
		}
	}

	err := p.updateActionHistory(tx, hashToActionInfo, hashToReceiptInfo, blk)
	if err != nil {
		return err
	}

	err = p.updateXrc20History(ctx, tx, blk)
	if err != nil {
		return err
	}

	if len(hermesReceipts) > 0 {
		epochNumber := p.epochCtx.GetEpochNumber(blk.Height())
		return p.updateHermesContract(tx, hermesReceipts, epochNumber, blk.Timestamp().String())
	}
	return nil
}

// getActionHistory returns action history by action hash
func (p *Protocol) getActionHistory(actionHash string) (*ActionHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectActionHistory,
		ActionHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(actionHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var actionHistory ActionHistory
	parsedRows, err := s.ParseSQLRows(rows, &actionHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	actHistory := parsedRows[0].(*ActionHistory)
	return actHistory, nil
}

// updateActionHistory stores action information into action history table
func (p *Protocol) updateActionHistory(
	tx *sql.Tx,
	hashToActionInfo map[hash.Hash256]*ActionInfo,
	hashToReceiptInfo map[hash.Hash256]*ReceiptInfo,
	block *block.Block,
) error {
	valStrs := make([]string, 0, len(block.Actions))
	valArgs := make([]interface{}, 0, len(block.Actions)*11)
	for _, selp := range block.Actions {
		actionInfo := hashToActionInfo[selp.Hash()]
		if actionInfo.ReceiptHash == hash.ZeroHash256 {
			return errors.New("action receipt is missing")
		}
		receiptInfo := hashToReceiptInfo[actionInfo.ReceiptHash]

		valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		valArgs = append(valArgs, actionInfo.ActionType, actionInfo.ActionHash, receiptInfo.ReceiptHash, block.Height(),
			actionInfo.From, actionInfo.To, actionInfo.GasPrice, receiptInfo.GasConsumed, actionInfo.Nonce,
			actionInfo.Amount, receiptInfo.ReceiptStatus)
	}
	insertQuery := fmt.Sprintf(insertActionHistory, ActionHistoryTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}
