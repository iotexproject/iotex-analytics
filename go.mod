module github.com/iotexproject/iotex-analytics

go 1.12

require (
	github.com/99designs/gqlgen v0.8.3
	github.com/agnivade/levenshtein v1.0.2 // indirect
	github.com/ethereum/go-ethereum v1.8.27
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/iotexproject/go-pkgs v0.1.1-0.20190708233003-85a24189bbd4
	github.com/iotexproject/iotex-address v0.2.0
	github.com/iotexproject/iotex-core v0.8.0
	github.com/iotexproject/iotex-election v0.1.17
	github.com/iotexproject/iotex-proto v0.2.1-0.20190711042234-eb3d2a61ab27
	github.com/jbenet/goprocess v0.1.2 // indirect
	github.com/libp2p/go-libp2p v0.0.22 // indirect
	github.com/libp2p/go-libp2p-circuit v0.0.6 // indirect
	github.com/libp2p/go-libp2p-connmgr v0.0.4 // indirect
	github.com/libp2p/go-libp2p-crypto v0.0.2 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.0.2 // indirect
	github.com/multiformats/go-multistream v0.0.3 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0 // indirect
	github.com/rs/zerolog v1.14.3
	github.com/stretchr/testify v1.3.0
	github.com/vektah/gqlparser v1.1.2
	go.uber.org/zap v1.10.0
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/xerrors v0.0.0-20190506180316-385005612d73 // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v1.7.4-0.20190216004546-2bbee71fbe61
