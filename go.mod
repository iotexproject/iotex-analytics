module github.com/iotexproject/iotex-analytics

go 1.17

require (
	github.com/99designs/gqlgen v0.8.3
	github.com/agnivade/levenshtein v1.0.2 // indirect
	github.com/apilayer/freegeoip v3.5.0+incompatible // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/docker/docker v1.13.1 // indirect
	github.com/elastic/gosigar v0.10.5 // indirect
	github.com/ethereum/go-ethereum v1.10.4
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.5.2
	github.com/howeyc/fsnotify v0.9.0 // indirect
	github.com/iotexproject/go-pkgs v0.1.12-0.20220209063039-b876814568a0
	github.com/iotexproject/iotex-address v0.2.8 // indirect
	github.com/iotexproject/iotex-core v1.8.1-rc0
	github.com/iotexproject/iotex-election v0.3.5-0.20210611041425-20ddf674363d
	github.com/iotexproject/iotex-proto v0.5.10-0.20220415042310-0d4bcef3febf
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/oschwald/maxminddb-golang v1.5.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/rs/zerolog v1.18.0
	github.com/stretchr/testify v1.7.0
	github.com/vektah/gqlparser v1.1.2
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.43.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/ethereum/go-ethereum v1.10.4 => github.com/iotexproject/go-ethereum v0.4.0

exclude github.com/dgraph-io/badger v2.0.0-rc.2+incompatible

exclude github.com/dgraph-io/badger v2.0.0-rc2+incompatible

exclude github.com/ipfs/go-ds-badger v0.0.3
