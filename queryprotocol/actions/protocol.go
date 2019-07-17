// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

var (
	transferSha3 = "a9059cbb"
)

type activeAccout struct {
	From        string
	BlockHeight uint64
}

// Contract
type Contract struct {
	Hash      string `json:"hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	Quantity  string `json:"quantity"`
	Timestamp string `json:"timestamp"`
	Data      string `json:"data"`
}

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
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

// GetContract
func (p *Protocol) GetContract(address string, numPerPage, page uint64) (ret []*Contract, err error) {
	//SELECT action_hash,`from`,`to`,amount,`timestamp` FROM action_history WHERE `to`='io1ly28u4nn9w3qxpusfuwg4u36xxdj02s7lxncyt' ORDER BY `timestamp` DESC LIMIT 0,10

	if _, ok := p.indexer.Registry.Find(actions.ProtocolID); !ok {
		return nil, errors.New("actions protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * numPerPage
	getQuery := fmt.Sprintf("SELECT action_hash,`from`,`to`,amount,`timestamp`,`data` FROM %s WHERE `to`='%s' and left(`data`,8)='%s' ORDER BY `timestamp` desc limit %d,%d", actions.ActionHistoryTableName, address, transferSha3, offset, numPerPage)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var contract Contract
	parsedRows, err := s.ParseSQLRows(rows, &contract)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return nil, err
	}

	for _, parsedRow := range parsedRows {
		con := parsedRow.(*Contract)
		con.To, con.Quantity, err = parseData(con.Data)
		if err != nil {
			return
		}
		ret = append(ret, con)
	}
	return
}

func parseData(data string) (to, amount string, err error) {
	if len(data) != 136 {
		err = errors.New("data's len is wrong")
		return
	}
	addr := data[32:72]
	addrHash, err := hex.DecodeString(addr)
	if err != nil {
		return
	}
	callerAddr, err := address.FromBytes(addrHash)
	if err != nil {
		return
	}
	to = callerAddr.String()
	amountString := data[72:]
	amountBig, ok := new(big.Int).SetString(amountString, 16)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	amount = amountBig.Text(10)
	return
}
