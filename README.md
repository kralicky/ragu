# ragù 🍝

ragu allows you to generate Go and gRPC protobuf code without using protoc.
It does not require any non-go dependencies **and does not shell out to other binaries.**

## Features

- Provides both a CLI and a Go library to generate code (use it with [mage](https://github.com/magefile/mage)!)
- Doesn't require protoc, other binaries, or cgo; it is completely self-contained
- Can optionally generate gRPC bindings
- Generates the same output as protoc
- Removes unnecessary output directory configuration. Files are generated into
the current directory by default, or into a directory specified with the `-o` flag.

## Instructions (CLI)

1. Download the CLI binary from the [releases page](https://github.com/kralicky/ragu/releases)
2. `$ ragu yourtypes.proto` (generates `./yourtypes.pb.go`, `./yourtypes_grpc.pb.go`) 
3. Yep, that's it.

## Instructions (Go)

1. `$ go get github.com/kralicky/ragu`
2. Generate code:

```go
files, err := ragu.GenerateCode("yourtypes.proto", true) // true = generate gRPC bindings
if err != nil {
  return err
}
for _, f := range files {
  // f will contain the generated code and file basename, e.g. yourtypes.pb.go
}
```

------

## FAQ

### How does it work?

Ragu works by parsing .proto files and building the same CodeGeneratorRequest
that protoc sends to plugins. Both the Go and gRPC protobuf generator packages 
are imported by ragu and used as libraries to simulate the plugins being called 
with the CodeGeneratorRequest that would normally be read via stdin when they are
invoked as external binaries. The result is a single static binary (and Go library) 
that is a drop-in replacement for `protoc`+`protoc-gen-go`+`protoc-gen-go-grpc`.

### What versions of protobuf, gRPC, etc. libraries does this require?

No idea, it's best not to think about it too much. Ragu uses `google.golang.org/protobuf v1.27.1` and v1.1.0 of the gRPC code generator.

### Are there differences between protoc and ragu generated code?

- The ragu version used will be patched into the comment header to replace the
unused protoc version

```diff
 // Code generated by protoc-gen-go-grpc. DO NOT EDIT.
 // versions:
 // - protoc-gen-go-grpc v1.1.0
-// - protoc             (unknown)
+// - ragù               v0.1.0
 // source: pkg/types/types.proto
```

- Comments in .proto files are currently being ignored and will not show up in
generated code. This will be added in a future version.

Otherwise, the generated code should be exactly identical. If you find any errors
in ragu's generated code, please submit a bug report!

### Is ragu as fast as protoc?

I made no attempt whatsoever to optimize this code, but it's pretty much the same as far as I can tell.

### Why the name?

This is left as an exercise to the reader.
