package accounts

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
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

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	ctx := indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient: chainClient,
	})

	testutil.CleanupDatabase(t, connectStr, dbName)

	store := s.NewMySQL(connectStr, dbName)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()

	p := NewProtocol(store, epochctx.NewEpochCtx(1, 1, 1))

	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(1), uint64(2))
	require.NoError(err)

	request := &iotexapi.GetEvmTransfersByBlockHeightRequest{BlockHeight: 1}
	chainClient.EXPECT().GetEvmTransfersByBlockHeight(gomock.Any(), request).Times(1).Return(nil,
		status.Error(codes.NotFound, ""))

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	request.BlockHeight = 2
	chainClient.EXPECT().GetEvmTransfersByBlockHeight(gomock.Any(), request).Times(1).Return(nil,
		status.Error(codes.NotFound, ""))

	blk2, err := testutil.BuildEmptyBlock(2)
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk2)
	}))

	// get balance history
	balanceHistory, err := p.getBalanceHistory(testutil.Addr1)
	require.NoError(err)
	require.Equal(2, len(balanceHistory))
	require.Equal("2", balanceHistory[1].Amount)

	// get account income
	accountIncome, err := p.getAccountIncome(uint64(1), testutil.Addr1)
	require.NoError(err)
	require.Equal("-2", accountIncome.Income)
}
