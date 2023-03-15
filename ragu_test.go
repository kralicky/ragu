package ragu_test

import (
	"testing"

	"github.com/kralicky/ragu"
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
