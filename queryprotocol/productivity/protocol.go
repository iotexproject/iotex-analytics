// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package productivity

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

type productivity struct {
	SumOfProduction         uint64
	SumOfExpectedProduction uint64
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetProductivityHistory gets productivity history
func (p *Protocol) GetProductivityHistory(startEpoch uint64, epochCount uint64, producerName string) (string, string, error) {
	if _, ok := p.indexer.Registry.Find(blocks.ProtocolID); !ok {
		return "", "", errors.New("blocks protocol is unregistered")
	}

	db := p.indexer.Store.GetDB()

	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number = ? and delegate_name = ?",
		blocks.ProductivityViewName), startEpoch, producerName)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return "", "", indexprotocol.ErrNotExist
	}

	endEpoch := startEpoch + epochCount - 1

	getQuery := fmt.Sprintf("SELECT SUM(production), SUM(expected_production) FROM %s WHERE "+
		"epoch_number >= %d AND epoch_number <= %d AND delegate_name=?", blocks.ProductivityViewName, startEpoch, endEpoch)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to prepare get query")
	}

	var production, expectedProduction string
	if err = stmt.QueryRow(producerName).Scan(&production, &expectedProduction); err != nil {
		return "", "", errors.Wrap(err, "failed to execute get query")
	}
	return production, expectedProduction, nil
}

// AverageProductivity handles AverageProductivity request
func (p *Protocol) AverageProductivity(startEpochNumber int, epochCount int) (averageProcucitvity float64, err error) {
	if _, ok := p.indexer.Registry.Find(blocks.ProtocolID); !ok {
		err = errors.New("blocks protocol is unregistered")
		return
	}
	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT MAX(epoch_number) FROM %s", blocks.ProductivityViewName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var tipHeight int
	if err = stmt.QueryRow().Scan(&tipHeight); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	if startEpochNumber > tipHeight {
		err = errors.New("epoch number is not exist")
		return
	}

	getQuery = fmt.Sprintf("SELECT SUM(production),SUM(expected_production) FROM %s WHERE epoch_number>=? AND epoch_number<=? GROUP BY delegate_name", blocks.ProductivityViewName)
	stmt, err = db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	rows, err := stmt.Query(startEpochNumber, startEpochNumber+epochCount-1)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var product productivity
	parsedRows, err := s.ParseSQLRows(rows, &product)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}

	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}
	var productivitySums float64
	for _, parsedRow := range parsedRows {
		p := parsedRow.(*productivity)
		productivitySums += float64(p.SumOfProduction) / float64(p.SumOfExpectedProduction)
	}
	averageProcucitvity = productivitySums / float64(len(parsedRows))
	return
}
