package ragu

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go/build"

	"golang.org/x/mod/module"

	"github.com/kralicky/grpc-gateway/v2/pkg/codegenerator"
	"github.com/kralicky/grpc-gateway/v2/pkg/descriptor"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-grpc-gateway/pkg/gengateway"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/pkg/genopenapi"
	"github.com/kralicky/ragu/internal/pointer"
	"github.com/kralicky/ragu/pkg/machinery"
	"github.com/kralicky/ragu/pkg/ragu/custom"
	"github.com/yoheimuta/go-protoparser/v4"
	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
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

type File = pluginpb.CodeGeneratorResponse_File

// Computes the path of dep relative to source, and returns a path relative to
// the current working directory, as such:
// (path/to/foo.bar, baz.bar) => path/to/baz.bar
// (path/to/foo.bar, ../baz.bar) => path/baz.bar
// (path/to/foo.bar, foo/baz.bar) => path/to/foo/baz.bar
func relativeDependencyPath(source, dep string) string {
	return filepath.Join(filepath.Dir(source), dep)
}

func resolveDependencies(desc *descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	deps := []*descriptorpb.FileDescriptorProto{}
	for i, dep := range desc.Dependency {
		rel := relativeDependencyPath(desc.GetName(), dep)
		// Check if file exists
		if _, err := os.Stat(rel); err == nil {
			// File exists
			proto, err := machinery.ParseProto(rel)
			if err != nil {
				log.Fatal(err)
			}
			if rel != dep {
				desc.Dependency[i] = rel
			}
			importedDesc := machinery.GenerateDescriptor(proto)
			deps = append(deps, resolveDependencies(importedDesc)...)
		} else if wk, err := machinery.ReadWellKnownType(dep); err == nil {
			// File does not exist, but is a well-known type
			reader := strings.NewReader(wk)
			proto, err := protoparser.Parse(reader, protoparser.WithFilename(dep))
			if err != nil {
				panic(err)
			}
			importedDesc := machinery.GenerateDescriptor(proto)
			deps = append(deps, resolveDependencies(importedDesc)...)
		} else if strings.HasPrefix(dep, "google/") {
			log.Println("Downloading dependency", dep)
			// download from the googleapis repo
			baseUrl := "https://raw.githubusercontent.com/googleapis/googleapis/master/"
			protoUrl := baseUrl + dep
			resp, err := http.Get(protoUrl)
			if err != nil {
				log.Fatalf("failed to download %s: %v", protoUrl, err)
			}
			defer resp.Body.Close()
			proto, err := protoparser.Parse(resp.Body, protoparser.WithFilename(dep))
			if err != nil {
				log.Fatalf("failed to parse %s: %v", protoUrl, err)
			}
			importedDesc := machinery.GenerateDescriptor(proto)
			deps = append(deps, resolveDependencies(importedDesc)...)
		} else {
			// Import from go module
			last := strings.LastIndex(dep, "/")
			if last != -1 {
				filename := dep[last+1:]
				if strings.HasSuffix(filename, ".proto") {
					modulePath := dep[:last]
					if err := module.CheckImportPath(modulePath); err == nil {
						// check module cache for the file
						var protoFilePath string
						pkg, err := build.Default.Import(modulePath, "", 0)
						if err != nil {
							cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modulePath)
							cmd.Env = append(os.Environ(),
								"GOOS="+runtime.GOOS,
								"GOARCH="+runtime.GOARCH,
								"GOROOT="+runtime.GOROOT(),
							)
							if out, err := cmd.Output(); err != nil {
								log.Fatalf("go/build: go list %s: %v\n%s\n", modulePath, err, err.Error())
							} else {
								protoFilePath = filepath.Join(strings.TrimSpace(string(out)), filename)
							}
						} else {
							protoFilePath = filepath.Join(pkg.Dir, filename)
						}
						// Check if proto file exists
						if _, err := os.Stat(protoFilePath); err == nil {
							// File exists
							proto, err := machinery.ParseProto(protoFilePath)
							if err != nil {
								log.Fatal(err)
							}
							desc.Dependency[i] = proto.Meta.Filename
							moduleDesc := machinery.GenerateDescriptor(proto)
							moduleDesc.Options.GoPackage = pointer.String(pkg.ImportPath)
							deps = append(deps, resolveDependencies(moduleDesc)...)
							continue
						} else {
							// File does not exist
							log.Fatalf("file not found in module cache: %s", protoFilePath)
						}
					}
				}
			}
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

type GenerateCodeOptions struct {
	experimentalHideEmptyMessages bool
}

type GenerateCodeOption func(*GenerateCodeOptions)

func (o *GenerateCodeOptions) Apply(opts ...GenerateCodeOption) {
	for _, op := range opts {
		op(o)
	}
}

// Enables an experimental feature that will omit parameters and return values
// of type "emptypb.Empty" from generated code. Enables more natural use of
// gRPC methods that accept no arguments and/or return no values
// (other than error).
func ExperimentalHideEmptyMessages() GenerateCodeOption {
	return func(o *GenerateCodeOptions) {
		o.experimentalHideEmptyMessages = true
	}
}

func GenerateCode(input string, raguOpts ...GenerateCodeOption) ([]*File, error) {
	raguOptions := &GenerateCodeOptions{}
	raguOptions.Apply(raguOpts...)

	proto, err := machinery.ParseProto(input)
	if err != nil {
		return nil, err
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
			Minor: pointer.Int32(2),
			Patch: pointer.Int32(3),
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
			if raguOptions.experimentalHideEmptyMessages {
				custom.GenerateFile(plugin, f)
			} else {
				generateFile(plugin, f)
			}
		}
	}
	if err := genGrpcGateway(plugin); err != nil {
		return nil, err
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

func genGrpcGateway(gen *protogen.Plugin) error {
	reg := descriptor.NewRegistry()

	codegenerator.SetSupportedFeaturesOnPluginGen(gen)

	generator := gengateway.New(reg, true, "Handler", false, false)

	if err := reg.LoadFromPlugin(gen); err != nil {
		return err
	}

	unboundHTTPRules := reg.UnboundExternalHTTPRules()
	if len(unboundHTTPRules) != 0 {
		return fmt.Errorf("HTTP rules without a matching selector: %s", strings.Join(unboundHTTPRules, ", "))
	}

	targets := make([]*descriptor.File, len(gen.Request.FileToGenerate))
	for i, target := range gen.Request.FileToGenerate {
		f, err := reg.LookupFile(target)
		if err != nil {
			return err
		}
		f.SourceCodeInfo = &descriptorpb.SourceCodeInfo{}
		targets[i] = f
	}

	files, err := generator.Generate(targets)
	if err != nil {
		return err
	}
	for _, f := range files {
		genFile := gen.NewGeneratedFile(f.GetName(), protogen.GoImportPath(f.GoPkg.Path))
		if _, err := genFile.Write([]byte(f.GetContent())); err != nil {
			return err
		}
	}

	if len(files) > 0 {
		g := genopenapi.New(reg)
		// silence warning about missing sourcecodeinfo, which is the only instance
		// of fmt.Fprintln in Generate
		oldStdErr := os.Stderr
		os.Stderr, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		out, err := g.Generate(targets)
		os.Stderr = oldStdErr
		if err != nil {
			return err
		}
		for _, f := range out {
			genFile := gen.NewGeneratedFile(f.GetName(), protogen.GoImportPath(f.GoPkg.Path))
			if _, err := genFile.Write([]byte(f.GetContent())); err != nil {
				return err
			}
		}
	}
	return nil
}
