package ragu

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
	"github.com/kralicky/protols/codegen"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type GeneratedFile struct {
	// Basename of the generated file.
	Name string
	// Path where this file can be written to, such that it will be in the same
	// directory as the source proto it was generated from. Calling WriteToDisk
	// will write the file to this path. This will be a relative path if
	// the source file was given as a relative path.
	SourceRelPath string
	// Go package (not including the file name) defined in the source proto.
	Package string
	// Generated file content.
	Content string
}

func (g *GeneratedFile) Read(p []byte) (int, error) {
	return copy(p, g.Content), nil
}

func (g *GeneratedFile) WriteToDisk() error {
	return os.WriteFile(g.SourceRelPath, []byte(g.Content), 0644)
}

// Generates code for each source file (or files matching a glob pattern)
// using one or more code generators.
func GenerateCode(generators []Generator, sources ...string) ([]*GeneratedFile, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	driver := codegen.NewDriver(wd, zap.NewNop())
	results, err := driver.Compile()
	if err != nil {
		return nil, err
	}
	for _, msg := range results.Messages {
		fmt.Fprintln(os.Stderr, msg)
	}
	if results.Error {
		return nil, fmt.Errorf("errors occurred during compilation")
	}

	sourcePkgDirs := map[string]string{}
	for _, desc := range results.AllDescriptors {
		uri := results.FileURIsByPath[desc.Path()]
		if uri.IsFile() {
			sourcePkgDirs[filepath.Dir(desc.Path())] = filepath.Dir(uri.Filename())
		}
		// fix up any incomplete go_package options if we have the info available
		// this will transform e.g. `go_package = "bar"` to `go_package = "github.com/foo/bar"`
		goPackage := desc.Options().(*descriptorpb.FileOptions).GetGoPackage()
		if !strings.Contains(goPackage, ".") && !strings.Contains(goPackage, "/") {
			p := path.Dir(desc.Path())
			if strings.HasSuffix(p, goPackage) {
				*desc.Options().(*descriptorpb.FileOptions).GoPackage = p
			}
		}
	}

	toGenerate := []string{}
	for _, wd := range results.WorkspaceLocalDescriptors {
		relPath := wd.Path()
		for _, source := range sources {
			if ok, _ := doublestar.Match(source, relPath); ok {
				toGenerate = append(toGenerate, relPath)
				break
			}
		}
	}

	codeGeneratorRequest := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: toGenerate,
		ProtoFile:      results.AllDescriptorProtos,
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

	var outputs []*GeneratedFile
	for _, f := range response.GetFile() {
		pkg, name := filepath.Split(f.GetName())
		pkg = strings.TrimSuffix(pkg, "/")
		dir, ok := sourcePkgDirs[pkg]
		if !ok {
			if strings.Contains(pkg, "google/") {
				dir = pkg[strings.Index(pkg, "google/"):]
			} else {
				dir = pkg
			}
		}
		relPath := path.Join(dir, name)
		outputs = append(outputs, &GeneratedFile{
			Name:          name,
			Package:       pkg,
			SourceRelPath: relPath,
			Content:       f.GetContent(),
		})
	}

	return outputs, nil
}
