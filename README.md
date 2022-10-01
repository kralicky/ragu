# ragù 🍝

ragù is a pure-go protobuf code generator library.

## Features

- Doesn't require protoc, other binaries, or cgo; it is completely self-contained.
- Can generate GRPC bindings, GRPC Gateway bindings, and Swagger definitions.
- Can generate Python bindings equivalent to python-betterproto
- Import proto files from project dependencies using go module paths (with gogoproto and Kubernetes compatibility!)

## How to use

Ragu is designed to be used as a Go library, as part of your build process. I recommend using [mage](https://github.com/magefile/mage) or a similar tool.

1. Import ragu: `go get github.com/kralicky/ragu@latest`
2. Generate code by calling `ragu.GenerateCode()`:

```go
files, err := ragu.GenerateCode(ragu.DefaultGenerators(), "**/*.proto")
if err != nil {
  return err
}
for _, f := range files {
  if err := file.WriteToDisk(); err != nil {
    return err
  }
}
```

## `go_package` and imports

The `go_package` file option is used to determine the import path of your protobuf definitions, as well as the default path of generated files.
It should be set to the full import path of the package where the generated code will be placed, similar to how Go modules work.

For example, given the following files:

```
pkg/
  foo/
    foo.proto
  bar/
    bar.proto
```

```protobuf
// pkg/foo/foo.proto
syntax = "proto3";
option go_package = "github.com/username/project/pkg/foo";

package foo;

message Foo {
  string str = 1;
}
```

```protobuf
// pkg/bar/bar.proto
syntax = "proto3";
option go_package = "github.com/username/project/pkg/bar";

import "github.com/username/project/pkg/foo/foo.proto"; // ragu-style import path
import "google/protobuf/empty.proto"                    // standard import path

package bar;

service Bar {
  rpc Test(foo.Foo) returns (google.protobuf.Empty);
}
```

`ragu.GenerateCode()` will generate the following new files:

```diff
pkg/
  foo/
    foo.proto
+   foo.pb.go
  bar/
    bar.proto
+   bar.pb.go
+   bar_grpc.pb.go
```

## GRPC Gateway

grpc-gateway is supported by default, and will generate code if the `google.api.http` option is set on a service method.

```protobuf
// pkg/baz/baz.proto
syntax = "proto3";
option go_package = "github.com/username/project/pkg/baz";

import "google/api/annotations.proto";
import "github.com/username/project/pkg/foo/foo.proto";

package baz;

service Baz {
  rpc Test(foo.Foo) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      post: "/test"
      body: "*"
    };
  }
}
```

`ragu.GenerateCode()` will generate the following new files:

```diff
pkg/
  foo/
    foo.proto
    foo.pb.go
  bar/
    bar.proto
    bar.pb.go
    bar_grpc.pb.go
  baz/
    baz.proto
+   baz.pb.go
+   baz_grpc.pb.go
+   baz.pb.gw.go
```

### Swagger definitions

Swagger definitions are generated only if the openapiv2_swagger file option is set when using grpc-gateway.

```protobuf
// pkg/baz/baz.proto
syntax = "proto3";
option go_package = "github.com/username/project/pkg/baz";

import "google/api/annotations.proto";
import "github.com/username/project/pkg/foo/foo.proto";
import "github.com/kralicky/grpc-gateway/v2/protoc-gen-openapiv2/options/annotations.proto";

package baz;

option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: {
    title: "Baz";
    version: "1.0";
    license: {
      name: "Apache 2.0";
      url: "https://github.com/rancher/opni/blob/main/LICENSE";
    };
  };
};

service Baz {
  rpc Test(foo.Foo) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      post: "/test"
      body: "*"
    };
  }
}
```

`ragu.GenerateCode()` will generate the following new files:

```diff
pkg/
  foo/
    foo.proto
    foo.pb.go
  bar/
    bar.proto
    bar.pb.go
    bar_grpc.pb.go
  baz/
    baz.proto
    baz.pb.go
    baz_grpc.pb.go
    baz.pb.gw.go
+   baz.swagger.json
```

## gogoproto compatibility

You can import gogoproto-generated protobuf definitions as follows:

```protobuf
import "github.com/cockroachdb/errors/errorspb/errors.proto";
import "k8s.io/api/core/v1/generated.proto";

message Foo {
  k8s.io.api.core.v1.Pod pod = 1;
  cockroach.errorspb.EncodedError err = 2;
}
```
### Notes:
- Any imported packages must exist in your `go.mod` file.
- If the imported protobuf code imports `gogoproto/gogo.proto`, you must import `github.com/kralicky/ragu/compat` where `ragu.GenerateCode()` is called.
- If you need to use the file descriptors from imported gogoproto definitions at runtime, call `compat.LoadGogoFileDescriptor()` to add the descriptor to `protoregistry.GlobalFiles`.

```go
import "github.com/kralicky/ragu/compat"
func init() {
	compat.LoadGogoFileDescriptor("k8s.io/api/core/v1/generated.proto")
}
