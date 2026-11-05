package go_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type TypedefArgs struct {
	TypedefNamePascalCase           string
	TypedefNameNameFirstLetter      string
	TypedefUnderlyingType           string
	TypedefUnderlyingTypePascalCase string
	IsUUID                          bool
}

const typedefTemplateStr = `
type {{.TypedefNamePascalCase}} {{.TypedefUnderlyingType}}

func New{{.TypedefNamePascalCase}}(value {{.TypedefUnderlyingType}}) *{{.TypedefNamePascalCase}} {
	{{.TypedefNameNameFirstLetter}} := {{.TypedefNamePascalCase}}(value)
	return &{{.TypedefNameNameFirstLetter}}
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Ptr() *{{.TypedefUnderlyingType}} {
	return (*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}})
}

func ({{.TypedefNameNameFirstLetter}} {{.TypedefNamePascalCase}}) Value() (driver.Value, error) {
{{if .IsUUID}}
	return (*uuid.UUID)(&{{.TypedefNameNameFirstLetter}}).Value()
{{else}}
	return {{.TypedefUnderlyingType}}({{.TypedefNameNameFirstLetter}}), nil
{{end}}
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Scan(src interface{}) error {
{{if .IsUUID}}
	return (*uuid.UUID)({{.TypedefNameNameFirstLetter}}).Scan(src)
{{else}}
	switch src := src.(type) {
	case {{.TypedefUnderlyingType}}:
		*{{.TypedefNameNameFirstLetter}}  = {{.TypedefNamePascalCase}}(src)
		return nil
	case nil:
		var def {{.TypedefNamePascalCase}}
		*{{.TypedefNameNameFirstLetter}} = def
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into type {{.TypedefNamePascalCase}}", src)
	}
{{end}}
}

{{- if .IsUUID}}

// MarshalText encodes the ID as a canonical UUID string, like uuid.UUID. It
// also makes the type usable as a JSON map key.
func ({{.TypedefNameNameFirstLetter}} {{.TypedefNamePascalCase}}) MarshalText() ([]byte, error) {
	return uuid.UUID({{.TypedefNameNameFirstLetter}}).MarshalText()
}

// UnmarshalText decodes a UUID string, like uuid.UUID.
func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) UnmarshalText(data []byte) error {
	return (*uuid.UUID)({{.TypedefNameNameFirstLetter}}).UnmarshalText(data)
}

// MarshalJSON encodes the ID as a JSON string, like uuid.UUID.
func ({{.TypedefNameNameFirstLetter}} {{.TypedefNamePascalCase}}) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID({{.TypedefNameNameFirstLetter}}))
}

// UnmarshalJSON decodes a JSON string, and also the array of 16 numbers that
// earlier generated code wrote for this type.
func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) > 0 && data[0] == '[' {
		var legacy [16]byte
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		*{{.TypedefNameNameFirstLetter}} = {{.TypedefNamePascalCase}}(legacy)
		return nil
	}
	return json.Unmarshal(data, (*uuid.UUID)({{.TypedefNameNameFirstLetter}}))
}
{{- end}}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) BitSize() int {
	return serialize.BitSize{{.TypedefUnderlyingTypePascalCase}}(*(*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}))
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Serialize(writer *serialize.Writer) {
	serialize.Serialize{{.TypedefUnderlyingTypePascalCase}}(writer, *(*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}))
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	return serialize.Deserialize{{.TypedefUnderlyingTypePascalCase}}((*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}), reader)
}
`

var (
	typedefTemplate = template.Must(template.New("typedefTemplateGo").Parse(typedefTemplateStr))
)

func generateTypedefGoCode(typdef *parse.TypedefDeclaration) (string, error) {

	typeName, err := mapDataTypeComparableDefinitionToGoType(typdef.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	typeNamePascalCase, err := getDataTypeComparableDefinitionMethodSuffix(typdef.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	args := TypedefArgs{
		TypedefNamePascalCase:           util.EnsurePascalCase(typdef.Name),
		TypedefNameNameFirstLetter:      util.FirstLetterAsLowercase(typdef.Name),
		TypedefUnderlyingType:           typeName,
		TypedefUnderlyingTypePascalCase: typeNamePascalCase,
		IsUUID:                          typdef.DataTypeDefinition.Type == parse.DataTypeComparableUUID,
	}

	buf := &bytes.Buffer{}
	err = typedefTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
