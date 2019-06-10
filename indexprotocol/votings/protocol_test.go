// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
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
					Name:               "616c6661616c6661616c6661",
					OperatorAddress:    testutil.Addr1,
					RewardAddress:      testutil.RewardAddr1,
					TotalWeightedVotes: big.NewInt(1000).String(),
				},
				{
					Name:               "627261766f627261766f6272",
					OperatorAddress:    testutil.Addr2,
					RewardAddress:      testutil.RewardAddr2,
					TotalWeightedVotes: big.NewInt(500).String(),
				},
			},
		}, nil,
	)

	request1 := &api.GetBucketsByCandidateRequest{
		Name:   "616c6661616c6661616c6661",
		Height: strconv.Itoa(1000),
		Offset: uint32(0),
		Limit:  math.MaxUint32,
	}
	electionClient.EXPECT().GetBucketsByCandidate(gomock.Any(), request1).Times(1).Return(
		&api.BucketResponse{
			Buckets: []*api.Bucket{
				{
					Voter:             "voter1",
					Votes:             "300",
					WeightedVotes:     "500",
					RemainingDuration: "1day",
				},
				{
					Voter:             "voter2",
					Votes:             "400",
					WeightedVotes:     "500",
					RemainingDuration: "2days",
				},
			},
		}, nil,
	)

	request2 := &api.GetBucketsByCandidateRequest{
		Name:   "627261766f627261766f6272",
		Height: strconv.Itoa(1000),
		Offset: uint32(0),
		Limit:  math.MaxUint32,
	}
	electionClient.EXPECT().GetBucketsByCandidate(gomock.Any(), request2).Times(1).Return(
		&api.BucketResponse{
			Buckets: []*api.Bucket{
				{
					Voter:             "voter3",
					Votes:             "200",
					WeightedVotes:     "300",
					RemainingDuration: "3days",
				},
				{
					Voter:             "voter4",
					Votes:             "150",
					WeightedVotes:     "200",
					RemainingDuration: "4days",
				},
			},
		}, nil,
	)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	votingHistoryList, err := p.getVotingHistory(uint64(1), "alfa")
	require.NoError(err)
	require.Equal(2, len(votingHistoryList))
	require.Equal("voter1", votingHistoryList[0].VoterAddress)
	require.Equal("300", votingHistoryList[0].Votes)
	require.Equal("500", votingHistoryList[0].WeightedVotes)
	require.Equal("1day", votingHistoryList[0].RemainingDuration)

	votingResult, err := p.getVotingResult(uint64(1), "bravo")
	require.NoError(err)
	require.Equal(testutil.Addr2, votingResult.OperatorAddress)
	require.Equal(testutil.RewardAddr2, votingResult.RewardAddress)
	require.Equal("500", votingResult.TotalWeightedVotes)
}
