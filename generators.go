package ragu

import (
	"github.com/kralicky/ragu/pkg/plugins/golang"
	"github.com/kralicky/ragu/pkg/plugins/golang/gateway"
	"github.com/kralicky/ragu/pkg/plugins/golang/grpc"
	"github.com/kralicky/ragu/pkg/plugins/python"
	"google.golang.org/protobuf/compiler/protogen"
)

type Generator interface {
	Name() string
	Generate(gen *protogen.Plugin) error
}

func DefaultGenerators() []Generator {
	return []Generator{
		golang.Generator,
		grpc.Generator,
		gateway.Generator,
	}
}

func AllGenerators() []Generator {
	return []Generator{
		golang.Generator,
		grpc.Generator,
		gateway.Generator,
		python.Generator,
	}
}
