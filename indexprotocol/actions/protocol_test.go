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

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr = "root:rootuser@tcp(127.0.0.1:3306)/"
	dbName     = "analytics"
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

	bp := blocks.NewProtocol(store, uint64(24), uint64(36), uint64(15))
	p := NewProtocol(store)

	require.NoError(bp.CreateTables(ctx))
	require.NoError(p.CreateTables(ctx))

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	bpctx := indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:    chainClient,
		ElectionClient: electionClient,
	})

	chainClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: byteutil.Uint64ToBytes(uint64(1000)),
	}, nil)
	electionClient.EXPECT().GetCandidates(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.CandidateResponse{
			Candidates: []*api.Candidate{
				{
					Name:            "alfa",
					OperatorAddress: testutil.Addr1,
				},
				{
					Name:            "bravo",
					OperatorAddress: testutil.Addr2,
				},
			},
		}, nil,
	)

	blk, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return bp.HandleBlock(bpctx, tx, blk)
	}))

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	// get action
	actionHash := blk.Actions[1].Hash()
	receiptHash := blk.Receipts[1].Hash()
	actionHistory, err := p.getActionHistory(hex.EncodeToString(actionHash[:]))
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
