// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"strconv"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func stringToHash256(str string) (hash.Hash256, error) {
	var receiptHash hash.Hash256
	for i := range receiptHash {
		receiptHash[i] = 0
	}
	for i := range receiptHash {
		tmpStr := str[i*2 : i*2+2]
		s, err := strconv.ParseUint(tmpStr, 16, 32)
		if err != nil {
			return receiptHash, err
		}
		receiptHash[i] = byte(s)
	}
	return receiptHash, nil
}

func TestGetDelegateNameFromTopic(t *testing.T) {
	require := require.New(t)
	delegateNameTopic, err := stringToHash256("746865626f74746f6b656e230000000000000000000000000000000000000000")
	require.NoError(err)
	delegateName := getDelegateNameFromTopic(delegateNameTopic)
	require.Equal("thebottoken#", delegateName)
}

func TestEmiterIsHermesByTopic(t *testing.T) {
	require := require.New(t)
	emiterTopic, err := stringToHash256("7de680eab607fdcc6137464e40d375ad63446cf255dcea9bd4a19676f7f24f56")
	require.NoError(err)
	require.True(emiterIsHermesByTopic(emiterTopic))

	emiterTopic, err = stringToHash256("6a5c4f52260adc90a8637fe2d8fbbc4141b625fa6840fca5f3e5cef6a4992293")
	require.NoError(err)
	require.False(emiterIsHermesByTopic(emiterTopic))
}
