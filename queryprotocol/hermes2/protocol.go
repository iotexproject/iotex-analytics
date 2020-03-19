package hermes2

import (
	"github.com/iotexproject/iotex-analytics/indexservice"
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
}

// ByDelegateResponse defines response for hermes2 byDelegate request
type ByDelegateResponse struct {
	Exist         bool
	VoterInfoList []VoterInfo
	Count         int
}

// DelegateInfo defines delegate information
type DelegateInfo struct {
	DelegateName string
	Amount       int
}

// ByVoterResponse defines response for hermes2 byVoter request
type ByVoterResponse struct {
	Exist            bool
	DelegateInfoList []DelegateInfo
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
	// TODO
	return ByDelegateResponse{}, nil
}

// GetHermes2ByVoter get hermes's delegate list by voter name
func (p *Protocol) GetHermes2ByVoter(arg HermesArg, voterName string) (ByVoterResponse, error) {
	// TODO
	return ByVoterResponse{}, nil
}
