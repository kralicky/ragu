package custom

import (
	"google.golang.org/protobuf/compiler/protogen"
)

// From protoc-gen-go-grpc/main.go
const version = "1.2.0"

var requireUnimplemented *bool = func() *bool {
	b := true
	return &b
}()

func SetRequireUnimplemented(req bool) {
	requireUnimplemented = &req
}

var (
	emptyPbIdent = protogen.GoIdent{
		GoName:       "Empty",
		GoImportPath: "google.golang.org/protobuf/types/known/emptypb",
	}
)

func methodUsesEmptypb(method *protogen.Method) (in bool, out bool) {
	return method.Input.GoIdent == emptyPbIdent,
		method.Output.GoIdent == emptyPbIdent
}

var GenerateFile = generateFile
