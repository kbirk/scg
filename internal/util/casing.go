package util

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	snakeCaseRegex = regexp.MustCompile(`([A-Z])`)
)

func EnsurePascalCase(s string) string {
	// Split the string into words
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	// Capitalize the first letter of each word
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}

	// Join the words together without spaces
	return strings.Join(words, "")
}

func EnsureCamelCase(s string) string {
	// Split the string into words
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	// Capitalize the first letter of each word and leave the rest of the word as it is
	for i := range fields {
		if len(fields[i]) > 0 {
			if i == 0 {
				fields[i] = string(unicode.ToLower(rune(fields[i][0]))) + fields[i][1:]
			} else {
				fields[i] = string(unicode.ToUpper(rune(fields[i][0]))) + fields[i][1:]
			}
		}
	}
	// Join the words together without spaces
	return strings.Join(fields, "")
}

func EnsureSnakeCase(s string) string {
	// Replace all uppercase letters with _lowercase
	s = snakeCaseRegex.ReplaceAllString(s, "_${1}")

	// Replace spaces and hyphens with underscores
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")

	// Convert to lowercase
	s = strings.ToLower(s)

	// If the string starts with an underscore, remove it
	if strings.HasPrefix(s, "_") {
		s = s[1:]
	}

	return s
}

func FirstLetterAsLowercase(s string) string {
	if len(s) == 0 {
		return ""
	}

	runes := []rune(s)
	return string(unicode.ToLower(runes[0]))
}
