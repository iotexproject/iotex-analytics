// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	zeroAddress                    = "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7"
	selectActionHistoryByTimestamp = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height " +
		"WHERE timestamp >= ? AND timestamp <= ? ORDER BY `timestamp` desc limit ?,?"
	selectActionHistoryByType = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE action_type =? ORDER BY `timestamp` desc limit ?,?"
	selectActionHistoryByHash = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s " +
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE action_hash = ?"
	selectActionHistoryByAddress = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s " +
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE `from` = ? OR `to` = ? ORDER BY `timestamp` desc limit ?,?"
	selectEvmTransferHistoryByHash    = "SELECT `from`, `to`, amount FROM %s WHERE action_type = 'execution' AND action_hash = ?"
	selectEvmTransferHistoryByAddress = "SELECT `from`, `to`, amount, action_hash, t1.block_height, timestamp " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height " +
		"WHERE action_type = 'execution' AND (`from` = ? OR `to` = ?) ORDER BY `timestamp` desc limit ?,?"
	selectEvmTransferCount     = "SELECT COUNT(*) FROM %s WHERE action_type='execution' AND (`from` = '%s' OR `to` = '%s')"
	selectActionHistory        = "SELECT DISTINCT `from`, block_height FROM %s ORDER BY block_height desc limit %d"
	selectXrc20History         = "SELECT * FROM %s WHERE address='%s' ORDER BY `timestamp` desc limit %d,%d"
	selectXrc20HoldersCount    = "SELECT COUNT(*) FROM %s WHERE contract='%s'"
	selectXrc20Holders         = "SELECT holder FROM %s WHERE contract='%s' ORDER BY `timestamp` desc limit %d,%d"
	selectXrc20HistoryByTopics = "SELECT * FROM %s WHERE topics like ? ORDER BY `timestamp` desc limit %d,%d"
	selectXrc20AddressesByPage = "SELECT address, MAX(`timestamp`) AS t FROM %s GROUP BY address ORDER BY t desc limit %d,%d"
	selectXrc20HistoryByPage   = "SELECT * FROM %s ORDER BY `timestamp` desc limit %d,%d"
	selectAccountIncome        = "SELECT address,SUM(income) AS balance FROM %s WHERE epoch_number<=%d and address<>'' and address<>'%s' GROUP BY address ORDER BY balance DESC LIMIT %d,%d"
	selectTotalNumberOfHolders = "SELECT COUNT(DISTINCT address) FROM %s WHERE address<>''"
)

