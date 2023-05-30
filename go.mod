module github.com/kralicky/ragu

go 1.20

require (
	github.com/bmatcuk/doublestar v1.3.4
	github.com/bufbuild/protocompile v0.5.2-0.20230523010820-2b297241d0ff
	github.com/davecgh/go-spew v1.1.1
	github.com/flosch/pongo2/v6 v6.0.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.3
	github.com/iancoleman/strcase v0.2.0
	github.com/jhump/protoreflect v1.15.1
	github.com/kralicky/grpc-gateway/v2 v2.15.2
	github.com/samber/lo v1.38.1
	github.com/spf13/pflag v1.0.5
	go.lsp.dev/jsonrpc2 v0.10.0
	go.lsp.dev/protocol v0.12.0
	go.uber.org/zap v1.21.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/mod v0.10.0
	google.golang.org/genproto/googleapis/api v0.0.0-20230526203410-71b5a4ffd15e
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230526203410-71b5a4ffd15e
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/golang/glog v1.1.1 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.3.4 // indirect
	go.lsp.dev/pkg v0.0.0-20210717090340-384b27a52fb2 // indirect
	go.lsp.dev/uri v0.3.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230526203410-71b5a4ffd15e // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/bufbuild/protocompile => ../protocompile
