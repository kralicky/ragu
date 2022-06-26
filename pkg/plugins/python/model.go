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
	Value   any    `json:"value"`
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
				Comment: value.GetSourceInfo().GetLeadingComments(),
				Name:    formatClassName(value.GetName()),
				Value:   int32(value.GetNumber()),
			}
			entries = append(entries, e)
		}
		enums = append(enums, Enum{
			Comment: e.GetSourceInfo().GetLeadingComments(),
			PyName:  formatFieldName(e.GetName()),
			Entries: entries,
		})
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
			Comment:    msg.GetSourceInfo().GetLeadingComments(),
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
	if f.IsRepeated() {
		annotation = fmt.Sprintf(": List[%s]", pyType)
		m.OutputFile.TypingImports = append(m.OutputFile.TypingImports, "List")
	} else {
		annotation = fmt.Sprintf(": %s", pyType)
	}
	fieldWraps := strings.HasPrefix(f.GetType().String(), ".google.protobuf")
	fieldArgs := []string{fmt.Sprint(f.GetNumber())}
	if fieldWraps {
		fieldArgs = append(fieldArgs, "wraps=True")
	}

	protoFieldType := strings.TrimPrefix(strings.ToLower(f.GetType().String()), "type_")
	fieldType := fmt.Sprintf("betterproto.%s_field(%s)", protoFieldType, strings.Join(fieldArgs, ", "))
	return Field{
		Comment:     f.GetSourceInfo().GetLeadingComments(),
		FieldString: fmt.Sprintf("%s%s = %s", name, annotation, fieldType),
	}
}

func (m *Model) buildServices(f *desc.FileDescriptor) []Service {
	anyClientStreaming := false
	anyServerStreaming := false
	services := []Service{}
	for _, s := range f.GetServices() {
		methods := []Method{}
		for _, m := range s.GetMethods() {
			methods = append(methods, Method{
				PyInputMessageParam: formatFieldName(m.GetInputType().GetName()),
				Comment:             m.GetSourceInfo().GetLeadingComments(),
				PyOutputMessageType: formatClassName(m.GetOutputType().GetName()),
				PyInputMessageType:  formatClassName(m.GetInputType().GetName()),
				PyName:              formatMethodName(m.GetName()),
				Route:               fmt.Sprintf("/%s/%s", s.GetFullyQualifiedName(), m.GetName()),
				PyInputMessage:      formatClassName(m.GetInputType().GetName()),
				ServerStreaming:     m.IsServerStreaming(),
				ClientStreaming:     m.IsClientStreaming(),
			})
			if m.IsClientStreaming() {
				anyClientStreaming = true
			}
			if m.IsServerStreaming() {
				anyServerStreaming = true
			}
		}
		services = append(services, Service{
			Comment: s.GetSourceInfo().GetLeadingComments(),
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
		fieldWraps := strings.HasPrefix(field.GetMessageType().GetFullyQualifiedName(), ".google.protobuf")
		return m.getTypeReference(field.GetFile().GetPackage(),
			field.GetFile().GetDependencies(),
			field.GetMessageType().GetFullyQualifiedName(), fieldWraps), nil
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
