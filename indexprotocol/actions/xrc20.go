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

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	topicsPlusDataLen = 256
	sha3Len           = 64
	contractParamsLen = 64
	addressLen        = 40
	successStatus     = uint64(1)
	revertStatus      = uint64(106)
	// xrc20 func
	//18160ddd -> totalSupply()
	totalSupplyString = "18160ddd"
	//70a08231 -> balanceOf(address)
	balanceOfString = "70a08231000000000000000000000000fea7d8ac16886585f1c232f13fefc3cfa26eb4cc"
	//dd62ed3e -> allowance(address,address)
	allowanceString = "dd62ed3e000000000000000000000000fea7d8ac16886585f1c232f13fefc3cfa26eb4cc000000000000000000000000fea7d8ac16886585f1c232f13fefc3cfa26eb4cc"
	//095ea7b3 -> approve(address,uint256)
	approveString = "095ea7b3000000000000000000000000fea7d8ac16886585f1c232f13fefc3cfa26eb4cc0000000000000000000000000000000000000000000000000000000000000001"

	// check xrc721 through read the following func:
	//18160ddd -> totalSupply()
	//70a08231 -> balanceOf(address)
	//095ea7b3 -> approve(address,uint256) receipt status 106
	//6352211e -> ownerOf(uint256)
	ownerOfString = "6352211e000000000000000000000000fea7d8ac16886585f1c232f13fefc3cfa26eb4cc"

	// Xrc20HistoryTableName is the table name of xrc20 history
	Xrc20HistoryTableName = "xrc20_history"
	// Xrc20HoldersTableName is the table name of xrc20 holders
	Xrc20HoldersTableName = "xrc20_holders"
	// Xrc721HistoryTableName is the table name of xrc721 history
	Xrc721HistoryTableName = "xrc721_history"
	// Xrc721HoldersTableName is the table name of xrc721 holders
	Xrc721HoldersTableName = "xrc721_holders"

	// transferSha3 is sha3 of xrc20's transfer event,keccak('Transfer(address,address,uint256)')
	transferSha3 = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	createXrc20History  = "CREATE TABLE IF NOT EXISTS %s (action_hash VARCHAR(64) NOT NULL, receipt_hash VARCHAR(64) NOT NULL UNIQUE, address VARCHAR(41) NOT NULL,`topics` VARCHAR(192),`data` VARCHAR(192),block_height DECIMAL(65, 0), `index` DECIMAL(65, 0),`timestamp` DECIMAL(65, 0),status VARCHAR(7) NOT NULL, PRIMARY KEY (action_hash,receipt_hash,topics))"
	createXrc20Holders  = "CREATE TABLE IF NOT EXISTS %s (contract VARCHAR(41) NOT NULL,holder VARCHAR(41) NOT NULL,`timestamp` DECIMAL(65, 0), PRIMARY KEY (contract,holder))"
	insertXrc20History  = "INSERT IGNORE INTO %s (action_hash, receipt_hash, address,topics,`data`,block_height, `index`,`timestamp`,status) VALUES %s"
	insertXrc20Holders  = "INSERT IGNORE INTO %s (contract, holder,`timestamp`) VALUES %s"
	selectXrc20History  = "SELECT * FROM %s WHERE address=?"
	selectXrc20Contract = "SELECT distinct address FROM %s"
)

var (
	totalSupply, _    = hex.DecodeString(totalSupplyString)
	balanceOf, _      = hex.DecodeString(balanceOfString)
	allowance, _      = hex.DecodeString(allowanceString)
	approve, _        = hex.DecodeString(approveString)
	ownerOf, _        = hex.DecodeString(ownerOfString)
	xrc20Contract     = make(map[string]bool)
	nonXrc20Contract  = make(map[string]bool)
	xrc721Contract    = make(map[string]bool)
	nonXrc721Contract = make(map[string]bool)
	nonce             = uint64(1)
	transferAmount    = big.NewInt(0)
	gasLimit          = uint64(100000)
	gasPrice          = big.NewInt(10000000)
	callerAddress     = identityset.Address(30).String()
)

type (
	// Xrc20History defines the base schema of "xrc20 history" table
	Xrc20History struct {
		ActionHash  string
		ReceiptHash string
		Address     string
		Topics      string
		Data        string
		BlockHeight string
		Index       string
		Timestamp   string
		Status      string
	}
)

// CreateXrc20Tables creates tables
func (p *Protocol) CreateXrc20Tables(ctx context.Context) error {
	// create block by action table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createXrc20History,
		Xrc20HistoryTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createXrc20Holders,
		Xrc20HoldersTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createXrc20History,
		Xrc721HistoryTableName)); err != nil {
		return err
	}
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createXrc20Holders,
		Xrc721HoldersTableName)); err != nil {
		return err
	}
	if err := p.initContract(Xrc20HistoryTableName, xrc20Contract); err != nil {
		return err
	}
	return p.initContract(Xrc721HistoryTableName, xrc721Contract)
}

