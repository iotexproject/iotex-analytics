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
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-antenna-go/v2/account"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
)

const (
	// HermesContractTableName is the table name of hermes contract
	HermesContractTableName = "hermes_contract"

	selectHermesContractInfo = "SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = " +
		"DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'"
	actionHashIndexName                 = "action_hash_index"
	createHermesContractActionHashIndex = "CREATE UNIQUE INDEX %s ON %s (`action_hash`)"
	createHermesContract                = "CREATE TABLE IF NOT EXISTS %s " +
		"(action_hash VARCHAR(64) NOT NULL, delegate_name VARCHAR(255) NOT NULL, timestamp VARCHAR(128) NOT NULL)"
	insertHermesContract = "INSERT INTO %s (action_hash, delegate_name, timestamp) VALUES %s"

	// HermesDistributionTableName is the table name of hermes distribution
	HermesDistributionTableName = "hermes_distribution"

	createHermesDistribution = "CREATE TABLE IF NOT EXISTS %s " +
		"(epoch_number DECIMAL(65, 0) NOT NULL, action_hash VARCHAR(64) NOT NULL, " +
		"delegate_name VARCHAR(255) NOT NULL, voter_address VARCHAR(40) NOT NULL, " +
		"amount DECIMAL(65, 0) NOT NULL, timestamp VARCHAR(128) NOT NULL)"

	insertHermesDistribution = "INSERT INTO %s SELECT t1.epoch_number, t1.action_hash, t2.delegate_name, t1.to, " +
		"t1.amount, t2.timestamp FROM (SELECT * FROM %s WHERE epoch_number > ? AND epoch_number <= ? AND `from` = ?) " +
		"AS t1 INNER JOIN %s AS t2 ON t1.action_hash = t2.action_hash"

	// DistributeMsgEmitter represents the distribute event in hermes contract
	DistributeMsgEmitter = "Distribute(uint256,uint256,bytes32,uint256,uint256)"

	// CommitMsgEmitter represents the commit event in hermes contract
	CommitMsgEmitter = "CommitDistributions(uint256,bytes32[])"

	// HermesABI defines the ABI of Hermes contract
	HermesABI = `[
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "bytes32"
        }
      ],
      "name": "distributedAmount",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "forwardRegistration",
      "outputs": [
        {
          "name": "",
          "type": "address"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "multisender",
      "outputs": [
        {
          "name": "",
          "type": "address"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "addrs",
          "type": "address[]"
        }
      ],
      "name": "removeAddressesFromWhitelist",
      "outputs": [
        {
          "name": "success",
          "type": "bool"
        }
      ],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "addr",
          "type": "address"
        }
      ],
      "name": "removeAddressFromWhitelist",
      "outputs": [
        {
          "name": "success",
          "type": "bool"
        }
      ],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "analyticsEndpoint",
      "outputs": [
        {
          "name": "",
          "type": "string"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "name": "endEpochs",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "addr",
          "type": "address"
        }
      ],
      "name": "addAddressToWhitelist",
      "outputs": [
        {
          "name": "success",
          "type": "bool"
        }
      ],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "bytes32"
        }
      ],
      "name": "distributedCount",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "owner",
      "outputs": [
        {
          "name": "",
          "type": "address"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "address"
        }
      ],
      "name": "whitelist",
      "outputs": [
        {
          "name": "",
          "type": "bool"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "contractStartEpoch",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "bytes32"
        },
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "name": "distributions",
      "outputs": [
        {
          "name": "distributedCount",
          "type": "uint256"
        },
        {
          "name": "amount",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": true,
      "inputs": [
        {
          "name": "",
          "type": "bytes32"
        },
        {
          "name": "",
          "type": "address"
        }
      ],
      "name": "recipientEpochTracker",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "addrs",
          "type": "address[]"
        }
      ],
      "name": "addAddressesToWhitelist",
      "outputs": [
        {
          "name": "success",
          "type": "bool"
        }
      ],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "transferOwnership",
      "outputs": [],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "name": "_contractStartEpoch",
          "type": "uint256"
        },
        {
          "name": "_multisendAddress",
          "type": "address"
        },
        {
          "name": "_forwardRegistrationAddress",
          "type": "address"
        },
        {
          "name": "_analyticsEndpoint",
          "type": "string"
        }
      ],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "constructor"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "name": "startEpoch",
          "type": "uint256"
        },
        {
          "indexed": false,
          "name": "endEpoch",
          "type": "uint256"
        },
        {
          "indexed": true,
          "name": "delegateName",
          "type": "bytes32"
        },
        {
          "indexed": false,
          "name": "numOfRecipients",
          "type": "uint256"
        },
        {
          "indexed": false,
          "name": "totalAmount",
          "type": "uint256"
        }
      ],
      "name": "Distribute",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "name": "endEpoch",
          "type": "uint256"
        },
        {
          "indexed": false,
          "name": "delegateNames",
          "type": "bytes32[]"
        }
      ],
      "name": "CommitDistributions",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "name": "addr",
          "type": "address"
        }
      ],
      "name": "WhitelistedAddressAdded",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "name": "addr",
          "type": "address"
        }
      ],
      "name": "WhitelistedAddressRemoved",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "name": "previousOwner",
          "type": "address"
        },
        {
          "indexed": true,
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "OwnershipTransferred",
      "type": "event"
    },
    {
      "constant": true,
      "inputs": [],
      "name": "getEndEpochCount",
      "outputs": [
        {
          "name": "",
          "type": "uint256"
        }
      ],
      "payable": false,
      "stateMutability": "view",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "_multisendAddress",
          "type": "address"
        }
      ],
      "name": "setMultisendAddress",
      "outputs": [],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "_endpoint",
          "type": "string"
        }
      ],
      "name": "setAnalyticsEndpoint",
      "outputs": [],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "delegateName",
          "type": "bytes32"
        },
        {
          "name": "endEpoch",
          "type": "uint256"
        },
        {
          "name": "recipients",
          "type": "address[]"
        },
        {
          "name": "amounts",
          "type": "uint256[]"
        }
      ],
      "name": "distributeRewards",
      "outputs": [],
      "payable": true,
      "stateMutability": "payable",
      "type": "function"
    },
    {
      "constant": false,
      "inputs": [
        {
          "name": "endEpoch",
          "type": "uint256"
        },
        {
          "name": "delegateNames",
          "type": "bytes32[]"
        }
      ],
      "name": "commitDistributions",
      "outputs": [],
      "payable": false,
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ]`
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
	Amount       string
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

func (p *Protocol) updateHermesContract(tx *sql.Tx, receipts []*action.Receipt, timestamp string) error {
	contractList := make([]HermesContractInfo, 0)
	for _, receipt := range receipts {
		delegateName, exist := getDelegateNameFromLog(receipt.Logs)
		if !exist {
			continue
		}
		actionHash := receipt.ActionHash
		contract := HermesContractInfo{
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

func (p *Protocol) updateHermesDistribution(tx *sql.Tx, chainClient iotexapi.APIServiceClient, receipts []*action.Receipt) error {
	var needUpdate bool
	for _, receipt := range receipts {
		if len(receipt.Logs) != 1 || len(receipt.Logs[0].Topics) != 1 {
			continue
		}
		if emitterIsHermesByTopic(receipt.Logs[0].Topics[0], CommitMsgEmitter) == true {
			needUpdate = true
		}
	}
	if !needUpdate {
		return nil
	}

	account, _ := account.NewAccount()
	c := iotex.NewAuthedClient(chainClient, account)

	caddr, err := address.FromString(p.hermesConfig.HermesContractAddress)
	if err != nil {
		return errors.Wrap(err, "failed to get hermes contract address from string")
	}
	hermesABI, err := abi.JSON(strings.NewReader(HermesABI))
	if err != nil {
		return errors.Wrap(err, "failed to get form hermes ABI")
	}
	data, err := c.Contract(caddr, hermesABI).Read("getEndEpochCount").Call(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to get end epoch count")
	}
	var endEpochCount *big.Int
	if err := data.Unmarshal(&endEpochCount); err != nil {
		return errors.Wrap(err, "failed to unmarshal end epoch count")
	}

	if endEpochCount.Cmp(big.NewInt(2)) == -1 {
		log.L().Warn("End epoch count is less than two")
		return nil
	}
	data, err = c.Contract(caddr, hermesABI).Read("endEpochs", endEpochCount.Sub(endEpochCount, big.NewInt(1))).Call(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to read last end epoch")
	}
	var lastEndEpoch *big.Int
	if err := data.Unmarshal(&lastEndEpoch); err != nil {
		return errors.Wrap(err, "failed to unmarshal last end epoch")
	}

	data, err = c.Contract(caddr, hermesABI).Read("endEpochs", endEpochCount.Sub(endEpochCount, big.NewInt(1))).Call(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to read last end epoch")
	}
	var secondToLastEndEpoch *big.Int
	if err := data.Unmarshal(&secondToLastEndEpoch); err != nil {
		return errors.Wrap(err, "failed to unmarshal second to last end epoch")
	}

	insertQuery := fmt.Sprintf(insertHermesDistribution, HermesDistributionTableName, accounts.BalanceHistoryTableName,
		HermesContractTableName)
	if _, err := tx.Exec(insertQuery, secondToLastEndEpoch.Uint64(), lastEndEpoch.Uint64(), p.hermesConfig.MultiSendContractAddress); err != nil {
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

func emitterIsHermesByTopic(logTopic hash.Hash256, expected string) bool {
	now := string(logTopic[:])
	emitter := string(crypto.Keccak256([]byte(expected))[:])
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
		if emitterIsHermesByTopic(emiterTopic, DistributeMsgEmitter) == false {
			continue
		}
		delegateNameTopic := log.Topics[1]
		delegateName := getDelegateNameFromTopic(delegateNameTopic)
		return delegateName, true
	}
	return "", false
}
