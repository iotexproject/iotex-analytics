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
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetProductivityHistory gets productivity history
func (p *Protocol) GetProductivityHistory(startEpoch uint64, epochCount uint64, producerName string) (string, string, error) {
	if _, ok := p.indexer.Registry.Find(blocks.ProtocolID); !ok {
		return "", "", errors.New("producers protocol is unregistered")
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
		err = errors.New("producers protocol is unregistered")
		return
	}

	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT AVG(production) FROM %s WHERE epoch_number>=? AND epoch_number<=?", blocks.ProductivityViewName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	if err = stmt.QueryRow(startEpochNumber, startEpochNumber+epochCount-1).Scan(&averageProcucitvity); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}
