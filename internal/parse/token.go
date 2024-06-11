package parse

type TokenType int

const (
	UnknownTokenType TokenType = iota
	MessageTokenType
	MessageFieldTokenType
	ServiceTokenType
	ServiceMethodTokenType
	ServiceMethodParamTokenType
	PackageTokenType
	TypedefTokenType
)

type Token struct {
	Content                    string
	Type                       TokenType
	LineStart                  int
	LineStartCharacterPosition int
	LineEnd                    int
	LineEndCharacterPosition   int
}
