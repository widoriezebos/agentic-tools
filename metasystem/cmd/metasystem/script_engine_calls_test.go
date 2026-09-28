package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// scriptEngineCallRE finds a literal command after an engine variable in a
// shell line: "$ms" lease announce, ${engine} internal proc started-at.
var scriptEngineCallRE = regexp.MustCompile(`"?\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?"?((?:\s+[a-z][a-z0-9-]*){1,3})(?:\s|$|;|\))`)

// TestScriptEngineCallsRoute is the witness that no committed shell file
// calls an engine command that does not exist: every literal command a
// script runs on an engine variable routes to a public action, an entry, a
// top-level internal form or a registered family verb. A deleted verb's last
// script caller, or a call to a verb deleted long ago (benchmark/provision.sh
// called `lease reclaim` for six weeks after its owner went), fails here.
func TestScriptEngineCallsRoute(t *testing.T) {
	t.Parallel()
	repository, _ := verbRatchetRoots(t)
	registered := families()
	topLevel := map[string]bool{}
	for _, form := range dispatchInternalTopLevelForms(t) {
		topLevel[form] = true
	}
	routes := func(words []string) bool {
		if len(words) > 0 && words[0] == "internal" {
			words = words[1:]
		}
		if len(words) == 0 {
			return true
		}
		if topLevel[words[0]] || words[0] == "help" || words[0] == "status" {
			return true
		}
		if len(words) < 2 {
			return !isIntentObject(words[0]) && !familyExists(registered, words[0])
		}
		if isIntentObject(words[0]) {
			if _, ok := findIntentAction(words[0], words[1]); ok {
				return true
			}
		}
		return familyHasVerb(registered, words[0], words[1])
	}
	var problems []string
	for _, path := range ratchetShellFiles(t, repository, ratchetShellSkipPrefixes) {
		rel, _ := filepath.Rel(repository, path)
		lines := readRatchetLines(t, path)
		engines := ratchetEngineVariables(lines)
		for number, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			for _, match := range scriptEngineCallRE.FindAllStringSubmatch(line, -1) {
				if !engines[match[1]] || match[1] == "METASYSTEM_BIN" {
					continue
				}
				words := strings.Fields(match[2])
				if !routes(words) {
					problems = append(problems, filepath.ToSlash(rel)+":"+strconv.Itoa(number+1)+": "+trimmed)
				}
			}
		}
	}
	if len(problems) > 0 {
		t.Fatalf("shell files call engine commands that do not route:\n  %s", strings.Join(problems, "\n  "))
	}
}

func familyExists(registered []family, name string) bool {
	for _, fam := range registered {
		if fam.name == name {
			return true
		}
	}
	return false
}

// toolGateWordsRE reads a tool-gate row's command words from its source.
var toolGateWordsRE = regexp.MustCompile(`words: \[\]string\{"metasystem"((?:, "[^"]*")*)\}`)

// TestToolGateRowsNameRoutedCommands is the witness that the context tool
// gate allows only commands that exist: every engine command a row of
// internal/adapter/toolgate.go names routes (a "<verb>" word stands for any
// verb of its family). A row for a command nobody can run (the gate listed
// `internal context resume`, which never routed) fails here.
func TestToolGateRowsNameRoutedCommands(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	data, err := os.ReadFile(filepath.Join(module, "internal", "adapter", "toolgate.go"))
	if err != nil {
		t.Fatal(err)
	}
	registered := families()
	topLevel := map[string]bool{}
	for _, form := range dispatchInternalTopLevelForms(t) {
		topLevel[form] = true
	}
	rows := toolGateWordsRE.FindAllStringSubmatch(string(data), -1)
	if len(rows) == 0 {
		t.Fatal("found no engine command rows in toolgate.go; the scan no longer reads it")
	}
	var problems []string
	for _, row := range rows {
		var words []string
		for _, quoted := range strings.Split(strings.TrimPrefix(row[1], ", "), ", ") {
			word, err := strconv.Unquote(quoted)
			if err != nil {
				t.Fatal(err)
			}
			words = append(words, word)
		}
		name := "metasystem " + strings.Join(words, " ")
		internal := len(words) > 0 && words[0] == "internal"
		if internal {
			words = words[1:]
		}
		switch {
		case len(words) == 0:
			problems = append(problems, name+": names no command")
		case topLevel[words[0]]:
		case len(words) == 1:
			problems = append(problems, name+": names an object or family without an action")
		case words[1] == "<verb>":
			if !familyExists(registered, words[0]) {
				problems = append(problems, name+": no such family")
			}
		case !internal && isIntentObject(words[0]):
			if command, ok := findIntentAction(words[0], words[1]); !ok || command.hidden {
				if !familyHasVerb(registered, words[0], words[1]) {
					problems = append(problems, name+": no such public action")
				}
			}
		case !familyHasVerb(registered, words[0], words[1]):
			problems = append(problems, name+": no such family verb")
		}
	}
	if len(problems) > 0 {
		t.Fatalf("tool-gate rows name commands that do not route:\n  %s", strings.Join(problems, "\n  "))
	}
}
