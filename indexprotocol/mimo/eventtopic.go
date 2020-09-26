// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mimo

import "strings"

// EventTopic defines the event topics
type EventTopic string

const (
	// AddLiquidity is an event of adding liquidity
	AddLiquidity EventTopic = "AddLiquidity"

	// RemoveLiquidity is an event of removing liquidity
	RemoveLiquidity = "RemoveLiquidity"

	// TokenPurchase is an event of purchasing token
	TokenPurchase = "TokenPurchase"

	// CoinPurchase is an event of purchasing coin
	CoinPurchase = "CoinPurchase"
)

// IsValid returns true if it is a valid topic
func (et EventTopic) IsValid() bool {
	switch et {
	case AddLiquidity, RemoveLiquidity, TokenPurchase, CoinPurchase:
		return true
	}
	return false
}

// JoinTopicsWithQuotes wraps topics with quotes and joins them with ","
func JoinTopicsWithQuotes(topics ...EventTopic) string {
	if len(topics) == 0 {
		return ""
	}
	strs := make([]string, len(topics))
	for i, topic := range topics {
		strs[i] = string(topic)
	}
	return "'" + strings.Join(strs, "','") + "'"
}
