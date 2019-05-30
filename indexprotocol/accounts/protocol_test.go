package accounts

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

func TestProtocol(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	p := NewProtocol(store, uint64(24), uint64(15))

	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	// get account history
	accountHistory, err := p.getAccountHistory(testutil.Addr1)
	require.NoError(err)
	require.Equal(3, len(accountHistory))
	require.Equal("0", accountHistory[2].In)
	require.Equal("2", accountHistory[2].Out)

	// get account balance change
	accountBalance, err := p.getAccountBalanceChange(uint64(1), testutil.Addr1)
	require.NoError(err)
	require.Equal("-2", accountBalance.BalanceChange)

	accountBalance, err = p.getAccountBalanceChange(uint64(1), testutil.Addr2)
	require.NoError(err)
	require.Equal("2", accountBalance.BalanceChange)
}
