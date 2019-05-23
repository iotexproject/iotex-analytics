// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainmetautil

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// GetCurrentEpochAndHeight gets current epoch number and tip block height
func GetCurrentEpochAndHeight(registry *indexprotocol.Registry, store s.Store) (uint64, uint64, error) {
	_, ok := registry.Find(blocks.ProtocolID)
	if !ok {
		return uint64(0), uint64(0), errors.New("blocks protocol is unregistered")
	}
	db := store.GetDB()
	getQuery := fmt.Sprintf("SELECT MAX(epoch_number),MAX(block_height) FROM %s", blocks.BlockHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return uint64(0), uint64(0), errors.Wrap(err, "failed to prepare get query")

	}
	var epoch, tipHeight uint64
	if err = stmt.QueryRow().Scan(&epoch, &tipHeight); err != nil {
		return uint64(0), uint64(0), errors.Wrap(err, "failed to execute get query")
	}
	return epoch, tipHeight, nil
}
