package python

import (
	_ "embed"
	"encoding/json"
	"path"

	"github.com/flosch/pongo2/v6"
	"github.com/jhump/protoreflect/desc"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
)

//go:embed template.py.j2
var template []byte

var Generator = generator{}

type generator struct{}

func (generator) Name() string {
	return "python"
}

func (generator) Generate(gen *protogen.Plugin) error {
	tpl, err := pongo2.FromBytes(template)
	if err != nil {
		return err
	}
	dirs := map[string]struct{}{}
	for _, f := range gen.Files {
		if f.Generate {
			dirs[path.Dir(f.GeneratedFilenamePrefix)] = struct{}{}
			filename := f.GeneratedFilenamePrefix + "_pb.py"
			g := gen.NewGeneratedFile(filename, "")

			fd, err := desc.CreateFileDescriptors(gen.Request.ProtoFile)
			if err != nil {
				return err
			}

			model := buildModel(fd[f.Proto.GetName()], lo.Values(fd))
			jsonData, err := json.Marshal(model)
			if err != nil {
				return err
			}
			modelMap := map[string]any{}
			err = json.Unmarshal(jsonData, &modelMap)
			if err != nil {
				return err
			}

			data, err := tpl.ExecuteBytes(modelMap)
			if err != nil {
				return err
			}

			if _, err := g.Write(data); err != nil {
				return err
			}
		}
	}
	for dir := range dirs {
		gen.NewGeneratedFile(path.Join(dir, "__init__.py"), "")
	}

	return nil
}
