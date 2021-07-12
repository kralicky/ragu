package machinery

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4/parser"
	"google.golang.org/protobuf/types/descriptorpb"
	"k8s.io/utils/pointer"
)

func stripQuotes(s string) string {
	if len(s) > 1 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

type descriptorGen struct {
	proto *parser.Proto
	desc  *descriptorpb.FileDescriptorProto
}

func GenerateDescriptor(proto *parser.Proto) *descriptorpb.FileDescriptorProto {
	desc := &descriptorpb.FileDescriptorProto{
		Name:    pointer.String(proto.Meta.Filename),
		Syntax:  pointer.String(stripQuotes(proto.Syntax.ProtobufVersion)),
		Options: &descriptorpb.FileOptions{},
	}
	gen := &descriptorGen{
		proto: proto,
		desc:  desc,
	}
	for _, v := range proto.ProtoBody {
		switch entry := v.(type) {
		case *parser.Import:
			desc.Dependency = append(desc.Dependency, stripQuotes(entry.Location))
			switch entry.Modifier {
			case parser.ImportModifierPublic:
				desc.PublicDependency = append(desc.PublicDependency,
					int32(len(desc.Dependency)-1))
			case parser.ImportModifierWeak:
				desc.WeakDependency = append(desc.WeakDependency,
					int32(len(desc.Dependency)-1))
			}
		case *parser.Package:
			desc.Package = &entry.Name
		case *parser.Option:
			gen.setOption(desc.Options, entry.OptionName, entry.Constant)
		case *parser.Message:
			desc.MessageType = append(desc.MessageType, gen.genMessageDescriptor(entry))
		case *parser.Enum:
			desc.EnumType = append(desc.EnumType, gen.genEnumDescriptor(entry))
		case *parser.Service:
			desc.Service = append(desc.Service, gen.genServiceDescriptor(entry))
		}
	}
	gen.finalize()
	return desc
}

func (gen *descriptorGen) finalize() {
	// Go back and resolve any ambiguous type ids
	for _, msg := range gen.desc.MessageType {
		gen.resolveTypeIds(msg)
	}

	// Go back and make sure all types are fully qualified
	for _, msg := range gen.desc.MessageType {
		gen.resolveTypeNames(msg, []string{*gen.desc.Package})
	}
}

func (gen *descriptorGen) resolveTypeNames(msg *descriptorpb.DescriptorProto, stack []string) {
	for _, field := range msg.Field {
		if field.Type != nil && field.TypeName != nil {
			// enum or message type
			if strings.Contains(field.GetTypeName(), ".") {
				// Already fully qualified or from another package
				if field.GetTypeName()[0] != '.' {
					// Don't know if this is an enum or message type, so let the
					// code generator figure it out later
					field.Type = nil
					// fully qualify the type name
					field.TypeName = pointer.String("." + *field.TypeName)
				}
				continue
			}
			tmpStack := append(append([]string{}, stack...), msg.GetName(), field.GetTypeName())
			checkFn := gen.messageTypeExists
			if *field.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
				checkFn = gen.enumTypeExists
			}
			for len(tmpStack) >= 2 {
				if checkFn(tmpStack) {
					name := "." + strings.Join(tmpStack, ".")
					field.TypeName = &name
					break
				} else {
					// Go up one level
					tmpStack = append(tmpStack[:len(tmpStack)-2], tmpStack[len(tmpStack)-1])
				}
			}
			if field.GetTypeName()[0] != '.' {
				panic("Failed to resolve typename " + field.GetTypeName())
			}
		}
	}
	for _, nested := range msg.NestedType {
		gen.resolveTypeNames(nested, append(stack, nested.GetName()))
	}
}

