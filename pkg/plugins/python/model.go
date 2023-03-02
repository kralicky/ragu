package python

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/jhump/protoreflect/desc"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Model struct {
	OutputFile *OutputFile `json:"output_file"`
}

type OutputFile struct {
	InputFilenames          []string  `json:"input_filenames"`
	Imports                 []string  `json:"imports"`
	DatetimeImports         []string  `json:"datetime_imports"`
	PythonModuleImports     []string  `json:"python_module_imports"`
	ImportsTypeCheckingOnly []string  `json:"imports_type_checking_only"`
	TypingImports           []string  `json:"typing_imports"`
	Enums                   []Enum    `json:"enums"`
	Messages                []Message `json:"messages"`
	Services                []Service `json:"services"`
}

type Enum struct {
	Comment string  `json:"comment"`
	PyName  string  `json:"py_name"`
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Comment string `json:"comment"`
	Name    string `json:"name"`
	Value   int32  `json:"value"`
}

type Message struct {
	Comment             string  `json:"comment"`
	Deprecated          bool    `json:"deprecated"`
	PyName              string  `json:"py_name"`
	HasDeprecatedFields bool    `json:"has_deprecated_fields"`
	DeprecatedFields    []Field `json:"deprecated_fields"`
	Fields              []Field `json:"fields"`
}

type Field struct {
	FieldString string `json:"field_string"`
	Comment     string `json:"comment"`
}

type Service struct {
	Comment string   `json:"comment"`
	PyName  string   `json:"py_name"`
	Methods []Method `json:"methods"`
}

type Method struct {
	PyInputMessageParam string `json:"py_input_message_param"`
	Comment             string `json:"comment"`
	PyOutputMessageType string `json:"py_output_message_type"`
	PyInputMessageType  string `json:"py_input_message_type"`
	PyName              string `json:"py_name"`
	Route               string `json:"route"`
	PyInputMessage      string `json:"py_input_message"`
	ServerStreaming     bool   `json:"server_streaming"`
	ClientStreaming     bool   `json:"client_streaming"`
}

func buildModel(f *desc.FileDescriptor, deps []*desc.FileDescriptor) *Model {
	m := &Model{
		OutputFile: &OutputFile{
			InputFilenames: []string{f.GetName()},
		},
	}
	m.OutputFile.Enums = m.buildEnums(f)
	m.OutputFile.Messages = m.buildMessages(f)
	m.OutputFile.Services = m.buildServices(f)
	m.OutputFile.cleanImports()

	return m
}

func (m *Model) buildEnums(f *desc.FileDescriptor) []Enum {
	enums := []Enum{}
	for _, e := range f.GetEnumTypes() {
		entries := []Entry{}
		for _, value := range e.GetValues() {
			e := Entry{
				Comment: formatComment(value.GetSourceInfo().GetLeadingComments(), 1),
				Name:    value.GetName(),
				Value:   value.GetNumber(),
			}
			entries = append(entries, e)
		}
		enums = append(enums, Enum{
			Comment: formatComment(e.GetSourceInfo().GetLeadingComments(), 1),
			PyName:  formatClassName(e.GetName()),
			Entries: entries,
		})
	}
	for _, msg := range f.GetMessageTypes() {
		for _, nestedEnum := range msg.GetNestedEnumTypes() {
			entries := []Entry{}
			for _, value := range nestedEnum.GetValues() {
				e := Entry{
					Comment: formatComment(value.GetSourceInfo().GetLeadingComments(), 1),
					Name:    value.GetName(),
					Value:   value.GetNumber(),
				}
				entries = append(entries, e)
			}
			enums = append(enums, Enum{
				Comment: formatComment(nestedEnum.GetSourceInfo().GetLeadingComments(), 1),
				PyName:  formatClassName(msg.GetName() + "_" + nestedEnum.GetName()),
				Entries: entries,
			})
		}
	}
	return enums
}

func (m *Model) buildMessages(f *desc.FileDescriptor) []Message {
	messages := []Message{}
	for _, msg := range f.GetMessageTypes() {
		fields := []Field{}
		for _, field := range msg.GetFields() {
			fields = append(fields, m.buildField(field))
		}
		messages = append(messages, Message{
			Comment:    formatComment(msg.GetSourceInfo().GetLeadingComments(), 1),
			PyName:     formatClassName(msg.GetName()),
			Deprecated: msg.GetMessageOptions().GetDeprecated(),
			Fields:     fields,
		})
	}
	return messages
}

func (m *Model) buildField(f *desc.FieldDescriptor) Field {
	name := formatFieldName(f.GetName())
	annotation := ""
	pyType, err := m.pyType(f)
	if err != nil {
		panic(err)
	}
	fieldWraps := strings.HasPrefix(f.GetType().String(), "google.protobuf")
	fieldArgs := []string{fmt.Sprint(f.GetNumber())}
	if fieldWraps {
		fieldArgs = append(fieldArgs, "wraps=True")
	}
	var protoFieldType string

	if f.IsMap() {
		keyType, err := m.pyType(f.GetMapKeyType())
		if err != nil {
			panic(err)
		}
		valueType, err := m.pyType(f.GetMapValueType())
		if err != nil {
			panic(err)
		}
		annotation = fmt.Sprintf(": Dict[%s, %s]", keyType, valueType)
		fieldArgs = append(fieldArgs, fmt.Sprintf("key_type=%s", keyType), fmt.Sprintf("value_type=%s", valueType))
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "Dict")
	} else if f.IsRepeated() {
		annotation = fmt.Sprintf(": List[%s]", pyType)
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "List")
	} else {
		annotation = fmt.Sprintf(": %s", pyType)
	}

	if f.IsMap() {
		protoFieldType = "map"
	} else {
		protoFieldType = strings.TrimPrefix(strings.ToLower(f.GetType().String()), "type_")
	}
	fieldType := fmt.Sprintf("betterproto.%s_field(%s)", protoFieldType, strings.Join(fieldArgs, ", "))
	return Field{
		Comment:     formatComment(f.GetSourceInfo().GetLeadingComments(), 1),
		FieldString: fmt.Sprintf("%s%s = %s", name, annotation, fieldType),
	}
}

