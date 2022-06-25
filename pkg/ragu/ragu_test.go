package ragu_test

import (
	"os"
	"testing"

	"github.com/kralicky/ragu/v2/pkg/ragu"
)

func TestGenerateCode(t *testing.T) {
	out, err := ragu.GenerateCode(
		ragu.FromFile("testdata/pkg1/test_1.proto"),
		ragu.FromFile("testdata/pkg1/test_2.proto"),
		ragu.FromFile("testdata/pkg2/test_3.proto"),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		os.WriteFile("testdata/"+f.GetName(), []byte(f.GetContent()), 0644)
	}
}
