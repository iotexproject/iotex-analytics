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
	github.com/iotexproject/iotex-core v0.7.1
	github.com/iotexproject/iotex-election v0.1.17
	github.com/iotexproject/iotex-proto v0.2.1-0.20190707070949-4abe81017bf3
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0 // indirect
	github.com/rs/zerolog v1.14.3
	github.com/stretchr/testify v1.3.0
	github.com/vektah/gqlparser v1.1.2
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // indirect
	golang.org/x/tools v0.0.0-20190815235612-5b08f89bfc0c // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v1.7.4-0.20190216004546-2bbee71fbe61

exclude github.com/dgraph-io/badger v2.0.0-rc.2+incompatible

exclude github.com/dgraph-io/badger v2.0.0-rc2+incompatible

exclude github.com/ipfs/go-ds-badger v0.0.3
