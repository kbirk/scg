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
	// FieldJSONKey is the field's JSON key: the snake_case field name, the same
	// key the Go generator uses in its json struct tags.
	FieldJSONKey      string
	FieldType         string
	FieldDefaultValue string
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
	inline void fromJSON(const std::vector<uint8_t>& json_bytes_);

	inline std::vector<uint8_t> toBytes() const;
	inline scg::error::Error fromBytes(const std::vector<uint8_t>& bytes_);
	inline scg::error::Error fromBytes(const uint8_t* bytes_, uint32_t size_);

};{{if gt (len .MessageFields) 0}}

inline void to_json(nlohmann::json& j, const {{.MessageNamePascalCase}}& m) {
	j = nlohmann::json::object();{{range .MessageFields}}
	j["{{.FieldJSONKey}}"] = scg::json::encode(m.{{.FieldNameCamelCase}});{{end}}
}

inline void from_json(const nlohmann::json& j, {{.MessageNamePascalCase}}& m) {
	// null leaves the message unchanged; absent keys leave their field
	// unchanged (as Go does, so JSON from an older schema still decodes)
	if (!scg::json::expect_object(j)) {
		return;
	}{{range .MessageFields}}
	if (auto it = j.find("{{.FieldJSONKey}}"); it != j.end()) {
		scg::json::decode(*it, m.{{.FieldNameCamelCase}});
	}{{end}}
}
{{else}}
inline void to_json(nlohmann::json& j, const {{.MessageNamePascalCase}}&) {
	j = nlohmann::json::object();
}

inline void from_json(const nlohmann::json& j, {{.MessageNamePascalCase}}&) {
	// an object (or null) is required, as Go rejects arrays and scalars
	scg::json::expect_object(j);
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
	// locals and parameters of member functions contain an underscore, so they
	// can never hide a (camelCase) field
	nlohmann::json json_;
	to_json(json_, *this);
	// invalid UTF-8 is replaced with U+FFFD, as Go does
	auto text_ = json_.dump(-1, ' ', false, nlohmann::json::error_handler_t::replace);
	return std::vector<uint8_t>(text_.begin(), text_.end());
}

void {{.MessageNamePascalCase}}::fromJSON(const std::vector<uint8_t>& json_bytes_)
{
	nlohmann::json json_ = scg::json::parse(std::string(json_bytes_.begin(), json_bytes_.end()));
	from_json(json_, *this);
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toBytes() const
{
	// Use view for copy elision
	std::vector<uint8_t> bytes_;
	const uint32_t num_bits_ = bit_size(*this);
	if (num_bits_ == 0) {
		// only zero-bit fields (e.g. empty messages): nothing to write
		return bytes_;
	}
	bytes_.resize(scg::serialize::bits_to_bytes(num_bits_), 0);
	scg::serialize::WriterView writer_(bytes_);
	serialize(writer_, *this);
	return bytes_;
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const std::vector<uint8_t>& bytes_)
{
	scg::serialize::ReaderView reader_(bytes_);
	return deserialize(*this, reader_);
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const uint8_t* bytes_, uint32_t size_)
{
	scg::serialize::ReaderView reader_(bytes_, size_);
	return deserialize(*this, reader_);
}

{{else}}

template <typename WriterType>
inline void serialize(WriterType&, const {{.MessageNamePascalCase}}&)
{
}

template <typename ReaderType>
inline scg::error::Error deserialize({{.MessageNamePascalCase}}&, ReaderType&)
{
	return nullptr;
}

inline uint32_t bit_size(const {{.MessageNamePascalCase}}&)
{
	return 0;
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toJSON() const
{
	// an empty object, as Go encodes a struct without fields
	return std::vector<uint8_t>{'{', '}'};
}

void {{.MessageNamePascalCase}}::fromJSON(const std::vector<uint8_t>& json_bytes_)
{
	nlohmann::json json_ = scg::json::parse(std::string(json_bytes_.begin(), json_bytes_.end()));
	from_json(json_, *this);
}

std::vector<uint8_t> {{.MessageNamePascalCase}}::toBytes() const
{
	return std::vector<uint8_t>();
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const std::vector<uint8_t>&)
{
	return nullptr;
}

scg::error::Error {{.MessageNamePascalCase}}::fromBytes(const uint8_t*, uint32_t)
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

// qualifiedCppTypeName returns the fully qualified name of a custom type. Every
// reference is qualified, also within the type's own package: inside a
// namespace a::b, `b::c::T` would resolve `b` to the enclosing a::b, and inside
// a message (which derives from scg::type::Message) an unqualified `Message`
// would resolve to the base class's injected class name.
func qualifiedCppTypeName(pkgName string, typeName string) string {
	if pkgName == "" {
		return util.EnsurePascalCase(typeName)
	}
	return fmt.Sprintf("::%s::%s", convertPackageNameToCppNamespacePrefix(pkgName), util.EnsurePascalCase(typeName))
}

func mapDataTypeComparableDefinitionToCppType(dataType *parse.DataTypeComparableDefinition) (string, error) {
	switch dataType.Type {
	case parse.DataTypeComparableCustom:
		return qualifiedCppTypeName(dataType.CustomTypePackage, dataType.CustomType), nil
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
		return qualifiedCppTypeName(dataType.CustomTypePackage, dataType.CustomType), nil
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

// isEnumType reports whether a custom type refers to an enum.
func isEnumType(p *parse.Parse, dataType *parse.DataTypeDefinition) bool {
	if p == nil || dataType.Type != parse.DataTypeCustom {
		return false
	}
	pkg, ok := p.Packages[dataType.CustomTypePackage]
	if !ok {
		return false
	}
	_, ok = pkg.Enums[dataType.CustomType]
	return ok
}

func getMessageFieldArg(p *parse.Parse, field *parse.MessageFieldDefinition) (MessageFieldArgs, error) {
	cppType, err := mapDataTypeDefinitionToCppType(field.DataTypeDefinition)
	if err != nil {
		return MessageFieldArgs{}, err
	}
	defaultValue, err := mapDataTypeDefinitionToDefaultValue(field.DataTypeDefinition)
	if err != nil {
		return MessageFieldArgs{}, err
	}
	if isEnumType(p, field.DataTypeDefinition) {
		// enum class members are otherwise left indeterminate when a message is
		// default-initialized; zero them like every other scalar field
		defaultValue = "{}"
	}
	return MessageFieldArgs{
		FieldNameCamelCase: util.EnsureCamelCase(field.Name),
		FieldJSONKey:       util.EnsureSnakeCase(field.Name),
		FieldType:          cppType,
		FieldDefaultValue:  defaultValue,
	}, nil
}

func generateMessageCppCode(p *parse.Parse, msg *parse.MessageDefinition) (string, error) {
	args := MessageArgs{
		MessageNamePascalCase: util.EnsurePascalCase(msg.Name),
		MessageFields:         []MessageFieldArgs{},
	}
	fields := []string{}
	for _, field := range msg.FieldsByIndex() {
		fieldArg, err := getMessageFieldArg(p, field)
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
