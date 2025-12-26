package go_gen

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type MessageFieldArgs struct {
	FieldNamePascalCase string
	FieldNameSnakeCase  string
	FieldType           string
}

type MessageArgs struct {
	MessageNamePascalCase  string
	MessageNameFirstLetter string
	MessageFields          []MessageFieldArgs
	BitSizeCode            string
	SerializeCode          string
	DeserializeCode        string
}

const messageTemplateStr = `
type {{.MessageNamePascalCase}} struct { {{- range .MessageFields}}
	{{.FieldNamePascalCase}} {{.FieldType}} ` + "`json:\"{{.FieldNameSnakeCase}}\"`" + `{{end}}
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) ToJSON() ([]byte, error) {
	jsonData, err := json.Marshal({{.MessageNameFirstLetter}})
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) FromJSON(data []byte) error {
	err := json.Unmarshal(data, {{.MessageNameFirstLetter}})
	if err != nil {
		return err
	}
	return nil
}
{{- if gt (len .MessageFields) 0 }}
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) ToBytes() []byte {
	size := {{.MessageNameFirstLetter}}.BitSize()
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	{{.MessageNameFirstLetter}}.Serialize(writer)
	return writer.Bytes()
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) FromBytes(bs []byte) error {
	return {{.MessageNameFirstLetter}}.Deserialize(serialize.NewReader(bs))
}
{{- else}}
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) ToBytes() []byte {
	return []byte{}
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) FromBytes(bs []byte) error {
	return nil
}
{{- end }}

{{.BitSizeCode}}
{{.SerializeCode}}
{{.DeserializeCode}}
`

type BitSizeMethodArgs struct {
	MessageNameFirstLetter  string
	MessageNamePascalCase   string
	FieldBitSizeMethodCalls []string
}

type SerializeMethodArgs struct {
	MessageNameFirstLetter    string
	MessageNamePascalCase     string
	FieldSerializeMethodCalls []string
}

type DeserializeMethodArgs struct {
	MessageNameFirstLetter      string
	MessageNamePascalCase       string
	FieldDeserializeMethodCalls []string
}

const messageBitSizeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) BitSize() int {
	size := 0{{range .FieldBitSizeMethodCalls}}
	size += {{.}}{{end}}
	return size
}`

const messageSerializeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) Serialize(writer *serialize.Writer) {
	{{- range .FieldSerializeMethodCalls}}
	{{.}}{{end}}
}`

const messageDeserializeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	{{- if gt (len .FieldDeserializeMethodCalls) 0 }}var err error{{end}}{{range .FieldDeserializeMethodCalls}}
	err = {{.}}
	if err != nil {
		return err
	}{{end}}
	return nil
}`

type BitSizeContainerMethodArgs struct {
	FullMethodName             string
	ArgType                    string
	KeyTypeBitSizeMethodCall   string
	ValueTypeBitSizeMethodCall string
}

type SerializeContainerMethodArgs struct {
	FullMethodName               string
	ArgType                      string
	KeyTypeSerializeMethodCall   string
	ValueTypeSerializeMethodCall string
}

type DeserializeContainerMethodArgs struct {
	FullMethodName                 string
	ArgType                        string
	KeyTypeDeserializeMethodCall   string
	ValueTypeDeserializeMethodCall string
	KeyType                        string
	ValueType                      string
}

const mapBitSizeMethodTemplateStr = `
func {{.FullMethodName}}(arg {{.ArgType}}) int {
	size := serialize.BitSizeUInt32(uint32(len(arg)))
	for k, v := range arg {
		size += {{.KeyTypeBitSizeMethodCall}} + {{.ValueTypeBitSizeMethodCall}}
	}
	return size
}`

const mapSerializeMethodTemplateStr = `
func {{.FullMethodName}}(writer *serialize.Writer, arg {{.ArgType}}) error {
	serialize.SerializeUInt32(writer, uint32(len(arg)))
	for k, v := range arg {
		{{.KeyTypeSerializeMethodCall}}
		{{.ValueTypeSerializeMethodCall}}
	}
	return nil
}`

const mapDeserializeMethodTemplateStr = `
func {{.FullMethodName}}(arg *{{.ArgType}}, reader *serialize.Reader) error {
	var length uint32
	err := serialize.DeserializeUInt32(&length, reader)
	if err != nil {
		return err
	}

	*arg = make({{.ArgType}}, int(length))

	for i := 0; i < int(length); i++ {
		var k {{.KeyType}}
		var v {{.ValueType}}
		err := {{.KeyTypeDeserializeMethodCall}}
		if err != nil {
			return err
		}
		err = {{.ValueTypeDeserializeMethodCall}}
		if err != nil {
			return err
		}
		(*arg)[k] = v
	}
	return nil
}`

const listBitSizeMethodTemplateStr = `
func {{.FullMethodName}}(arg {{.ArgType}}) int {
	size := serialize.BitSizeUInt32(uint32(len(arg)))
	for _, v := range arg {
		size += {{.ValueTypeBitSizeMethodCall}}
	}
	return size
}`

const listSerializeMethodTemplateStr = `
func {{.FullMethodName}}(writer *serialize.Writer, arg {{.ArgType}}) error {
	serialize.SerializeUInt32(writer, uint32(len(arg)))
	for _, v := range arg {
		{{.ValueTypeSerializeMethodCall}}
	}
	return nil
}`

const listDeserializeMethodTemplateStr = `
func {{.FullMethodName}}(arg *{{.ArgType}}, reader *serialize.Reader) error {
	var length uint32
	err := serialize.DeserializeUInt32(&length, reader)
	if err != nil {
		return err
	}

	*arg = make({{.ArgType}}, int(length))

	for i := 0; i < int(length); i++ {
		var v {{.ValueType}}
		err := {{.ValueTypeDeserializeMethodCall}}
		if err != nil {
			return err
		}
		(*arg)[i] = v
	}
	return nil
}`

var (
	messageTemplate                  = template.Must(template.New("messageTemplateGo").Parse(messageTemplateStr))
	messageBitSizeMethodTemplate     = template.Must(template.New("messageBitSizeMethodTemplateGo").Parse(messageBitSizeMethodTemplateStr))
	messageSerializeMethodTemplate   = template.Must(template.New("messageSerializeMethodTemplateGo").Parse(messageSerializeMethodTemplateStr))
	messageDeserializeMethodTemplate = template.Must(template.New("messageDeserializeMethodTemplateGo").Parse(messageDeserializeMethodTemplateStr))
	// container methods
	mapBitSizeMethodTemplate      = template.Must(template.New("mapBitSizeMethodTemplateGo").Parse(mapBitSizeMethodTemplateStr))
	listBitSizeMethodTemplate     = template.Must(template.New("listBitSizeMethodTemplateGo").Parse(listBitSizeMethodTemplateStr))
	mapSerializeMethodTemplate    = template.Must(template.New("mapSerializeMethodTemplateGo").Parse(mapSerializeMethodTemplateStr))
	listSerializeMethodTemplate   = template.Must(template.New("listSerializeMethodTemplateGo").Parse(listSerializeMethodTemplateStr))
	mapDeserializeMethodTemplate  = template.Must(template.New("mapDeserializeMethodTemplateGo").Parse(mapDeserializeMethodTemplateStr))
	listDeserializeMethodTemplate = template.Must(template.New("listDeserializeMethodTemplateGo").Parse(listDeserializeMethodTemplateStr))
)

func mapDataTypeToGoType(dataType parse.DataType) (string, error) {
	switch dataType {
	case parse.DataTypeByte:
		return "byte", nil
	case parse.DataTypeBool:
		return "bool", nil
	case parse.DataTypeUInt8:
		return "uint8", nil
	case parse.DataTypeUInt16:
		return "uint16", nil
	case parse.DataTypeUInt32:
		return "uint32", nil
	case parse.DataTypeUInt64:
		return "uint64", nil
	case parse.DataTypeInt8:
		return "int8", nil
	case parse.DataTypeInt16:
		return "int16", nil
	case parse.DataTypeInt32:
		return "int32", nil
	case parse.DataTypeInt64:
		return "int64", nil
	case parse.DataTypeString:
		return "string", nil
	case parse.DataTypeTimestamp:
		return "time.Time", nil
	case parse.DataTypeUUID:
		return "uuid.UUID", nil
	case parse.DataTypeFloat32:
		return "float32", nil
	case parse.DataTypeFloat64:
		return "float64", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapDataTypeDefinitionToGoType(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
	case parse.DataTypeMap:
		key, err := mapDataTypeComparableDefinitionToGoType(dataType.Key)
		if err != nil {
			return "", err
		}
		subtype, err := mapDataTypeDefinitionToGoType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("map[%s]%s", key, subtype), nil
	case parse.DataTypeList:

		subtype, err := mapDataTypeDefinitionToGoType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[]%s", subtype), nil
	case parse.DataTypeCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%s.%s", convertPackageNameToGoPackage(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		} else {
			return util.EnsurePascalCase(dataType.CustomType), nil
		}
	}

	return mapDataTypeToGoType(dataType.Type)
}

func mapDataTypeComparableToGoType(dataType parse.DataTypeComparable) (string, error) {
	switch dataType {
	case parse.DataTypeComparableUInt8:
		return "uint8", nil
	case parse.DataTypeComparableUInt16:
		return "uint16", nil
	case parse.DataTypeComparableUInt32:
		return "uint32", nil
	case parse.DataTypeComparableUInt64:
		return "uint64", nil
	case parse.DataTypeComparableInt8:
		return "int8", nil
	case parse.DataTypeComparableInt16:
		return "int16", nil
	case parse.DataTypeComparableInt32:
		return "int32", nil
	case parse.DataTypeComparableInt64:
		return "int64", nil
	case parse.DataTypeComparableString:
		return "string", nil
	case parse.DataTypeComparableUUID:
		return "uuid.UUID", nil
	case parse.DataTypeComparableFloat32:
		return "float32", nil
	case parse.DataTypeComparableFloat64:
		return "float64", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapDataTypeComparableDefinitionToGoType(dataType *parse.DataTypeComparableDefinition) (string, error) {
	switch dataType.Type {
	case parse.DataTypeComparableCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%s.%s", convertPackageNameToGoPackage(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		} else {
			return util.EnsurePascalCase(dataType.CustomType), nil
		}
	}
	return mapDataTypeComparableToGoType(dataType.Type)
}

type FunctionCallArgs struct {
	VariableName string
}

const (
	// calc bytes templates
	byteSizeByteTemplateStr      = `serialize.BitSizeUInt8({{.VariableName}})`
	byteSizeBoolTemplateStr      = `serialize.BitSizeBool({{.VariableName}})`
	byteSizeUInt8TemplateStr     = `serialize.BitSizeUInt8({{.VariableName}})`
	byteSizeInt8TemplateStr      = `serialize.BitSizeInt8({{.VariableName}})`
	byteSizeUInt16TemplateStr    = `serialize.BitSizeUInt16({{.VariableName}})`
	byteSizeInt16TemplateStr     = `serialize.BitSizeInt16({{.VariableName}})`
	byteSizeUInt32TemplateStr    = `serialize.BitSizeUInt32({{.VariableName}})`
	byteSizeInt32TemplateStr     = `serialize.BitSizeInt32({{.VariableName}})`
	byteSizeUInt64TemplateStr    = `serialize.BitSizeUInt64({{.VariableName}})`
	byteSizeInt64TemplateStr     = `serialize.BitSizeInt64({{.VariableName}})`
	byteSizeFloat32TemplateStr   = `serialize.BitSizeFloat32({{.VariableName}})`
	byteSizeFloat64TemplateStr   = `serialize.BitSizeFloat64({{.VariableName}})`
	byteSizeStringTemplateStr    = `serialize.BitSizeString({{.VariableName}})`
	byteSizeTimestampTemplateStr = `serialize.BitSizeTime({{.VariableName}})`
	byteSizeUUIDTemplateStr      = `serialize.BitSizeUUID({{.VariableName}})`
	byteSizeCustomTemplateStr    = `{{.VariableName}}.BitSize()`
	// serialization templates
	serializeByteTemplateStr      = `serialize.SerializeUInt8(writer, {{.VariableName}})`
	serializeBoolTemplateStr      = `serialize.SerializeBool(writer, {{.VariableName}})`
	serializeUInt8TemplateStr     = `serialize.SerializeUInt8(writer, {{.VariableName}})`
	serializeInt8TemplateStr      = `serialize.SerializeInt8(writer, {{.VariableName}})`
	serializeUInt16TemplateStr    = `serialize.SerializeUInt16(writer, {{.VariableName}})`
	serializeInt16TemplateStr     = `serialize.SerializeInt16(writer, {{.VariableName}})`
	serializeUInt32TemplateStr    = `serialize.SerializeUInt32(writer, {{.VariableName}})`
	serializeInt32TemplateStr     = `serialize.SerializeInt32(writer, {{.VariableName}})`
	serializeUInt64TemplateStr    = `serialize.SerializeUInt64(writer, {{.VariableName}})`
	serializeInt64TemplateStr     = `serialize.SerializeInt64(writer, {{.VariableName}})`
	serializeFloat32TemplateStr   = `serialize.SerializeFloat32(writer, {{.VariableName}})`
	serializeFloat64TemplateStr   = `serialize.SerializeFloat64(writer, {{.VariableName}})`
	serializeStringTemplateStr    = `serialize.SerializeString(writer, {{.VariableName}})`
	serializeTimestampTemplateStr = `serialize.SerializeTime(writer, {{.VariableName}})`
	serializeUUIDTemplateStr      = `serialize.SerializeUUID(writer, {{.VariableName}})`
	serializeCustomTemplateStr    = `{{.VariableName}}.Serialize(writer)`
	// deserialization templates
	deserializeByteTemplateStr      = `serialize.DeserializeUInt8(&{{.VariableName}}, reader)`
	deserializeBoolTemplateStr      = `serialize.DeserializeBool(&{{.VariableName}}, reader)`
	deserializeUInt8TemplateStr     = `serialize.DeserializeUInt8(&{{.VariableName}}, reader)`
	deserializeInt8TemplateStr      = `serialize.DeserializeInt8(&{{.VariableName}}, reader)`
	deserializeUInt16TemplateStr    = `serialize.DeserializeUInt16(&{{.VariableName}}, reader)`
	deserializeInt16TemplateStr     = `serialize.DeserializeInt16(&{{.VariableName}}, reader)`
	deserializeUInt32TemplateStr    = `serialize.DeserializeUInt32(&{{.VariableName}}, reader)`
	deserializeInt32TemplateStr     = `serialize.DeserializeInt32(&{{.VariableName}}, reader)`
	deserializeUInt64TemplateStr    = `serialize.DeserializeUInt64(&{{.VariableName}}, reader)`
	deserializeInt64TemplateStr     = `serialize.DeserializeInt64(&{{.VariableName}}, reader)`
	deserializeFloat32TemplateStr   = `serialize.DeserializeFloat32(&{{.VariableName}}, reader)`
	deserializeFloat64TemplateStr   = `serialize.DeserializeFloat64(&{{.VariableName}}, reader)`
	deserializeStringTemplateStr    = `serialize.DeserializeString(&{{.VariableName}}, reader)`
	deserializeTimestampTemplateStr = `serialize.DeserializeTime(&{{.VariableName}}, reader)`
	deserializeUUIDTemplateStr      = `serialize.DeserializeUUID(&{{.VariableName}}, reader)`
	deserializeCustomTemplateStr    = `{{.VariableName}}.Deserialize(reader)`
)

var (
	// calc byte size templates
	byteSizeByteTemplate      = template.Must(template.New("byteSizeByteTemplateGo").Parse(byteSizeByteTemplateStr))
	byteSizeBoolTemplate      = template.Must(template.New("byteSizeBoolTemplateGo").Parse(byteSizeBoolTemplateStr))
	byteSizeUInt8Template     = template.Must(template.New("byteSizeUInt8TemplateGo").Parse(byteSizeUInt8TemplateStr))
	byteSizeInt8Template      = template.Must(template.New("byteSizeInt8TemplateGo").Parse(byteSizeInt8TemplateStr))
	byteSizeUInt16Template    = template.Must(template.New("byteSizeUInt16TemplateGo").Parse(byteSizeUInt16TemplateStr))
	byteSizeInt16Template     = template.Must(template.New("byteSizeInt16TemplateGo").Parse(byteSizeInt16TemplateStr))
	byteSizeUInt32Template    = template.Must(template.New("byteSizeUInt32TemplateGo").Parse(byteSizeUInt32TemplateStr))
	byteSizeInt32Template     = template.Must(template.New("byteSizeInt32TemplateGo").Parse(byteSizeInt32TemplateStr))
	byteSizeUInt64Template    = template.Must(template.New("byteSizeUInt64TemplateGo").Parse(byteSizeUInt64TemplateStr))
	byteSizeInt64Template     = template.Must(template.New("byteSizeInt64TemplateGo").Parse(byteSizeInt64TemplateStr))
	byteSizeFloat32Template   = template.Must(template.New("byteSizeFloat32TemplateGo").Parse(byteSizeFloat32TemplateStr))
	byteSizeFloat64Template   = template.Must(template.New("byteSizeFloat64TemplateGo").Parse(byteSizeFloat64TemplateStr))
	byteSizeStringTemplate    = template.Must(template.New("byteSizeStringTemplateGo").Parse(byteSizeStringTemplateStr))
	byteSizeTimestampTemplate = template.Must(template.New("byteSizeTimestampTemplateGo").Parse(byteSizeTimestampTemplateStr))
	byteSizeUUIDTemplate      = template.Must(template.New("byteSizeUUIDTemplateGo").Parse(byteSizeUUIDTemplateStr))
	byteSizeCustomTemplate    = template.Must(template.New("byteSizeCustomTemplateGo").Parse(byteSizeCustomTemplateStr))
	// serialization templates
	serializeByteTemplate      = template.Must(template.New("serializeByteTemplateGo").Parse(serializeByteTemplateStr))
	serializeBoolTemplate      = template.Must(template.New("serializeBoolTemplateGo").Parse(serializeBoolTemplateStr))
	serializeUInt8Template     = template.Must(template.New("serializeUInt8TemplateGo").Parse(serializeUInt8TemplateStr))
	serializeInt8Template      = template.Must(template.New("serializeInt8TemplateGo").Parse(serializeInt8TemplateStr))
	serializeUInt16Template    = template.Must(template.New("serializeUInt16TemplateGo").Parse(serializeUInt16TemplateStr))
	serializeInt16Template     = template.Must(template.New("serializeInt16TemplateGo").Parse(serializeInt16TemplateStr))
	serializeUInt32Template    = template.Must(template.New("serializeUInt32TemplateGo").Parse(serializeUInt32TemplateStr))
	serializeInt32Template     = template.Must(template.New("serializeInt32TemplateGo").Parse(serializeInt32TemplateStr))
	serializeUInt64Template    = template.Must(template.New("serializeUInt64TemplateGo").Parse(serializeUInt64TemplateStr))
	serializeInt64Template     = template.Must(template.New("serializeInt64TemplateGo").Parse(serializeInt64TemplateStr))
	serializeFloat32Template   = template.Must(template.New("serializeFloat32TemplateGo").Parse(serializeFloat32TemplateStr))
	serializeFloat64Template   = template.Must(template.New("serializeFloat64TemplateGo").Parse(serializeFloat64TemplateStr))
	serializeStringTemplate    = template.Must(template.New("serializeStringTemplateGo").Parse(serializeStringTemplateStr))
	serializeTimestampTemplate = template.Must(template.New("serializeTimestampTemplateGo").Parse(serializeTimestampTemplateStr))
	serializeUUIDTemplate      = template.Must(template.New("serializeUUIDTemplateGo").Parse(serializeUUIDTemplateStr))
	serializeCustomTemplate    = template.Must(template.New("serializeCustomTemplateGo").Parse(serializeCustomTemplateStr))
	// deserialization templates
	deserializeByteTemplate      = template.Must(template.New("deserializeByteTemplateGo").Parse(deserializeByteTemplateStr))
	deserializeBoolTemplate      = template.Must(template.New("deserializeBoolTemplateGo").Parse(deserializeBoolTemplateStr))
	deserializeUInt8Template     = template.Must(template.New("deserializeUInt8TemplateGo").Parse(deserializeUInt8TemplateStr))
	deserializeInt8Template      = template.Must(template.New("deserializeInt8TemplateGo").Parse(deserializeInt8TemplateStr))
	deserializeUInt16Template    = template.Must(template.New("deserializeUInt16TemplateGo").Parse(deserializeUInt16TemplateStr))
	deserializeInt16Template     = template.Must(template.New("deserializeInt16TemplateGo").Parse(deserializeInt16TemplateStr))
	deserializeUInt32Template    = template.Must(template.New("deserializeUInt32TemplateGo").Parse(deserializeUInt32TemplateStr))
	deserializeInt32Template     = template.Must(template.New("deserializeInt32TemplateGo").Parse(deserializeInt32TemplateStr))
	deserializeUInt64Template    = template.Must(template.New("deserializeUInt64TemplateGo").Parse(deserializeUInt64TemplateStr))
	deserializeInt64Template     = template.Must(template.New("deserializeInt64TemplateGo").Parse(deserializeInt64TemplateStr))
	deserializeFloat32Template   = template.Must(template.New("deserializeFloat32TemplateGo").Parse(deserializeFloat32TemplateStr))
	deserializeFloat64Template   = template.Must(template.New("deserializeFloat64TemplateGo").Parse(deserializeFloat64TemplateStr))
	deserializeStringTemplate    = template.Must(template.New("deserializeStringTemplateGo").Parse(deserializeStringTemplateStr))
	deserializeTimestampTemplate = template.Must(template.New("deserializeTimestampTemplateGo").Parse(deserializeTimestampTemplateStr))
	deserializeUUIDTemplate      = template.Must(template.New("deserializeUUIDTemplateGo").Parse(deserializeUUIDTemplateStr))
	deserializeCustomTemplate    = template.Must(template.New("deserializeCustomTemplateGo").Parse(deserializeCustomTemplateStr))
)

func getDataTypeComparableMethodSuffix(dataType parse.DataTypeComparable) (string, error) {
	switch dataType {
	case parse.DataTypeComparableUInt8:
		return "UInt8", nil
	case parse.DataTypeComparableUInt16:
		return "UInt16", nil
	case parse.DataTypeComparableUInt32:
		return "UInt32", nil
	case parse.DataTypeComparableUInt64:
		return "UInt64", nil
	case parse.DataTypeComparableInt8:
		return "Int8", nil
	case parse.DataTypeComparableInt16:
		return "Int16", nil
	case parse.DataTypeComparableInt32:
		return "Int32", nil
	case parse.DataTypeComparableInt64:
		return "Int64", nil
	case parse.DataTypeComparableString:
		return "String", nil
	case parse.DataTypeComparableFloat32:
		return "Float32", nil
	case parse.DataTypeComparableFloat64:
		return "Float64", nil
	case parse.DataTypeComparableUUID:
		return "UUID", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func getDataTypeComparableDefinitionMethodSuffix(dataType *parse.DataTypeComparableDefinition) (string, error) {
	switch dataType.Type {
	case parse.DataTypeComparableCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%sPkg%s", util.EnsurePascalCase(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		}
		return util.EnsurePascalCase(dataType.CustomType), nil
	}
	return getDataTypeComparableMethodSuffix(dataType.Type)
}

func getDataTypeMethodSuffix(dataType parse.DataType) (string, error) {
	switch dataType {
	case parse.DataTypeByte:
		return "Byte", nil
	case parse.DataTypeBool:
		return "Bool", nil
	case parse.DataTypeUInt8:
		return "UInt8", nil
	case parse.DataTypeUInt16:
		return "UInt16", nil
	case parse.DataTypeUInt32:
		return "UInt32", nil
	case parse.DataTypeUInt64:
		return "UInt64", nil
	case parse.DataTypeInt8:
		return "Int8", nil
	case parse.DataTypeInt16:
		return "Int16", nil
	case parse.DataTypeInt32:
		return "Int32", nil
	case parse.DataTypeInt64:
		return "Int64", nil
	case parse.DataTypeString:
		return "String", nil
	case parse.DataTypeTimestamp:
		return "Time", nil
	case parse.DataTypeFloat32:
		return "Float32", nil
	case parse.DataTypeFloat64:
		return "Float64", nil
	case parse.DataTypeUUID:
		return "UUID", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func getDataTypeDefinitionMethodSuffix(dataType *parse.DataTypeDefinition) (string, error) {
	switch dataType.Type {
	case parse.DataTypeMap:
		key, err := getDataTypeComparableDefinitionMethodSuffix(dataType.Key)
		if err != nil {
			return "", err
		}
		subNames, err := getDataTypeDefinitionMethodSuffix(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Map%s%s", key, subNames), nil

	case parse.DataTypeList:

		subNames, err := getDataTypeDefinitionMethodSuffix(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("List%s", subNames), nil

	case parse.DataTypeCustom:
		if dataType.ImportedFromOtherPackage {
			return fmt.Sprintf("%sPkg%s", util.EnsurePascalCase(dataType.CustomTypePackage), util.EnsurePascalCase(dataType.CustomType)), nil
		}
		return util.EnsurePascalCase(dataType.CustomType), nil

	}

	return getDataTypeMethodSuffix(dataType.Type)
}

func generateBitSizeContainerMethod(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	fullTypeName, err := getDataTypeDefinitionMethodSuffix(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_BitSize%s", util.EnsureCamelCase(messageName), fullTypeName)

	serializationMethodCode := map[string]string{}

	argType, err := mapDataTypeDefinitionToGoType(dataType)
	if err != nil {
		return "", nil, err
	}

	valueMethodCall, methodCode, err := generateFieldBitSizeMethodCall(messageName, "v", dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	args := BitSizeContainerMethodArgs{
		FullMethodName:             methodFullName,
		ArgType:                    argType,
		ValueTypeBitSizeMethodCall: valueMethodCall,
	}

	buf := &bytes.Buffer{}
	if dataType.Type == parse.DataTypeList {

		// list

		err = listBitSizeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else if dataType.Type == parse.DataTypeMap {

		// map

		keyMethodCall, err := generateKeyFieldBitSizeMethodCall("k", dataType.Key.Type)
		if err != nil {
			return "", nil, err
		}
		args.KeyTypeBitSizeMethodCall = keyMethodCall

		err = mapBitSizeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else {
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	serializationMethodCode[methodFullName] = buf.String()
	serializationMethodCode = util.MergeMap(serializationMethodCode, methodCode)

	fullMethodCall := fmt.Sprintf("%s(%s)", methodFullName, varName)

	return fullMethodCall, serializationMethodCode, nil
}

func generateSerializeContainerMethod(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	fullTypeName, err := getDataTypeDefinitionMethodSuffix(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_Serialize%s", util.EnsureCamelCase(messageName), fullTypeName)

	serializationMethodCode := map[string]string{}

	argType, err := mapDataTypeDefinitionToGoType(dataType)
	if err != nil {
		return "", nil, err
	}

	valueMethodCall, methodCode, err := generateFieldSerializationMethodCall(messageName, "v", dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	args := SerializeContainerMethodArgs{
		FullMethodName:               methodFullName,
		ArgType:                      argType,
		ValueTypeSerializeMethodCall: valueMethodCall,
	}

	buf := &bytes.Buffer{}
	if dataType.Type == parse.DataTypeList {

		// list

		err = listSerializeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else if dataType.Type == parse.DataTypeMap {

		// map

		keyMethodCall, err := generateKeyFieldSerializationMethodCall("k", dataType.Key.Type)
		if err != nil {
			return "", nil, err
		}
		args.KeyTypeSerializeMethodCall = keyMethodCall

		err = mapSerializeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else {
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	serializationMethodCode[methodFullName] = buf.String()
	serializationMethodCode = util.MergeMap(serializationMethodCode, methodCode)

	fullMethodCall := fmt.Sprintf("%s(writer, %s)", methodFullName, varName)

	return fullMethodCall, serializationMethodCode, nil
}

func generateDeserializeContainerMethod(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	fullTypeName, err := getDataTypeDefinitionMethodSuffix(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_Deserialize%s", util.EnsureCamelCase(messageName), fullTypeName)

	deserializationMethodCode := map[string]string{}

	argType, err := mapDataTypeDefinitionToGoType(dataType)
	if err != nil {
		return "", nil, err
	}

	valueMethodCall, methodCode, err := generateFieldDeserializationMethodCall(messageName, "v", dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	valueType, err := mapDataTypeDefinitionToGoType(dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	args := DeserializeContainerMethodArgs{
		FullMethodName:                 methodFullName,
		ArgType:                        argType,
		ValueTypeDeserializeMethodCall: valueMethodCall,
		ValueType:                      valueType,
	}

	buf := &bytes.Buffer{}
	if dataType.Type == parse.DataTypeList {

		// list

		err = listDeserializeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else if dataType.Type == parse.DataTypeMap {

		// map

		keyType, err := mapDataTypeComparableDefinitionToGoType(dataType.Key)
		if err != nil {
			return "", nil, err
		}

		keyMethodCall, err := generateKeyFieldDeserializationMethodCall("k", dataType.Key.Type)
		if err != nil {
			return "", nil, err
		}
		args.KeyTypeDeserializeMethodCall = keyMethodCall
		args.KeyType = keyType

		err = mapDeserializeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else {
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	deserializationMethodCode[methodFullName] = buf.String()
	deserializationMethodCode = util.MergeMap(deserializationMethodCode, methodCode)

	fullMethodCall := fmt.Sprintf("%s(&%s, reader)", methodFullName, varName)

	return fullMethodCall, deserializationMethodCode, nil
}

func generateKeyFieldBitSizeMethodCall(varName string, dataType parse.DataTypeComparable) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataTypeComparableUInt8:
		tmpl = byteSizeUInt8Template
	case parse.DataTypeComparableUInt16:
		tmpl = byteSizeUInt16Template
	case parse.DataTypeComparableUInt32:
		tmpl = byteSizeUInt32Template
	case parse.DataTypeComparableUInt64:
		tmpl = byteSizeUInt64Template
	case parse.DataTypeComparableInt8:
		tmpl = byteSizeInt8Template
	case parse.DataTypeComparableInt16:
		tmpl = byteSizeInt16Template
	case parse.DataTypeComparableInt32:
		tmpl = byteSizeInt32Template
	case parse.DataTypeComparableInt64:
		tmpl = byteSizeInt64Template
	case parse.DataTypeComparableFloat32:
		tmpl = byteSizeFloat32Template
	case parse.DataTypeComparableFloat64:
		tmpl = byteSizeFloat64Template
	case parse.DataTypeComparableString:
		tmpl = byteSizeStringTemplate
	case parse.DataTypeComparableUUID:
		tmpl = byteSizeUUIDTemplate
	case parse.DataTypeComparableCustom:
		tmpl = byteSizeCustomTemplate
	default:
		return "", fmt.Errorf("unrecognized key type: %v", dataType)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateKeyFieldSerializationMethodCall(varName string, dataType parse.DataTypeComparable) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataTypeComparableUInt8:
		tmpl = serializeUInt8Template
	case parse.DataTypeComparableUInt16:
		tmpl = serializeUInt16Template
	case parse.DataTypeComparableUInt32:
		tmpl = serializeUInt32Template
	case parse.DataTypeComparableUInt64:
		tmpl = serializeUInt64Template
	case parse.DataTypeComparableInt8:
		tmpl = serializeInt8Template
	case parse.DataTypeComparableInt16:
		tmpl = serializeInt16Template
	case parse.DataTypeComparableInt32:
		tmpl = serializeInt32Template
	case parse.DataTypeComparableInt64:
		tmpl = serializeInt64Template
	case parse.DataTypeComparableFloat32:
		tmpl = serializeFloat32Template
	case parse.DataTypeComparableFloat64:
		tmpl = serializeFloat64Template
	case parse.DataTypeComparableString:
		tmpl = serializeStringTemplate
	case parse.DataTypeComparableUUID:
		tmpl = serializeUUIDTemplate
	case parse.DataTypeComparableCustom:
		tmpl = serializeCustomTemplate
	default:
		return "", fmt.Errorf("unrecognized key type: %v", dataType)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateKeyFieldDeserializationMethodCall(varName string, dataType parse.DataTypeComparable) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataTypeComparableUInt8:
		tmpl = deserializeUInt8Template
	case parse.DataTypeComparableUInt16:
		tmpl = deserializeUInt16Template
	case parse.DataTypeComparableUInt32:
		tmpl = deserializeUInt32Template
	case parse.DataTypeComparableUInt64:
		tmpl = deserializeUInt64Template
	case parse.DataTypeComparableInt8:
		tmpl = deserializeInt8Template
	case parse.DataTypeComparableInt16:
		tmpl = deserializeInt16Template
	case parse.DataTypeComparableInt32:
		tmpl = deserializeInt32Template
	case parse.DataTypeComparableInt64:
		tmpl = deserializeInt64Template
	case parse.DataTypeComparableFloat32:
		tmpl = deserializeFloat32Template
	case parse.DataTypeComparableFloat64:
		tmpl = deserializeFloat64Template
	case parse.DataTypeComparableString:
		tmpl = deserializeStringTemplate
	case parse.DataTypeComparableUUID:
		tmpl = deserializeUUIDTemplate
	case parse.DataTypeComparableCustom:
		tmpl = deserializeCustomTemplate
	default:
		return "", fmt.Errorf("unrecognized key type: %v", dataType)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateFieldBitSizeMethodCall(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType.Type {
	case parse.DataTypeByte:
		tmpl = byteSizeByteTemplate
	case parse.DataTypeBool:
		tmpl = byteSizeBoolTemplate
	case parse.DataTypeUInt8:
		tmpl = byteSizeUInt8Template
	case parse.DataTypeUInt16:
		tmpl = byteSizeUInt16Template
	case parse.DataTypeUInt32:
		tmpl = byteSizeUInt32Template
	case parse.DataTypeUInt64:
		tmpl = byteSizeUInt64Template
	case parse.DataTypeInt8:
		tmpl = byteSizeInt8Template
	case parse.DataTypeInt16:
		tmpl = byteSizeInt16Template
	case parse.DataTypeInt32:
		tmpl = byteSizeInt32Template
	case parse.DataTypeInt64:
		tmpl = byteSizeInt64Template
	case parse.DataTypeFloat32:
		tmpl = byteSizeFloat32Template
	case parse.DataTypeFloat64:
		tmpl = byteSizeFloat64Template
	case parse.DataTypeString:
		tmpl = byteSizeStringTemplate
	case parse.DataTypeTimestamp:
		tmpl = byteSizeTimestampTemplate
	case parse.DataTypeUUID:
		tmpl = byteSizeUUIDTemplate
	case parse.DataTypeCustom:
		tmpl = byteSizeCustomTemplate
	case parse.DataTypeMap,
		parse.DataTypeList:
		// special case for containers
		return generateBitSizeContainerMethod(messageName, varName, dataType)

	default:
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", nil, err
	}

	return buf.String(), nil, nil
}

func generateFieldSerializationMethodCall(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType.Type {
	case parse.DataTypeByte:
		tmpl = serializeByteTemplate
	case parse.DataTypeBool:
		tmpl = serializeBoolTemplate
	case parse.DataTypeUInt8:
		tmpl = serializeUInt8Template
	case parse.DataTypeUInt16:
		tmpl = serializeUInt16Template
	case parse.DataTypeUInt32:
		tmpl = serializeUInt32Template
	case parse.DataTypeUInt64:
		tmpl = serializeUInt64Template
	case parse.DataTypeInt8:
		tmpl = serializeInt8Template
	case parse.DataTypeInt16:
		tmpl = serializeInt16Template
	case parse.DataTypeInt32:
		tmpl = serializeInt32Template
	case parse.DataTypeInt64:
		tmpl = serializeInt64Template
	case parse.DataTypeFloat32:
		tmpl = serializeFloat32Template
	case parse.DataTypeFloat64:
		tmpl = serializeFloat64Template
	case parse.DataTypeString:
		tmpl = serializeStringTemplate
	case parse.DataTypeTimestamp:
		tmpl = serializeTimestampTemplate
	case parse.DataTypeUUID:
		tmpl = serializeUUIDTemplate
	case parse.DataTypeCustom:
		tmpl = serializeCustomTemplate
	case parse.DataTypeMap,
		parse.DataTypeList:
		// special case for containers
		return generateSerializeContainerMethod(messageName, varName, dataType)

	default:
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", nil, err
	}

	return buf.String(), nil, nil
}

func generateFieldDeserializationMethodCall(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType.Type {
	case parse.DataTypeByte:
		tmpl = deserializeByteTemplate
	case parse.DataTypeBool:
		tmpl = deserializeBoolTemplate
	case parse.DataTypeUInt8:
		tmpl = deserializeUInt8Template
	case parse.DataTypeUInt16:
		tmpl = deserializeUInt16Template
	case parse.DataTypeUInt32:
		tmpl = deserializeUInt32Template
	case parse.DataTypeUInt64:
		tmpl = deserializeUInt64Template
	case parse.DataTypeInt8:
		tmpl = deserializeInt8Template
	case parse.DataTypeInt16:
		tmpl = deserializeInt16Template
	case parse.DataTypeInt32:
		tmpl = deserializeInt32Template
	case parse.DataTypeInt64:
		tmpl = deserializeInt64Template
	case parse.DataTypeFloat32:
		tmpl = deserializeFloat32Template
	case parse.DataTypeFloat64:
		tmpl = deserializeFloat64Template
	case parse.DataTypeString:
		tmpl = deserializeStringTemplate
	case parse.DataTypeTimestamp:
		tmpl = deserializeTimestampTemplate
	case parse.DataTypeUUID:
		tmpl = deserializeUUIDTemplate
	case parse.DataTypeCustom:
		tmpl = deserializeCustomTemplate
	case parse.DataTypeMap,
		parse.DataTypeList:
		// special case for containers
		return generateDeserializeContainerMethod(messageName, varName, dataType)

	default:
		return "", nil, fmt.Errorf("unrecognized type: %v", dataType.Type)
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, args)
	if err != nil {
		return "", nil, err
	}

	return buf.String(), nil, nil
}

func valuesSortedByKey(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]string, 0, len(m))
	for _, k := range keys {
		values = append(values, m[k])
	}

	return values
}

func generateMessageBitSizeMethod(msg *parse.MessageDefinition) (string, error) {
	args := BitSizeMethodArgs{
		MessageNameFirstLetter: util.FirstLetterAsLowercase(msg.Name),
		MessageNamePascalCase:  util.EnsurePascalCase(msg.Name),
	}

	additionalFunctionCode := map[string]string{}

	for _, field := range msg.FieldsByIndex() {

		fieldName := fmt.Sprintf("%s.%s", util.FirstLetterAsLowercase(msg.Name), util.EnsurePascalCase(field.Name))

		fieldBitSizeMethodCall, methodCode, err := generateFieldBitSizeMethodCall(msg.Name, fieldName, field.DataTypeDefinition)
		if err != nil {
			return "", err
		}
		additionalFunctionCode = util.MergeMap(additionalFunctionCode, methodCode)
		args.FieldBitSizeMethodCalls = append(args.FieldBitSizeMethodCalls, fieldBitSizeMethodCall)
	}

	buf := &bytes.Buffer{}
	err := messageBitSizeMethodTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	code := ""
	for _, c := range valuesSortedByKey(additionalFunctionCode) {
		code += c + "\n"
	}
	return code + "\n" + buf.String(), nil
}

func generateMessageSerializationMethod(msg *parse.MessageDefinition) (string, error) {
	args := SerializeMethodArgs{
		MessageNameFirstLetter: util.FirstLetterAsLowercase(msg.Name),
		MessageNamePascalCase:  util.EnsurePascalCase(msg.Name),
	}

	additionalFunctionCode := map[string]string{}

	for _, field := range msg.FieldsByIndex() {

		fieldName := fmt.Sprintf("%s.%s", util.FirstLetterAsLowercase(msg.Name), util.EnsurePascalCase(field.Name))

		fieldSerializeMethodCall, methodCode, err := generateFieldSerializationMethodCall(msg.Name, fieldName, field.DataTypeDefinition)
		if err != nil {
			return "", err
		}
		additionalFunctionCode = util.MergeMap(additionalFunctionCode, methodCode)
		args.FieldSerializeMethodCalls = append(args.FieldSerializeMethodCalls, fieldSerializeMethodCall)
	}

	buf := &bytes.Buffer{}
	err := messageSerializeMethodTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	code := ""
	for _, c := range valuesSortedByKey(additionalFunctionCode) {
		code += c + "\n"
	}
	return code + "\n" + buf.String(), nil
}

func generateMessageDeserializationMethod(msg *parse.MessageDefinition) (string, error) {
	args := DeserializeMethodArgs{
		MessageNameFirstLetter: util.FirstLetterAsLowercase(msg.Name),
		MessageNamePascalCase:  util.EnsurePascalCase(msg.Name),
	}

	additionalFunctionCode := map[string]string{}

	for _, field := range msg.FieldsByIndex() {

		fieldName := fmt.Sprintf("%s.%s", util.FirstLetterAsLowercase(msg.Name), util.EnsurePascalCase(field.Name))

		fieldDeserializeMethodCall, methodCode, err := generateFieldDeserializationMethodCall(msg.Name, fieldName, field.DataTypeDefinition)
		if err != nil {
			return "", err
		}
		additionalFunctionCode = util.MergeMap(additionalFunctionCode, methodCode)
		args.FieldDeserializeMethodCalls = append(args.FieldDeserializeMethodCalls, fieldDeserializeMethodCall)
	}

	buf := &bytes.Buffer{}
	err := messageDeserializeMethodTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	code := ""
	for _, c := range valuesSortedByKey(additionalFunctionCode) {
		code += c + "\n"
	}
	return code + "\n" + buf.String(), nil
}

func getMessageFieldArg(field *parse.MessageFieldDefinition) (MessageFieldArgs, error) {
	goType, err := mapDataTypeDefinitionToGoType(field.DataTypeDefinition)
	if err != nil {
		return MessageFieldArgs{}, err
	}
	return MessageFieldArgs{
		FieldNamePascalCase: util.EnsurePascalCase(field.Name),
		FieldNameSnakeCase:  util.EnsureSnakeCase(field.Name),
		FieldType:           goType,
	}, nil
}

func generateMessageGoCode(msg *parse.MessageDefinition) (string, error) {

	args := MessageArgs{
		MessageNamePascalCase:  util.EnsurePascalCase(msg.Name),
		MessageNameFirstLetter: util.FirstLetterAsLowercase(msg.Name),
		MessageFields:          []MessageFieldArgs{},
	}
	for _, field := range msg.FieldsByIndex() {
		fieldArg, err := getMessageFieldArg(field)
		if err != nil {
			return "", err
		}
		args.MessageFields = append(args.MessageFields, fieldArg)
	}

	byteSizeCode, err := generateMessageBitSizeMethod(msg)
	if err != nil {
		return "", err
	}

	serializeCode, err := generateMessageSerializationMethod(msg)
	if err != nil {
		return "", err
	}

	deserializeCode, err := generateMessageDeserializationMethod(msg)
	if err != nil {
		return "", err
	}

	args.BitSizeCode = byteSizeCode
	args.SerializeCode = serializeCode
	args.DeserializeCode = deserializeCode

	buf := &bytes.Buffer{}
	err = messageTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
