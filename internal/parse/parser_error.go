package parse

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
)

type ParsingError struct {
	Message  string
	Token    *Token
	Filename string
	Content  string
}

// getContentForError renders the lines surrounding lineNumber (0-based) with a
// caret under characterPos (0-based). Line numbers are displayed 1-based. It
// never panics on out-of-range positions; they are clamped.
func getContentForError(content string, lineNumber int, characterPos int) string {

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}

	if lineNumber < 0 {
		lineNumber = 0
	}
	if lineNumber > len(lines)-1 {
		lineNumber = len(lines) - 1
	}
	if characterPos < 0 {
		characterPos = 0
	}

	linesBeforeAndAfter := 2

	lineStart := lineNumber - linesBeforeAndAfter
	if lineStart < 0 {
		lineStart = 0
	}
	lineEnd := lineNumber + linesBeforeAndAfter
	if lineEnd > len(lines)-1 {
		lineEnd = len(lines) - 1
	}

	numStr := strconv.Itoa(lineEnd + 1)
	numDigits := len(numStr)

	lineFmt := fmt.Sprintf("%%%dd", numDigits)

	red := color.New(color.FgRed).SprintFunc()

	res := ""
	for i := lineStart; i <= lineEnd; i++ {
		prefix := fmt.Sprintf(lineFmt, i+1)
		line := lines[i]
		if i == lineNumber {
			lineLen := utf8.RuneCountInString(line)
			pos := characterPos
			if pos > lineLen {
				pos = lineLen
			}
			numTildes := lineLen - pos - 1
			if numTildes < 0 {
				numTildes = 0
			}
			res += red(fmt.Sprintf("%s | %s\n", prefix, line))
			res += red(strings.Repeat(" ", len(prefix)) + " | " + strings.Repeat(" ", pos) + "^" + strings.Repeat("~", numTildes) + "\n")
		} else {
			res += fmt.Sprintf("%s | %s\n", prefix, line)
		}
	}
	return res
}

func (p *ParsingError) Error() error {

	msg := p.Message

	if p.Filename != "" {
		msg += fmt.Sprintf(", file: %s", p.Filename)
	}
	if p.Token != nil && p.Token.LineStart >= 0 {
		// positions are stored 0-based but displayed 1-based, as editors do
		msg += fmt.Sprintf(", line: %d, character: %d", p.Token.LineStart+1, p.Token.LineStartCharacterPosition+1)
		msg += fmt.Sprintf("\n%s", getContentForError(p.Content, p.Token.LineStart, p.Token.LineStartCharacterPosition))
	}

	return errors.New(msg)
}
