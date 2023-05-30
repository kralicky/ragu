package ragu

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
	"github.com/jhump/protoreflect/desc"
	"github.com/kralicky/ragu/pkg/util"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
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

var commonMissingImports = map[string]string{
	"unknown extension google.api.http": "google/api/annotations.proto",
	"no message found: Status":          "google/rpc/status.proto",
	"unknown extension grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger": "github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/options/annotations.proto",
}

// Generates code for each source file (or files matching a glob pattern)
// using one or more code generators.
func GenerateCode(generators []Generator, sources ...string) (_ []*GeneratedFile, generateCodeErr error) {
	defer func() {
		if generateCodeErr == nil {
			return
		}
		msg := generateCodeErr.Error()
		for str, imp := range commonMissingImports {
			if strings.Contains(msg, str) {
				generateCodeErr = fmt.Errorf("%w (try importing %s)", generateCodeErr, imp)
			}
		}
	}()

	if resolved, err := ResolvePatterns(sources); err != nil {
		return nil, err
	} else {
		sources = resolved
	}

	sourcePackages := map[string]string{}
	sourcePkgDirs := map[string]string{}
	for _, source := range sources {
		goPkg, err := FastLookupGoModule(source)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup go module for %s: %w", source, err)
		}
		sourcePkgDirs[goPkg] = filepath.Dir(source)
		sourcePackages[path.Join(goPkg, path.Base(source))] = source
	}

	sourceDescriptors, err := ParseFiles(SourceAccessor(sourcePackages), lo.Keys(sourcePackages)...)
	if err != nil {
		return nil, err
	}
	allDescriptors := desc.ToFileDescriptorSet(sourceDescriptors...).File

	for _, desc := range allDescriptors {
		// fix up any incomplete go_package options if we have the info available
		// this will transform e.g. `go_package = "bar"` to `go_package = "github.com/foo/bar"`
		goPackage := desc.GetOptions().GetGoPackage()
		if !strings.Contains(goPackage, ".") && !strings.Contains(goPackage, "/") {
			p := path.Dir(desc.GetName())
			if strings.HasSuffix(p, goPackage) {
				*desc.Options.GoPackage = p
			}
		}
	}

	codeGeneratorRequest := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: util.Map(sourceDescriptors, (*desc.FileDescriptor).GetName),
		ProtoFile:      allDescriptors,
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
			return nil, fmt.Errorf("bug: failed to find source package %q in list %v", pkg, lo.Keys(sourcePkgDirs))
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

func ResolvePatterns(sources []string) ([]string, error) {
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

func FastLookupGoModule(filename string) (string, error) {
	// Search the .proto file for `option go_package = "...";`
	// We know this will be somewhere at the top of the file.
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, "option") {
			continue
		}
		index := strings.Index(line, "go_package")
		if index == -1 {
			continue
		}
		for ; index < len(line); index++ {
			if line[index] == '=' {
				break
			}
		}
		for ; index < len(line); index++ {
			if line[index] == '"' {
				break
			}
		}
		if index == len(line) {
			continue
		}
		startIdx := index + 1
		endIdx := strings.LastIndexByte(line, '"')
		if endIdx <= startIdx {
			continue
		}
		return line[startIdx:endIdx], nil
	}
	return "", fmt.Errorf("no go_package option found")
}
