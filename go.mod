module github.com/iotexproject/iotex-analytics

go 1.12

require (
	github.com/99designs/gqlgen v0.8.3
	github.com/agnivade/levenshtein v1.0.2 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/ethereum/go-ethereum v1.8.27
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/iotexproject/go-pkgs v0.1.1
	github.com/iotexproject/iotex-address v0.2.1
	github.com/iotexproject/iotex-core v0.8.1-0.20191007232750-b79fb5c7ebaa
	github.com/iotexproject/iotex-election v0.2.7-0.20191008203349-58450eac6656
	github.com/iotexproject/iotex-proto v0.2.6-0.20200327040553-157f35632918 // indirect
	github.com/pkg/errors v0.8.1
	github.com/rs/zerolog v1.14.3
	github.com/stretchr/testify v1.3.0
	github.com/vektah/gqlparser v1.1.2
	go.opencensus.io v0.22.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // indirect
	golang.org/x/tools v0.0.0-20200318150045-ba25ddc85566 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.2.0

exclude github.com/dgraph-io/badger v2.0.0-rc.2+incompatible

exclude github.com/dgraph-io/badger v2.0.0-rc2+incompatible

exclude github.com/ipfs/go-ds-badger v0.0.3
