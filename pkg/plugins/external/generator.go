package external

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

type GeneratorOptions struct {
	Opt                       string
	CodeGeneratorRequestHook  func(*pluginpb.CodeGeneratorRequest)
	CodeGeneratorResponseHook func(*pluginpb.CodeGeneratorResponse)
}

func NewGenerator(pluginPath string, opts GeneratorOptions) *extGenerator {
	return &extGenerator{
		GeneratorOptions: opts,
		pluginPath:       pluginPath,
	}
}

type extGenerator struct {
	GeneratorOptions
	pluginPath string
}

func (g *extGenerator) Name() string {
	return "x-" + path.Base(g.pluginPath)
}

func (g *extGenerator) Generate(gen *protogen.Plugin) error {
	cmd := exec.Command(g.pluginPath)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr

	reqClone := proto.Clone(gen.Request).(*pluginpb.CodeGeneratorRequest)
	if g.Opt != "" {
		reqClone.Parameter = &g.Opt
	}
	if g.CodeGeneratorRequestHook != nil {
		g.CodeGeneratorRequestHook(reqClone)
	}

	requestWire, err := proto.Marshal(reqClone)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write(requestWire); err != nil {
		return err
	}

	if err := stdin.Close(); err != nil {
		return err
	}

	responseWire, err := io.ReadAll(stdout)
	if err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("plugin error: %w", err)
	}

	response := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(responseWire, response); err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("plugin error: %s", response.GetError())
	}

	if g.CodeGeneratorResponseHook != nil {
		g.CodeGeneratorResponseHook(response)
	}
	for i, f := range response.File {
		if f.GetName() == "" {
			continue
		}
		gen.NewGeneratedFile(f.GetName(), gen.Files[i].GoImportPath).Write([]byte(f.GetContent()))
	}

	return nil
}