func (m *Model) messageTypeRef(from *desc.FileDescriptor, msg *desc.MessageDescriptor) string {
	fieldWraps := strings.HasPrefix(msg.GetFullyQualifiedName(), "google.protobuf")
	return m.getTypeReference(from, msg.GetFile(),
		msg.GetFile().GetDependencies(),
		msg.GetFullyQualifiedName(), fieldWraps)
}

func (m *Model) buildServices(f *desc.FileDescriptor) []Service {
	anyClientStreaming := false
	anyServerStreaming := false
	services := []Service{}
	for _, s := range f.GetServices() {
		methods := []Method{}
		for _, method := range s.GetMethods() {

			methods = append(methods, Method{
				PyInputMessageParam: formatFieldName(method.GetInputType().GetName()),
				Comment:             formatComment(method.GetSourceInfo().GetLeadingComments(), 2),
				PyOutputMessageType: m.messageTypeRef(f, method.GetOutputType()),
				PyInputMessageType:  m.messageTypeRef(f, method.GetInputType()),
				PyName:              formatMethodName(method.GetName()),
				Route:               fmt.Sprintf("/%s/%s", s.GetFullyQualifiedName(), method.GetName()),
				PyInputMessage:      formatClassName(method.GetInputType().GetName()),
				ServerStreaming:     method.IsServerStreaming(),
				ClientStreaming:     method.IsClientStreaming(),
			})
			if method.IsClientStreaming() {
				anyClientStreaming = true
			}
			if method.IsServerStreaming() {
				anyServerStreaming = true
			}
		}
		services = append(services, Service{
			Comment: formatComment(s.GetSourceInfo().GetLeadingComments(), 1),
			PyName:  formatClassName(s.GetName()),
			Methods: methods,
		})
	}
	if len(services) > 0 {
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "Dict", "Optional")
		m.OutputFile.ImportsTypeCheckingOnly = append(m.OutputFile.ImportsTypeCheckingOnly,
			"from betterproto.grpc.grpclib_client import MetadataLike",
			"from grpclib.metadata import Deadline",
		)
	}
	if anyClientStreaming {
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "AsyncIterable", "Iterable", "Union")
	}
	if anyClientStreaming || anyServerStreaming {
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "AsyncIterator")
	}
	return services
}

func formatClassName[T ~string](name T) string {
	return strcase.ToCamel(string(name))
}

func formatFieldName[T ~string](name T) string {
	return strcase.ToSnake(string(name))
}

func formatMethodName[T ~string](name T) string {
	return strcase.ToSnake(string(name))
}

func (m *Model) pyType(field *desc.FieldDescriptor) (string, error) {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
		descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float", nil
	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "int", nil
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool", nil
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "str", nil
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes", nil
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		if field.IsMap() {
			keyType, err := m.pyType(field.GetMapKeyType())
			if err != nil {
				return "", err
			}
			valueType, err := m.pyType(field.GetMapValueType())
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Dict[%s, %s]", keyType, valueType), nil
		}
		fieldWraps := strings.HasPrefix(field.GetMessageType().GetFullyQualifiedName(), "google.protobuf")
		return m.getTypeReference(
			field.GetFile(), field.GetMessageType().GetFile(),
			field.GetFile().GetDependencies(),
			field.GetMessageType().GetFullyQualifiedName(), fieldWraps), nil
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return m.getTypeReference(
			field.GetFile(), field.GetEnumType().GetFile(),
			field.GetFile().GetDependencies(),
			field.GetEnumType().GetFullyQualifiedName(), false), nil
	}
	return "", fmt.Errorf("unknown/unsupported type %s", field.GetType())
}

func (o *OutputFile) cleanImports() {
	slices.Sort(o.Imports)
	slices.Sort(o.DatetimeImports)
	slices.Sort(o.ImportsTypeCheckingOnly)
	slices.Sort(o.PythonModuleImports)
	slices.Sort(o.TypingImports)
	o.Imports = slices.Compact(o.Imports)
	o.DatetimeImports = slices.Compact(o.DatetimeImports)
	o.ImportsTypeCheckingOnly = slices.Compact(o.ImportsTypeCheckingOnly)
	o.PythonModuleImports = slices.Compact(o.PythonModuleImports)
	o.TypingImports = slices.Compact(o.TypingImports)
}

func formatComment(comment string, indent int) string {
	var lines []string
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	space := strings.Repeat(" ", indent*4)
	if len(lines) == 1 {
		return fmt.Sprintf(space+`""" %s """`, lines[0])
	}

	buf := strings.Builder{}
	buf.WriteString(space + `"""` + "\n")
	for _, line := range lines {
		buf.WriteString(space + line + "\n")
	}
	buf.WriteString(space + `"""`)
	return buf.String()
}
