module github.com/kralicky/ragu

go 1.18

require (
	github.com/bmatcuk/doublestar v1.3.4
	github.com/flosch/pongo2/v6 v6.0.0
	github.com/iancoleman/strcase v0.2.0
	github.com/jhump/protoreflect v1.12.0
	github.com/kralicky/grpc-gateway/v2 v2.11.0-1
	github.com/samber/lo v1.21.0
	golang.org/x/exp v0.0.0-20220613132600-b0d781184e0d
	golang.org/x/mod v0.6.0-dev.0.20220106191415-9b9b3d81d5e3
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	google.golang.org/genproto v0.0.0-20220719170305-83ca9fad585f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kralicky/grpc-gateway/v2 => ../grpc-gateway
