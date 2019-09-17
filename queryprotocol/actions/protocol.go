// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	topicsPlusDataLen = 256
	sha3Len           = 64
	contractParamsLen = 64
	addressLen        = 40
)

type activeAccout struct {
	From        string
	BlockHeight uint64
}

// ActionInfo defines action information
type ActionInfo struct {
	ActHash   string
	BlkHash   string
	TimeStamp uint64
	ActType   string
	Sender    string
	Recipient string
	Amount    string
}

// ActionDetail defines action detail information
type ActionDetail struct {
	ActionInfo   *ActionInfo
	EvmTransfers []*EvmTransfer
}

// EvmTransfer defines evm transfer information
type EvmTransfer struct {
	From     string
	To       string
	Quantity string
}

// Xrc20Info defines xrc20 transfer info
type Xrc20Info struct {
	Hash      string
	From      string
	To        string
	Quantity  string
	Timestamp string
	Contract  string
}

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetActionsByDates gets actions by start date and end date
func (p *Protocol) GetActionsByDates(startDate, endDate uint64) ([]*ActionInfo, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount FROM %s "+
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE timestamp >= ? AND timestamp <= ?", actions.ActionHistoryTableName, blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var actInfo ActionInfo
	parsedRows, err := s.ParseSQLRows(rows, &actInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}

	actionInfoList := make([]*ActionInfo, 0)
	for _, parsedRow := range parsedRows {
		actionInfoList = append(actionInfoList, parsedRow.(*ActionInfo))
	}
	return actionInfoList, nil
}

// GetActionDetailByHash gets action detail information by action hash
func (p *Protocol) GetActionDetailByHash(actHash string) (*ActionDetail, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	if _, ok := p.indexer.Registry.Find(accounts.ProtocolID); !ok {
		return nil, errors.New("accounts protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount FROM %s "+
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE action_hash = ?", actions.ActionHistoryTableName, blocks.BlockHistoryTableName)

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(actHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	if err := stmt.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close stmt")
	}

	parsedRows, err := s.ParseSQLRows(rows, &ActionInfo{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}

	actionDetail := &ActionDetail{ActionInfo: parsedRows[0].(*ActionInfo)}

	getQuery = fmt.Sprintf("SELECT `from`, `to`, amount FROM %s WHERE action_type = 'execution' AND action_hash = ?", accounts.BalanceHistoryTableName)

	stmt, err = db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err = stmt.Query(actHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	if err := stmt.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close stmt")
	}

	parsedRows, err = s.ParseSQLRows(rows, &EvmTransfer{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	for _, parsedRow := range parsedRows {
		actionDetail.EvmTransfers = append(actionDetail.EvmTransfers, parsedRow.(*EvmTransfer))
	}

	return actionDetail, nil
}

// GetActiveAccount gets active account address
func (p *Protocol) GetActiveAccount(count int) ([]string, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT DISTINCT `from`, block_height FROM %s ORDER BY block_height desc limit %d", actions.ActionHistoryTableName, count)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var acc activeAccout
	parsedRows, err := s.ParseSQLRows(rows, &acc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}

	var addrs []string
	for _, parsedRow := range parsedRows {
		acc := parsedRow.(*activeAccout)
		addrs = append(addrs, acc.From)
	}
	return addrs, nil
}

// GetXrc20 gets xrc20 transfer info by contract address
func (p *Protocol) GetXrc20(address string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * numPerPage
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE address='%s' ORDER BY `timestamp` desc limit %d,%d", actions.Xrc20HistoryTableName, address, offset, numPerPage)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var ret actions.Xrc20History
	parsedRows, err := s.ParseSQLRows(rows, &ret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}
	for _, parsedRow := range parsedRows {
		con := &Xrc20Info{}
		r := parsedRow.(*actions.Xrc20History)
		con.From, con.To, con.Quantity, err = parseContractData(r.Topics, r.Data)
		if err != nil {
			return
		}
		con.Hash = r.ActionHash
		con.Timestamp = r.Timestamp
		con.Contract = r.Address
		cons = append(cons, con)
	}
	return
}

// GetXrc20ByAddress gets xrc20 transfer info by sender or recipient address
func (p *Protocol) GetXrc20ByAddress(addr string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	a, err := address.FromString(addr)
	if err != nil {
		return nil, errors.New("address is invalid")
	}

	db := p.indexer.Store.GetDB()
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * numPerPage
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE topics like ? ORDER BY `timestamp` desc limit %d,%d", actions.Xrc20HistoryTableName, offset, numPerPage)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()
	like := "%" + common.BytesToAddress(a.Bytes()).String()[2:] + "%"
	rows, err := stmt.Query(like)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var ret actions.Xrc20History
	parsedRows, err := s.ParseSQLRows(rows, &ret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}
	for _, parsedRow := range parsedRows {
		con := &Xrc20Info{}
		r := parsedRow.(*actions.Xrc20History)
		con.From, con.To, con.Quantity, err = parseContractData(r.Topics, r.Data)
		if err != nil {
			return
		}
		con.Hash = r.ActionHash
		con.Timestamp = r.Timestamp
		con.Contract = r.Address
		cons = append(cons, con)
	}
	return
}

// GetXrc20ByPage gets xrc20 transfer info by page
func (p *Protocol) GetXrc20ByPage(numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * numPerPage
	getQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY `timestamp` desc limit %d,%d", actions.Xrc20HistoryTableName, offset, numPerPage)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var ret actions.Xrc20History
	parsedRows, err := s.ParseSQLRows(rows, &ret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}
	for _, parsedRow := range parsedRows {
		con := &Xrc20Info{}
		r := parsedRow.(*actions.Xrc20History)
		con.From, con.To, con.Quantity, err = parseContractData(r.Topics, r.Data)
		if err != nil {
			return
		}
		con.Hash = r.ActionHash
		con.Timestamp = r.Timestamp
		con.Contract = r.Address
		cons = append(cons, con)
	}
	return
}

func parseContractData(topics, data string) (from, to, amount string, err error) {
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