func (p *Protocol) initContract(table string, xrcContract map[string]bool) (err error) {
	db := p.Store.GetDB()
	getQuery := fmt.Sprintf(selectXrc20Contract, table)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return errors.Wrap(err, "failed to execute get query")
	}
	type contractStruct struct {
		Contract string
	}
	var c contractStruct
	parsedRows, err := s.ParseSQLRows(rows, &c)
	if err != nil {
		return errors.Wrap(err, "failed to parse results")
	}
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*contractStruct)
		xrcContract[r.Contract] = true
	}
	return
}

// updateXrc20History stores Xrc20 information into Xrc20 history table
func (p *Protocol) updateXrc20History(
	ctx context.Context,
	tx *sql.Tx,
	blk *block.Block,
) error {
	valStrs := make([]string, 0)
	valArgs := make([]interface{}, 0)
	holdersStrs := make([]string, 0)
	holdersArgs := make([]interface{}, 0)

	xrc721ValStrs := make([]string, 0)
	xrc721ValArgs := make([]interface{}, 0)
	xrc721HoldersStrs := make([]string, 0)
	xrc721HoldersArgs := make([]interface{}, 0)
	for _, receipt := range blk.Receipts {
		receiptStatus := "failure"
		if receipt.Status == successStatus {
			receiptStatus = "success"
		}
		for _, l := range receipt.Logs() {
			data := hex.EncodeToString(l.Data)
			var topics string
			for _, t := range l.Topics {
				topics += hex.EncodeToString(t[:])
			}
			isXrc20 := p.checkIsXrc20(ctx, l.Address, topics, data)
			isXrc721 := p.checkIsXrc721(ctx, l.Address, topics, data)
			if !isXrc20 && !isXrc721 {
				continue
			}

			ah := hex.EncodeToString(l.ActionHash[:])
			receiptHash := receipt.Hash()

			rh := hex.EncodeToString(receiptHash[:])
			if isXrc721 {
				// field `topics` in db is varchar(192)
				// if we do not want change table xrc721_history
				// we can split 64 bytes to field `data` from 'topic'
				data = topics[192:]
				topics = topics[:192]
				xrc721ValStrs = append(xrc721ValStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
				xrc721ValArgs = append(xrc721ValArgs, ah, rh, l.Address, topics, data, l.BlockHeight, l.Index, blk.Timestamp().Unix(), receiptStatus)
			}
			if isXrc20 {
				valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
				valArgs = append(valArgs, ah, rh, l.Address, topics, data, l.BlockHeight, l.Index, blk.Timestamp().Unix(), receiptStatus)
			}

			from, to, _, err := ParseContractData(topics, data)
			if err != nil {
				log.L().Error("parse contract data error", zap.Error(err))
				continue
			}
			if isXrc721 {
				xrc721HoldersStrs = append(xrc721HoldersStrs, "(?, ?, ?)")
				xrc721HoldersArgs = append(xrc721HoldersArgs, l.Address, from, blk.Timestamp().Unix())
				xrc721HoldersStrs = append(xrc721HoldersStrs, "(?, ?, ?)")
				xrc721HoldersArgs = append(xrc721HoldersArgs, l.Address, to, blk.Timestamp().Unix())
			}
			if isXrc20 {
				holdersStrs = append(holdersStrs, "(?, ?, ?)")
				holdersArgs = append(holdersArgs, l.Address, from, blk.Timestamp().Unix())
				holdersStrs = append(holdersStrs, "(?, ?, ?)")
				holdersArgs = append(holdersArgs, l.Address, to, blk.Timestamp().Unix())
			}
		}
	}
	if len(valArgs) != 0 {
		insertQuery := fmt.Sprintf(insertXrc20History, Xrc20HistoryTableName, strings.Join(valStrs, ","))

		if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
			return err
		}

		insertQuery = fmt.Sprintf(insertXrc20Holders, Xrc20HoldersTableName, strings.Join(holdersStrs, ","))

		if _, err := tx.Exec(insertQuery, holdersArgs...); err != nil {
			return err
		}
	}

	if len(xrc721HoldersArgs) != 0 {
		insertQuery := fmt.Sprintf(insertXrc20History, Xrc721HistoryTableName, strings.Join(xrc721ValStrs, ","))

		if _, err := tx.Exec(insertQuery, xrc721ValArgs...); err != nil {
			return err
		}

		insertQuery = fmt.Sprintf(insertXrc20Holders, Xrc721HoldersTableName, strings.Join(xrc721HoldersStrs, ","))

		if _, err := tx.Exec(insertQuery, xrc721HoldersArgs...); err != nil {
			return err
		}
	}

	return nil
}

