# ragù 🍝

ragu allows you to generate Go and GRPC protobuf code without using protoc.
ragu does not require any non-go dependencies **and does not shell out to other binaries.**

## Features

- Provides both a CLI and a Go library to generate code
- Doesn't require protoc or other binaries; it is completely self-contained
- Can optionally generate GRPC bindings
- Handles well-known types

## Instructions (CLI)
1. Download the CLI binary from the [releases page](https://github.com/kralicky/ragu/releases)
2. `$ ragu yourtypes.proto` (generates `./yourtypes.pb.go`, `./yourtypes_grpc.pb.go`) 
3. Yep, that's it.

## Instructions (Go)
1. `$ go get github.com/kralicky/ragu`
2. Generate code:
```go
files, err := ragu.GenerateCode("yourtypes.proto", true) // true = generate GRPC bindings
if err != nil {
  return err
}
for _, f := range files {
  // f will contain the generated code and filename 
}
```
