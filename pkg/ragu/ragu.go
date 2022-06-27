package ragu

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/kralicky/ragu/pkg/util"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

type GeneratedFile struct {
	Name         string
	RelativePath string
	PackagePath  string
	Content      string
}

func (g *GeneratedFile) Read(p []byte) (int, error) {
	return copy(p, g.Content), nil
}

func (g *GeneratedFile) WriteToDisk() error {
	return os.WriteFile(g.RelativePath, []byte(g.Content), 0644)
}

// Generates code for each source file (or files matching a glob pattern)
// using one or more code generators.
func GenerateCode(generators []Generator, sources ...string) ([]*GeneratedFile, error) {
	if resolved, err := resolvePatterns(sources); err != nil {
		return nil, err
	} else {
		sources = resolved
	}

	localPkg, err := util.CallingFuncPackage()
	if err != nil {
		return nil, fmt.Errorf("failed to find calling package: %w", err)
	}

	descs, err := (protoparse.Parser{
		InterpretOptionsInUnlinkedFiles: true,
	}).ParseFilesButDoNotLink(sources...)
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
		Accessor:              SourceAccessor(localPkg),
		LookupImport:          desc.LoadFileDescriptor,
	}
	descriptors, err := parser.ParseFiles(sourcePkgPaths...)
	if err != nil {
		return nil, err
	}

	outputs := []*GeneratedFile{}
	for _, d := range descriptors {
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

		plugin, err := (protogen.Options{}).New(codeGeneratorRequest)
		if err != nil {
			return nil, err
		}

		for _, g := range generators {
			if err := g.Generate(plugin); err != nil {
				return nil, err
			}
		}

		response := plugin.Response()
		if response.Error != nil {
			return nil, errors.New(response.GetError())
		}

		for _, f := range response.GetFile() {
			outputs = append(outputs, &GeneratedFile{
				Name:         path.Base(f.GetName()),
				PackagePath:  f.GetName(),
				RelativePath: strings.TrimPrefix(strings.TrimPrefix(*f.Name, localPkg.ImportPath), "/"),
				Content:      f.GetContent(),
			})
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

func resolvePatterns(sources []string) ([]string, error) {
	resolved := []string{}
	for _, source := range sources {
		if strings.Contains(source, "*") {
			matches, err := doublestar.Glob(source)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, matches...)
		} else {
			resolved = append(resolved, source)
		}
	}
	return resolved, nil
}
