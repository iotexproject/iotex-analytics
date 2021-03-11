// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	// this is required for mysql usage
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// NewMySQL instantiates a mysql
func NewMySQL(connectStr string, dbName string) Store {
	return newStoreBase("mysql", connectStr, dbName)
}

// CreateMySQLStore instantiates a mysql
func CreateMySQLStore(username, password, host, port, dbName string) Store {
	connectStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/", username, password, host, port)
	return newStoreBase("mysql", connectStr, dbName)
}
