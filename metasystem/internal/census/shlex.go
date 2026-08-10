package census

import (
	"fmt"
	"strings"
)

// shellSplit tokenizes a POSIX shell command line: whitespace separates
// words; single quotes are literal; double quotes allow backslash escaping of
// $ ` " and backslash; a bare backslash escapes the next character. An
// unbalanced quote is an error. ArgvPaths relies on this to find path tokens
// in a process's argv.
func shellSplit(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inWord := false

	const (
		none = iota
		single
		double
	)
	quote := none

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch quote {
		case single:
			if c == '\'' {
				quote = none
			} else {
				current.WriteRune(c)
			}
			continue
		case double:
			if c == '\\' && i+1 < len(runes) {
				next := runes[i+1]
				// In double quotes, backslash escapes only these; otherwise
				// the backslash is literal (POSIX shell behavior).
				if next == '$' || next == '`' || next == '"' || next == '\\' {
					current.WriteRune(next)
					i++
				} else {
					current.WriteRune('\\')
				}
			} else if c == '"' {
				quote = none
			} else {
				current.WriteRune(c)
			}
			continue
		}

		switch {
		case c == '\'':
			quote = single
			inWord = true
		case c == '"':
			quote = double
			inWord = true
		case c == '\\':
			if i+1 < len(runes) {
				current.WriteRune(runes[i+1])
				i++
			}
			inWord = true
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				tokens = append(tokens, current.String())
				current.Reset()
				inWord = false
			}
		default:
			current.WriteRune(c)
			inWord = true
		}
	}
	if quote != none {
		return nil, fmt.Errorf("no closing quotation")
	}
	if inWord {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}
