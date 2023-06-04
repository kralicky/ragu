package external

import (
	"fmt"
	"io"
	"os/exec"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func NewGenerator(pluginPath string, opt string) *extGenerator {
	return &extGenerator{
		pluginPath: pluginPath,
		option:     opt,
	}
}

type extGenerator struct {
	pluginPath string
	option     string
}

func (g *extGenerator) Name() string {
	return "x-" + path.Base(g.pluginPath)
}

func (g *extGenerator) Generate(gen *protogen.Plugin) error {
	cmd := exec.Command(g.pluginPath)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	reqClone := proto.Clone(gen.Request).(*pluginpb.CodeGeneratorRequest)
	if g.option != "" {
		reqClone.Parameter = &g.option
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
		return err
	}

	response := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(responseWire, response); err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("plugin error: %s", response.GetError())
	}

	for i, f := range response.File {
		if f.GetName() == "" {
			continue
		}
		gen.NewGeneratedFile(f.GetName(), gen.Files[i].GoImportPath).Write([]byte(f.GetContent()))
	}

	return nil
}
