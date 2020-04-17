// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/db"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-election/pb/election"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
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

	p, err := NewProtocol(store, epochctx.NewEpochCtx(36, 24, 15), indexprotocol.GravityChain{}, indexprotocol.Poll{
		VoteThreshold:        "100000000000000000000",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	})
	require.NoError(err)
	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(361), uint64(721))
	require.NoError(err)

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	ctx = indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: "ROLLDPOS",
	})

	// first call GetGravityChainStartHeight
	readStateRequestForGravityHeight := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(indexprotocol.PollProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(1, 10))},
	}
	first := chainClient.EXPECT().ReadState(gomock.Any(), readStateRequestForGravityHeight).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: []byte(strconv.FormatUint(1000, 10)),
	}, nil)

	// second call ProbationListByEpoch
	probationListByEpochRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(indexprotocol.PollProtocolID),
		MethodName: []byte("ProbationListByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(2, 10))},
	}
	pb := &iotextypes.ProbationCandidateList{}
	data, err := proto.Marshal(pb)
	second := chainClient.EXPECT().ReadState(gomock.Any(), probationListByEpochRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: data,
	}, nil)
	gomock.InOrder(
		first,
		second,
	)
	timestamp, err := ptypes.TimestampProto(time.Unix(1000, 0))
	require.NoError(err)

	chainClient.EXPECT().GetElectionBuckets(gomock.Any(), gomock.Any()).Times(1).Return(&iotexapi.GetElectionBucketsResponse{
		Buckets: []*iotextypes.ElectionBucket{},
	}, db.ErrNotExist)

	electionClient.EXPECT().GetRawData(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.RawDataResponse{
			Timestamp: timestamp,
			Buckets: []*election.Bucket{
				{
					Voter:     []byte("1111"),
					Candidate: []byte("616c6661"),
					StartTime: timestamp,
					Duration:  ptypes.DurationProto(time.Duration(10 * 24)),
					Decay:     true,
					Amount:    []byte("100"),
				},
			},
			Registrations: []*election.Registration{
				{
					Name:              []byte("616c6661"),
					Address:           []byte("112233"),
					OperatorAddress:   []byte(testutil.Addr1),
					RewardAddress:     []byte(testutil.RewardAddr1),
					SelfStakingWeight: 100,
				},
				{
					Name:              []byte("627261766f"),
					Address:           []byte("445566"),
					OperatorAddress:   []byte(testutil.Addr2),
					RewardAddress:     []byte(testutil.RewardAddr2),
					SelfStakingWeight: 102,
				},
			},
		}, nil,
	)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))
}
