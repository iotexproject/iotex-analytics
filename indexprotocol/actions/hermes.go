// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
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
	createHermesContractActionHashIndex = "CREATE INDEX %s ON %s (`action_hash`)"
	createHermesContract                = "CREATE TABLE IF NOT EXISTS %s " +
		"(action_hash VARCHAR(64) NOT NULL, delegate_name VARCHAR(256) NOT NULL)"
	insertHermesContract = "INSERT INTO %s (action_hash, delegate_name) VALUES %s"

	// HermesMsgEmiter is the function name for emiting contract info
	HermesMsgEmiter = "Distribute(uint256,uint256,bytes32,uint256,uint256)"
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

func (p *Protocol) updateHermes(tx *sql.Tx, receipts []*action.Receipt) error {
	contractList := make([]HermesContractInfo, 0)
	for _, receipt := range receipts {
		delegateName, err := getDelegateNameFromLog(receipt.Logs)
		if err != nil {
			continue
		}
		receiptHash := receipt.Hash()
		contract := HermesContractInfo{
			ActionHash:   hex.EncodeToString(receiptHash[:]),
			DelegateName: delegateName,
		}
		contractList = append(contractList, contract)
	}
	if err := p.insertHermesContract(tx, contractList); err != nil {
		return err
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

func emiterIsHermes(receiptHash hash.Hash256) bool {
	now := string(receiptHash[:])
	emiter := string(crypto.Keccak256([]byte(HermesMsgEmiter))[:])
	if strings.Compare(emiter, now) != 0 {
		return false
	}
	return true
}

func getDelegateNameFromTopic(receiptHash hash.Hash256) string {
	var buf []byte
	for _, b := range receiptHash {
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return string(buf[:])
}

func getDelegateNameFromLog(logs []*action.Log) (string, error) {
	num := len(logs)
	for num >= 0 {
		log := logs[num-1]
		if len(log.Topics) < 2 {
			continue
		}
		emiter := log.Topics[0]
		delegateNameHash := log.Topics[1]
		if emiterIsHermes(emiter) == false {
			continue
		}
		delegateName := getDelegateNameFromTopic(delegateNameHash)
		return delegateName, nil
	}
	return "", errors.New("not found delegateName")
}
