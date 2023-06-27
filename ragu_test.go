package ragu_test

import (
	"testing"

	"github.com/kralicky/ragu"
	"github.com/kralicky/ragu/pkg/plugins/external"
	"github.com/kralicky/ragu/pkg/plugins/golang"
)

func TestGenerateCode(t *testing.T) {
	out, err := ragu.GenerateCode(ragu.AllGenerators(), "**/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if err := f.WriteToDisk(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExternal(t *testing.T) {
	out, err := ragu.GenerateCode([]ragu.Generator{
		golang.Generator,
		external.NewGenerator([]string{"npx", "protoc-gen-es"}, external.GeneratorOptions{Opt: "target=ts"}),
	}, "**/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if err := f.WriteToDisk(); err != nil {
			t.Fatal(err)
		}
	}

	out, err = ragu.GenerateCode([]ragu.Generator{
		golang.Generator,
		external.NewGenerator("protoc-gen-es", external.GeneratorOptions{Opt: "target=ts"}),
	}, "**/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if err := f.WriteToDisk(); err != nil {
			t.Fatal(err)
		}
	}
}
