package cpp_gen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type MessageFieldArgs struct {
	FieldNameCamelCase string
	FieldType          string
	FieldDefaultValue  string
}

type MessageArgs struct {
	MessageNamePascalCase       string
	MessageFields               []MessageFieldArgs
	MessageFieldsCommaSeparated string
}

const messageTemplateStr = `
struct {{.MessageNamePascalCase}} : scg::type::Message { {{- range .MessageFields}}
{{- if ne .FieldDefaultValue ""}}
	{{.FieldType}} {{.FieldNameCamelCase}} = {{.FieldDefaultValue}};
{{- else}}
	{{.FieldType}} {{.FieldNameCamelCase}};
{{- end}}{{- end}}

	inline std::vector<uint8_t> toJSON() const;
	inline void fromJSON(const std::vector<uint8_t>& data);

	inline std::vector<uint8_t> toBytes() const;
	inline scg::error::Error fromBytes(const std::vector<uint8_t>& data);
	inline scg::error::Error fromBytes(const uint8_t* data, uint32_t size);

};{{if gt (len .MessageFields) 0}}
NLOHMANN_DEFINE_TYPE_NON_INTRUSIVE({{.MessageNamePascalCase}}, {{.MessageFieldsCommaSeparated}}){{else}}
inline void to_json(nlohmann::json& j, const {{.MessageNamePascalCase}}& m) {
	j = nlohmann::json::object();
}

inline void from_json(const nlohmann::json& j, {{.MessageNamePascalCase}}& m) {
}
{{end}}

{{if gt (len .MessageFields) 0 }}

template <typename WriterType>
inline void serialize(WriterType& writer, const {{.MessageNamePascalCase}}& value)
{
	{{range .MessageFields}}writer.write(value.{{.FieldNameCamelCase}});
	{{- end}}
}

template <typename ReaderType>
inline scg::error::Error deserialize({{.MessageNamePascalCase}}& value, ReaderType& reader)
{
	scg::error::Error err;
	{{range .MessageFields}}err = reader.read(value.{{.FieldNameCamelCase}});
	if (err) {
		return err;
	}
	{{end}}return nullptr;
}

inline uint32_t bit_size(const {{.MessageNamePascalCase}}& value)
{
	using scg::serialize::bit_size; // adl trickery
	uint32_t size = 0;{{- range .MessageFields}}
	size += bit_size(value.{{.FieldNameCamelCase}});{{end}}
	return size;
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toJSON() const
{
	nlohmann::json j({ {{- range $index, $element := .MessageFields}}{{if $index}}, {{end}}
		{"{{$element.FieldNameCamelCase}}", {{$element.FieldNameCamelCase}} }{{end}} });
	auto str = j.dump();
	return std::vector<uint8_t>(str.begin(), str.end());
}

void {{.MessageNamePascalCase}}::fromJSON(const std::vector<uint8_t>& data)
{
	nlohmann::json j = nlohmann::json::parse(std::string(data.begin(), data.end()));
	{{range .MessageFields}}j.at("{{.FieldNameCamelCase}}").get_to({{.FieldNameCamelCase}});
	{{end}}
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toBytes() const
{
	std::vector<uint8_t> data;
	data.reserve(scg::serialize::bits_to_bytes(bit_size(*this)));
	scg::serialize::WriterView writer(data);
	serialize(writer, *this);
	return data;
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const std::vector<uint8_t>& data)
{
	scg::serialize::ReaderView reader(data);
	return deserialize(*this, reader);
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const uint8_t* data, uint32_t size)
{
	scg::serialize::ReaderView reader(data, size);
	return deserialize(*this, reader);
}

{{else}}

template <typename WriterType>
inline void serialize(WriterType& writer, const {{.MessageNamePascalCase}}& value)
{
}

template <typename ReaderType>
inline scg::error::Error deserialize({{.MessageNamePascalCase}}& value, ReaderType& reader)
{
	return nullptr;
}

inline uint32_t bit_size(const {{.MessageNamePascalCase}}& value)
{
	return 0;
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toJSON() const
{
	nlohmann::json j;
	auto str = j.dump();
	return std::vector<uint8_t>(str.begin(), str.end());
}

void {{.MessageNamePascalCase}}::fromJSON(const std::vector<uint8_t>& data)
{
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toBytes() const
{
	return std::vector<uint8_t>();
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const std::vector<uint8_t>& data)
{
	return nullptr;
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const uint8_t* data, uint32_t size)
{
	return nullptr;
}
{{end}}
`

var (
	messageTemplate = template.Must(template.New("messageTemplateCpp").Parse(messageTemplateStr))
)

func convertPackageNameToCppNamespaces(name string) []string {
	return strings.Split(strings.ToLower(name), ".")
}

func convertPackageNameToCppNamespacePrefix(name string) string {
	return strings.Join(convertPackageNameToCppNamespaces(name), "::")
}

