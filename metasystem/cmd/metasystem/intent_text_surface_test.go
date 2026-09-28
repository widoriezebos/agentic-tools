package main

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The old first words of the public surface; none of them is public now.
var removedFirstWords = []string{
	"goals", "show", "approve", "budget", "pause", "resume", "done", "open", "edit", "claim", "release", "accept-risk",
	"pin", "prioritize", "reopen", "abandon", "block", "unblock", "unapprove", "revoke", "split", "group", "ungroup",
	"notes", "repair", "incidents", "brief", "build", "wait", "review", "revise", "land", "start", "stop", "restart",
	"enroll", "ask", "answer", "check", "delegate", "watch", "health", "arm",
}

// oldStatusKinds are the words the old status read as a target kind.
var oldStatusKinds = []string{"job", "run", "work", "mission", "ui", "checkout", "goal", "--machines"}

// textSurfaceExceptions are places that name a process entry or an old
// spelling on purpose, each with the reason it stays.
var textSurfaceExceptions = []struct{ file, text, reason string }{
	{"metasystem/cmd/metasystem/testing_merge.go", "metasystem testing merge-driver", "the Git merge driver entry's own setup line; Git runs it by this argv"},
	{"metasystem/cmd/metasystem/rearm_on_landed.go", "metasystem up", "reports the up entry the re-arm actually ran"},
	{"metasystem/internal/supervise/arming.go", "metasystem up", "the owner lock's recorded holder label"},
	{"metasystem/internal/seat/launch/", "metasystem up", "the destination step's recorded argv"},
	{"metasystem/internal/seat/launch/", "metasystem internal steward arm", "the destination step's recorded argv"},
	{"metasystem/internal/adapter/toolgate.go", "metasystem ", "the tool gate recognizes the commands an agent runs, including family forms that still route"},
	{"metasystem/cmd/metasystem/intent_worktree.go", "metasystem goal worktree", "Git's worktree lock reason, not a command"},
}

// textProseAfterObject are words prose puts after "metasystem OBJECT" that
// are not an action: "the metasystem system is ...". Any other word there
// must be a current action of that object.
var textProseAfterObject = map[string]bool{
	"is": true, "are": true, "was": true, "and": true, "or": true, "of": true, "to": true, "in": true, "on": true,
	"for": true, "the": true, "that": true, "it": true, "has": true, "can": true, "will": true, "with": true, "as": true,
	"object": true, "objects": true, "action": true, "actions": true,
}

var textCommandPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.\-/])((?:bin/)?metasystem) ([a-z][a-z0-9-]*)(?: ([a-z][a-z0-9-]*|--[a-z][a-z-]*))?`)

// TestIntentTextNamesOnlyPublicForms is the witness that text shown to
// people and agents names only public forms: every "metasystem W W" in a Go
// string literal, a live document, a skill, a role packet, the agent
// contract, the routing index or the interface source is a public pair, the
// top-level status or help, or the explicit internal form. Records, historical
// plans and memory keep their old spellings and are not read.
func TestIntentTextNamesOnlyPublicForms(t *testing.T) {
	t.Parallel()
	repository := filepath.Join("..", "..", "..")
	registered := families()
	var problems []string
	check := func(file string, line int, text string) {
		for _, match := range textCommandPattern.FindAllStringSubmatch(text, -1) {
			first, second := match[2], match[3]
			quoted := strings.TrimSpace(strings.Join([]string{match[1], first, second}, " "))
			bad := ""
			switch {
			case first == "help" || first == "internal":
			case first == "status":
				if slices.Contains(oldStatusKinds, second) {
					bad = "an old status form"
				}
			case isIntentObject(first):
				if command, ok := findIntentAction(first, second); ok && command.hidden {
					bad = "a process entry"
				} else if !ok && familyHasVerb(registered, first, second) {
					bad = "a family verb without internal"
				} else if !ok && second != "" && !strings.HasPrefix(second, "--") && !textProseAfterObject[second] {
					bad = "an action the " + first + " object does not have"
				}
			case slices.Contains(removedFirstWords, first):
				bad = "a removed public spelling"
			case first == "up" || familyHasVerb(registered, first, second):
				bad = "an engine verb without internal"
			}
			if bad == "" {
				continue
			}
			excepted := false
			for _, exception := range textSurfaceExceptions {
				if strings.HasPrefix(file, exception.file) && strings.HasPrefix(strings.TrimPrefix(quoted, "bin/"), exception.text) {
					excepted = true
				}
			}
			if !excepted {
				problems = append(problems, file+":"+strconv.Itoa(line)+": "+quoted+" is "+bad)
			}
		}
	}
	walk := func(root string, keep func(string) bool, read func(string, []byte)) {
		filepath.WalkDir(filepath.Join(repository, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			relative, _ := filepath.Rel(repository, path)
			relative = filepath.ToSlash(relative)
			if entry.IsDir() {
				if slices.Contains([]string{"node_modules", "dist", "records", "memory", "reviews", "testdata"}, entry.Name()) || strings.HasPrefix(relative, "metasystem/plans") {
					return filepath.SkipDir
				}
				return nil
			}
			if !keep(relative) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: %v", relative, err)
				return nil
			}
			read(relative, data)
			return nil
		})
	}
	readLines := func(file string, data []byte) {
		for index, line := range strings.Split(string(data), "\n") {
			check(file, index+1, line)
		}
	}
	readGoStrings := func(file string, data []byte) {
		fileSet := token.NewFileSet()
		handle := fileSet.AddFile(file, -1, len(data))
		var lexer scanner.Scanner
		lexer.Init(handle, data, nil, 0)
		for {
			position, kind, literal := lexer.Scan()
			if kind == token.EOF {
				break
			}
			if kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(literal)
			if err != nil {
				value = literal
			}
			for offset, line := range strings.Split(value, "\n") {
				check(file, fileSet.Position(position).Line+offset, line)
			}
		}
	}
	walk("metasystem/cmd", func(path string) bool { return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") }, readGoStrings)
	walk("metasystem/internal", func(path string) bool {
		if strings.Contains(path, "/_app/src/") {
			return (strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")) && !strings.Contains(path, ".test.")
		}
		return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
	}, func(file string, data []byte) {
		if strings.HasSuffix(file, ".go") {
			readGoStrings(file, data)
			return
		}
		readLines(file, data)
	})
	markdown := func(path string) bool {
		return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".txt")
	}
	for _, root := range []string{"metasystem/docs", "metasystem/skills", "metasystem/optional-skills", "metasystem/internal/protocol/roles", "metasystem/internal/protocol/templates", ".claude", ".devin"} {
		walk(root, markdown, readLines)
	}
	for _, file := range []string{"metasystem/AGENTS.md", "metasystem/wow.md", "AGENTS.md", "metasystem/README.md"} {
		data, err := os.ReadFile(filepath.Join(repository, file))
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		readLines(file, data)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}