type activeAccount struct {
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
	GasFee    string
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

// EvmTransferDetail defines evm transfer detail information
type EvmTransferDetail struct {
	From      string
	To        string
	Quantity  string
	ActHash   string
	BlkHash   string
	TimeStamp sql.NullInt64 // for timestamp is NULL
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

// TopHolder defines top holder
type TopHolder struct {
	Address string
	Balance string
}

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// Address defines the address struct
type Address struct {
	Address   string
	TimeStamp uint64
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetActionsByDates gets actions by start date and end date
func (p *Protocol) GetActionsByDates(startDate, endDate uint64, offset, size uint64) ([]*ActionInfo, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectActionHistoryByTimestamp, actions.ActionHistoryTableName, blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startDate, endDate, offset, size)
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

// GetActionsByType gets actions by type
func (p *Protocol) GetActionsByType(actionType string, offset, size uint64) ([]*ActionInfo, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectActionHistoryByType, actions.ActionHistoryTableName, blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()
	rows, err := stmt.Query(actionType, offset, size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var actInfo ActionInfo
	parsedRows, err := s.ParseSQLRows(rows, &actInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
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

	getQuery := fmt.Sprintf(selectActionHistoryByHash, actions.ActionHistoryTableName, blocks.BlockHistoryTableName)

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

	getQuery = fmt.Sprintf(selectEvmTransferHistoryByHash, accounts.BalanceHistoryTableName)

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

// GetActionsByAddress gets action information list by address
func (p *Protocol) GetActionsByAddress(address string, offset, size uint64) ([]*ActionInfo, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	if _, ok := p.indexer.Registry.Find(accounts.ProtocolID); !ok {
		return nil, errors.New("accounts protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectActionHistoryByAddress, actions.ActionHistoryTableName, blocks.BlockHistoryTableName)

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(address, address, offset, size)
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

	actionInfoList := make([]*ActionInfo, 0)
	for _, parsedRow := range parsedRows {
		actionInfoList = append(actionInfoList, parsedRow.(*ActionInfo))
	}

	return actionInfoList, nil
}

// GetEvmTransferDetailListByAddress gets evm transfer detail information list by address
func (p *Protocol) GetEvmTransferDetailListByAddress(address string, offset, size uint64) ([]*EvmTransferDetail, error) {
	if _, ok := p.indexer.Registry.Find(accounts.ProtocolID); !ok {
		return nil, errors.New("accounts protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectEvmTransferHistoryByAddress, accounts.BalanceHistoryTableName, blocks.BlockHistoryTableName)

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(address, address, offset, size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	if err := stmt.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close stmt")
	}

	parsedRows, err := s.ParseSQLRows(rows, &EvmTransferDetail{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}

	evmTransferList := make([]*EvmTransferDetail, 0)
	for _, parsedRow := range parsedRows {
		evmTransferList = append(evmTransferList, parsedRow.(*EvmTransferDetail))
	}

	return evmTransferList, nil
}

// GetActiveAccount gets active account address
func (p *Protocol) GetActiveAccount(count int) ([]string, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectActionHistory, actions.ActionHistoryTableName, count)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var acc activeAccount
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
		acc := parsedRow.(*activeAccount)
		addrs = append(addrs, acc.From)
	}
	return addrs, nil
}

// GetXrc20 gets xrc20 transfer info by contract address
func (p *Protocol) GetXrc20(address string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	return p.getXrc(address, actions.Xrc20HistoryTableName, numPerPage, page)
}

// GetXrc721 gets xrc721 transfer info by contract address
func (p *Protocol) GetXrc721(address string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	return p.getXrc(address, actions.Xrc721HistoryTableName, numPerPage, page)
}

// getXrc gets xrc transfer info by contract address
func (p *Protocol) getXrc(address, table string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * numPerPage
	getQuery := fmt.Sprintf(selectXrc20History, table, address, offset, numPerPage)
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
		con.From, con.To, con.Quantity, err = actions.ParseContractData(r.Topics, r.Data)
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
	return p.getXrcByAddress(addr, actions.Xrc20HistoryTableName, numPerPage, page)
}

// GetXrc721ByAddress gets xrc721 transfer info by sender or recipient address
func (p *Protocol) GetXrc721ByAddress(addr string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
	return p.getXrcByAddress(addr, actions.Xrc721HistoryTableName, numPerPage, page)
}

// getXrcByAddress gets xrc transfer info by sender or recipient address
func (p *Protocol) getXrcByAddress(addr, table string, numPerPage, page uint64) (cons []*Xrc20Info, err error) {
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
	getQuery := fmt.Sprintf(selectXrc20HistoryByTopics, table, offset, numPerPage)
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
		con.From, con.To, con.Quantity, err = actions.ParseContractData(r.Topics, r.Data)
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

// GetXrc20HolderCount gets xrc20 holders's address
func (p *Protocol) GetXrc20HolderCount(addr string) (count int, err error) {
	return p.getCount(selectXrc20HoldersCount, actions.Xrc20HoldersTableName, addr, false)
}

// GetXrc721HolderCount gets xrc721 holders's address
func (p *Protocol) GetXrc721HolderCount(addr string) (count int, err error) {
	return p.getCount(selectXrc20HoldersCount, actions.Xrc721HoldersTableName, addr, false)
}

// GetEvmTransferCount gets execution count
func (p *Protocol) GetEvmTransferCount(addr string) (count int, err error) {
	return p.getCount(selectEvmTransferCount, accounts.BalanceHistoryTableName, addr, true)
}

func (p *Protocol) getCount(selectSQL, table, addr string, evmTransfer bool) (count int, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return 0, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	var getQuery string
	if !evmTransfer {
		getQuery = fmt.Sprintf(selectSQL, table, addr)
	} else {
		getQuery = fmt.Sprintf(selectSQL, table, addr, addr)
	}
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return 0, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute get query")
	}
	type countStruct struct {
		Count int
	}
	var c countStruct
	parsedRows, err := s.ParseSQLRows(rows, &c)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return 0, err
	}
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*countStruct)
		count = r.Count
	}
	return
}

// GetXrc20Holders gets xrc20 holders
func (p *Protocol) GetXrc20Holders(addr string, offset, size uint64) (rets []*string, err error) {
	return p.getXrcHolders(addr, actions.Xrc20HoldersTableName, offset, size)
}

// GetXrc721Holders gets xrc721 holders
func (p *Protocol) GetXrc721Holders(addr string, offset, size uint64) (rets []*string, err error) {
	return p.getXrcHolders(addr, actions.Xrc721HoldersTableName, offset, size)
}

func (p *Protocol) getXrcHolders(addr, table string, offset, size uint64) (rets []*string, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	a, err := address.FromString(addr)
	if err != nil {
		return nil, errors.New("address is invalid")
	}

	db := p.indexer.Store.GetDB()
	if size < 1 {
		size = 1
	}
	getQuery := fmt.Sprintf(selectXrc20Holders, table, a, offset, size)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}
	type holdersStruct struct {
		Holder string
	}
	var h holdersStruct
	parsedRows, err := s.ParseSQLRows(rows, &h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*holdersStruct)
		rets = append(rets, &r.Holder)
	}
	return
}

// GetXrc20ByPage gets xrc20 transfer info by page
func (p *Protocol) GetXrc20ByPage(offset, limit uint64) (cons []*Xrc20Info, err error) {
	return p.getXrcByPage(actions.Xrc20HistoryTableName, offset, limit)
}

// GetXrc721ByPage gets xrc721 transfer info by page
func (p *Protocol) GetXrc721ByPage(offset, limit uint64) (cons []*Xrc20Info, err error) {
	return p.getXrcByPage(actions.Xrc721HistoryTableName, offset, limit)
}

// getXrcByPage gets xrc transfer info by page
func (p *Protocol) getXrcByPage(table string, offset, limit uint64) (cons []*Xrc20Info, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectXrc20HistoryByPage, table, offset, limit)
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
		con.From, con.To, con.Quantity, err = actions.ParseContractData(r.Topics, r.Data)
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

// GetXrc20Addresses gets xrc20 addresses by page
func (p *Protocol) GetXrc20Addresses(offset, limit uint64) (addresses []*string, err error) {
	return p.getXrcAddresses(actions.Xrc20HistoryTableName, offset, limit)
}

// GetXrc721Addresses gets xrc721 addresses by page
func (p *Protocol) GetXrc721Addresses(offset, limit uint64) (addresses []*string, err error) {
	return p.getXrcAddresses(actions.Xrc721HistoryTableName, offset, limit)
}

// getXrcAddresses gets xrc addresses by page
func (p *Protocol) getXrcAddresses(table string, offset, limit uint64) (addresses []*string, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectXrc20AddressesByPage, table, offset, limit)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var ret Address
	parsedRows, err := s.ParseSQLRows(rows, &ret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*Address)
		addresses = append(addresses, &r.Address)
	}
	return
}

// GetTopHolders gets top holders
func (p *Protocol) GetTopHolders(endEpochNumber, skip, first uint64) (holders []*TopHolder, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectAccountIncome, accounts.AccountIncomeTableName, endEpochNumber, zeroAddress, skip, first)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var ret TopHolder
	parsedRows, err := s.ParseSQLRows(rows, &ret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}
	for _, parsedRow := range parsedRows {
		holders = append(holders, parsedRow.(*TopHolder))
	}
	return
}

// GetTotalNumberOfHolders gets total num of holders
func (p *Protocol) GetTotalNumberOfHolders() (count int, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return 0, errors.New("actions protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectTotalNumberOfHolders, accounts.AccountIncomeTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return 0, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute get query")
	}
	type countStruct struct {
		Count int
	}
	var c countStruct
	parsedRows, err := s.ParseSQLRows(rows, &c)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse results")
	}
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*countStruct)
		count = r.Count
	}
	return
}