func mapDataTypeComparableToCppType(dataType parse.DataTypeComparable) (string, error) {
	switch dataType {
	case parse.DataTypeComparableUInt8:
		return "uint8_t", nil
	case parse.DataTypeComparableUInt16:
		return "uint16_t", nil
	case parse.DataTypeComparableUInt32:
		return "uint32_t", nil
	case parse.DataTypeComparableUInt64:
		return "uint64_t", nil
	case parse.DataTypeComparableInt8:
		return "int8_t", nil
	case parse.DataTypeComparableInt16:
		return "int16_t", nil
	case parse.DataTypeComparableInt32:
		return "int32_t", nil
	case parse.DataTypeComparableInt64:
		return "int64_t", nil
	case parse.DataTypeComparableString:
		return "std::string", nil
	case parse.DataTypeComparableUUID:
		return "scg::type::uuid", nil
	case parse.DataTypeComparableFloat32:
		return "float32_t", nil
	case parse.DataTypeComparableFloat64:
		return "float64_t", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapDataTypeComparableDefinitionToCppType(dataType *parse.DataTypeComparableDefinition) (string, error) {
	switch dataType.Type {
	case parse.DataTypeComparableCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%s::%s", convertPackageNameToCppNamespacePrefix(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		}
		return util.EnsurePascalCase(dataType.CustomType), nil
	}
	return mapDataTypeComparableToCppType(dataType.Type)
}

func mapDataTypeToCppType(dataType parse.DataType) (string, error) {

	switch dataType {
	case parse.DataTypeByte:
		return "uint8_t", nil
	case parse.DataTypeBool:
		return "bool", nil
	case parse.DataTypeUInt8:
		return "uint8_t", nil
	case parse.DataTypeUInt16:
		return "uint16_t", nil
	case parse.DataTypeUInt32:
		return "uint32_t", nil
	case parse.DataTypeUInt64:
		return "uint64_t", nil
	case parse.DataTypeInt8:
		return "int8_t", nil
	case parse.DataTypeInt16:
		return "int16_t", nil
	case parse.DataTypeInt32:
		return "int32_t", nil
	case parse.DataTypeInt64:
		return "int64_t", nil
	case parse.DataTypeString:
		return "std::string", nil
	case parse.DataTypeTimestamp:
		return "scg::type::timestamp", nil
	case parse.DataTypeUUID:
		return "scg::type::uuid", nil
	case parse.DataTypeFloat32:
		return "float32_t", nil
	case parse.DataTypeFloat64:
		return "float64_t", nil
	}

	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapDataTypeToCppDefaultValue(dataType parse.DataType) (string, error) {

	switch dataType {
	case parse.DataTypeByte,
		parse.DataTypeUInt8,
		parse.DataTypeUInt16,
		parse.DataTypeUInt32,
		parse.DataTypeUInt64,
		parse.DataTypeInt8,
		parse.DataTypeInt16,
		parse.DataTypeInt32,
		parse.DataTypeInt64:
		return "0", nil
	case parse.DataTypeBool:
		return "false", nil
	case parse.DataTypeFloat32:
		return "0.0f", nil
	case parse.DataTypeFloat64:
		return "0.0", nil
	}

	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapDataTypeDefinitionToCppType(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
	case parse.DataTypeMap:
		key, err := mapDataTypeComparableDefinitionToCppType(dataType.Key)
		if err != nil {
			return "", err
		}
		subtype, err := mapDataTypeDefinitionToCppType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("std::map<%s, %s>", key, subtype), nil
	case parse.DataTypeList:

		subtype, err := mapDataTypeDefinitionToCppType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("std::vector<%s>", subtype), nil
	case parse.DataTypeCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%s::%s", convertPackageNameToCppNamespacePrefix(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		}
		return util.EnsurePascalCase(dataType.CustomType), nil
	}

	return mapDataTypeToCppType(dataType.Type)
}

func mapDataTypeDefinitionToDefaultValue(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
	case parse.DataTypeMap,
		parse.DataTypeList,
		parse.DataTypeCustom,
		parse.DataTypeString,
		parse.DataTypeTimestamp,
		parse.DataTypeUUID:
		return "", nil
	}

	return mapDataTypeToCppDefaultValue(dataType.Type)
}

func getMessageFieldArg(field *parse.MessageFieldDefinition) (MessageFieldArgs, error) {
	cppType, err := mapDataTypeDefinitionToCppType(field.DataTypeDefinition)
	if err != nil {
		return MessageFieldArgs{}, err
	}
	defaultValue, err := mapDataTypeDefinitionToDefaultValue(field.DataTypeDefinition)
	return MessageFieldArgs{
		FieldNameCamelCase: util.EnsureCamelCase(field.Name),
		FieldType:          cppType,
		FieldDefaultValue:  defaultValue,
	}, nil
}

func generateMessageCppCode(msg *parse.MessageDefinition) (string, error) {
	args := MessageArgs{
		MessageNamePascalCase: util.EnsurePascalCase(msg.Name),
		MessageFields:         []MessageFieldArgs{},
	}
	fields := []string{}
	for _, field := range msg.FieldsByIndex() {
		fieldArg, err := getMessageFieldArg(field)
		if err != nil {
			return "", err
		}
		args.MessageFields = append(args.MessageFields, fieldArg)
		fields = append(fields, fieldArg.FieldNameCamelCase)
	}
	args.MessageFieldsCommaSeparated = strings.Join(fields, ", ")

	buf := &bytes.Buffer{}
	err := messageTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
