package actions

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"golang.org/x/crypto/sha3"
)

func actionToRLP(core *iotextypes.ActionCore) (*types.Transaction, error) {
	var (
		to      string
		amount  = new(big.Int)
		payload []byte
	)

	switch {
	case core.GetTransfer() != nil:
		tsf := core.GetTransfer()
		to = tsf.Recipient
		amount.SetString(tsf.Amount, 10)
		payload = tsf.Payload
	case core.GetExecution() != nil:
		exec := core.GetExecution()
		to = exec.Contract
		amount.SetString(exec.Amount, 10)
		payload = exec.Data
	default:
		return nil, fmt.Errorf("invalid action type %T not supported", core)
	}

	// generate raw tx
	gasPrice := new(big.Int)
	gasPrice.SetString(core.GetGasPrice(), 10)
	if to != "" {
		addr, err := address.FromString(to)
		if err != nil {
			return nil, err
		}
		ethAddr := common.BytesToAddress(addr.Bytes())
		return types.NewTransaction(core.GetNonce(), ethAddr, amount, core.GetGasLimit(), gasPrice, payload), nil
	}
	return types.NewContractCreation(core.GetNonce(), amount, core.GetGasLimit(), gasPrice, payload), nil
}

func rlpSignedHash(tx *types.Transaction, chainID uint32, sig []byte) (hash.Hash256, error) {
	if len(sig) != 65 {
		return hash.ZeroHash256, fmt.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	signedTx, err := tx.WithSignature(types.NewEIP155Signer(big.NewInt(int64(chainID))), sc)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := sha3.NewLegacyKeccak256()
	rlp.Encode(h, signedTx)
	return hash.BytesToHash256(h.Sum(nil)), nil
}
