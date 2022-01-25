//go:build mage

package main

import (
	"path"
	"path/filepath"
	"sync"

	"emperror.dev/errors"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = All

func All() {
	mg.SerialDeps(Vendor, Build)
}

var (
	protoDownloadURL   = "https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf"
	grpcGenDownloadURL = "https://raw.githubusercontent.com/grpc/grpc-go/master/cmd/protoc-gen-go-grpc/grpc.go"
)

func download(url string, target string) error {
	return sh.Run("curl", "-sfL", url, "-o", target)
}

func Vendor() (vendorErr error) {
	wellKnown := []string{
		"any.proto",
		"api.proto",
		"descriptor.proto",
		"duration.proto",
		"empty.proto",
		"field_mask.proto",
		"source_context.proto",
		"struct.proto",
		"timestamp.proto",
		"type.proto",
		"wrappers.proto",
	}

	wg := sync.WaitGroup{}
	for _, filename := range wellKnown {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			if err := download(
				path.Join(protoDownloadURL, filename),
				filepath.Join("pkg/machinery/google/protobuf", filename),
			); err != nil {
				vendorErr = errors.Append(vendorErr, err)
			}
		}(filename)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := download(
			grpcGenDownloadURL,
			"pkg/ragu/upstream_grpc.go",
		); err != nil {
			vendorErr = errors.Append(vendorErr, err)
		}
		sh.Run("sed", "-i",
			"s/package main/package ragu/",
			"pkg/ragu/upstream_grpc.go",
		)
	}()
	wg.Wait()
	return
}

func Build() error {
	return sh.RunWith(map[string]string{
		"CGO_ENABLED": "0",
	}, "go", "build", "-ldflags", `-w -s`, ".")
}
