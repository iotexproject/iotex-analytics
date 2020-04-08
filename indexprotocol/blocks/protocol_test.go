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
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
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
	ctx := context.Background()

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

	blk1, err := testutil.BuildCompleteBlock(uint64(1), uint64(2))
	require.NoError(err)

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	ctx = indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: "ROLLDPOS",
	})

	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(blk1.Height(), 10))},
	}
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: []byte(strconv.FormatUint(1000, 10)),
	}, nil)
	electionClient.EXPECT().GetCandidates(gomock.Any(), gomock.Any()).Times(2).Return(
		&api.CandidateResponse{
			Candidates: []*api.Candidate{
				{
					Name:            "616c6661",
					OperatorAddress: testutil.Addr1,
				},
				{
					Name:            "627261766f",
					OperatorAddress: testutil.Addr2,
				},
			},
		}, nil,
	)
	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(1, 10))},
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

	blk2, err := testutil.BuildEmptyBlock(2)
	require.NoError(err)

	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(blk2.Height(), 10))},
	}
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: []byte(strconv.FormatUint(uint64(1100), 10)),
	}, nil)

	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(uint64(2), 10))},
	}
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: data,
	}, nil)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk2)
	}))

	blockHistory, err := p.getBlockHistory(uint64(1))
	require.NoError(err)

	blk1Hash := blk1.HashBlock()
	require.Equal(uint64(1), blockHistory.EpochNumber)
	require.Equal(hex.EncodeToString(blk1Hash[:]), blockHistory.BlockHash)
	require.Equal("616c6661", blockHistory.ProducerName)
	require.Equal("627261766f", blockHistory.ExpectedProducerName)

	productivityHistory, err := p.getProductivityHistory(uint64(1), "627261766f")
	require.NoError(err)
	require.Equal(uint64(0), productivityHistory.Production)
	require.Equal(uint64(1), productivityHistory.ExpectedProduction)
}
