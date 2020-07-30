// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"database/sql"
	"fmt"
	"math/big"
	"strings"

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
	selectActionHistoryByTimestamp = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height " +
		"WHERE timestamp >= ? AND timestamp <= ? ORDER BY `timestamp` desc limit ?,?"
	selectActionHistoryByType = "SELECT action_hash, block_hash, `timestamp`, action_type, `from`, `to`, amount, gas_price*gas_consumed FROM %s AS t1 LEFT JOIN (SELECT block_height,block_hash,`timestamp` FROM %s) t2 ON t1.block_height=t2.block_height WHERE action_type =? ORDER BY `timestamp` DESC limit ?,?"
	selectActionHistoryByHash = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s " +
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE action_hash = ?"
	selectActionHistoryByAddress = "SELECT action_hash, block_hash, timestamp, action_type, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s " +
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE `from` = ? OR `to` = ? ORDER BY `timestamp` desc limit ?,?"
	selectActionHistoryByAddressAndType = "SELECT action_hash, block_hash, timestamp, `action_type`, `from`, `to`, amount, t1.gas_price*t1.gas_consumed FROM %s " +
		"AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height WHERE (`from` = ? OR `to` = ?) AND `action_type` = ? ORDER BY `timestamp` desc limit ?,?"
	selectEvmTransferHistoryByHash    = "SELECT `from`, `to`, amount FROM %s WHERE action_type = 'execution' AND action_hash = ?"
	selectEvmTransferHistoryByAddress = "SELECT `from`, `to`, amount, action_hash, t1.block_height, timestamp " +
		"FROM %s AS t1 LEFT JOIN %s AS t2 ON t1.block_height=t2.block_height " +
		"WHERE action_type = 'execution' AND (`from` = ? OR `to` = ?) ORDER BY `timestamp` desc limit ?,?"
	selectEvmTransferCount     = "SELECT COUNT(*) FROM %s WHERE action_type='execution' AND (`from` = '%s' OR `to` = '%s')"
	selectActionHistory        = "SELECT DISTINCT `from`, block_height FROM %s ORDER BY block_height desc limit %d"
	selectXrc20History         = "SELECT * FROM %s WHERE address='%s' ORDER BY `timestamp` desc limit %d,%d"
	selectCount                = "SELECT COUNT(*) FROM %s"
	selectXrcHolderCount       = selectCount + " WHERE contract='%s'"
	selectXrcTransactionCount  = selectCount + " WHERE address='%s'"
	selectActionCountByDates   = selectCount + " WHERE timestamp >= %d AND timestamp <= %d"
	selectActionCountByAddress = selectCount + " WHERE `from` = '%s' OR `to` = '%s'"
	selectActCntByAddrAndType  = selectCount + " WHERE (`from` = '%s' OR `to` = '%s') AND `action_type` = '%s'"
	selectActionCountByType    = selectCount + " WHERE action_type = '%s'"
	selectXrc20Holders         = "SELECT holder FROM %s WHERE contract='%s' ORDER BY `timestamp` desc limit %d,%d"
	selectXrc20HistoryByTopics = "SELECT * FROM %s WHERE topics like ? ORDER BY `timestamp` desc limit %d,%d"
	selectXrcHistoryCount      = selectCount + " WHERE topics like %s"
	selectXrc20AddressesByPage = "SELECT address, MAX(`timestamp`) AS t FROM %s GROUP BY address ORDER BY t desc limit %d,%d"
	selectXrc20HistoryByPage   = "SELECT * FROM %s ORDER BY `timestamp` desc limit %d,%d"
	selectAccountIncome        = "SELECT address,SUM(income) AS balance FROM %s WHERE epoch_number<=%d and address<>'' and address<>'%s' GROUP BY address ORDER BY balance DESC LIMIT %d,%d"
	selectTotalNumberOfHolders = "SELECT COUNT(DISTINCT address) FROM %s WHERE address<>''"
	selectTotalAccountSupply   = "SELECT SUM(income) from %s WHERE epoch_number<>0 and address=''"
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

// GetActionCountByDates gets action counts by dates
func (p *Protocol) GetActionCountByDates(startDate, endDate uint64) (count int, err error) {
	getQuery := fmt.Sprintf(selectActionCountByDates, blocks.BlockHistoryTableName, startDate, endDate)
	return p.getCount(getQuery)
}

// GetActionCountByAddressAndType gets action counts by address and type
func (p *Protocol) GetActionCountByAddressAndType(addr, actionType string) (count int, err error) {
	getQuery := fmt.Sprintf(selectActCntByAddrAndType, actions.ActionHistoryTableName, addr, addr, actionType)
	return p.getCount(getQuery)
}

