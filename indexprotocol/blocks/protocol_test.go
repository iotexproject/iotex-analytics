// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocks

import (
	"context"
	"database/sql"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
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

	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(blk1.Height())},
	}
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
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
	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(uint64(1))},
	}
	candidateList := state.CandidateList{
		{
			Address:       testutil.Addr1,
			RewardAddress: testutil.RewardAddr1,
			Votes:         big.NewInt(100),
		},
		{
			Address:       testutil.Addr2,
			RewardAddress: testutil.RewardAddr2,
			Votes:         big.NewInt(10),
		},
	}
	data, err := candidateList.Serialize()
	require.NoError(err)
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: data,
	}, nil)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk1)
	}))

	blockHistory, err := p.getBlockHistory(uint64(1))
	require.NoError(err)

	blk1Hash := blk1.HashBlock()
	require.Equal(uint64(1), blockHistory.EpochNumber)
	require.Equal(hex.EncodeToString(blk1Hash[:]), blockHistory.BlockHash)
	require.Equal("alfa", blockHistory.ProducerName)
	require.Equal("bravo", blockHistory.ExpectedProducerName)

	productivityHistory, err := p.getProductivityHistory(uint64(1), "bravo")
	require.NoError(err)
	require.Equal(uint64(0), productivityHistory.Production)
	require.Equal(uint64(1), productivityHistory.ExpectedProduction)
}
