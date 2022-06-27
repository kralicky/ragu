package gateway

import (
	"fmt"
	"strings"

	"github.com/kralicky/grpc-gateway/v2/pkg/codegenerator"
	"github.com/kralicky/grpc-gateway/v2/pkg/descriptor"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-grpc-gateway/pkg/gengateway"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/pkg/genopenapi"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

var Generator = generator{}

type generator struct{}

func (generator) Name() string {
	return "go-grpc-gateway"
}

func (generator) Generate(gen *protogen.Plugin) error {
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
		out, err := g.Generate(targets)
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