func (gen *descriptorGen) messageTypeExists(stack []string) bool {
	if stack[0] == *gen.desc.Package {
		stack = stack[1:]
	}
	var current interface{} = gen.desc
	for len(stack) > 0 {
		var msgList []*descriptorpb.DescriptorProto
		switch cur := current.(type) {
		case *descriptorpb.FileDescriptorProto:
			msgList = cur.MessageType
		case *descriptorpb.DescriptorProto:
			msgList = cur.NestedType
		}
		found := false
		for _, msg := range msgList {
			if msg.GetName() == stack[0] {
				current = msg
				stack = stack[1:]
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(stack) == 0
}

func (gen *descriptorGen) enumTypeExists(stack []string) bool {
	var current interface{} = gen.desc
	for len(stack) > 2 {
		var msgList []*descriptorpb.DescriptorProto
		switch cur := current.(type) {
		case *descriptorpb.FileDescriptorProto:
			msgList = cur.MessageType
		case *descriptorpb.DescriptorProto:
			msgList = cur.NestedType
		}
		found := false
		for _, msg := range msgList {
			if msg.GetName() == stack[1] {
				current = msg
				stack = stack[1:]
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	switch cur := current.(type) {
	case *descriptorpb.FileDescriptorProto:
		for _, msg := range cur.EnumType {
			if msg.GetName() == stack[1] {
				return true
			}
		}
	case *descriptorpb.DescriptorProto:
		for _, msg := range cur.EnumType {
			if msg.GetName() == stack[1] {
				return true
			}
		}
	}
	return false
}

func (gen *descriptorGen) resolveTypeIds(msg *descriptorpb.DescriptorProto) {
	for _, field := range msg.Field {
		if field.Type != nil && *field.Type == 0 {
			field.Type = gen.typeId(*field.TypeName)
			// If it was set to 0 again, we are sure it is a message type.
			if *field.Type == 0 {
				field.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			}
		}
	}
	for _, nested := range msg.NestedType {
		gen.resolveTypeIds(nested)
	}
}

func (gen *descriptorGen) genMessageDescriptor(msg *parser.Message) *descriptorpb.DescriptorProto {
	desc := &descriptorpb.DescriptorProto{
		Name:       &msg.MessageName,
		OneofDecl:  []*descriptorpb.OneofDescriptorProto{},
		Field:      []*descriptorpb.FieldDescriptorProto{},
		EnumType:   []*descriptorpb.EnumDescriptorProto{},
		NestedType: []*descriptorpb.DescriptorProto{},
		Options:    &descriptorpb.MessageOptions{},
	}
	for _, v := range msg.MessageBody {
		switch entry := v.(type) {
		case *parser.Option:
			gen.setOption(desc.Options, entry.OptionName, entry.Constant)
		case *parser.Oneof:
			oneofDesc, fields := gen.genOneofDescriptor(entry)
			desc.OneofDecl = append(desc.OneofDecl, oneofDesc)
			// Populate the oneof index in each field
			for _, field := range fields {
				field.OneofIndex = pointer.Int32(int32(len(desc.OneofDecl) - 1))
			}
			desc.Field = append(desc.Field, fields...)
		case *parser.Field:
			desc.Field = append(desc.Field, gen.genFieldDescriptor(entry))
		case *parser.MapField:
			field, entryType := gen.genMapFieldDescriptor(msg.MessageName, entry)
			desc.Field = append(desc.Field, field)
			desc.NestedType = append(desc.NestedType, entryType)
		case *parser.Enum:
			desc.EnumType = append(desc.EnumType, gen.genEnumDescriptor(entry))
		case *parser.Message:
			desc.NestedType = append(desc.NestedType, gen.genMessageDescriptor(entry))
		}
	}
	return desc
}

func (gen *descriptorGen) genEnumDescriptor(enum *parser.Enum) *descriptorpb.EnumDescriptorProto {
	desc := &descriptorpb.EnumDescriptorProto{
		Name:    &enum.EnumName,
		Value:   []*descriptorpb.EnumValueDescriptorProto{},
		Options: &descriptorpb.EnumOptions{},
	}

	// Fill in values
	for _, v := range enum.EnumBody {
		switch entry := v.(type) {
		case *parser.Option:
			gen.setOption(desc.Options, entry.OptionName, entry.Constant)
		case *parser.EnumField:
			num, err := strconv.Atoi(entry.Number)
			if err != nil {
				panic(err)
			}
			val := &descriptorpb.EnumValueDescriptorProto{
				Name:   &entry.Ident,
				Number: pointer.Int32(int32(num)),
			}
			// Add options
			for _, v := range entry.EnumValueOptions {
				gen.setOption(val, v.OptionName, v.Constant)
			}
			desc.Value = append(desc.Value, val)
		}
	}

	return desc
}

func (gen *descriptorGen) genServiceDescriptor(service *parser.Service) *descriptorpb.ServiceDescriptorProto {
	desc := &descriptorpb.ServiceDescriptorProto{
		Name:    &service.ServiceName,
		Method:  []*descriptorpb.MethodDescriptorProto{},
		Options: &descriptorpb.ServiceOptions{},
	}
	for _, v := range service.ServiceBody {
		switch entry := v.(type) {
		case *parser.Option:
			gen.setOption(desc.Options, entry.OptionName, entry.Constant)
		case *parser.RPC:
			desc.Method = append(desc.Method, gen.genMethodDescriptor(entry))
		}
	}
	return desc
}

func (gen *descriptorGen) genMethodDescriptor(rpc *parser.RPC) *descriptorpb.MethodDescriptorProto {
	requestType := rpc.RPCRequest.MessageType
	responseType := rpc.RPCResponse.MessageType
	if !strings.Contains(requestType, ".") {
		requestType = "." + *gen.desc.Package + "." + requestType
	} else if requestType[0] != '.' {
		requestType = "." + requestType
	}
	if !strings.Contains(responseType, ".") {
		responseType = "." + *gen.desc.Package + "." + responseType
	} else if responseType[0] != '.' {
		responseType = "." + responseType
	}
	desc := &descriptorpb.MethodDescriptorProto{
		Name:            &rpc.RPCName,
		InputType:       &requestType,
		OutputType:      &responseType,
		ClientStreaming: pointer.Bool(rpc.RPCRequest.IsStream),
		ServerStreaming: pointer.Bool(rpc.RPCResponse.IsStream),
		Options:         &descriptorpb.MethodOptions{},
	}
	// Add options
	for _, v := range rpc.Options {
		gen.setOption(desc.Options, v.OptionName, v.Constant)
	}
	return desc
}

var typeIds = map[string]descriptorpb.FieldDescriptorProto_Type{
	"double":   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
	"float":    descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
	"int64":    descriptorpb.FieldDescriptorProto_TYPE_INT64,
	"uint64":   descriptorpb.FieldDescriptorProto_TYPE_UINT64,
	"int32":    descriptorpb.FieldDescriptorProto_TYPE_INT32,
	"fixed64":  descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
	"fixed32":  descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
	"bool":     descriptorpb.FieldDescriptorProto_TYPE_BOOL,
	"string":   descriptorpb.FieldDescriptorProto_TYPE_STRING,
	"bytes":    descriptorpb.FieldDescriptorProto_TYPE_BYTES,
	"uint32":   descriptorpb.FieldDescriptorProto_TYPE_UINT32,
	"sfixed32": descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
	"sfixed64": descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
	"sint32":   descriptorpb.FieldDescriptorProto_TYPE_SINT32,
	"sint64":   descriptorpb.FieldDescriptorProto_TYPE_SINT64,
}

func isBuiltInType(t string) bool {
	_, ok := typeIds[t]
	return ok
}

func (gen *descriptorGen) typeId(t string) *descriptorpb.FieldDescriptorProto_Type {
	if isBuiltInType(t) {
		id := typeIds[t]
		return &id
	}

	// Try to disambiguate between enum and message types by checking if the
	// type is a valid enum type.

	// Check top-level enums
	for _, v := range gen.desc.EnumType {
		if v.GetName() == t {
			return descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		}
	}

	// Check nested enums
	for _, v := range gen.desc.MessageType {
		for _, f := range v.EnumType {
			if f.GetName() == t {
				return descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
			}
		}
	}

	// Couldn't find an enum, but we can't be sure yet until we are done. Set the
	// type to 0 to indicate that we need to resolve this later.
	zero := descriptorpb.FieldDescriptorProto_Type(0)
	return &zero
}

func (gen *descriptorGen) genFieldDescriptor(field *parser.Field) *descriptorpb.FieldDescriptorProto {
	fd := &descriptorpb.FieldDescriptorProto{
		Name:    &field.FieldName,
		Options: &descriptorpb.FieldOptions{},
	}
	fd.Name = &field.FieldName

	i, err := strconv.Atoi(field.FieldNumber)
	if err != nil {
		panic(err)
	}
	fd.Number = pointer.Int32(int32(i))

	fd.Type = gen.typeId(field.Type)
	if !isBuiltInType(field.Type) {
		fd.TypeName = &field.Type
	}

	// Fill in options
	for _, v := range field.FieldOptions {
		gen.setOption(fd.Options, v.OptionName, v.Constant)
	}

	if field.IsRepeated {
		value := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
		fd.Label = &value
	} else {
		value := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
		fd.Label = &value
	}

	return fd
}

func (gen *descriptorGen) genOneofFieldDescriptor(field *parser.OneofField) *descriptorpb.FieldDescriptorProto {
	fd := &descriptorpb.FieldDescriptorProto{
		Name:    &field.FieldName,
		Options: &descriptorpb.FieldOptions{},
	}
	fd.Name = &field.FieldName
	i, err := strconv.Atoi(field.FieldNumber)
	if err != nil {
		panic(err)
	}
	fd.Number = pointer.Int32(int32(i))

	fd.Type = gen.typeId(field.Type)
	if !isBuiltInType(field.Type) {
		fd.TypeName = &field.Type
	}

	// Fill in options
	for _, v := range field.FieldOptions {
		gen.setOption(fd.Options, v.OptionName, v.Constant)
	}

	fd.Label = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	return fd
}

func (gen *descriptorGen) genMapFieldDescriptor(
	containingMsg string,
	field *parser.MapField,
) (*descriptorpb.FieldDescriptorProto, *descriptorpb.DescriptorProto) {
	fd := &descriptorpb.FieldDescriptorProto{
		Name:     pointer.String(field.MapName),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: pointer.String(fmt.Sprintf("%sEntry", field.MapName)),
	}

	i, err := strconv.Atoi(field.FieldNumber)
	if err != nil {
		panic(err)
	}
	fd.Number = pointer.Int32(int32(i))
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED // this is set for maps
	fd.Label = &repeated

	// Fill in options
	for _, v := range field.FieldOptions {
		gen.setOption(fd.Options, v.OptionName, v.Constant)
	}
	return fd, gen.genMapEntryType(field)
}

func (gen *descriptorGen) genMapEntryType(field *parser.MapField) *descriptorpb.DescriptorProto {
	entry := &descriptorpb.DescriptorProto{
		Name:  pointer.String(fmt.Sprintf("%sEntry", field.MapName)),
		Field: []*descriptorpb.FieldDescriptorProto{},
		Options: &descriptorpb.MessageOptions{
			MapEntry: pointer.Bool(true),
		},
	}
	entry.Field = append(entry.Field, &descriptorpb.FieldDescriptorProto{
		Name:     pointer.String("key"),
		Number:   pointer.Int32(1),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     gen.typeId(field.KeyType), // always a built-in type
		JsonName: pointer.String("key"),
	})
	value := &descriptorpb.FieldDescriptorProto{
		Name:     pointer.String("value"),
		Number:   pointer.Int32(2),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: pointer.String("value"),
	}
	value.Type = gen.typeId(field.Type)
	if !isBuiltInType(field.Type) {
		value.TypeName = &field.Type
	}
	entry.Field = append(entry.Field, value)
	return entry
}

func (gen *descriptorGen) setOption(options interface{}, name string, value string) {
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"%s":%s}`, name, value)), options); err != nil {
		panic(err)
	}
}

func (gen *descriptorGen) genOneofDescriptor(
	oneof *parser.Oneof,
) (*descriptorpb.OneofDescriptorProto, []*descriptorpb.FieldDescriptorProto) {
	desc := &descriptorpb.OneofDescriptorProto{
		Name:    &oneof.OneofName,
		Options: &descriptorpb.OneofOptions{},
	}
	fieldDescriptors := []*descriptorpb.FieldDescriptorProto{}
	for _, field := range oneof.OneofFields {
		fd := gen.genOneofFieldDescriptor(field)
		fieldDescriptors = append(fieldDescriptors, fd)
	}
	return desc, fieldDescriptors
}

// Go through all message types from all protos, and find any message fields
// which have a typename set but not a type. These will (should) be types
// imported from other packages where we couldn't tell if they were enum or
// message types at the time. According to the docs, we should be able to
// leave the type field empty, but that does not appear to be correct as it
// casuses proto to think some enums are messages.
func ResolveKindsFromDependencies(files []*descriptorpb.FileDescriptorProto) {
	fileMap := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range files {
		fileMap[f.GetPackage()] = f
	}
	for _, file := range files {
		for _, message := range file.GetMessageType() {
			resolveKindsRecursive(fileMap, file, message)
		}
	}
}

func resolveKindsRecursive(
	fileMap map[string]*descriptorpb.FileDescriptorProto,
	currentFile *descriptorpb.FileDescriptorProto,
	message *descriptorpb.DescriptorProto,
) {
	for _, field := range message.GetField() {
		if field.TypeName != nil && field.Type == nil {
			// find out which package name the typename starts with
			var pkgName string
			typename := field.GetTypeName()[1:] // trim off the leading dot
			for k := range fileMap {
				if strings.HasPrefix(typename, k) {
					pkgName = k
				}
			}
			if pkgName == "" {
				panic("this shouldn't happen")
			}
			targetFile, ok := fileMap[pkgName]
			if !ok {
				panic("this shouldn't happen")
			}

			parts := strings.SplitAfter(typename, pkgName+".")
			if len(parts) != 2 {
				panic("bug")
			}
			parts = strings.Split(parts[1], ".")

			msgsToSearch := targetFile.MessageType
			enumsToSearch := targetFile.EnumType

			for len(parts) > 0 {
				for _, msg := range msgsToSearch {
					if *msg.Name == parts[0] {
						msgsToSearch = msg.NestedType
						enumsToSearch = msg.EnumType
						if len(parts) == 1 {
							field.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
							goto next
						}
						parts = parts[1:]
						break
					}
				}
				for _, enum := range enumsToSearch {
					if *enum.Name == parts[0] {
						if len(parts) == 1 {
							field.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
							goto next
						} else {
							panic("this shouldn't happen")
						}
					}
				}
			}
			panic("Could not resolve type " + field.GetName())
		next:
		}
	}

	for _, nested := range message.NestedType {
		resolveKindsRecursive(fileMap, currentFile, nested)
	}
}
