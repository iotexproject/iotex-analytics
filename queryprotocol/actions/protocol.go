// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

type activeAccout struct {
	From        string
	BlockHeight uint64
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
