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
		ragu.FromFile("testdata/grpc1/grpc_1.proto"),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if err := os.WriteFile(f.GetName(), []byte(f.GetContent()), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
