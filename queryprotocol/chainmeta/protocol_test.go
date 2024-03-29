package chainmeta

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr = "bfe10c7cf8aa29:8bed5959@tcp(us-cdbr-east-04.cleardb.com:3306)/"
	dbName     = "heroku_067cec75e0ba5ba"
)

func TestProtocol_MostRecentTPS(t *testing.T) {

	require := require.New(t)
	ctx := context.Background()
	var err error

	testutil.CleanupDatabase(t, connectStr, dbName)

	store := s.NewMySQL(connectStr, dbName, false)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()

	var cfg indexservice.Config
	cfg.Poll = indexprotocol.Poll{
		VoteThreshold:        "100000000000000000000",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	}
	idx := indexservice.NewIndexer(store, cfg)
	p := NewProtocol(idx)

	t.Run("Testing unregistered", func(t *testing.T) {
		_, err = p.MostRecentTPS(1)
		require.EqualError(err, "blocks protocol is unregistered")
	})

	idx.RegisterDefaultProtocols()

	t.Run("Testing 0 range", func(t *testing.T) {
		_, err = p.MostRecentTPS(0)
		assert.EqualError(t, err, "TPS block window should be greater than 0")
	})

}
