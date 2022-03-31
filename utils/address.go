package utils

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-address/address/bech32"
)

func FixErrorAddress(addr string) (address.Address, error) {
	_, payload, err := bech32.Decode(addr)
	if err != nil || len(payload) < 20 {
		return nil, err
	}

	payload = payload[len(payload)-20:]
	return address.FromBytes(payload)
}
