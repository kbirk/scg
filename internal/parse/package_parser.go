package parse

import (
	"regexp"
)

var (
	packageRegex = regexp.MustCompile(`package\s+([a-zA-Z][a-zA-Z_0-9.]*)\s*\;`)
)

type PackageDeclaration struct {
	Name  string
	Token *Token
}

func parsePackageDeclaration(tokens []*Token) (*PackageDeclaration, *ParsingError) {

	var pkg *PackageDeclaration

	for _, token := range tokens {
		if token.Type != PackageTokenType {
			continue
		}

		match, perr := FindOneMatch(packageRegex, token)
		if perr != nil || len(match.Captures) != 1 {
			return nil, &ParsingError{
				Message: "invalid package definition",
				Token:   token,
			}
		}

		if pkg != nil {
			return nil, &ParsingError{
				Message: "multiple package declarations found",
				Token:   token,
			}
		}

		pkg = &PackageDeclaration{
			Name:  match.Captures[0].Content,
			Token: token,
		}
	}

	return pkg, nil
}
