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
					Name:               "alfa",
					OperatorAddress:    testutil.Addr1,
					RewardAddress:      testutil.RewardAddr1,
					TotalWeightedVotes: big.NewInt(1000).String(),
				},
				{
					Name:               "bravo",
					OperatorAddress:    testutil.Addr2,
					RewardAddress:      testutil.RewardAddr2,
					TotalWeightedVotes: big.NewInt(500).String(),
				},
			},
		}, nil,
	)

	request1 := &api.GetBucketsByCandidateRequest{
		Name:   "alfa",
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
		Name:   "bravo",
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
