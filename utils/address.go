package utils

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-address/address/bech32"
)

func FixErrorAddress(addr string) (address.Address, error) {
	_, grouped, err := bech32.Decode(addr)
	if err != nil {
		return nil, err
	}
	payload, err := bech32.ConvertBits(grouped, 5, 8, false)
	if err != nil || len(payload) < 20 {
		return nil, err
	}

	payload = payload[len(payload)-20:]
	return address.FromBytes(payload)
}
