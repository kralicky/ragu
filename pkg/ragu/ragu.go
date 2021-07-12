package ragu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kralicky/ragu/pkg/machinery"
	"github.com/yoheimuta/go-protoparser/v4"
	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
	"k8s.io/utils/pointer"
)

// From protoc-gen-go-grpc/main.go
const version = "1.1.0"

var requireUnimplemented *bool = pointer.Bool(true)

func SetRequireUnimplemented(req bool) {
	requireUnimplemented = &req
}

type File = pluginpb.CodeGeneratorResponse_File

func resolveDependencies(desc *descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	deps := []*descriptorpb.FileDescriptorProto{}
	for _, dep := range desc.Dependency {
		// Check if file exists
		if _, err := os.Stat(dep); err == nil {
			// File exists
			proto, err := machinery.ParseProto(dep)
			if err != nil {
				panic(err)
			}
			desc := machinery.GenerateDescriptor(proto)
			deps = append(deps, resolveDependencies(desc)...)
		} else if wk, err := machinery.ReadWellKnownType(dep); err == nil {
			// File does not exist, but is a well-known type
			reader := strings.NewReader(wk)
			proto, err := protoparser.Parse(reader, protoparser.WithFilename(dep))
			if err != nil {
				panic(err)
			}
			desc := machinery.GenerateDescriptor(proto)
			deps = append(deps, resolveDependencies(desc)...)
		} else {
			// File does not exist
			fmt.Fprintln(os.Stderr, "Warning: Dependency", dep, "not found")
		}
	}
	deps = append(deps, desc)

	// remove duplicates
	depsFiltered := []*descriptorpb.FileDescriptorProto{}
	for _, dep := range deps {
		exists := false
		for _, existing := range depsFiltered {
			if existing.GetName() == dep.GetName() {
				exists = true
				break
			}
		}
		if !exists {
			depsFiltered = append(depsFiltered, dep)
		}
	}
	return depsFiltered
}

func GenerateCode(input string, grpc bool) ([]*File, error) {
	proto, err := machinery.ParseProto(input)
	if err != nil {
		return nil, err
	}
	if proto.Syntax.ProtobufVersion != "proto3" {
		return nil, errors.New("only proto3 is supported")
	}

	desc := machinery.GenerateDescriptor(proto)

	// Generate descriptors for dependencies, including well-known types
	allProtos := resolveDependencies(desc)
	machinery.ResolveKindsFromDependencies(allProtos)

	codeGeneratorRequest := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{input}, // Only generate the input file
		ProtoFile:      allProtos,
		CompilerVersion: &pluginpb.Version{
			Major: pointer.Int32(0),
			Minor: pointer.Int32(1),
			Patch: pointer.Int32(0),
		},
	}

	opts := protogen.Options{}
	plugin, err := opts.New(codeGeneratorRequest)
	if err != nil {
		return nil, err
	}

	for _, f := range plugin.Files {
		if f.Generate {
			gengo.GenerateFile(plugin, f)
			if grpc {
				generateFile(plugin, f)
			}
		}
	}

	resp := plugin.Response()
	if resp.Error != nil {
		return nil, errors.New(resp.GetError())
	}

	for _, f := range resp.File {
		*f.Name = filepath.Base(*f.Name)
		// these generators produce different headers
		if strings.HasSuffix(*f.Name, "_grpc.pb.go") {
			*f.Content = strings.Replace(*f.Content,
				`// - protoc `,
				`// - ragù   `,
				1,
			)
		} else if strings.HasSuffix(*f.Name, ".pb.go") {
			*f.Content = strings.Replace(*f.Content,
				`// 	protoc `,
				`// 	ragù   `,
				1,
			)
		}
	}
	return resp.File, nil
}
