package ragu

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/kralicky/ragu/v2/pkg/plugins/golang"
	"github.com/kralicky/ragu/v2/pkg/plugins/golang/gateway"
	"github.com/kralicky/ragu/v2/pkg/plugins/golang/grpc"
	"github.com/kralicky/ragu/v2/pkg/plugins/python"
	"github.com/kralicky/ragu/v2/pkg/util"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func GenerateCode(sources ...Source) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	localPkg, err := util.CallingFuncPackage()
	if err != nil {
		return nil, fmt.Errorf("failed to find calling package: %w", err)
	}

	descs, err := (protoparse.Parser{
		InterpretOptionsInUnlinkedFiles: true,
	}).ParseFilesButDoNotLink(util.Map(sources, Source.AbsolutePath)...)
	if err != nil {
		return nil, err
	}
	sourcePkgPaths := []string{}
	for _, desc := range descs {
		goPkg := desc.Options.GoPackage
		if goPkg == nil {
			return nil, fmt.Errorf("%s: missing go_package option", desc.GetName())
		}
		goPath := path.Join(*goPkg, filepath.Base(desc.GetName()))
		sourcePkgPaths = append(sourcePkgPaths, goPath)
	}

	parser := protoparse.Parser{
		InferImportPaths:      false,
		IncludeSourceCodeInfo: true,
		Accessor: func(filename string) (io.ReadCloser, error) {
			if strings.HasPrefix(filename, localPkg.ImportPath) {
				// local to this package
				localPath := path.Join(localPkg.Dir, strings.TrimPrefix(filename, localPkg.ImportPath))
				return os.Open(localPath)
			}

			return os.Open(filename)
		},
		LookupImport: func(s string) (*desc.FileDescriptor, error) {
			d, err := desc.LoadFileDescriptor(s)
			return d, err
		},
	}
	descriptors, err := parser.ParseFiles(sourcePkgPaths...)
	if err != nil {
		return nil, err
	}

	outputs := []*pluginpb.CodeGeneratorResponse_File{}
	for i, d := range descriptors {
		descs := util.Map(recursiveDeps(d, map[string]struct{}{}), (*desc.FileDescriptor).AsFileDescriptorProto)
		descs = append(descs, d.AsFileDescriptorProto())

		codeGeneratorRequest := &pluginpb.CodeGeneratorRequest{
			FileToGenerate: []string{d.GetName()},
			ProtoFile:      descs,
			CompilerVersion: &pluginpb.Version{
				Major: lo.ToPtr[int32](1),
				Minor: lo.ToPtr[int32](0),
				Patch: lo.ToPtr[int32](0),
			},
		}

		opts := protogen.Options{}
		plugin, err := opts.New(codeGeneratorRequest)
		if err != nil {
			return nil, err
		}

		if err := golang.Generate(plugin); err != nil {
			return nil, err
		}

		if err := grpc.Generate(plugin); err != nil {
			return nil, err
		}

		if err := gateway.Generate(plugin); err != nil {
			return nil, err
		}

		if err := python.Generate(plugin); err != nil {
			return nil, err
		}

		response := plugin.Response()
		if response.Error != nil {
			return nil, errors.New(response.GetError())
		}

		for _, f := range response.GetFile() {
			*f.Name = filepath.Join(filepath.Dir(sources[i].AbsolutePath()), filepath.Base(*f.Name))
			outputs = append(outputs, f)
		}
	}

	return outputs, nil
}

func recursiveDeps(d *desc.FileDescriptor, alreadySeen map[string]struct{}) []*desc.FileDescriptor {
	if _, ok := alreadySeen[d.GetName()]; ok {
		return nil
	}
	deps := []*desc.FileDescriptor{}
	for _, dep := range d.GetDependencies() {
		deps = append(deps, recursiveDeps(dep, alreadySeen)...)
		deps = append(deps, dep)
	}
	return deps
}
