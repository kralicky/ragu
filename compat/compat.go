package compat

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	_ "github.com/gogo/protobuf/gogoproto"
	gproto "github.com/gogo/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const k8sVendorPrefix = "k8s.io/kubernetes/vendor/"

func init() {
	gproto.RegisterFile("github.com/gogo/protobuf/gogoproto/gogo.proto", gproto.FileDescriptor("gogo.proto"))
	LoadGogoFileDescriptor("github.com/gogo/protobuf/gogoproto/gogo.proto")
	gproto.RegisterFile("gogoproto/gogo.proto", gproto.FileDescriptor("gogo.proto"))
	LoadGogoFileDescriptor("gogoproto/gogo.proto")
}

var (
	alreadyLoadedFilesMu sync.Mutex
	alreadyLoadedFiles   = map[string]struct{}{}
)

type CompatLoadOptions struct {
	rename string
}

type CompatLoadOption func(*CompatLoadOptions)

func (o *CompatLoadOptions) apply(opts ...CompatLoadOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithRename(importPath string) CompatLoadOption {
	return func(o *CompatLoadOptions) {
		o.rename = importPath
	}
}

func LoadGogoFileDescriptor(filename string, opts ...CompatLoadOption) {
	alreadyLoadedFilesMu.Lock()
	options := CompatLoadOptions{}
	options.apply(opts...)

	key := filename
	if options.rename != "" {
		key = options.rename
	}
	defer alreadyLoadedFilesMu.Unlock()
	if _, ok := alreadyLoadedFiles[key]; ok {
		return
	}

	alreadyLoadedFiles[key] = struct{}{}

	fileDescs := createGogoFileDescWithDeps(createOptions{
		filename: filename,
		rename:   options.rename,
		seen:     map[string]*descriptorpb.FileDescriptorProto{},
	})
	descriptors, err := desc.CreateFileDescriptors(fileDescs)
	if err != nil {
		panic(err)
	}
	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{},
	}
	for _, v := range descriptors {
		dp := v.AsFileDescriptorProto()
		if *dp.Name == "gogo.proto" {
			*dp.Name = "github.com/gogo/protobuf/gogoproto/gogo.proto"
		}
		set.File = append(set.File, dp)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		panic(err)
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if _, err := protoregistry.GlobalFiles.FindFileByPath(fd.Path()); errors.Is(err, protoregistry.NotFound) {
			if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
				panic(err)
			}
		}
		return true
	})
}

type createOptions struct {
	filename string
	rename   string
	seen     map[string]*dpb.FileDescriptorProto
}

func createGogoFileDescWithDeps(o createOptions) []*dpb.FileDescriptorProto {
	filename := o.filename
	if strings.HasPrefix(filename, "k8s.io") && !strings.HasPrefix(filename, k8sVendorPrefix) {
		filename = k8sVendorPrefix + filename
	}
	if _, ok := o.seen[filename]; ok {
		return []*dpb.FileDescriptorProto{}
	}
	var fileDesc *dpb.FileDescriptorProto
	fn := filename
	if fn == "github.com/gogo/protobuf/gogoproto/gogo.proto" || fn == "gogoproto/gogo.proto" {
		fn = "gogo.proto"
	}
	if raw := gproto.FileDescriptor(fn); raw != nil {
		fd, err := DecodeFileDescriptor("file", raw)
		if err != nil {
			panic(err)
		}
		if o.rename != "" {
			fd.Name = &o.rename
		}
		if fn == "gogo.proto" {
			*fd.Name = "github.com/gogo/protobuf/gogoproto/gogo.proto"
		} else if strings.HasPrefix(fn, k8sVendorPrefix) {
			*fd.Name = strings.TrimPrefix(fn, k8sVendorPrefix)
		}
		fileDesc = fd
	} else if fd, err := desc.LoadFileDescriptor(fn); err == nil {
		fileDesc = fd.AsFileDescriptorProto()
	} else {
		panic("failed to load file descriptor: " + fn)
	}

	var fileDescs []*dpb.FileDescriptorProto
	o.seen[filename] = fileDesc
	for i, dep := range fileDesc.Dependency {
		if dep == "gogo.proto" || dep == "gogoproto/gogo.proto" {
			fileDesc.Dependency[i] = "github.com/gogo/protobuf/gogoproto/gogo.proto"
		}
		if _, ok := o.seen[dep]; !ok {
			fileDescs = append(fileDescs, createGogoFileDescWithDeps(createOptions{
				filename: dep,
				seen:     o.seen,
			})...)
		}
	}
	fileDescs = append(fileDescs, fileDesc)
	return fileDescs
}

// Internal code from:
// https://github.com/jhump/protoreflect/blob/master/internal/standard_files.go#L101

// DecodeFileDescriptor decodes the bytes of a registered file descriptor.
// Registered file descriptors are first "proto encoded" (e.g. binary format
// for the descriptor protos) and then gzipped. So this function gunzips and
// then unmarshals into a descriptor proto.
func DecodeFileDescriptor(element string, fdb []byte) (*dpb.FileDescriptorProto, error) {
	raw, err := decompress(fdb)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress %q descriptor: %v", element, err)
	}
	fd := dpb.FileDescriptorProto{}
	if err := proto.Unmarshal(raw, &fd); err != nil {
		return nil, fmt.Errorf("bad descriptor for %q: %v", element, err)
	}
	return &fd, nil
}

func decompress(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("bad gzipped descriptor: %v", err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("bad gzipped descriptor: %v", err)
	}
	return out, nil
}
