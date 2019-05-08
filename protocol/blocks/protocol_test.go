package blocks

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

	p := NewProtocol(store, uint64(24), uint64(36), uint64(15))

	require.NoError(p.CreateTables(ctx))

	blk1, err := testutil.BuildCompleteBlock(uint64(1), uint64(361))
	require.NoError(err)

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	ctx = indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
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
	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk1)
	}))

	blkHash, produderName, expectedProducerName, err := p.GetBlockHistory(uint64(1))
	require.NoError(err)

	blk1Hash := blk1.HashBlock()
	require.Equal(hex.EncodeToString(blk1Hash[:]), blkHash)
	require.Equal("alfa", produderName)
	require.Equal("alfa", expectedProducerName)
}
