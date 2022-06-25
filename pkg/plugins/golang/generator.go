package golang

import (
	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
)

func Generate(gen *protogen.Plugin) error {
	for _, f := range gen.Files {
		if f.Generate {
			gengo.GenerateFile(gen, f)
		}
	}
	return nil
}
