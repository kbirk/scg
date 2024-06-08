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
	CalcByteSizeCode       string
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

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) ToBytes() []byte {
	size := {{.MessageNameFirstLetter}}.CalcByteSize()
	writer := serialize.NewFixedSizeWriter(size)
	{{.MessageNameFirstLetter}}.Serialize(writer)
	return writer.Bytes()
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) FromBytes(bs []byte) error {
	return {{.MessageNameFirstLetter}}.Deserialize(serialize.NewReader(bs))
}

func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) ByteSize() int {
	return {{.MessageNameFirstLetter}}.CalcByteSize()
}
{{.CalcByteSizeCode}}
{{.SerializeCode}}
{{.DeserializeCode}}
`

type CalcByteSizeMethodArgs struct {
	MessageNameFirstLetter       string
	MessageNamePascalCase        string
	FieldCalcByteSizeMethodCalls []string
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

const messageCalcByteSizeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) CalcByteSize() int {
	size := 0{{range .FieldCalcByteSizeMethodCalls}}
	size += {{.}}{{end}}
	return size
}`

const messageSerializeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) Serialize(writer *serialize.FixedSizeWriter) {
	{{range .FieldSerializeMethodCalls}}
	{{.}}{{end}}
}`

const messageDeserializeMethodTemplateStr = `
func ({{.MessageNameFirstLetter}} *{{.MessageNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	var err error{{range .FieldDeserializeMethodCalls}}
	err = {{.}}
	if err != nil {
		return err
	}{{end}}
	return nil
}`

type CalcByteSizeContainerMethodArgs struct {
	FullMethodName                  string
	ArgType                         string
	KeyTypeCalcByteSizeMethodCall   string
	ValueTypeCalcByteSizeMethodCall string
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

const mapCalcByteSizeMethodTemplateStr = `
func {{.FullMethodName}}(arg {{.ArgType}}) int {
	size := 4
	for k, v := range arg {
		size += {{.KeyTypeCalcByteSizeMethodCall}} + {{.ValueTypeCalcByteSizeMethodCall}}
	}
	return size
}`

const mapSerializeMethodTemplateStr = `
func {{.FullMethodName}}(writer *serialize.FixedSizeWriter, arg {{.ArgType}}) error {
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

const listCalcByteSizeMethodTemplateStr = `
func {{.FullMethodName}}(arg {{.ArgType}}) int {
	size := 4
	for _, v := range arg {
		size += {{.ValueTypeCalcByteSizeMethodCall}}
	}
	return size
}`

const listSerializeMethodTemplateStr = `
func {{.FullMethodName}}(writer *serialize.FixedSizeWriter, arg {{.ArgType}}) error {
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
	messageTemplate                   = template.Must(template.New("messageTemplateGo").Parse(messageTemplateStr))
	messageCalcByteSizeMethodTemplate = template.Must(template.New("messageCalcByteSizeMethodTemplateGo").Parse(messageCalcByteSizeMethodTemplateStr))
	messageSerializeMethodTemplate    = template.Must(template.New("messageSerializeMethodTemplateGo").Parse(messageSerializeMethodTemplateStr))
	messageDeserializeMethodTemplate  = template.Must(template.New("messageDeserializeMethodTemplateGo").Parse(messageDeserializeMethodTemplateStr))
	// container methods
	mapCalcByteSizeMethodTemplate  = template.Must(template.New("mapCalcByteSizeMethodTemplateGo").Parse(mapCalcByteSizeMethodTemplateStr))
	listCalcByteSizeMethodTemplate = template.Must(template.New("listCalcByteSizeMethodTemplateGo").Parse(listCalcByteSizeMethodTemplateStr))
	mapSerializeMethodTemplate     = template.Must(template.New("mapSerializeMethodTemplateGo").Parse(mapSerializeMethodTemplateStr))
	listSerializeMethodTemplate    = template.Must(template.New("listSerializeMethodTemplateGo").Parse(listSerializeMethodTemplateStr))
	mapDeserializeMethodTemplate   = template.Must(template.New("mapDeserializeMethodTemplateGo").Parse(mapDeserializeMethodTemplateStr))
	listDeserializeMethodTemplate  = template.Must(template.New("listDeserializeMethodTemplateGo").Parse(listDeserializeMethodTemplateStr))
)

func mapComparableTypeToGoType(dataType parse.DataComparableType) (string, error) {
	switch dataType {
	case parse.DataComparableTypeUInt8:
		return "uint8", nil
	case parse.DataComparableTypeUInt16:
		return "uint16", nil
	case parse.DataComparableTypeUInt32:
		return "uint32", nil
	case parse.DataComparableTypeUInt64:
		return "uint64", nil
	case parse.DataComparableTypeInt8:
		return "int8", nil
	case parse.DataComparableTypeInt16:
		return "int16", nil
	case parse.DataComparableTypeInt32:
		return "int32", nil
	case parse.DataComparableTypeInt64:
		return "int64", nil
	case parse.DataComparableTypeString:
		return "string", nil
	case parse.DataComparableTypeFloat32:
		return "float32", nil
	case parse.DataComparableTypeFloat64:
		return "float64", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func mapTypeToGoType(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
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
	case parse.DataTypeFloat32:
		return "float32", nil
	case parse.DataTypeFloat64:
		return "float64", nil
	case parse.DataTypeMap:
		key, err := mapComparableTypeToGoType(dataType.Key)
		if err != nil {
			return "", err
		}
		subtype, err := mapTypeToGoType(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("map[%s]%s", key, subtype), nil
	case parse.DataTypeList:

		subtype, err := mapTypeToGoType(dataType.SubType)
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

	return "", fmt.Errorf("unrecognized type: %v", dataType.Type)
}

type FunctionCallArgs struct {
	VariableName string
}

const (
	// calc bytes templates
	calcByteSizeByteTemplateStr    = `serialize.CalcByteSizeUInt8({{.VariableName}})`
	calcByteSizeBoolTemplateStr    = `serialize.CalcByteSizeBool({{.VariableName}})`
	calcByteSizeUInt8TemplateStr   = `serialize.CalcByteSizeUInt8({{.VariableName}})`
	calcByteSizeInt8TemplateStr    = `serialize.CalcByteSizeInt8({{.VariableName}})`
	calcByteSizeUInt16TemplateStr  = `serialize.CalcByteSizeUInt16({{.VariableName}})`
	calcByteSizeInt16TemplateStr   = `serialize.CalcByteSizeInt16({{.VariableName}})`
	calcByteSizeUInt32TemplateStr  = `serialize.CalcByteSizeUInt32({{.VariableName}})`
	calcByteSizeInt32TemplateStr   = `serialize.CalcByteSizeInt32({{.VariableName}})`
	calcByteSizeUInt64TemplateStr  = `serialize.CalcByteSizeUInt64({{.VariableName}})`
	calcByteSizeInt64TemplateStr   = `serialize.CalcByteSizeInt64({{.VariableName}})`
	calcByteSizeFloat32TemplateStr = `serialize.CalcByteSizeFloat32({{.VariableName}})`
	calcByteSizeFloat64TemplateStr = `serialize.CalcByteSizeFloat64({{.VariableName}})`
	calcByteSizeStringTemplateStr  = `serialize.CalcByteSizeString({{.VariableName}})`
	calcByteSizeCustomTemplateStr  = `{{.VariableName}}.CalcByteSize()`
	// serialization templates
	serializeByteTemplateStr    = `serialize.SerializeUInt8(writer, {{.VariableName}})`
	serializeBoolTemplateStr    = `serialize.SerializeBool(writer, {{.VariableName}})`
	serializeUInt8TemplateStr   = `serialize.SerializeUInt8(writer, {{.VariableName}})`
	serializeInt8TemplateStr    = `serialize.SerializeInt8(writer, {{.VariableName}})`
	serializeUInt16TemplateStr  = `serialize.SerializeUInt16(writer, {{.VariableName}})`
	serializeInt16TemplateStr   = `serialize.SerializeInt16(writer, {{.VariableName}})`
	serializeUInt32TemplateStr  = `serialize.SerializeUInt32(writer, {{.VariableName}})`
	serializeInt32TemplateStr   = `serialize.SerializeInt32(writer, {{.VariableName}})`
	serializeUInt64TemplateStr  = `serialize.SerializeUInt64(writer, {{.VariableName}})`
	serializeInt64TemplateStr   = `serialize.SerializeInt64(writer, {{.VariableName}})`
	serializeFloat32TemplateStr = `serialize.SerializeFloat32(writer, {{.VariableName}})`
	serializeFloat64TemplateStr = `serialize.SerializeFloat64(writer, {{.VariableName}})`
	serializeStringTemplateStr  = `serialize.SerializeString(writer, {{.VariableName}})`
	serializeCustomTemplateStr  = `{{.VariableName}}.Serialize(writer)`
	// deserialization templates
	deserializeByteTemplateStr    = `serialize.DeserializeUInt8(&{{.VariableName}}, reader)`
	deserializeBoolTemplateStr    = `serialize.DeserializeBool(&{{.VariableName}}, reader)`
	deserializeUInt8TemplateStr   = `serialize.DeserializeUInt8(&{{.VariableName}}, reader)`
	deserializeInt8TemplateStr    = `serialize.DeserializeInt8(&{{.VariableName}}, reader)`
	deserializeUInt16TemplateStr  = `serialize.DeserializeUInt16(&{{.VariableName}}, reader)`
	deserializeInt16TemplateStr   = `serialize.DeserializeInt16(&{{.VariableName}}, reader)`
	deserializeUInt32TemplateStr  = `serialize.DeserializeUInt32(&{{.VariableName}}, reader)`
	deserializeInt32TemplateStr   = `serialize.DeserializeInt32(&{{.VariableName}}, reader)`
	deserializeUInt64TemplateStr  = `serialize.DeserializeUInt64(&{{.VariableName}}, reader)`
	deserializeInt64TemplateStr   = `serialize.DeserializeInt64(&{{.VariableName}}, reader)`
	deserializeFloat32TemplateStr = `serialize.DeserializeFloat32(&{{.VariableName}}, reader)`
	deserializeFloat64TemplateStr = `serialize.DeserializeFloat64(&{{.VariableName}}, reader)`
	deserializeStringTemplateStr  = `serialize.DeserializeString(&{{.VariableName}}, reader)`
	deserializeCustomTemplateStr  = `{{.VariableName}}.Deserialize(reader)`
)

var (
	// calc byte size templates
	calcByteSizeByteTemplate    = template.Must(template.New("calcByteSizeByteTemplateGo").Parse(calcByteSizeByteTemplateStr))
	calcByteSizeBoolTemplate    = template.Must(template.New("calcByteSizeBoolTemplateGo").Parse(calcByteSizeBoolTemplateStr))
	calcByteSizeUInt8Template   = template.Must(template.New("calcByteSizeUInt8TemplateGo").Parse(calcByteSizeUInt8TemplateStr))
	calcByteSizeInt8Template    = template.Must(template.New("calcByteSizeInt8TemplateGo").Parse(calcByteSizeInt8TemplateStr))
	calcByteSizeUInt16Template  = template.Must(template.New("calcByteSizeUInt16TemplateGo").Parse(calcByteSizeUInt16TemplateStr))
	calcByteSizeInt16Template   = template.Must(template.New("calcByteSizeInt16TemplateGo").Parse(calcByteSizeInt16TemplateStr))
	calcByteSizeUInt32Template  = template.Must(template.New("calcByteSizeUInt32TemplateGo").Parse(calcByteSizeUInt32TemplateStr))
	calcByteSizeInt32Template   = template.Must(template.New("calcByteSizeInt32TemplateGo").Parse(calcByteSizeInt32TemplateStr))
	calcByteSizeUInt64Template  = template.Must(template.New("calcByteSizeUInt64TemplateGo").Parse(calcByteSizeUInt64TemplateStr))
	calcByteSizeInt64Template   = template.Must(template.New("calcByteSizeInt64TemplateGo").Parse(calcByteSizeInt64TemplateStr))
	calcByteSizeFloat32Template = template.Must(template.New("calcByteSizeFloat32TemplateGo").Parse(calcByteSizeFloat32TemplateStr))
	calcByteSizeFloat64Template = template.Must(template.New("calcByteSizeFloat64TemplateGo").Parse(calcByteSizeFloat64TemplateStr))
	calcByteSizeStringTemplate  = template.Must(template.New("calcByteSizeStringTemplateGo").Parse(calcByteSizeStringTemplateStr))
	calcByteSizeCustomTemplate  = template.Must(template.New("calcByteSizeCustomTemplateGo").Parse(calcByteSizeCustomTemplateStr))
	// serialization templates
	serializeByteTemplate    = template.Must(template.New("serializeByteTemplateGo").Parse(serializeByteTemplateStr))
	serializeBoolTemplate    = template.Must(template.New("serializeBoolTemplateGo").Parse(serializeBoolTemplateStr))
	serializeUInt8Template   = template.Must(template.New("serializeUInt8TemplateGo").Parse(serializeUInt8TemplateStr))
	serializeInt8Template    = template.Must(template.New("serializeInt8TemplateGo").Parse(serializeInt8TemplateStr))
	serializeUInt16Template  = template.Must(template.New("serializeUInt16TemplateGo").Parse(serializeUInt16TemplateStr))
	serializeInt16Template   = template.Must(template.New("serializeInt16TemplateGo").Parse(serializeInt16TemplateStr))
	serializeUInt32Template  = template.Must(template.New("serializeUInt32TemplateGo").Parse(serializeUInt32TemplateStr))
	serializeInt32Template   = template.Must(template.New("serializeInt32TemplateGo").Parse(serializeInt32TemplateStr))
	serializeUInt64Template  = template.Must(template.New("serializeUInt64TemplateGo").Parse(serializeUInt64TemplateStr))
	serializeInt64Template   = template.Must(template.New("serializeInt64TemplateGo").Parse(serializeInt64TemplateStr))
	serializeFloat32Template = template.Must(template.New("serializeFloat32TemplateGo").Parse(serializeFloat32TemplateStr))
	serializeFloat64Template = template.Must(template.New("serializeFloat64TemplateGo").Parse(serializeFloat64TemplateStr))
	serializeStringTemplate  = template.Must(template.New("serializeStringTemplateGo").Parse(serializeStringTemplateStr))
	serializeCustomTemplate  = template.Must(template.New("serializeCustomTemplateGo").Parse(serializeCustomTemplateStr))
	// deserialization templates
	deserializeByteTemplate    = template.Must(template.New("deserializeByteTemplateGo").Parse(deserializeByteTemplateStr))
	deserializeBoolTemplate    = template.Must(template.New("deserializeBoolTemplateGo").Parse(deserializeBoolTemplateStr))
	deserializeUInt8Template   = template.Must(template.New("deserializeUInt8TemplateGo").Parse(deserializeUInt8TemplateStr))
	deserializeInt8Template    = template.Must(template.New("deserializeInt8TemplateGo").Parse(deserializeInt8TemplateStr))
	deserializeUInt16Template  = template.Must(template.New("deserializeUInt16TemplateGo").Parse(deserializeUInt16TemplateStr))
	deserializeInt16Template   = template.Must(template.New("deserializeInt16TemplateGo").Parse(deserializeInt16TemplateStr))
	deserializeUInt32Template  = template.Must(template.New("deserializeUInt32TemplateGo").Parse(deserializeUInt32TemplateStr))
	deserializeInt32Template   = template.Must(template.New("deserializeInt32TemplateGo").Parse(deserializeInt32TemplateStr))
	deserializeUInt64Template  = template.Must(template.New("deserializeUInt64TemplateGo").Parse(deserializeUInt64TemplateStr))
	deserializeInt64Template   = template.Must(template.New("deserializeInt64TemplateGo").Parse(deserializeInt64TemplateStr))
	deserializeFloat32Template = template.Must(template.New("deserializeFloat32TemplateGo").Parse(deserializeFloat32TemplateStr))
	deserializeFloat64Template = template.Must(template.New("deserializeFloat64TemplateGo").Parse(deserializeFloat64TemplateStr))
	deserializeStringTemplate  = template.Must(template.New("deserializeStringTemplateGo").Parse(deserializeStringTemplateStr))
	deserializeCustomTemplate  = template.Must(template.New("deserializeCustomTemplateGo").Parse(deserializeCustomTemplateStr))
)

func getMapKeyTypeName(dataType parse.DataComparableType) (string, error) {
	switch dataType {
	case parse.DataComparableTypeUInt8:
		return "UInt8", nil
	case parse.DataComparableTypeUInt16:
		return "UInt16", nil
	case parse.DataComparableTypeUInt32:
		return "UInt32", nil
	case parse.DataComparableTypeUInt64:
		return "UInt64", nil
	case parse.DataComparableTypeInt8:
		return "Int8", nil
	case parse.DataComparableTypeInt16:
		return "Int16", nil
	case parse.DataComparableTypeInt32:
		return "Int32", nil
	case parse.DataComparableTypeInt64:
		return "Int64", nil
	case parse.DataComparableTypeString:
		return "String", nil
	case parse.DataComparableTypeFloat32:
		return "Float32", nil
	case parse.DataComparableTypeFloat64:
		return "Float64", nil
	}
	return "", fmt.Errorf("unrecognized type: %v", dataType)
}

func getContainerTypesRecursively(dataType *parse.DataTypeDefinition) (string, error) {

	switch dataType.Type {
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
	case parse.DataTypeFloat32:
		return "Float32", nil
	case parse.DataTypeFloat64:
		return "Float64", nil
	case parse.DataTypeMap:
		key, err := getMapKeyTypeName(dataType.Key)
		if err != nil {
			return "", err
		}
		subNames, err := getContainerTypesRecursively(dataType.SubType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Map%s%s", key, subNames), nil

	case parse.DataTypeList:

		subNames, err := getContainerTypesRecursively(dataType.SubType)
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

	return "", fmt.Errorf("unrecognized type: %v", dataType.Type)
}

func generateCalcByteSizeContainerMethod(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	fullTypeName, err := getContainerTypesRecursively(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_CalcByteSize%s", util.EnsureCamelCase(messageName), fullTypeName)

	serializationMethodCode := map[string]string{}

	argType, err := mapTypeToGoType(dataType)
	if err != nil {
		return "", nil, err
	}

	valueMethodCall, methodCode, err := generateFieldCalcByteSizeMethodCall(messageName, "v", dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	args := CalcByteSizeContainerMethodArgs{
		FullMethodName:                  methodFullName,
		ArgType:                         argType,
		ValueTypeCalcByteSizeMethodCall: valueMethodCall,
	}

	buf := &bytes.Buffer{}
	if dataType.Type == parse.DataTypeList {

		// list

		err = listCalcByteSizeMethodTemplate.Execute(buf, args)
		if err != nil {
			return "", nil, err
		}

	} else if dataType.Type == parse.DataTypeMap {

		// map

		keyMethodCall, err := generateKeyFieldCalcByteSizeMethodCall("k", dataType.Key)
		if err != nil {
			return "", nil, err
		}
		args.KeyTypeCalcByteSizeMethodCall = keyMethodCall

		err = mapCalcByteSizeMethodTemplate.Execute(buf, args)
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

	fullTypeName, err := getContainerTypesRecursively(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_Serialize%s", util.EnsureCamelCase(messageName), fullTypeName)

	serializationMethodCode := map[string]string{}

	argType, err := mapTypeToGoType(dataType)
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

		keyMethodCall, err := generateKeyFieldSerializationMethodCall("k", dataType.Key)
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

	fullTypeName, err := getContainerTypesRecursively(dataType)
	if err != nil {
		return "", nil, err
	}

	methodFullName := fmt.Sprintf("%s_Deserialize%s", util.EnsureCamelCase(messageName), fullTypeName)

	deserializationMethodCode := map[string]string{}

	argType, err := mapTypeToGoType(dataType)
	if err != nil {
		return "", nil, err
	}

	valueMethodCall, methodCode, err := generateFieldDeserializationMethodCall(messageName, "v", dataType.SubType)
	if err != nil {
		return "", nil, err
	}

	valueType, err := mapTypeToGoType(dataType.SubType)
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

		keyType, err := mapComparableTypeToGoType(dataType.Key)
		if err != nil {
			return "", nil, err
		}

		keyMethodCall, err := generateKeyFieldDeserializationMethodCall("k", dataType.Key)
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

func generateKeyFieldCalcByteSizeMethodCall(varName string, dataType parse.DataComparableType) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataComparableTypeUInt8:
		tmpl = calcByteSizeUInt8Template
	case parse.DataComparableTypeUInt16:
		tmpl = calcByteSizeUInt16Template
	case parse.DataComparableTypeUInt32:
		tmpl = calcByteSizeUInt32Template
	case parse.DataComparableTypeUInt64:
		tmpl = calcByteSizeUInt64Template
	case parse.DataComparableTypeInt8:
		tmpl = calcByteSizeInt8Template
	case parse.DataComparableTypeInt16:
		tmpl = calcByteSizeInt16Template
	case parse.DataComparableTypeInt32:
		tmpl = calcByteSizeInt32Template
	case parse.DataComparableTypeInt64:
		tmpl = calcByteSizeInt64Template
	case parse.DataComparableTypeFloat32:
		tmpl = calcByteSizeFloat32Template
	case parse.DataComparableTypeFloat64:
		tmpl = calcByteSizeFloat64Template
	case parse.DataComparableTypeString:
		tmpl = calcByteSizeStringTemplate
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

func generateKeyFieldSerializationMethodCall(varName string, dataType parse.DataComparableType) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataComparableTypeUInt8:
		tmpl = serializeUInt8Template
	case parse.DataComparableTypeUInt16:
		tmpl = serializeUInt16Template
	case parse.DataComparableTypeUInt32:
		tmpl = serializeUInt32Template
	case parse.DataComparableTypeUInt64:
		tmpl = serializeUInt64Template
	case parse.DataComparableTypeInt8:
		tmpl = serializeInt8Template
	case parse.DataComparableTypeInt16:
		tmpl = serializeInt16Template
	case parse.DataComparableTypeInt32:
		tmpl = serializeInt32Template
	case parse.DataComparableTypeInt64:
		tmpl = serializeInt64Template
	case parse.DataComparableTypeFloat32:
		tmpl = serializeFloat32Template
	case parse.DataComparableTypeFloat64:
		tmpl = serializeFloat64Template
	case parse.DataComparableTypeString:
		tmpl = serializeStringTemplate
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

func generateKeyFieldDeserializationMethodCall(varName string, dataType parse.DataComparableType) (string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType {
	case parse.DataComparableTypeUInt8:
		tmpl = deserializeUInt8Template
	case parse.DataComparableTypeUInt16:
		tmpl = deserializeUInt16Template
	case parse.DataComparableTypeUInt32:
		tmpl = deserializeUInt32Template
	case parse.DataComparableTypeUInt64:
		tmpl = deserializeUInt64Template
	case parse.DataComparableTypeInt8:
		tmpl = deserializeInt8Template
	case parse.DataComparableTypeInt16:
		tmpl = deserializeInt16Template
	case parse.DataComparableTypeInt32:
		tmpl = deserializeInt32Template
	case parse.DataComparableTypeInt64:
		tmpl = deserializeInt64Template
	case parse.DataComparableTypeFloat32:
		tmpl = deserializeFloat32Template
	case parse.DataComparableTypeFloat64:
		tmpl = deserializeFloat64Template
	case parse.DataComparableTypeString:
		tmpl = deserializeStringTemplate
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

func generateFieldCalcByteSizeMethodCall(messageName string, varName string, dataType *parse.DataTypeDefinition) (string, map[string]string, error) {

	args := FunctionCallArgs{
		VariableName: varName,
	}

	var tmpl *template.Template

	switch dataType.Type {
	case parse.DataTypeByte:
		tmpl = calcByteSizeByteTemplate
	case parse.DataTypeBool:
		tmpl = calcByteSizeBoolTemplate
	case parse.DataTypeUInt8:
		tmpl = calcByteSizeUInt8Template
	case parse.DataTypeUInt16:
		tmpl = calcByteSizeUInt16Template
	case parse.DataTypeUInt32:
		tmpl = calcByteSizeUInt32Template
	case parse.DataTypeUInt64:
		tmpl = calcByteSizeUInt64Template
	case parse.DataTypeInt8:
		tmpl = calcByteSizeInt8Template
	case parse.DataTypeInt16:
		tmpl = calcByteSizeInt16Template
	case parse.DataTypeInt32:
		tmpl = calcByteSizeInt32Template
	case parse.DataTypeInt64:
		tmpl = calcByteSizeInt64Template
	case parse.DataTypeFloat32:
		tmpl = calcByteSizeFloat32Template
	case parse.DataTypeFloat64:
		tmpl = calcByteSizeFloat64Template
	case parse.DataTypeString:
		tmpl = calcByteSizeStringTemplate
	case parse.DataTypeCustom:
		tmpl = calcByteSizeCustomTemplate
	case parse.DataTypeMap,
		parse.DataTypeList:
		// special case for containers
		return generateCalcByteSizeContainerMethod(messageName, varName, dataType)

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

func generateMessageCalcByteSizeMethod(msg *parse.MessageDefinition) (string, error) {
	args := CalcByteSizeMethodArgs{
		MessageNameFirstLetter: util.FirstLetterAsLowercase(msg.Name),
		MessageNamePascalCase:  util.EnsurePascalCase(msg.Name),
	}

	additionalFunctionCode := map[string]string{}

	for _, field := range msg.FieldsByIndex() {

		fieldName := fmt.Sprintf("%s.%s", util.FirstLetterAsLowercase(msg.Name), util.EnsurePascalCase(field.Name))

		fieldCalcByteSizeMethodCall, methodCode, err := generateFieldCalcByteSizeMethodCall(msg.Name, fieldName, field.DataTypeDefinition)
		if err != nil {
			return "", err
		}
		additionalFunctionCode = util.MergeMap(additionalFunctionCode, methodCode)
		args.FieldCalcByteSizeMethodCalls = append(args.FieldCalcByteSizeMethodCalls, fieldCalcByteSizeMethodCall)
	}

	buf := &bytes.Buffer{}
	err := messageCalcByteSizeMethodTemplate.Execute(buf, args)
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
	goType, err := mapTypeToGoType(field.DataTypeDefinition)
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

	calcByteSizeCode, err := generateMessageCalcByteSizeMethod(msg)
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

	args.CalcByteSizeCode = calcByteSizeCode
	args.SerializeCode = serializeCode
	args.DeserializeCode = deserializeCode

	buf := &bytes.Buffer{}
	err = messageTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
