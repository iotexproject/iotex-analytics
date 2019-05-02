// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"context"
	"database/sql"
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/require"

	s "github.com/iotexproject/iotex-api/sql"
	"github.com/iotexproject/iotex-api/testutil"
)

func TestProtocol(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg := config.Default
	testPath := cfg.DB.SQLITE3.SQLite3File
	testutil.CleanupPath(t, testPath)

	store := s.NewSQLite3(cfg.DB.SQLITE3)
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

	// get receipt
	blkHash, err := p.GetBlockByReceipt(blk.Receipts[0].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	blkHash, err = p.GetBlockByReceipt(blk.Receipts[1].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	// get action
	actionHashes, err := p.GetActionHistory(testutil.Addr1)
	require.Nil(err)
	require.Equal(6, len(actionHashes))
	action := blk.Actions[0].Hash()
	require.Equal(action, actionHashes[0])

	// action map to block
	blkHash4, err := p.GetBlockByAction(blk.Actions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash4, blk.HashBlock())
}
