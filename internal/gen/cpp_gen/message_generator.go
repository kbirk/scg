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
}

type MessageArgs struct {
	MessageNamePascalCase       string
	MessageFields               []MessageFieldArgs
	MessageFieldsCommaSeparated string
}

const messageTemplateStr = `
struct {{.MessageNamePascalCase}} { {{- range .MessageFields}}
	{{.FieldType}} {{.FieldNameCamelCase}};{{end}}

	inline std::vector<uint8_t> toJSON() const
	{
		nlohmann::json j({ {{- range $index, $element := .MessageFields}}{{if $index}}, {{end}}
			{"{{$element.FieldNameCamelCase}}", {{$element.FieldNameCamelCase}} }{{end}} });
		auto str = j.dump();
		return std::vector<uint8_t>(str.begin(), str.end());
	}

	inline void fromJSON(const std::vector<uint8_t>& data)
	{
		nlohmann::json j = nlohmann::json::parse(std::string(data.begin(), data.end()));
		{{range .MessageFields}}j.at("{{.FieldNameCamelCase}}").get_to({{.FieldNameCamelCase}});
		{{end}}
	}

	inline std::vector<uint8_t> toBytes() const
	{
		uint32_t size = 0;{{- range .MessageFields}}
		size += scg::serialize::calc_byte_size({{.FieldNameCamelCase}});{{end}}

		scg::serialize::FixedSizeWriter writer(size); {{- range .MessageFields}}
		scg::serialize::serialize(writer, {{.FieldNameCamelCase}});{{end}}
		return writer.bytes();
	}

	inline scg::error::Error fromBytes(const std::vector<uint8_t>& data)
	{
		scg::error::Error err;
		scg::serialize::Reader reader(data);{{- range .MessageFields}}
		err = scg::serialize::deserialize({{.FieldNameCamelCase}}, reader);
		if (err) {
			return err;
		}
		{{end}}
		return nullptr;
	}

	inline void serialize(scg::serialize::FixedSizeWriter& writer) const
	{
		{{range .MessageFields}}scg::serialize::serialize(writer, {{.FieldNameCamelCase}});{{end}}
	}

	inline scg::error::Error deserialize(scg::serialize::Reader& reader)
	{
		scg::error::Error err;
		{{range .MessageFields}}err = scg::serialize::deserialize({{.FieldNameCamelCase}}, reader);
		if (err) {
			return err;
		}
		{{end}}
		return nullptr;
	}

	inline uint32_t byteSize() const
	{
		uint32_t size = 0;{{- range .MessageFields}}
		size += scg::serialize::calc_byte_size({{.FieldNameCamelCase}});{{end}}
		return size;
	}

};{{if gt (len .MessageFields) 0}}
NLOHMANN_DEFINE_TYPE_NON_INTRUSIVE({{.MessageNamePascalCase}}, {{.MessageFieldsCommaSeparated}}){{else}}
NLOHMANN_DEFINE_TYPE_NON_INTRUSIVE({{.MessageNamePascalCase}}){{end}}`

var (
	messageTemplate = template.Must(template.New("messageTemplateCpp").Parse(messageTemplateStr))
)

func convertPackageNameToCppNamespaces(name string) []string {
	return strings.Split(strings.ToLower(name), ".")
}

func convertPackageNameToCppNamespacePrefix(name string) string {
	return strings.Join(convertPackageNameToCppNamespaces(name), "::")
}

func mapComparableTypeToCppType(dataType *parse.DataTypeComparableDefinition) (string, error) {
	switch dataType.Type {
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
	case parse.DataTypeComparableFloat32:
		return "float32_t", nil
	case parse.DataTypeComparableFloat64:
		return "float64_t", nil
	case parse.DataTypeComparableCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%s::%s", convertPackageNameToCppNamespacePrefix(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		}
		return util.EnsurePascalCase(dataType.CustomType), nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapTypeToCppType(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
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
	case parse.DataTypeFloat32:
		return "float32_t", nil
	case parse.DataTypeFloat64:
		return "float64_t", nil
	case parse.DataTypeMap:
		key, err := mapComparableTypeToCppType(dataType.Key)
		if err != nil {
			return "", err
		}
		subtype, err := mapTypeToCppType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("std::map<%s, %s>", key, subtype), nil
	case parse.DataTypeList:

		subtype, err := mapTypeToCppType(dataType.SubType)
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

	return "", fmt.Errorf("unrecognized type: %v", dataType.Type)
}

func getMessageFieldArg(field *parse.MessageFieldDefinition) (MessageFieldArgs, error) {
	goType, err := mapTypeToCppType(field.DataTypeDefinition)
	if err != nil {
		return MessageFieldArgs{}, err
	}
	return MessageFieldArgs{
		FieldNameCamelCase: util.EnsureCamelCase(field.Name),
		FieldType:          goType,
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
