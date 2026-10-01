package agentgate

import (
	"errors"
	"strings"
)

// splitCommands reads a Bash command as the gate admits it: simple commands
// of plain or quoted words, joined by &&, ||, ; , | or a newline. Anything
// the shell would expand, redirect, glob, background or group is refused,
// because the gate could no longer know which words run.
func splitCommands(command string) ([][]string, error) {
	var commands [][]string
	var words []string
	var word strings.Builder
	inWord := false
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func(final bool) error {
		endWord()
		if len(words) == 0 {
			if final {
				return nil
			}
			return errors.New("has an empty command")
		}
		commands = append(commands, words)
		words = nil
		return nil
	}
	for index := 0; index < len(command); index++ {
		character := command[index]
		switch character {
		case ' ', '\t':
			endWord()
		case '\n':
			if len(words) == 0 && !inWord {
				continue
			}
			if err := endCommand(false); err != nil {
				return nil, err
			}
		case ';':
			if err := endCommand(false); err != nil {
				return nil, err
			}
		case '&':
			if index+1 >= len(command) || command[index+1] != '&' {
				return nil, errors.New("runs a command in the background")
			}
			index++
			if err := endCommand(false); err != nil {
				return nil, err
			}
		case '|':
			if index+1 < len(command) && command[index+1] == '&' {
				return nil, errors.New("pipes standard error")
			}
			if index+1 < len(command) && command[index+1] == '|' {
				index++
			}
			if err := endCommand(false); err != nil {
				return nil, err
			}
		case '\'':
			end := strings.IndexByte(command[index+1:], '\'')
			if end < 0 {
				return nil, errors.New("has an unterminated quote")
			}
			word.WriteString(command[index+1 : index+1+end])
			inWord = true
			index += end + 1
		case '"':
			end := index + 1
			for ; end < len(command) && command[end] != '"'; end++ {
				switch command[end] {
				case '$', '`', '\\':
					return nil, errors.New("expands inside double quotes")
				}
			}
			if end >= len(command) {
				return nil, errors.New("has an unterminated quote")
			}
			word.WriteString(command[index+1 : end])
			inWord = true
			index = end
		case '$', '`':
			return nil, errors.New("expands a variable or a command")
		case '<', '>':
			return nil, errors.New("redirects input or output")
		case '(', ')', '{', '}':
			return nil, errors.New("groups commands")
		case '*', '?', '[', ']':
			return nil, errors.New("uses an unquoted pattern")
		case '\\':
			return nil, errors.New("escapes a character")
		case '#', '!':
			return nil, errors.New("uses a comment or history character")
		case '~':
			if !inWord {
				return nil, errors.New("expands a home directory")
			}
			word.WriteByte(character)
		default:
			word.WriteByte(character)
			inWord = true
		}
	}
	if err := endCommand(true); err != nil {
		return nil, err
	}
	if len(commands) == 0 {
		return nil, errors.New("is empty")
	}
	return commands, nil
}
