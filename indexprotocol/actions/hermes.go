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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// HermesContractTableName is the table name of hermes contract
	HermesContractTableName = "hermes_contract"

	selectHermesContractInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	actionHashIndexName                 = "action_hash_index"
	createHermesContractActionHashIndex = "CREATE INDEX %s ON %s (`action_hash`)"
	createHermesContract                = "CREATE TABLE IF NOT EXISTS %s " +
		"(action_hash VARCHAR(64) NOT NULL, delegate_name VARCHAR(256) NOT NULL, timestamp VARCHAR(128) NOT NULL)"
	insertHermesContract = "INSERT INTO %s (action_hash, delegate_name, timestamp) VALUES %s"

	// HermesDistributionTableName is the table name of hermes distribution
	HermesDistributionTableName = "hemes_distributition"

	createHermesDistribution = "CREATE TABLE IF NOT EXISTS %s " +
		"(epoch_number INT(64) NOT NULL, action_hash VARCHAR(64) NOT NULL, " +
		"delegate_name VARCHAR(255) NOT NULL, voter_address VARCHAR(41) NOT NULL, " +
		"amount INT(64) NOT NULL, timestamp VARCHAR(128) NOT NULL)"
	insertHermesDistribution = "INSERT INTO %s (epoch_number, action_hash, delegate_name, voter_address," +
		"amount, timestamp) VALUES %s"

	hermesJoin = "SELECT t1.epoch_number, t1.action_hash, t2.delegate_name, t1.to, " +
		"t1.amount, t2.timestamp FROM %s AS t1 INNER JOIN %s AS t2 WHERE t1.action_hash = t2.action_hash AND t1.from = %s" +
		" t1.epoch_number >= %d AND t1.epoch_number < %d"

	// HermesMsgEmiter is the function name for emiting contract info
	HermesMsgEmiter = "Distribute(uint256,uint256,bytes32,uint256,uint256)"
)

// HermesContractInfo defines a contract info for hermes
type HermesContractInfo struct {
	ActionHash   string
	DelegateName string
	Timestamp    string
}

// HermesDistributionInfo defines HermesDistribution records
type HermesDistributionInfo struct {
	EpochNumber  uint64
	ActionHash   string
	DelegateName string
	VoterAddress string
	Amount       uint64
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
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf(createHermesDistribution, HermesDistributionTableName)); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) updateHermes(tx *sql.Tx, blk *block.Block) error {
	timestamp := blk.Timestamp().String()
	contractList := make([]HermesContractInfo, 0)
	for _, receipt := range blk.Receipts {
		if strings.Compare(receipt.ContractAddress, p.hermesConfig.HermesContractAddress) != 0 {
			continue
		}
		delegateName, exist := getDelegateNameFromLog(receipt.Logs)
		if !exist {
			continue
		}
		receiptHash := receipt.Hash()
		contract := HermesContractInfo{
			ActionHash:   hex.EncodeToString(receiptHash[:]),
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

func (p *Protocol) joinHermes(tx *sql.Tx, epochNumber uint64) error {
	/*
		hermesJoin = "SELECT t1.epoch_number, t1.action_hash, t2.delegate_name, t1.to, " +
			"t1.amount, t2.timestamp FROM %s AS t1 INNER JOIN %s AS t2 WHERE t1.action_hash = t2.action_hash AND t1.from = %s"
	*/
	joinSQL := fmt.Sprintf(hermesJoin, accounts.BalanceHistoryTableName, HermesContractTableName,
		p.hermesConfig.MultiSendContractAddress, epochNumber-24, epochNumber)

	db := p.Store.GetDB()
	stmt, err := db.Prepare(joinSQL)
	if err != nil {
		return errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(joinSQL)
	if err != nil {
		return errors.Wrap(err, "failed to execute get query")
	}

	var info HermesDistributionInfo
	parsedRows, err := s.ParseSQLRows(rows, &info)
	if err != nil {
		return errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		// no need to join
		return nil
	}

	infoList := make([]*HermesDistributionInfo, 0)
	for _, parsedRow := range parsedRows {
		infoList = append(infoList, parsedRow.(*HermesDistributionInfo))
	}

	return p.insertHermesDistribution(tx, infoList)
}

func (p *Protocol) insertHermesDistribution(tx *sql.Tx, infoList []*HermesDistributionInfo) error {
	valStrs := make([]string, 0, len(infoList))
	valArgs := make([]interface{}, 0, len(infoList))
	for _, list := range infoList {
		valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?)")
		valArgs = append(valArgs, list.EpochNumber, list.ActionHash,
			list.DelegateName, list.VoterAddress, list.Amount, list.Timestamp)
	}
	insertQuery := fmt.Sprintf(insertHermesDistribution, HermesDistributionTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) insertHermesContract(tx *sql.Tx, contractList []HermesContractInfo) error {
	valStrs := make([]string, 0, len(contractList))
	valArgs := make([]interface{}, 0, len(contractList))
	for _, list := range contractList {
		valStrs = append(valStrs, "(?, ?, ?)")
		valArgs = append(valArgs, list.ActionHash, list.DelegateName, list.Timestamp)
	}
	insertQuery := fmt.Sprintf(insertHermesContract, HermesContractTableName, strings.Join(valStrs, ","))

	if _, err := tx.Exec(insertQuery, valArgs...); err != nil {
		return err
	}
	return nil
}

func emiterIsHermesByTopic(logTopic hash.Hash256) bool {
	now := string(logTopic[:])
	emiter := string(crypto.Keccak256([]byte(HermesMsgEmiter))[:])
	if strings.Compare(emiter, now) != 0 {
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
	for num >= 0 {
		log := logs[num-1]
		if len(log.Topics) < 2 {
			continue
		}
		emiterTopic := log.Topics[0]
		if emiterIsHermesByTopic(emiterTopic) == false {
			continue
		}
		delegateNameTopic := log.Topics[1]
		delegateName := getDelegateNameFromTopic(delegateNameTopic)
		return delegateName, true
	}
	return "", false
}
