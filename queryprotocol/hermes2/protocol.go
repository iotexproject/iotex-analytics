package hermes2

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	selectHermesDistributionByDelegateName = "SELECT t1.to, t1.amount, t1.action_hash, t2.timestamp " +
		"FROM (SELECT * FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND `from` = ?) " +
		"AS t1 INNER JOIN (SELECT * FROM %s WHERE epoch_number >= ? AND epoch_number <= ?) AS t2 ON t1.action_hash = t2.action_hash " +
		"WHERE t2.delegate_name = ? ORDER BY t2.timestamp desc limit ?,?"
	selectHermesDistributionByVoterAddress = "SELECT t2.delegate_name, t1.amount, t1.action_hash, t2.timestamp " +
		"FROM (SELECT * FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND `from` = ?) " +
		"AS t1 INNER JOIN (SELECT * FROM %s WHERE epoch_number >= ? AND epoch_number <= ?) AS t2 ON t1.action_hash = t2.action_hash " +
		"WHERE t1.to = ? ORDER BY t2.timestamp desc limit ?,?"
)

// HermesArg defines hermes request parameters
type HermesArg struct {
	StartEpoch int
	EpochCount int
	Offset     uint64
	Size       uint64
}

// VoterInfo defines voter information
type VoterInfo struct {
	VoterAddress string
	Amount       string
	ActionHash   string
	Timestamp    string
}

// ByDelegateResponse defines response for hermes2 byDelegate request
type ByDelegateResponse struct {
	Exist         bool
	VoterInfoList []*VoterInfo
	Count         int
}

// DelegateInfo defines delegate information
type DelegateInfo struct {
	DelegateName string
	Amount       string
	ActionHash   string
	Timestamp    string
}

// ByVoterResponse defines response for hermes2 byVoter request
type ByVoterResponse struct {
	Exist            bool
	DelegateInfoList []*DelegateInfo
	Count            int
}

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer      *indexservice.Indexer
	hermesConfig indexprotocol.HermesConfig
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer, cfg indexprotocol.HermesConfig) *Protocol {
	return &Protocol{
		indexer:      idx,
		hermesConfig: cfg,
	}
}

// GetHermes2ByDelegate get hermes's voter list by delegate name
func (p *Protocol) GetHermes2ByDelegate(arg HermesArg, delegateName string) (ByDelegateResponse, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectHermesDistributionByDelegateName, accounts.BalanceHistoryTableName, actions.HermesContractTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return ByDelegateResponse{}, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	endEpoch := arg.StartEpoch + arg.EpochCount - 1
	rows, err := stmt.Query(arg.StartEpoch, endEpoch, p.hermesConfig.MultiSendContractAddress, arg.StartEpoch, endEpoch,
		delegateName, arg.Offset, arg.Size)
	if err != nil {
		return ByDelegateResponse{}, errors.Wrap(err, "failed to execute get query")
	}

	var voterInfo VoterInfo
	parsedRows, err := s.ParseSQLRows(rows, &voterInfo)
	if err != nil {
		return ByDelegateResponse{}, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return ByDelegateResponse{Exist: false}, err
	}

	voterInfoList := make([]*VoterInfo, 0)
	for _, parsedRow := range parsedRows {
		voterInfoList = append(voterInfoList, parsedRow.(*VoterInfo))
	}

	return ByDelegateResponse{
		Exist:         true,
		VoterInfoList: voterInfoList,
		Count:         len(voterInfoList),
	}, nil
}

// GetHermes2ByVoter get hermes's delegate list by voter name
func (p *Protocol) GetHermes2ByVoter(arg HermesArg, voterName string) (ByVoterResponse, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectHermesDistributionByVoterAddress, accounts.BalanceHistoryTableName, actions.HermesContractTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return ByVoterResponse{}, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	endEpoch := arg.StartEpoch + arg.EpochCount - 1
	rows, err := stmt.Query(arg.StartEpoch, endEpoch, p.hermesConfig.MultiSendContractAddress, arg.StartEpoch, endEpoch,
		voterName, arg.Offset, arg.Size)
	if err != nil {
		return ByVoterResponse{}, errors.Wrap(err, "failed to execute get query")
	}

	var delegateInfo DelegateInfo
	parsedRows, err := s.ParseSQLRows(rows, &delegateInfo)
	if err != nil {
		return ByVoterResponse{}, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return ByVoterResponse{Exist: false}, err
	}

	delegateInfoList := make([]*DelegateInfo, 0)
	for _, parsedRow := range parsedRows {
		delegateInfoList = append(delegateInfoList, parsedRow.(*DelegateInfo))
	}

	return ByVoterResponse{
		Exist:            true,
		DelegateInfoList: delegateInfoList,
		Count:            len(delegateInfoList),
	}, nil
}
