// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"testing"

	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr = "bfe10c7cf8aa29:8bed5959@tcp(us-cdbr-east-04.cleardb.com:3306)/"
	dbName     = "heroku_067cec75e0ba5ba"
)

func TestMySQLStorePutGet(t *testing.T) {
	testutil.CleanupDatabase(t, connectStr, dbName)
	testRDSStorePutGet := TestStorePutGet
	t.Run("MySQL Store", func(t *testing.T) {
		testRDSStorePutGet(NewMySQL(connectStr, dbName, false), t)
	})
	testutil.CleanupDatabase(t, connectStr, dbName)
}

func TestMySQLStoreTransaction(t *testing.T) {
	testutil.CleanupDatabase(t, connectStr, dbName)
	testSQLite3StoreTransaction := TestStoreTransaction
	t.Run("MySQL Store", func(t *testing.T) {
		testSQLite3StoreTransaction(NewMySQL(connectStr, dbName, false), t)
	})
	testutil.CleanupDatabase(t, connectStr, dbName)
}
