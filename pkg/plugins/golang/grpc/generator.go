package grpc

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
)

const version = "1.2.0"

var requireUnimplemented *bool = lo.ToPtr(true)

func SetRequireUnimplemented(req bool) {
	*requireUnimplemented = req
}

func Generate(gen *protogen.Plugin) error {
	for _, f := range gen.Files {
		if f.Generate {
			generateFile(gen, f)
		}
	}
	return nil
}
