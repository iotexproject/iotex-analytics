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

const (
	connectStr = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName     = "heroku_7fed0b046078f80"
)

func TestProtocol(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupDatabase(t, connectStr, dbName)

	store := s.NewMySQL(connectStr, dbName)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
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
