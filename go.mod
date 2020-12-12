module github.com/iotexproject/iotex-analytics

go 1.13

require (
	github.com/99designs/gqlgen v0.8.3
	github.com/agnivade/levenshtein v1.0.2 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/ethereum/go-ethereum v1.9.5
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.2
	github.com/iotexproject/go-pkgs v0.1.2-0.20200523040337-5f1d9ddaa8ee
	github.com/iotexproject/iotex-address v0.2.2
	github.com/iotexproject/iotex-core v1.1.3
	github.com/iotexproject/iotex-election v0.3.5-0.20201031050050-c3ab4f339a54
	github.com/iotexproject/iotex-proto v0.4.4-0.20201029172022-a8466422b0f1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/rs/zerolog v1.18.0
	github.com/stretchr/testify v1.5.1
	github.com/vektah/gqlparser v1.1.2
	go.uber.org/zap v1.10.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/tools v0.0.0-20200318150045-ba25ddc85566 // indirect
	google.golang.org/grpc v1.28.0
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.3.1

exclude github.com/dgraph-io/badger v2.0.0-rc.2+incompatible

exclude github.com/dgraph-io/badger v2.0.0-rc2+incompatible

exclude github.com/ipfs/go-ds-badger v0.0.3
