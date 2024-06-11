package util

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	snakeCaseRegex = regexp.MustCompile(`([A-Z])`)
	splitRegex     = regexp.MustCompile(`([A-Z][a-z0-9]*|[a-z0-9]+)`)
)

func splitNameIntoParts(name string) []string {
	// Split the string into words
	matches := splitRegex.FindAllStringSubmatch(name, -1)

	// Extract the matched strings from the matches
	words := make([]string, len(matches))
	for i, match := range matches {
		words[i] = match[0]
	}

	return words
}

func EnsurePascalCase(s string) string {

	words := splitNameIntoParts(s)

	// Capitalize the first letter of each word
	for i, word := range words {
		if len(word) > 0 {
			word = strings.ToLower(word)
			if word == "id" {
				words[i] = "ID"
				continue
			}
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}

	// Join the words together without spaces
	return strings.Join(words, "")
}

func EnsureCamelCase(s string) string {

	fields := splitNameIntoParts(s)

	// Capitalize the first letter of each word and leave the rest of the word as it is
	for i := range fields {
		if len(fields[i]) > 0 {
			fields[i] = strings.ToLower(fields[i])
			if i == 0 {
				fields[i] = string(unicode.ToLower(rune(fields[i][0]))) + fields[i][1:]
			} else {
				if fields[i] == "id" {
					fields[i] = "ID"
					continue
				}
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
