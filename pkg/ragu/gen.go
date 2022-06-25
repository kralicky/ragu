package ragu

type CodegenOptions struct {
	// Enables an experimental feature that will omit parameters and return values
	// of type "emptypb.Empty" from generated code. Enables more natural use of
	// gRPC methods that accept no arguments and/or return no values
	// (other than error).
	HideEmptyMessages bool
}

type Generator struct {
	sources []Source
}

func NewGenerator(sources ...Source) *Generator {
	return &Generator{sources: sources}
}
