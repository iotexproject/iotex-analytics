// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

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
					Name:          "alfa",
					RewardAddress: testutil.RewardAddr1,
				},
				{
					Name:          "bravo",
					RewardAddress: testutil.RewardAddr2,
				},
				{
					Name:          "charlie",
					RewardAddress: testutil.RewardAddr3,
				},
			},
		}, nil,
	)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

	actionHash1 := blk.Actions[4].Hash()
	rewardHistoryList, err := p.getRewardHistory(hex.EncodeToString(actionHash1[:]))
	require.NoError(err)
	require.Equal(1, len(rewardHistoryList))
	require.Equal(uint64(1), rewardHistoryList[0].EpochNumber)
	require.Equal("alfa", rewardHistoryList[0].CandidateName)
	require.Equal(testutil.RewardAddr1, rewardHistoryList[0].RewardAddress)
	require.Equal("16", rewardHistoryList[0].BlockReward)
	require.Equal("0", rewardHistoryList[0].EpochReward)
	require.Equal("0", rewardHistoryList[0].FoundationBonus)

	actionHash2 := blk.Actions[5].Hash()
	rewardHistoryList, err = p.getRewardHistory(hex.EncodeToString(actionHash2[:]))
	require.NoError(err)
	require.Equal(3, len(rewardHistoryList))

	accountReward, err := p.getAccountReward(uint64(1), "alfa")
	require.NoError(err)
	require.Equal("16", accountReward.BlockReward)
	require.Equal("10", accountReward.EpochReward)
	require.Equal("100", accountReward.FoundationBonus)
}
