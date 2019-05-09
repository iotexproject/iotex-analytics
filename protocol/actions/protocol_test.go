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
	"testing"

	"github.com/stretchr/testify/require"

	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

func TestProtocol(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	testPath := "analytics.db"
	testutil.CleanupPath(t, testPath)

	store := s.NewSQLite3(testPath)
	require.NoError(store.Start(ctx))
	defer func() {
		require.NoError(store.Stop(ctx))
		testutil.CleanupPath(t, testPath)
	}()

	p := NewProtocol(store)

	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	// get action
	actionHash := blk.Actions[1].Hash()
	receiptHash := blk.Receipts[1].Hash()
	actionHistory, err := p.GetActionHistory(hex.EncodeToString(actionHash[:]))
	require.NoError(err)

	require.Equal("transfer", actionHistory.ActionType)
	require.Equal(hex.EncodeToString(receiptHash[:]), actionHistory.ReceiptHash)
	require.Equal(uint64(180), actionHistory.BlockHeight)
	require.Equal(testutil.Addr1, actionHistory.From)
	require.Equal(testutil.Addr2, actionHistory.To)
	require.Equal("0", actionHistory.GasPrice)
	require.Equal(uint64(2), actionHistory.GasConsumed)
	require.Equal(uint64(102), actionHistory.Nonce)
	require.Equal("2", actionHistory.Amount)
	require.Equal("success", actionHistory.ReceiptStatus)
}
