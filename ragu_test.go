package ragu_test

import (
	"testing"

	ragu2 "github.com/kralicky/ragu"
)

func TestGenerateCode(t *testing.T) {
	out, err := ragu2.GenerateCode(ragu2.AllGenerators(), "**/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if err := f.WriteToDisk(); err != nil {
			t.Fatal(err)
		}
	}
}
