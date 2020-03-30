package hermes2

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/actions"
	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	selectHermesDistributionByDelegateName = "SELECT voter_address, amount, timestamp WHERE epoch_number > ? AND " +
		"epoch_number < ? AND delegate_name = ? desc limit ?,?"
	selectHermesDistributionByVoterAddress = "SELECT delegateName, amount, timestamp WHERE epoch_number > ? AND " +
		"epoch_number < ? AND voter_address = ? desc limit ?,?"
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
	Amount       int
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
	Amount       int
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
	indexer *indexservice.Indexer
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetHermes2ByDelegate get hermes's voter list by delegate name
func (p *Protocol) GetHermes2ByDelegate(arg HermesArg, delegateName string) (ByDelegateResponse, error) {
	db := p.indexer.Store.GetDB()
	stmt, err := db.Prepare(selectHermesDistributionByDelegateName)
	if err != nil {
		return ByDelegateResponse{}, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(actions.HermesDistributionTableName, arg.StartEpoch, arg.EpochCount,
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
	stmt, err := db.Prepare(selectHermesDistributionByVoterAddress)
	if err != nil {
		return ByVoterResponse{}, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(arg.StartEpoch, arg.EpochCount, voterName, arg.Offset, arg.Size)
	if err != nil {
		return ByVoterResponse{}, errors.Wrap(err, "failed to execute get query")
	}

	var deletegateInfo DelegateInfo
	parsedRows, err := s.ParseSQLRows(rows, &deletegateInfo)
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
