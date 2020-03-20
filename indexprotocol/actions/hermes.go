// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-core/action"
	//s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// HermesContractTableName is the table name of hermes contract
	HermesContractTableName = "hermes_contract"

	selectHermesContractInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	actionHashIndexName                 = "action_hash_index"
	createHermesContractActionHashIndex = "CREATE INDEX %s ON %s (`action_hash`)"
	createHermesContract                = "CREATE TABLE IF NOT EXISTS %s " +
		"(action_hash VARCHAR(64) NOT NULL, delegate_name VARCHAR(256) NOT NULL)"
	insertHermesContract = "INSERT INTO %s (action_hash, delegate_name) VALUES %s"
)

// HermesContractInfo defines a contract info for hermes
type HermesContractInfo struct {
	ActionHash   string
	DelegateName string
}

// CreateHermesTables creates tables
func (p *Protocol) CreateHermesTables(ctx context.Context) error {
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createHermesContract, HermesContractTableName)); err != nil {
		return err
	}
	var exist uint64
	if err := p.Store.GetDB().QueryRow(fmt.Sprintf(selectHermesContractInfo, HermesContractTableName, actionHashIndexName)).Scan(&exist); err != nil {
		return err
	}
	if exist == 0 {
		if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createHermesContractActionHashIndex, actionHashIndexName, HermesContractTableName)); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) insertHermesContract(tx *sql.Tx, contractList []HermesContractInfo) error {
	valStrs := make([]string, 0, len(contractList))
	valArgs := make([]interface{}, 0, len(contractList))
	for _, list := range contractList {
		valStrs = append(valStrs, "(?, ?)")
		valArgs = append(valArgs, list.ActionHash, list.DelegateName)
	}
	insertQuery := fmt.Sprintf(insertHermesContract, HermesContractTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) getDelegateNameFromLog(logs []*action.Log) (string, error) {
	// TODO
	return "", nil
}
