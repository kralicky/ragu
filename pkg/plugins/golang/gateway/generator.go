package gateway

import (
	"fmt"
	"strings"

	"github.com/kralicky/grpc-gateway/v2/pkg/codegenerator"
	"github.com/kralicky/grpc-gateway/v2/pkg/descriptor"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-grpc-gateway/pkg/gengateway"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/pkg/genopenapi"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
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

	gatewayTargets := []*descriptor.File{}
	openapiTargets := []*descriptor.File{}

	for _, target := range gen.Request.FileToGenerate {
		f, err := reg.LookupFile(target)
		if err != nil {
			return err
		}
		gatewayTargets = append(gatewayTargets, f)
		if proto.HasExtension(f.GetOptions(), options.E_Openapiv2Swagger) {
			openapiTargets = append(openapiTargets, f)
		}
	}

	if len(gatewayTargets) > 0 {
		files, err := generator.Generate(gatewayTargets)
		if err != nil {
			return err
		}
		for _, f := range files {
			genFile := gen.NewGeneratedFile(f.GetName(), protogen.GoImportPath(f.GoPkg.Path))
			if _, err := genFile.Write([]byte(f.GetContent())); err != nil {
				return err
			}
		}
	}

	if len(openapiTargets) > 0 {
		g := genopenapi.New(reg, genopenapi.FormatJSON)
		out, err := g.Generate(openapiTargets)
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