// GetActionsByAddressAndType gets action information list by address and type
func (p *Protocol) GetActionsByAddressAndType(address, actionType string, offset, size uint64) ([]*ActionInfo, error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectActionHistoryByAddressAndType, actions.ActionHistoryTableName, blocks.BlockHistoryTableName)

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(address, address, actionType, offset, size)
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

// GetActionCountByType gets action counts by type
func (p *Protocol) GetActionCountByType(actionType string) (count int, err error) {
	getQuery := fmt.Sprintf(selectActionCountByType, actions.ActionHistoryTableName, actionType)
	return p.getCount(getQuery)
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

// GetActionCountByAddress gets action counts by address
func (p *Protocol) GetActionCountByAddress(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectActionCountByAddress, actions.ActionHistoryTableName, addr, addr)
	return p.getCount(getQuery)
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

// GetXrc20TransactionCount gets xrc20 transaction count by contract address
func (p *Protocol) GetXrc20TransactionCount(address string) (count int, err error) {
	getQuery := fmt.Sprintf(selectXrcTransactionCount, actions.Xrc20HistoryTableName, address)
	return p.getCount(getQuery)
}

// GetXrc721TransactionCount gets xrc721 transaction count by contract address
func (p *Protocol) GetXrc721TransactionCount(address string) (count int, err error) {
	getQuery := fmt.Sprintf(selectXrcTransactionCount, actions.Xrc721HistoryTableName, address)
	return p.getCount(getQuery)
}

// GetXrc20HistoryCount gets xrc20 transaction count by topics
func (p *Protocol) GetXrc20HistoryCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectXrcHistoryCount, actions.Xrc20HistoryTableName, addrToLike(addr))
	return p.getCount(getQuery)
}

// GetXrc721HistoryCount gets xrc721 transaction count by topics
func (p *Protocol) GetXrc721HistoryCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectXrcHistoryCount, actions.Xrc721HistoryTableName, addrToLike(addr))
	return p.getCount(getQuery)
}

// GetXrc721Count gets xrc721 all transaction count
func (p *Protocol) GetXrc721Count(address string) (count int, err error) {
	getQuery := fmt.Sprintf(selectCount, actions.Xrc721HistoryTableName)
	return p.getCount(getQuery)
}

// GetXrc20Count gets xrc20 all transaction count
func (p *Protocol) GetXrc20Count(address string) (count int, err error) {
	getQuery := fmt.Sprintf(selectCount, actions.Xrc20HistoryTableName)
	return p.getCount(getQuery)
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
	getQuery := fmt.Sprintf(selectXrcHolderCount, actions.Xrc20HoldersTableName, addr)
	return p.getCount(getQuery)
}

// GetXrc20AddressesCount gets xrc20 holders's address
func (p *Protocol) GetXrc20AddressesCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectTotalNumberOfHolders, actions.Xrc20HistoryTableName)
	return p.getCount(getQuery)
}

// GetXrc721AddressesCount gets xrc721 holders's address
func (p *Protocol) GetXrc721AddressesCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectTotalNumberOfHolders, actions.Xrc721HistoryTableName)
	return p.getCount(getQuery)
}

// GetXrc721HolderCount gets xrc721 holders's address
func (p *Protocol) GetXrc721HolderCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectXrcHolderCount, actions.Xrc721HoldersTableName, addr)
	return p.getCount(getQuery)
}

// GetEvmTransferCount gets execution count
func (p *Protocol) GetEvmTransferCount(addr string) (count int, err error) {
	getQuery := fmt.Sprintf(selectEvmTransferCount, accounts.BalanceHistoryTableName, addr, addr)
	return p.getCount(getQuery)
}

func (p *Protocol) getCount(getQuery string) (count int, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return 0, errors.New("actions protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	if err = stmt.QueryRow().Scan(&count); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
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
	getQuery := fmt.Sprintf(selectAccountIncome, accounts.AccountIncomeTableName, endEpochNumber, address.ZeroAddress, skip, first)
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

// GetTotalAccountSupply gets balance of all accounts
func (p *Protocol) GetTotalAccountSupply() (count string, err error) {
	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return "0", errors.New("actions protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectTotalAccountSupply, accounts.AccountIncomeTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()
	if err = stmt.QueryRow().Scan(&count); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	ret, ok := new(big.Int).SetString(count, 10)
	if !ok {
		err = errors.New("failed to format to big int:" + count)
		return
	}
	if ret.Sign() < 0 {
		count = ret.Abs(ret).String()
	}
	return
}

func addrToLike(addr string) string {
	a, err := address.FromString(addr)
	if err != nil {
		return ""
	}
	return "'%" + strings.ToLower(common.BytesToAddress(a.Bytes()).String()[2:]) + "%'"
}
