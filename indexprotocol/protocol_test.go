// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexprotocol

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

const candidateName = "726f626f7462703030303030"

func TestEnDecodeName(t *testing.T) {
	require := require.New(t)
	decoded, err := DecodeDelegateName(candidateName)
	require.NoError(err)

	encoded, err := EncodeDelegateName(decoded)
	require.NoError(err)
	require.Equal(candidateName, encoded)
}

func TestActiveDelegates(t *testing.T) {
	require := require.New(t)

	conn, err := grpc.Dial("api.iotex.one:80", grpc.WithBlock(), grpc.WithInsecure())
	require.NoError(err)
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	ad := make(map[uint64]map[string]bool)
	for _, i := range []uint64{11409481, 11410201, 11410921} {
		candidateList, err := GetAllStakingCandidates(cli, i)
		require.NoError(err)
		ad[i] = make(map[string]bool)
		for _, c := range candidateList.Candidates {
			ad[i][c.Name] = true
		}
	}
	require.Equal(3, len(ad))

	// active
	active := []string{
		"nodeasy", "rocketfuel", "tgb", "capitmu", "pubxpayments", "yvalidator", "consensusnet",
		"zhcapital", "metanyx", "bcf", "binancevote", "thebottoken", "iotfi", "slowmist", "blockboost",
		"iosg", "hashquark", "longz", "cryptozoo", "hofancrypto", "satoshi", "gamefantasy", "bittaker",
		"swft", "iotask", "infstones", "blockfolio", "staking4all", "rockx", "elitex", "eatliverun",
		"cobo", "cpc", "iotexcore", "iotexlab", "hackster", "coingecko", "matrix", "chainshield",
		"iotexteam", "hashbuy", "satoshimusk", "sesameseed", "iotexicu", "royalland", "a4x", "smartstake",
		"coredev", "ducapital", "hotbit", "pnp", "emmasiotx", "mrtrump", "enlightiv", "keys", "wetez",
		"droute", "iotexunion", "xeon",
	}
	// these delegates are no longer active
	inactive := []string{
		"ratel", "square2", "citex2018", "offline", "iotxplorerio",
		"ra", "kita", "draperdragon", "airfoil", "square4",
	}

	for _, m := range ad {
		for _, name := range active {
			require.True(m[name])
		}
		for _, name := range inactive {
			require.False(m[name])
		}
	}
}
