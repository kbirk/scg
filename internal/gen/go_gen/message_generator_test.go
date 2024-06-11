package go_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateMessagesGo(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample0")
	require.Nil(t, err)

	for _, pkg := range parse.Packages {
		for _, msg := range pkg.MessageDefinitions {
			str, err := generateMessageGoCode(msg)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

	fmt.Println("done")
}

func TestGetContainerTypesRecursively(t *testing.T) {

	// map[string][]map[uint64][]custom.CustomType
	dt := &parse.DataTypeDefinition{
		Type: parse.DataTypeMap,
		Key: &parse.DataTypeComparableDefinition{
			Type:                     parse.DataTypeComparableCustom,
			CustomType:               "TypedefType",
			CustomTypePackage:        "other",
			UnderlyingType:           parse.DataTypeComparableString,
			ImportedFromOtherPackage: true,
		},
		SubType: &parse.DataTypeDefinition{
			Type: parse.DataTypeList,
			SubType: &parse.DataTypeDefinition{
				Type: parse.DataTypeMap,
				Key: &parse.DataTypeComparableDefinition{
					Type: parse.DataTypeComparableUInt64,
				},
				SubType: &parse.DataTypeDefinition{
					Type: parse.DataTypeList,
					SubType: &parse.DataTypeDefinition{
						Type:                     parse.DataTypeCustom,
						CustomType:               "CustomType",
						CustomTypePackage:        "custom",
						ImportedFromOtherPackage: true,
					},
				},
			},
		},
	}

	fullName, err := getContainerTypesRecursively(dt)
	require.Nil(t, err)

	assert.Equal(t, "MapOtherPkgTypedefTypeListMapUInt64ListCustomPkgCustomType", fullName)
}

func TestGenerateSerializeContainerMethod(t *testing.T) {

	// map[string][]map[uint64][]custom.CustomType
	dt := &parse.DataTypeDefinition{
		Type: parse.DataTypeMap,
		Key: &parse.DataTypeComparableDefinition{
			Type:                     parse.DataTypeComparableCustom,
			CustomType:               "TypedefType",
			CustomTypePackage:        "other",
			UnderlyingType:           parse.DataTypeComparableString,
			ImportedFromOtherPackage: true,
		},
		SubType: &parse.DataTypeDefinition{
			Type: parse.DataTypeList,
			SubType: &parse.DataTypeDefinition{
				Type: parse.DataTypeMap,
				Key: &parse.DataTypeComparableDefinition{
					Type: parse.DataTypeComparableUInt64,
				},
				SubType: &parse.DataTypeDefinition{
					Type: parse.DataTypeList,
					SubType: &parse.DataTypeDefinition{
						Type:                     parse.DataTypeCustom,
						CustomType:               "CustomType",
						CustomTypePackage:        "custom",
						ImportedFromOtherPackage: true,
					},
				},
			},
		},
	}

	str, code, err := generateSerializeContainerMethod("MyMessage", "t.SomeField", dt)
	require.Nil(t, err)

	fmt.Println(str)
	for _, c := range code {
		fmt.Println(c)
	}
	fmt.Println("done")
}

func TestGenerateMessageSerializeMethod(t *testing.T) {

	msg := &parse.MessageDefinition{
		Name: "MyMessage",
		Fields: map[string]*parse.MessageFieldDefinition{
			"SomeField": {
				Name:  "SomeField",
				Index: 0,
				DataTypeDefinition: &parse.DataTypeDefinition{
					Type: parse.DataTypeMap,
					Key: &parse.DataTypeComparableDefinition{
						Type:                     parse.DataTypeComparableCustom,
						CustomType:               "TypedefType",
						CustomTypePackage:        "other",
						UnderlyingType:           parse.DataTypeComparableString,
						ImportedFromOtherPackage: true,
					},
					SubType: &parse.DataTypeDefinition{
						Type: parse.DataTypeList,
						SubType: &parse.DataTypeDefinition{
							Type: parse.DataTypeMap,
							Key: &parse.DataTypeComparableDefinition{
								Type: parse.DataTypeComparableUInt64,
							},
							SubType: &parse.DataTypeDefinition{
								Type: parse.DataTypeList,
								SubType: &parse.DataTypeDefinition{
									Type:                     parse.DataTypeCustom,
									CustomType:               "CustomType",
									CustomTypePackage:        "custom",
									ImportedFromOtherPackage: true,
								},
							},
						},
					},
				},
			},
			"AnotherField": {
				Name:  "AnotherField",
				Index: 1,
				DataTypeDefinition: &parse.DataTypeDefinition{
					Type: parse.DataTypeList,
					SubType: &parse.DataTypeDefinition{
						Type:              parse.DataTypeCustom,
						CustomType:        "CustomType",
						CustomTypePackage: "custom",
					},
				},
			},
			"ThirdType": {
				Name:  "ThirdType",
				Index: 2,
				DataTypeDefinition: &parse.DataTypeDefinition{
					Type: parse.DataTypeList,
					SubType: &parse.DataTypeDefinition{
						Type: parse.DataTypeMap,
						Key: &parse.DataTypeComparableDefinition{
							Type: parse.DataTypeComparableString,
						},
						SubType: &parse.DataTypeDefinition{
							Type: parse.DataTypeString,
						},
					},
				},
			},
		},
	}

	code, err := generateMessageSerializationMethod(msg)
	require.Nil(t, err)

	fmt.Println(code)
	fmt.Println("done")
}
