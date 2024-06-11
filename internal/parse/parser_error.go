package parse

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

type ParsingError struct {
	Message  string
	Token    *Token
	Filename string
	Content  string
}

func getContentForError(content string, lineNumber int, characterPos int) string {

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i != len(lines)-1 {
			lines[i] = line + "\n"
		} else {
			lines[i] = line
		}
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

	numStr := strconv.Itoa(lineEnd)
	numDigits := len(numStr)

	lineFmt := fmt.Sprintf("%%%dd", numDigits)

	red := color.New(color.FgRed).SprintFunc()

	res := ""
	for i := lineStart; i <= lineEnd; i++ {
		prefix := fmt.Sprintf(lineFmt, i)
		if i == lineNumber {
			res += red(fmt.Sprintf("%s | %s", prefix, lines[i]))
			res += red(strings.Repeat(" ", len(prefix)) + " | " + strings.Repeat(" ", characterPos) + "^" + strings.Repeat("~", len(lines[i])-characterPos-2) + "\n")
		} else {
			res += fmt.Sprintf("%s | %s", prefix, lines[i])
		}
	}
	return res
}

func getContentForErrorBlock(content string, lineStart int, lineEnd int) string {

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i != len(lines)-1 {
			lines[i] = line + "\n"
		} else {
			lines[i] = line
		}
	}

	numStr := strconv.Itoa(lineEnd)
	numDigits := len(numStr)

	lineFmt := fmt.Sprintf("%%%dd", numDigits)

	res := ""
	for i := lineStart; i <= lineEnd; i++ {
		prefix := fmt.Sprintf(lineFmt, i)
		res += fmt.Sprintf("%s | %s", prefix, lines[i])
	}
	return res
}

func (p *ParsingError) Error() error {

	msg := p.Message

	if p.Filename != "" {
		msg += fmt.Sprintf(", file: %s", p.Filename)
	}
	if p.Token != nil {
		msg += fmt.Sprintf(", line: %d, character: %d", p.Token.LineStart, p.Token.LineStartCharacterPosition)
		msg += fmt.Sprintf("\n%s", getContentForError(p.Content, p.Token.LineStart, p.Token.LineStartCharacterPosition))
		// msg += fmt.Sprintf("\n%s", getContentForErrorBlock(p.Content, p.Token.LineStart, p.Token.LineEnd))
	}

	return errors.New(msg)
}
