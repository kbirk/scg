package util

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	snakeCaseRegex = regexp.MustCompile(`([A-Z])`)
	splitRegex     = regexp.MustCompile(`([A-Z]+[a-z0-9]*|[a-z0-9]+)`)
)

func splitNameIntoParts(name string) []string {
	// Split the string into words
	matches := splitRegex.FindAllStringSubmatch(name, -1)
	words := make([]string, len(matches))
	for i, match := range matches {
		words[i] = match[0]
	}
	return words
}

func EnsurePascalCase(s string) string {
	words := splitNameIntoParts(s)
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
	return strings.Join(words, "")
}

func EnsureCamelCase(s string) string {
	words := splitNameIntoParts(s)
	for i := range words {
		if len(words[i]) > 0 {
			words[i] = strings.ToLower(words[i])
			if i == 0 {
				words[i] = string(unicode.ToLower(rune(words[i][0]))) + words[i][1:]
			} else {
				if words[i] == "id" {
					words[i] = "ID"
					continue
				}
				words[i] = string(unicode.ToUpper(rune(words[i][0]))) + words[i][1:]
			}
		}
	}
	return strings.Join(words, "")
}

func EnsureSnakeCase(s string) string {
	words := splitNameIntoParts(s)
	for i, word := range words {
		words[i] = strings.ToLower(word)
	}
	return strings.Join(words, "_")
}

func FirstLetterAsLowercase(s string) string {
	if len(s) == 0 {
		return ""
	}

	runes := []rune(s)
	return string(unicode.ToLower(runes[0]))
}
