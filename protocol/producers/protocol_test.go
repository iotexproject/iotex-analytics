// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producers

import (
	"context"
	"database/sql"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/stretchr/testify/require"

	s "github.com/iotexproject/iotex-api/sql"
	"github.com/iotexproject/iotex-api/testutil"
)

func TestProtocol(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg := config.Default
	testPath := cfg.DB.SQLITE3.SQLite3File
	testutil.CleanupPath(t, testPath)

	store := s.NewSQLite3(cfg.DB.SQLITE3)
	require.NoError(store.Start(ctx))
	defer func() {
		require.NoError(store.Stop(ctx))
		testutil.CleanupPath(t, testPath)
	}()

	p := NewProtocol(store, uint64(24), uint64(36), uint64(15))

	require.NoError(p.CreateTables(ctx))

	initialBPList := make(state.CandidateList, 0)
	initialBPList = append(initialBPList, &state.Candidate{
		Address:       testutil.Addr1,
		Votes:         big.NewInt(10),
		RewardAddress: testutil.RewardAddr1,
	})
	// TODO: Update initial block producers information right after creating tables
	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.updateBlockProducersHistory(tx, uint64(1), initialBPList)
	}))

	blockProducers, err := p.GetBlockProducersHistory(uint64(1))
	require.NoError(err)
	require.Equal(len(initialBPList), len(blockProducers))

	blk1, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk1)
	}))

	blockProducers, err = p.GetBlockProducersHistory(uint64(2))
	require.NoError(err)
	require.Equal(2, len(blockProducers))

	production, expectedProduction, err := p.GetProductivityHistory(uint64(1), testutil.Addr1)
	require.NoError(err)
	require.Equal(uint64(1), production)
	require.Equal(uint64(1), expectedProduction)

	blk2, err := testutil.BuildEmptyBlock(uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk2)
	}))

	production1, expectedProduction1, err := p.GetProductivityHistory(uint64(2), testutil.Addr1)
	require.NoError(err)
	require.Equal(uint64(1), production1)
	require.Equal(uint64(0), expectedProduction1)

	production2, expectedProduction2, err := p.GetProductivityHistory(uint64(2), testutil.Addr2)
	require.NoError(err)
	require.Equal(uint64(0), production2)
	require.Equal(uint64(1), expectedProduction2)
}
