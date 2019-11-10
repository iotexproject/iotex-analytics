module github.com/iotexproject/iotex-analytics

go 1.12

require (
	cloud.google.com/go v0.39.0 // indirect
	github.com/99designs/gqlgen v0.8.3
	github.com/AndreasBriese/bbloom v0.0.0-20190306092124-e2d15f34fcf9 // indirect
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/agnivade/levenshtein v1.0.2 // indirect
	github.com/btcsuite/goleveldb v1.0.0 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dgryski/go-sip13 v0.0.0-20190329191031-25c5027a8c7b // indirect
	github.com/ethereum/go-ethereum v1.8.27
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/google/pprof v0.0.0-20190515194954-54271f7e092f // indirect
	github.com/iotexproject/go-pkgs v0.1.1
	github.com/iotexproject/iotex-address v0.2.1
	github.com/iotexproject/iotex-antenna-go v0.0.0-20190522194402-4d96cae2af68 // indirect
	github.com/iotexproject/iotex-core v0.8.1-0.20191007232750-b79fb5c7ebaa
	github.com/iotexproject/iotex-election v0.2.7-0.20191008203349-58450eac6656
	github.com/iotexproject/iotex-proto v0.2.5
	github.com/ipfs/go-ds-badger v0.0.4 // indirect
	github.com/ipfs/go-ds-leveldb v0.0.2 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/karalabe/hid v1.0.0 // indirect
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pty v1.1.4 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/miekg/dns v1.1.13 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/rogpeppe/fastuuid v1.1.0 // indirect
	github.com/rs/zerolog v1.14.3
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.4.0 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/ugorji/go v1.1.5-pre // indirect
	github.com/vektah/gqlparser v1.1.2
	go.opencensus.io v0.22.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522 // indirect
	golang.org/x/image v0.0.0-20190523035834-f03afa92d3ff // indirect
	golang.org/x/mobile v0.0.0-20190509164839-32b2708ab171 // indirect
	golang.org/x/mod v0.1.0 // indirect
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // indirect
	golang.org/x/tools v0.0.0-20190815235612-5b08f89bfc0c // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
	honnef.co/go/tools v0.0.0-20190604153307-63e9ff576adb // indirect
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.2.0

exclude github.com/dgraph-io/badger v2.0.0-rc.2+incompatible

exclude github.com/dgraph-io/badger v2.0.0-rc2+incompatible

exclude github.com/ipfs/go-ds-badger v0.0.3