func (p *Protocol) checkTopics(topics, data string) bool {
	if topics == "" || len(topics) > 64*3 || len(data) > 64*3 {
		return false
	}
	if !strings.Contains(topics, transferSha3) {
		return false
	}
	return true
}

func (p *Protocol) checkXrc721Topics(topics, data string) bool {
	if topics == "" || len(topics) != 256 || len(data) != 0 {
		return false
	}
	if !strings.Contains(topics, transferSha3) {
		return false
	}
	return true
}

func (p *Protocol) checkIsXrc20(ctx context.Context, addr, topics, data string) bool {
	if !p.checkTopics(topics, data) {
		return false
	}
	if _, ok := nonXrc20Contract[addr]; ok {
		return false
	}
	if _, ok := xrc20Contract[addr]; ok {
		return true
	}
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	if indexCtx.ChainClient == nil {
		return false
	}
	ret := readContract(indexCtx.ChainClient, addr, totalSupply)
	if !ret {
		nonXrc20Contract[addr] = true
		return false
	}

	ret = readContract(indexCtx.ChainClient, addr, balanceOf)
	if !ret {
		nonXrc20Contract[addr] = true
		return false
	}
	ret = readContract(indexCtx.ChainClient, addr, allowance)
	if !ret {
		nonXrc20Contract[addr] = true
		return false
	}
	ret = readContract(indexCtx.ChainClient, addr, approve)
	if !ret {
		nonXrc20Contract[addr] = true
		return false
	}
	xrc20Contract[addr] = true
	return true
}

func (p *Protocol) checkIsXrc721(ctx context.Context, addr, topics, data string) bool {
	if !p.checkXrc721Topics(topics, data) {
		return false
	}
	if _, ok := nonXrc721Contract[addr]; ok {
		return false
	}
	if _, ok := xrc721Contract[addr]; ok {
		return true
	}
	indexCtx := indexcontext.MustGetIndexCtx(ctx)
	if indexCtx.ChainClient == nil {
		return false
	}

	ret := readContract(indexCtx.ChainClient, addr, totalSupply)
	if !ret {
		nonXrc721Contract[addr] = true
		return false
	}

	ret = readContract(indexCtx.ChainClient, addr, balanceOf)
	if !ret {
		nonXrc721Contract[addr] = true
		return false
	}

	ret = readContract(indexCtx.ChainClient, addr, approve)
	if !ret {
		nonXrc721Contract[addr] = true
		return false
	}

	ret = readContract(indexCtx.ChainClient, addr, ownerOf)
	if !ret {
		nonXrc721Contract[addr] = true
		return false
	}

	xrc721Contract[addr] = true
	return true
}

func readContract(cli iotexapi.APIServiceClient, addr string, callData []byte) bool {
	execution, err := action.NewExecution(addr, nonce, transferAmount, gasLimit, gasPrice, callData)
	if err != nil {
		return false
	}
	request := &iotexapi.ReadContractRequest{
		Execution:     execution.Proto(),
		CallerAddress: callerAddress,
	}

	res, err := cli.ReadContract(context.Background(), request)
	if err != nil {
		return false
	}
	if (res.Receipt.Status == successStatus || res.Receipt.Status == revertStatus) && res.Data != "" {
		return true
	}
	return false
}

// getActionHistory returns action history by action hash
func (p *Protocol) getXrc20History(address string) ([]*Xrc20History, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectXrc20History,
		Xrc20HistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var xrc20History Xrc20History
	parsedRows, err := s.ParseSQLRows(rows, &xrc20History)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}
	ret := make([]*Xrc20History, 0)
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*Xrc20History)
		ret = append(ret, r)
	}

	return ret, nil
}

// ParseContractData parse xrc20 topics
func ParseContractData(topics, data string) (from, to, amount string, err error) {
	// This should cover input of indexed or not indexed ,i.e., len(topics)==192 len(data)==64 or len(topics)==64 len(data)==192
	all := topics + data
	if len(all) != topicsPlusDataLen {
		err = errors.New("data's len is wrong")
		return
	}
	fromEth := all[sha3Len+contractParamsLen-addressLen : sha3Len+contractParamsLen]
	ethAddress := common.HexToAddress(fromEth)
	ioAddress, err := address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return
	}
	from = ioAddress.String()

	toEth := all[sha3Len+contractParamsLen*2-addressLen : sha3Len+contractParamsLen*2]
	ethAddress = common.HexToAddress(toEth)
	ioAddress, err = address.FromBytes(ethAddress.Bytes())
	if err != nil {
		return
	}
	to = ioAddress.String()

	amountBig, ok := new(big.Int).SetString(all[sha3Len+contractParamsLen*2:], 16)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	amount = amountBig.Text(10)
	return
}
