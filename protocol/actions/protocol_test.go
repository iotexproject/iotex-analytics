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
	err := store.Start(ctx)
	require.NoError(err)
	defer func() {
		err = store.Stop(ctx)
		require.NoError(err)
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
