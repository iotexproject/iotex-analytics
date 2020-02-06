// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package graphql

type Account struct {
	ActiveAccounts  []string         `json:"activeAccounts"`
	Alias           *Alias           `json:"alias"`
	OperatorAddress *OperatorAddress `json:"operatorAddress"`
}

type Action struct {
	ByDates               *ActionList      `json:"byDates"`
	ByHash                *ActionDetail    `json:"byHash"`
	ByAddress             *ActionList      `json:"byAddress"`
	EvmTransfersByAddress *EvmTransferList `json:"evmTransfersByAddress"`
}

type ActionDetail struct {
	ActionInfo   *ActionInfo    `json:"actionInfo"`
	EvmTransfers []*EvmTransfer `json:"evmTransfers"`
}

type ActionInfo struct {
	ActHash   string `json:"actHash"`
	BlkHash   string `json:"blkHash"`
	TimeStamp int    `json:"timeStamp"`
	ActType   string `json:"actType"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
	GasFee    string `json:"gasFee"`
}

type ActionList struct {
	Exist   bool          `json:"exist"`
	Actions []*ActionInfo `json:"actions"`
	Count   int           `json:"count"`
}

type Alias struct {
	Exist     bool   `json:"exist"`
	AliasName string `json:"aliasName"`
}

type AverageHermesStats struct {
	Exist           bool             `json:"exist"`
	AveragePerEpoch []*HermesAverage `json:"averagePerEpoch"`
}

type Bookkeeping struct {
	Exist              bool                  `json:"exist"`
	RewardDistribution []*RewardDistribution `json:"rewardDistribution"`
	Count              int                   `json:"count"`
}

type BucketInfo struct {
	VoterEthAddress   string `json:"voterEthAddress"`
	VoterIotexAddress string `json:"voterIotexAddress"`
	IsNative          bool   `json:"isNative"`
	Votes             string `json:"votes"`
	WeightedVotes     string `json:"weightedVotes"`
	RemainingDuration string `json:"remainingDuration"`
	StartTime         string `json:"startTime"`
	Decay             bool   `json:"decay"`
}

type BucketInfoList struct {
	EpochNumber int           `json:"epochNumber"`
	BucketInfo  []*BucketInfo `json:"bucketInfo"`
	Count       int           `json:"count"`
}

type BucketInfoOutput struct {
	Exist          bool              `json:"exist"`
	BucketInfoList []*BucketInfoList `json:"bucketInfoList"`
}

type CandidateInfo struct {
	Name               string `json:"name"`
	Address            string `json:"address"`
	TotalWeightedVotes string `json:"totalWeightedVotes"`
	SelfStakingTokens  string `json:"selfStakingTokens"`
	OperatorAddress    string `json:"operatorAddress"`
	RewardAddress      string `json:"rewardAddress"`
}

type CandidateInfoList struct {
	EpochNumber int              `json:"epochNumber"`
	Candidates  []*CandidateInfo `json:"candidates"`
}

type CandidateMeta struct {
	EpochNumber        int    `json:"epochNumber"`
	TotalCandidates    int    `json:"totalCandidates"`
	ConsensusDelegates int    `json:"consensusDelegates"`
	TotalWeightedVotes string `json:"totalWeightedVotes"`
	VotedTokens        string `json:"votedTokens"`
}

type Chain struct {
	MostRecentEpoch       int               `json:"mostRecentEpoch"`
	MostRecentBlockHeight int               `json:"mostRecentBlockHeight"`
	VotingResultMeta      *VotingResultMeta `json:"votingResultMeta"`
	MostRecentTps         float64           `json:"mostRecentTPS"`
	NumberOfActions       *NumberOfActions  `json:"numberOfActions"`
}

type Delegate struct {
	Reward       *Reward           `json:"reward"`
	Productivity *Productivity     `json:"productivity"`
	Bookkeeping  *Bookkeeping      `json:"bookkeeping"`
	BucketInfo   *BucketInfoOutput `json:"bucketInfo"`
	Staking      *StakingOutput    `json:"staking"`
}

type DelegateAmount struct {
	DelegateName string `json:"delegateName"`
	Amount       string `json:"amount"`
}

type EpochRange struct {
	StartEpoch int `json:"startEpoch"`
	EpochCount int `json:"epochCount"`
}

type EvmTransfer struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Quantity string `json:"quantity"`
}

type EvmTransferDetail struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Quantity  string `json:"quantity"`
	ActHash   string `json:"actHash"`
	BlkHash   string `json:"blkHash"`
	TimeStamp int    `json:"timeStamp"`
}

type EvmTransferList struct {
	Exist        bool                 `json:"exist"`
	EvmTransfers []*EvmTransferDetail `json:"evmTransfers"`
	Count        int                  `json:"count"`
}

type Hermes struct {
	Exist              bool                  `json:"exist"`
	HermesDistribution []*HermesDistribution `json:"hermesDistribution"`
}

type HermesAverage struct {
	DelegateName       string `json:"delegateName"`
	RewardDistribution string `json:"rewardDistribution"`
	TotalWeightedVotes string `json:"totalWeightedVotes"`
}

type HermesDistribution struct {
	DelegateName        string                `json:"delegateName"`
	RewardDistribution  []*RewardDistribution `json:"rewardDistribution"`
	StakingIotexAddress string                `json:"stakingIotexAddress"`
	VoterCount          int                   `json:"voterCount"`
	WaiveServiceFee     bool                  `json:"waiveServiceFee"`
	Refund              string                `json:"refund"`
}

type NumberOfActions struct {
	Exist bool `json:"exist"`
	Count int  `json:"count"`
}

type OperatorAddress struct {
	Exist           bool   `json:"exist"`
	OperatorAddress string `json:"operatorAddress"`
}

type Pagination struct {
	Skip  int `json:"skip"`
	First int `json:"first"`
}

type Productivity struct {
	Exist              bool   `json:"exist"`
	Production         string `json:"production"`
	ExpectedProduction string `json:"expectedProduction"`
}

type Reward struct {
	Exist           bool   `json:"exist"`
	BlockReward     string `json:"blockReward"`
	EpochReward     string `json:"epochReward"`
	FoundationBonus string `json:"foundationBonus"`
}

type RewardDistribution struct {
	VoterEthAddress   string `json:"voterEthAddress"`
	VoterIotexAddress string `json:"voterIotexAddress"`
	Amount            string `json:"amount"`
}

type RewardSources struct {
	Exist                 bool              `json:"exist"`
	DelegateDistributions []*DelegateAmount `json:"delegateDistributions"`
}

type StakingInformation struct {
	EpochNumber  int    `json:"epochNumber"`
	TotalStaking string `json:"totalStaking"`
	SelfStaking  string `json:"selfStaking"`
}

type StakingOutput struct {
	Exist       bool                  `json:"exist"`
	StakingInfo []*StakingInformation `json:"stakingInfo"`
}

type TopHolder struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
}

type Voting struct {
	CandidateInfo []*CandidateInfoList `json:"candidateInfo"`
	VotingMeta    *VotingMeta          `json:"votingMeta"`
	RewardSources *RewardSources       `json:"rewardSources"`
}

type VotingMeta struct {
	Exist         bool             `json:"exist"`
	CandidateMeta []*CandidateMeta `json:"candidateMeta"`
}

type VotingResultMeta struct {
	TotalCandidates    int    `json:"totalCandidates"`
	TotalWeightedVotes string `json:"totalWeightedVotes"`
	VotedTokens        string `json:"votedTokens"`
}

type Xrc20 struct {
	ByContractAddress    *XrcList              `json:"byContractAddress"`
	ByAddress            *XrcList              `json:"byAddress"`
	ByPage               *XrcList              `json:"byPage"`
	Xrc20Addresses       *XrcAddressList       `json:"xrc20Addresses"`
	TokenHolderAddresses *XrcHolderAddressList `json:"tokenHolderAddresses"`
}

type Xrc721 struct {
	ByContractAddress    *XrcList              `json:"byContractAddress"`
	ByAddress            *XrcList              `json:"byAddress"`
	ByPage               *XrcList              `json:"byPage"`
	Xrc721Addresses      *XrcAddressList       `json:"xrc721Addresses"`
	TokenHolderAddresses *XrcHolderAddressList `json:"tokenHolderAddresses"`
}

type XrcAddressList struct {
	Exist     bool      `json:"exist"`
	Addresses []*string `json:"addresses"`
	Count     int       `json:"count"`
}

type XrcHolderAddressList struct {
	Addresses []*string `json:"addresses"`
	Count     int       `json:"count"`
}

type XrcInfo struct {
	Contract  string `json:"contract"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
	From      string `json:"from"`
	To        string `json:"to"`
	Quantity  string `json:"quantity"`
}

type XrcList struct {
	Exist bool       `json:"exist"`
	Xrc   []*XrcInfo `json:"xrc"`
	Count int        `json:"count"`
}
