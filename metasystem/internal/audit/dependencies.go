package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Bash, Git, Go, and core utilities are the supported script platform; these
// executable interpreters must not regrow in the scripts tree.
var forbiddenInterpreters = map[string]bool{
	"deno": true, "node": true, "perl": true, "php": true, "ruby": true,
}

// These four published fixture owners are the complete legacy Python debt.
// The inventory is basename-exact so no new source path inherits it.
var declaredPythonFiles = map[string]bool{
	"channel-fixtures.sh": true, "dispatch-fixtures.sh": true,
	"preflight-commands.sh": true, "validate-metasystem.sh": true,
}

// DependencyFinding names one executable interpreter command that violates
// the shell dependency inventory.
type DependencyFinding struct {
	Interpreter string
	Path        string
	Line        int
	PythonDebt  bool
}

func (finding DependencyFinding) String() string {
	kind := "banned interpreter"
	if finding.PythonDebt {
		kind = "python3 outside the declared legacy sites"
	}
	return fmt.Sprintf("%s %s: %s:%d", kind, finding.Interpreter, finding.Path, finding.Line)
}

// AuditDependencies scans shell sources for command-position interpreter use.
// Quoted data and ordinary arguments are inert, while nested command
// substitutions are scanned as their own command lists.
func AuditDependencies(root string) ([]DependencyFinding, error) {
	scripts := filepath.Join(root, "scripts")
	info, err := os.Stat(scripts)
	if err != nil {
		return nil, fmt.Errorf("dependency audit scripts root unreadable: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("dependency audit scripts root is not a directory: %s", scripts)
	}
	var findings []DependencyFinding
	err = filepath.WalkDir(scripts, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sh" || entry.Name() == "dependency-ratchet.sh" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return relativeErr
		}
		for _, command := range shellCommands(string(data)) {
			interpreter := filepath.Base(command.Word)
			if forbiddenInterpreters[interpreter] {
				findings = append(findings, DependencyFinding{Interpreter: interpreter, Path: filepath.ToSlash(relative), Line: command.Line})
			}
			if interpreter == "python3" && !declaredPythonFiles[entry.Name()] {
				findings = append(findings, DependencyFinding{Interpreter: interpreter, Path: filepath.ToSlash(relative), Line: command.Line, PythonDebt: true})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("dependency audit scan failed: %w", err)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Interpreter < findings[j].Interpreter
	})
	return findings, nil
}

var shellControlWords = map[string]bool{
	"!": true, "do": true, "done": true, "elif": true, "else": true,
	"fi": true, "if": true, "then": true, "time": true, "until": true,
	"while": true,
}

var shellCommandPrefixes = map[string]bool{"command": true, "env": true, "exec": true}

func shellPrefixOptionTakesOperand(prefix, option string) bool {
	return (prefix == "env" && option == "-u") || (prefix == "exec" && option == "-a")
}

type shellCommand struct {
	Word string
	Line int
}

func shellCommands(source string) []shellCommand {
	scanner := shellWordScanner{source: source, line: 1}
	scanner.scan(0)
	return scanner.commands
}

func shellCommandWords(source string) []string {
	commands := shellCommands(source)
	words := make([]string, 0, len(commands))
	for _, command := range commands {
		words = append(words, command.Word)
	}
	return words
}

type shellWordScanner struct {
	source   string
	position int
	line     int
	commands []shellCommand
}

func (scanner *shellWordScanner) scan(terminator byte) {
	expectingCommand := true
	commandPrefix := ""
	optionOperand := false
	var word strings.Builder
	wordLine := 0
	writeWord := func(character byte) {
		if wordLine == 0 {
			wordLine = scanner.line
		}
		word.WriteByte(character)
	}
	finishWord := func() {
		if word.Len() == 0 {
			return
		}
		value := word.String()
		word.Reset()
		line := wordLine
		wordLine = 0
		if !expectingCommand {
			return
		}
		if optionOperand {
			optionOperand = false
			return
		}
		if shellControlWords[value] || shellAssignment(value) {
			return
		}
		if shellCommandPrefixes[value] {
			commandPrefix = value
			return
		}
		if commandPrefix != "" && strings.HasPrefix(value, "-") {
			optionOperand = shellPrefixOptionTakesOperand(commandPrefix, value)
			return
		}
		scanner.commands = append(scanner.commands, shellCommand{Word: value, Line: line})
		expectingCommand = false
		commandPrefix = ""
	}
	for scanner.position < len(scanner.source) {
		character := scanner.source[scanner.position]
		if terminator != 0 && character == terminator {
			finishWord()
			scanner.position++
			return
		}
		switch character {
		case ' ', '\t', '\r':
			finishWord()
			scanner.position++
		case '\n':
			finishWord()
			expectingCommand = true
			commandPrefix = ""
			optionOperand = false
			scanner.line++
			scanner.position++
		case ';', '&', '|', '{', '}':
			finishWord()
			expectingCommand = true
			commandPrefix = ""
			optionOperand = false
			scanner.position++
		case '(':
			finishWord()
			scanner.position++
			scanner.scan(')')
			expectingCommand = true
			commandPrefix = ""
			optionOperand = false
		case ')':
			finishWord()
			expectingCommand = true
			commandPrefix = ""
			optionOperand = false
			scanner.position++
		case '#':
			if word.Len() == 0 {
				for scanner.position < len(scanner.source) && scanner.source[scanner.position] != '\n' {
					scanner.position++
				}
				break
			}
			writeWord(character)
			scanner.position++
		case '\\':
			scanner.position++
			if scanner.position < len(scanner.source) {
				if scanner.source[scanner.position] == '\n' {
					scanner.line++
					scanner.position++
					break
				}
				writeWord(scanner.source[scanner.position])
				scanner.position++
			}
		case '\'':
			if wordLine == 0 {
				wordLine = scanner.line
			}
			scanner.copyQuoted(&word, '\'', false)
		case '"':
			if wordLine == 0 {
				wordLine = scanner.line
			}
			scanner.copyQuoted(&word, '"', true)
		case '`':
			if wordLine == 0 {
				wordLine = scanner.line
			}
			scanner.position++
			scanner.scan('`')
			word.WriteByte('_')
		case '$':
			if scanner.position+1 < len(scanner.source) && scanner.source[scanner.position+1] == '(' {
				if wordLine == 0 {
					wordLine = scanner.line
				}
				scanner.position += 2
				if scanner.position < len(scanner.source) && scanner.source[scanner.position] == '(' {
					scanner.skipArithmetic()
				} else {
					scanner.scan(')')
				}
				word.WriteByte('_')
			} else {
				writeWord(character)
				scanner.position++
			}
		default:
			writeWord(character)
			scanner.position++
		}
	}
	finishWord()
}

func (scanner *shellWordScanner) copyQuoted(word *strings.Builder, quote byte, substitutions bool) {
	scanner.position++
	for scanner.position < len(scanner.source) {
		character := scanner.source[scanner.position]
		if character == quote {
			scanner.position++
			return
		}
		if substitutions && character == '$' && scanner.position+1 < len(scanner.source) && scanner.source[scanner.position+1] == '(' {
			scanner.position += 2
			if scanner.position < len(scanner.source) && scanner.source[scanner.position] == '(' {
				scanner.skipArithmetic()
			} else {
				scanner.scan(')')
			}
			word.WriteByte('_')
			continue
		}
		if substitutions && character == '`' {
			scanner.position++
			scanner.scan('`')
			word.WriteByte('_')
			continue
		}
		if character == '\\' && substitutions && scanner.position+1 < len(scanner.source) {
			scanner.position++
			character = scanner.source[scanner.position]
			if character == '\n' {
				scanner.line++
				scanner.position++
				continue
			}
		}
		word.WriteByte(character)
		if character == '\n' {
			scanner.line++
		}
		scanner.position++
	}
}

func (scanner *shellWordScanner) skipArithmetic() {
	depth := 1
	scanner.position++
	for scanner.position < len(scanner.source) && depth > 0 {
		character := scanner.source[scanner.position]
		if character == '\n' {
			scanner.line++
			scanner.position++
			continue
		}
		if character == '\\' && scanner.position+1 < len(scanner.source) {
			scanner.position++
			if scanner.source[scanner.position] == '\n' {
				scanner.line++
			}
			scanner.position++
			continue
		}
		if character == '$' && scanner.position+1 < len(scanner.source) && scanner.source[scanner.position+1] == '(' {
			scanner.position += 2
			if scanner.position < len(scanner.source) && scanner.source[scanner.position] == '(' {
				scanner.skipArithmetic()
			} else {
				scanner.scan(')')
			}
			continue
		}
		if character == '`' {
			scanner.position++
			scanner.scan('`')
			continue
		}
		if character == '\'' || character == '"' {
			var ignored strings.Builder
			scanner.copyQuoted(&ignored, character, character == '"')
			continue
		}
		if character == '(' {
			depth++
		} else if character == ')' {
			depth--
		}
		scanner.position++
	}
	if scanner.position < len(scanner.source) && scanner.source[scanner.position] == ')' {
		scanner.position++
	}
}

func shellAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for index, character := range name {
		if character != '_' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (index == 0 || character < '0' || character > '9') {
			return false
		}
	}
	return true
}
