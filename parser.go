package ragu

import (
	"context"
	"io"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/walk"
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type FileAccessor = func(path string) (io.ReadCloser, error)

func NewResolver(accessor FileAccessor) protocompile.Resolver {
	return protocompile.CompositeResolver{
		&protocompile.SourceResolver{
			Accessor: accessor,
		},
		protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			fd, err := desc.LoadFileDescriptor(path)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Desc: fd.UnwrapFile()}, nil
		}),
	}
}

func ParseFiles(accessor FileAccessor, filenames ...string) ([]*desc.FileDescriptor, error) {
	res := NewResolver(accessor)
	c := protocompile.Compiler{
		Resolver:       res,
		MaxParallelism: -1,
		SourceInfoMode: protocompile.SourceInfoExtraComments,
		Reporter:       reporter.NewReporter(nil, nil),
	}
	results, err := c.Compile(context.Background(), filenames...)
	if err != nil {
		return nil, err
	}

	fds := make([]protoreflect.FileDescriptor, len(results))
	for i, result := range results {
		if linkRes, ok := result.(linker.Result); ok {
			removeDynamicExtensions(linkRes.FileDescriptorProto())
		}
		fds[i] = results[i]
	}
	return desc.WrapFiles(fds)
}

// =================================================================
// unexported code from protoreflect/desc/protoparse/parser.go below
// =================================================================

func removeDynamicExtensions(fd *descriptorpb.FileDescriptorProto) {
	// protocompile returns descriptors with dynamic extension fields for custom options.
	// But protoparse only used known custom options and everything else defined in the
	// sources would be stored as unrecognized fields. So to bridge the difference in
	// behavior, we need to remove custom options from the given file and add them back
	// via serializing-then-de-serializing them back into the options messages. That way,
	// statically known options will be properly typed and others will be unrecognized.
	//
	// This is best effort. So if an error occurs, we'll still return a result, but it
	// may include a dynamic extension.
	fd.Options = removeDynamicExtensionsFromOptions(fd.Options)
	_ = walk.DescriptorProtos(fd, func(_ protoreflect.FullName, msg proto.Message) error {
		switch msg := msg.(type) {
		case *descriptorpb.DescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
			for _, extr := range msg.ExtensionRange {
				extr.Options = removeDynamicExtensionsFromOptions(extr.Options)
			}
		case *descriptorpb.FieldDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.OneofDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumValueDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.ServiceDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.MethodDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		}
		return nil
	})
}

type ptrMsg[T any] interface {
	*T
	proto.Message
}

type fieldValue struct {
	fd  protoreflect.FieldDescriptor
	val protoreflect.Value
}

func removeDynamicExtensionsFromOptions[O ptrMsg[T], T any](opts O) O {
	if opts == nil {
		return nil
	}
	var dynamicExtensions []fieldValue
	opts.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if fd.IsExtension() {
			dynamicExtensions = append(dynamicExtensions, fieldValue{fd: fd, val: val})
		}
		return true
	})

	// serialize only these custom options
	optsWithOnlyDyn := opts.ProtoReflect().Type().New()
	for _, fv := range dynamicExtensions {
		optsWithOnlyDyn.Set(fv.fd, fv.val)
	}
	data, err := proto.MarshalOptions{AllowPartial: true}.Marshal(optsWithOnlyDyn.Interface())
	if err != nil {
		// oh, well... can't fix this one
		return opts
	}

	// and then replace values by clearing these custom options and deserializing
	optsClone := proto.Clone(opts).ProtoReflect()
	for _, fv := range dynamicExtensions {
		optsClone.Clear(fv.fd)
	}
	err = proto.UnmarshalOptions{AllowPartial: true, Merge: true}.Unmarshal(data, optsClone.Interface())
	if err != nil {
		// bummer, can't fix this one
		return opts
	}

	return optsClone.Interface().(O)
}
