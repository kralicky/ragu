module github.com/kralicky/ragu

go 1.20

require (
	github.com/bmatcuk/doublestar v1.3.4
	github.com/bufbuild/protocompile v0.5.2-0.20230523010820-2b297241d0ff
	github.com/flosch/pongo2/v6 v6.0.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.3
	github.com/iancoleman/strcase v0.2.0
	github.com/jhump/protoreflect v1.15.1
	github.com/kralicky/grpc-gateway/v2 v2.15.2
	github.com/samber/lo v1.38.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.21.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/mod v0.11.0
	golang.org/x/tools v0.6.0
	golang.org/x/tools/gopls v0.0.0-00010101000000-000000000000
	google.golang.org/genproto/googleapis/api v0.0.0-20230526203410-71b5a4ffd15e
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230526203410-71b5a4ffd15e
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/golang/glog v1.1.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	golang.org/x/vuln v0.0.0-20230110180137-6ad3e3d07815 // indirect
	google.golang.org/genproto v0.0.0-20230526203410-71b5a4ffd15e // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/bufbuild/protocompile => ../protocompile

replace golang.org/x/tools/gopls => ../tools/gopls

replace golang.org/x/tools => github.com/kralicky/tools v0.0.0-20230614234516-e1d90db7570d

// replace golang.org/x/tools/gopls => github.com/kralicky/tools/gopls v0.0.0-20230614234516-e1d90db7570d
