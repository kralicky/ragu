module github.com/kralicky/ragu

go 1.21

toolchain go1.21.1

require (
	github.com/flosch/pongo2/v6 v6.0.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.3
	github.com/iancoleman/strcase v0.3.0
	github.com/kralicky/grpc-gateway/v2 v2.15.2
	github.com/kralicky/protols v0.0.0-20231031175015-9c7a7b5aea8e
	github.com/samber/lo v1.38.1
	google.golang.org/genproto/googleapis/api v0.0.0-20231030173426-d783a09b4405
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231030173426-d783a09b4405
	google.golang.org/protobuf v1.31.0
)

require (
	cloud.google.com/go/dlp v1.10.3 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230512164433-5d1fd1a340c9 // indirect
	github.com/bufbuild/protocompile v0.5.2-0.20230523010820-2b297241d0ff // indirect
	github.com/bufbuild/protovalidate-go v0.3.4 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/google/cel-go v0.18.1 // indirect
	github.com/kralicky/gpkg v0.0.0-20220311205216-0d8ea9557555 // indirect
	github.com/plar/go-adaptive-radix-tree v1.0.5 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/telemetry v0.0.0-20231011160506-788d5629a052 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	golang.org/x/tools/gopls v0.12.4 // indirect
	golang.org/x/vuln v1.0.1 // indirect
	google.golang.org/genproto v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/grpc v1.59.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/bufbuild/protocompile => github.com/kralicky/protocompile v0.0.0-20231030041518-0ad723357927
	golang.org/x/tools => github.com/kralicky/tools v0.0.0-20231015012334-9bbd10d902a7
	golang.org/x/tools/gopls => github.com/kralicky/tools/gopls v0.0.0-20231015012334-9bbd10d902a7
)
