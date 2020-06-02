package epochctx

import "github.com/iotexproject/iotex-core/pkg/log"

// EpochCtx defines epoch context
type EpochCtx struct {
	numCandidateDelegates   uint64
	numDelegates            uint64
	numSubEpochs            uint64
	numSubEpochsDardanelles uint64
	dardanellesHeight       uint64
	dardanellesOn           bool
	fairbankHeight          uint64
}

// Option is optional setting for epoch context
type Option func(*EpochCtx) error

// EnableDardanellesSubEpoch will set give numSubEpochs at give height.
func EnableDardanellesSubEpoch(height, numSubEpochs uint64) Option {
	return func(e *EpochCtx) error {
		e.dardanellesOn = true
		e.numSubEpochsDardanelles = numSubEpochs
		e.dardanellesHeight = height
		return nil
	}
}

// FairbankHeight will set fairbank height.
func FairbankHeight(height uint64) Option {
	return func(e *EpochCtx) error {
		e.fairbankHeight = height
		return nil
	}
}

// NewEpochCtx returns a new epoch context
func NewEpochCtx(numCandidateDelegates, numDelegates, numSubEpochs uint64, opts ...Option) *EpochCtx {
	if numCandidateDelegates < numDelegates {
		numCandidateDelegates = numDelegates
	}
	e := &EpochCtx{
		numCandidateDelegates: numCandidateDelegates,
		numDelegates:          numDelegates,
		numSubEpochs:          numSubEpochs,
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			log.S().Panicf("Failed to execute epoch context creation option %p: %v", opt, err)
		}
	}
	return e
}

// GetEpochNumber returns the number of the epoch for a given height
func (e *EpochCtx) GetEpochNumber(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	if !e.dardanellesOn || height <= e.dardanellesHeight {
		return (height-1)/e.numDelegates/e.numSubEpochs + 1
	}
	dardanellesEpoch := e.GetEpochNumber(e.dardanellesHeight)
	dardanellesEpochHeight := e.GetEpochHeight(dardanellesEpoch)
	return dardanellesEpoch + (height-dardanellesEpochHeight)/e.numDelegates/e.numSubEpochsDardanelles
}

// GetEpochHeight returns the start height of an epoch
func (e *EpochCtx) GetEpochHeight(epochNum uint64) uint64 {
	if epochNum == 0 {
		return 0
	}
	dardanellesEpoch := e.GetEpochNumber(e.dardanellesHeight)
	if !e.dardanellesOn || epochNum <= dardanellesEpoch {
		return (epochNum-1)*e.numDelegates*e.numSubEpochs + 1
	}
	dardanellesEpochHeight := e.GetEpochHeight(dardanellesEpoch)
	return dardanellesEpochHeight + (epochNum-dardanellesEpoch)*e.numDelegates*e.numSubEpochsDardanelles
}

// NumCandidateDelegates returns the number of candidate delegates
func (e *EpochCtx) NumCandidateDelegates() uint64 {
	return e.numCandidateDelegates
}

// FairbankEffectiveHeight returns the effective height of fairbank
func (e *EpochCtx) FairbankEffectiveHeight() uint64 {
	return e.fairbankHeight + e.numDelegates*e.numSubEpochsDardanelles
}
