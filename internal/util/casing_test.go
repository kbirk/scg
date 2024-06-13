package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsurePascalCase(t *testing.T) {
	assert.Equal(t, "UpperCaseInput", EnsurePascalCase("UPPER_CASE_INPUT"))
	assert.Equal(t, "SnakeCaseInput", EnsurePascalCase("snake_case_input"))
	assert.Equal(t, "PascalCaseInput", EnsurePascalCase("PascalCaseInput"))
	assert.Equal(t, "CamelCaseInput", EnsurePascalCase("camelCaseInput"))
	assert.Equal(t, "InputWithSpaces", EnsurePascalCase("input with spaces"))
	assert.Equal(t, "SomethingWithID", EnsurePascalCase("something_with_id"))
}

func TestEnsureCamelCase(t *testing.T) {
	assert.Equal(t, "upperCaseInput", EnsureCamelCase("UPPER_CASE_INPUT"))
	assert.Equal(t, "snakeCaseInput", EnsureCamelCase("snake_case_input"))
	assert.Equal(t, "pascalCaseInput", EnsureCamelCase("PascalCaseInput"))
	assert.Equal(t, "camelCaseInput", EnsureCamelCase("camelCaseInput"))
	assert.Equal(t, "inputWithSpaces", EnsureCamelCase("input with spaces"))
	assert.Equal(t, "somethingWithID", EnsureCamelCase("something_with_id"))
}

func TestEnsureSnakeCase(t *testing.T) {
	assert.Equal(t, "upper_case_input", EnsureSnakeCase("UPPER_CASE_INPUT"))
	assert.Equal(t, "snake_case_input", EnsureSnakeCase("snake_case_input"))
	assert.Equal(t, "pascal_case_input", EnsureSnakeCase("PascalCaseInput"))
	assert.Equal(t, "camel_case_input", EnsureSnakeCase("camelCaseInput"))
	assert.Equal(t, "input_with_spaces", EnsureSnakeCase("input with spaces"))
	assert.Equal(t, "something_with_id", EnsureSnakeCase("something_with_id"))
}
