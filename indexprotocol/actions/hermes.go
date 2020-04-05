// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
)

const (
	// HermesContractTableName is the table name of hermes contract
	HermesContractTableName = "hermes_contract"

	selectHermesContractInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	actionHashIndexName                 = "action_hash_index"
	createHermesContractActionHashIndex = "CREATE UNIQUE INDEX %s ON %s (`action_hash`)"
	createHermesContract                = "CREATE TABLE IF NOT EXISTS %s " +
		"(epoch_number DECIMAL(65, 0) NOT NULL, action_hash VARCHAR(64) NOT NULL, delegate_name VARCHAR(255) NOT NULL, timestamp VARCHAR(128) NOT NULL)"
	insertHermesContract = "INSERT INTO %s (epoch_number, action_hash, delegate_name, timestamp) VALUES %s"

	// DistributeMsgEmitter represents the distribute event in hermes contract
	DistributeMsgEmitter = "Distribute(uint256,uint256,bytes32,uint256,uint256)"
)

// HermesContractInfo defines a contract info for hermes
type HermesContractInfo struct {
	EpochNumber  uint64
	ActionHash   string
	DelegateName string
	Timestamp    string
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

func (p *Protocol) updateHermesContract(tx *sql.Tx, receipts []*action.Receipt, epochNumber uint64, timestamp string) error {
	contractList := make([]HermesContractInfo, 0)
	for _, receipt := range receipts {
		delegateName, exist := getDelegateNameFromLog(receipt.Logs)
		if !exist {
			continue
		}
		actionHash := receipt.ActionHash
		contract := HermesContractInfo{
			EpochNumber:  epochNumber,
			ActionHash:   hex.EncodeToString(actionHash[:]),
			DelegateName: delegateName,
			Timestamp:    timestamp,
		}
		contractList = append(contractList, contract)
	}
	if len(contractList) == 0 {
		return nil
	}
	return p.insertHermesContract(tx, contractList)
}

func (p *Protocol) insertHermesContract(tx *sql.Tx, contractList []HermesContractInfo) error {
	valStrs := make([]string, 0, len(contractList))
	valArgs := make([]interface{}, 0, len(contractList))
	for _, list := range contractList {
		valStrs = append(valStrs, "(?, ?, ?, ?)")
		valArgs = append(valArgs, list.EpochNumber, list.ActionHash, list.DelegateName, list.Timestamp)
	}
	insertQuery := fmt.Sprintf(insertHermesContract, HermesContractTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func emitterIsDistributeByTopic(logTopic hash.Hash256) bool {
	now := string(logTopic[:])
	emitter := string(crypto.Keccak256([]byte(DistributeMsgEmitter))[:])
	if strings.Compare(emitter, now) != 0 {
		return false
	}
	return true
}

func getDelegateNameFromTopic(logTopic hash.Hash256) string {
	n := bytes.IndexByte(logTopic[:], 0)
	return string(logTopic[:n])
}

func getDelegateNameFromLog(logs []*action.Log) (string, bool) {
	num := len(logs)
	// reverse range
	for num > 0 {
		num--
		log := logs[num]
		if len(log.Topics) < 2 {
			continue
		}
		emiterTopic := log.Topics[0]
		if emitterIsDistributeByTopic(emiterTopic) == false {
			continue
		}
		delegateNameTopic := log.Topics[1]
		delegateName := getDelegateNameFromTopic(delegateNameTopic)
		return delegateName, true
	}
	return "", false
}
