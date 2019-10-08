// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-election/pb/election"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"

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

	p, err := NewProtocol(store, uint64(24), uint64(15), indexprotocol.GravityChain{}, indexprotocol.Poll{
		VoteThreshold:        "100000000000000000000",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	})

	require.NoError(err)
	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(1), uint64(361))
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
					Name:               "616c6661",
					OperatorAddress:    testutil.Addr1,
					RewardAddress:      testutil.RewardAddr1,
					TotalWeightedVotes: big.NewInt(1000).String(),
				},
				{
					Name:               "627261766f",
					OperatorAddress:    testutil.Addr2,
					RewardAddress:      testutil.RewardAddr2,
					TotalWeightedVotes: big.NewInt(500).String(),
				},
			},
		}, nil,
	)

	timestamp, err := ptypes.TimestampProto(time.Unix(1000, 0))
	require.NoError(err)

	electionClient.EXPECT().GetRawData(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.RawDataResponse{
			Timestamp: timestamp,
			Buckets: []*election.Bucket{
				{
					Voter:     []byte("14234"),
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

	votingResult, err := p.getVotingResult(uint64(1), "627261766f")
	require.NoError(err)
	require.Equal(testutil.Addr2, votingResult.OperatorAddress)
	require.Equal(testutil.RewardAddr2, votingResult.RewardAddress)
	require.Equal("500", votingResult.TotalWeightedVotes)
}
