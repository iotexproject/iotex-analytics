package actions

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"

)

const (
	connectStr = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName     = "heroku_7fed0b046078f80"
	ActionHistoryTableName = "action_history"
)

func TestProtocol_GetActiveAccount(t *testing.T) {

	// Creating temporary store
	require := require.New(t)
	ctx := context.Background()
	var err error

	testutil.CleanupDatabase(t, connectStr, dbName)

	store := s.NewMySQL(connectStr, dbName)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()

	// Creating protocol
	var cfg indexservice.Config
	idx := indexservice.NewIndexer(store, cfg)
	p := NewProtocol(idx)

	// Testing unregistered
	t.Run("Testing unregistered", func(t *testing.T) {
		_,err = p.GetActiveAccount(1)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "actions protocol is unregistered"))
	})

	// Testing no table
	t.Run("Testing no table", func(t *testing.T) {
		idx.RegisterDefaultProtocols()
		_,err = p.GetActiveAccount(1)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "failed to prepare get query"))
	})

	// Testing empty table
	t.Run("Testing empty table", func(t *testing.T) {
		// Creating table
		_, err = store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
			"(block_height DECIMAL(65, 0), "+
			"`from` VARCHAR(41))",
			ActionHistoryTableName))
		require.NoError(err)

		_, err = p.GetActiveAccount(1)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "not exist in DB"))
	})


	// Populate Table
	type testdata struct {
		input int
		listSize int
		hList []uint64
		fList []string
		output []string
	}

	var testsituations = []testdata {
		{2, 3, []uint64 {1, 2, 3}, []string {"a", "b", "c"}, []string {"c", "b"}},
		{2, 3, []uint64 {1, 2, 2}, []string {"a", "b", "b"}, []string{"b", "a"}},
		{2, 3, []uint64 {3, 2, 1}, []string {"a", "b", "c"}, []string{"a", "b"}},
	}

	for _, tCase := range testsituations {
		valStrs := make([]string, 0, tCase.listSize)
		valArgs := make([]interface{}, 0, tCase.listSize*2)
		for i := 0; i < tCase.listSize; i++ {
			valStrs = append(valStrs, "(?, ?)")
			valArgs = append(valArgs, tCase.hList[i], tCase.fList[i])
		}

		_, err = store.GetDB().Exec("SET AUTOCOMMIT = 1")
		insertQuery := fmt.Sprintf("INSERT INTO %s (block_height, `from`) VALUES %s", ActionHistoryTableName, strings.Join(valStrs, ","))
		_, err = store.GetDB().Exec(insertQuery, valArgs...)
		require.NoError(err)

		t.Run("Testing data cases", func(t *testing.T) {
			test, err := p.GetActiveAccount(tCase.input)
			require.NoError(err)
			require.ElementsMatch(test, tCase.output)
		})

		// Drop table
		_, err = store.GetDB().Exec("DROP TABLE " + ActionHistoryTableName)
		require.NoError(err)

		// Create empty table
		_, err = store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
			"(block_height DECIMAL(65, 0), "+
			"`from` VARCHAR(41))",
			ActionHistoryTableName))
		require.NoError(err)
	}

}

