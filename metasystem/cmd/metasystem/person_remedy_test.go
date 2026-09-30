package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// H1 (a refusal names a working remedy): a refusal that needs a person at
// the enrolled terminal names the command that enrolls one, then the retry.
// "At their enrolled terminal" alone is no remedy for a person at a terminal
// that is not enrolled (Wido, 2026-09-29, helm held, disk clean --strays
// refused TERMINAL_NOT_REACHED).
const personEnrollCommand = "metasystem system enroll --name NAME"

func TestEveryPersonActRefusalNamesSystemEnroll(t *testing.T) {
	t.Parallel()
	notReached := errors.New(humanauthority.OutcomeTerminalMissing)
	disk := func(act string) func() string {
		return func() string {
			owners := diskOwners{person: func(string) (string, error) { return "", notReached }}
			_, problem := diskPerson(&intentInvocation{}, owners, "/nowhere", act)
			if problem == nil {
				return ""
			}
			return problem.Summary + "\n" + shellCommand(problem.next) + "  (" + problem.nextReason + ")"
		}
	}
	for _, test := range []struct {
		name, retry string
		refusal     func() string
	}{
		{"disk clean --strays", "then repeat this command", disk("--strays")},
		{"disk clean --release", "then repeat this command", disk("--release s1")},
		{"disk clean --discard", "then repeat this command", disk("--discard j2:c1")},
		{"a human verb's proof", "", func() string {
			return humanProofRemedy(newHumanVerbValues("park", nil), false, "", "", notReached).command
		}},
		{"session stop by a shell off the enrolled terminal", "then repeat this command", func() string {
			return sessionStopRefusal("", "", notReached)
		}},
	} {
		got := test.refusal()
		if !strings.Contains(got, personEnrollCommand) || !strings.Contains(got, test.retry) {
			t.Errorf("%s: the refusal does not name %q and then the retry %q:\n%s", test.name, personEnrollCommand, test.retry, got)
		}
		if strings.Contains(got, humanauthority.OutcomeTerminalMissing) {
			t.Errorf("%s: the refusal shows the proof's code instead of plain words:\n%s", test.name, got)
		}
	}
}

// personRemedyHelp are the functions whose "enrolled terminal" texts are help
// or status, not refusals: each is file#function.
var personRemedyHelp = map[string]bool{
	"cmd/metasystem/intent.go#":                                true, // flag and detail help of the intent table
	"cmd/metasystem/intent_process.go#processIntentCommands":   true,
	"cmd/metasystem/intent_planning.go#intentPlanningCommands": true,
	"cmd/metasystem/intent_disk.go#diskIntentCommands":         true,
	"cmd/metasystem/intent_delivery.go#intentDeliveryCommands": true,
	"cmd/metasystem/intent_helm.go#helmTerminalLines":          true,
	"internal/ui/act/act.go#Line":                              true,
	"internal/ui/httpd/walkthrough/trouble.go#":                true,
}

// TestAuditNoRefusalSendsAPersonToAnEnrolledTerminalWithoutEnroll reads every
// production string literal: one that sends a person to "the enrolled
// terminal" must also name system enroll, which the shared remedy does.
func TestAuditNoRefusalSendsAPersonToAnEnrolledTerminalWithoutEnroll(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var findings []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(module, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(module, path)
			rel = filepath.ToSlash(rel)
			for _, decl := range file.Decls {
				name := ""
				if fn, ok := decl.(*ast.FuncDecl); ok {
					name = fn.Name.Name
				}
				if personRemedyHelp[rel+"#"+name] {
					continue
				}
				ast.Inspect(decl, func(node ast.Node) bool {
					// A text built together with the shared remedy names
					// system enroll through it.
					if call, ok := node.(*ast.CallExpr); ok && namesPersonRemedy(call) {
						return false
					}
					literal, ok := node.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						return true
					}
					text, err := strconv.Unquote(literal.Value)
					if err == nil && sendsToEnrolledTerminal(text) && !strings.Contains(text, "system enroll") {
						findings = append(findings, rel+"#"+name+": "+strconv.Quote(text))
					}
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(findings) > 0 {
		t.Fatalf("these texts send a person to the enrolled terminal without naming %s; use humanauthority.PersonActRemedy:\n%s",
			personEnrollCommand, strings.Join(findings, "\n"))
	}
}

// namesPersonRemedy reports that the call passes humanauthority's
// PersonActRemedy or EnrollCommand among its arguments.
func namesPersonRemedy(call *ast.CallExpr) bool {
	found := false
	for _, argument := range call.Args {
		ast.Inspect(argument, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && (selector.Sel.Name == "PersonActRemedy" || selector.Sel.Name == "EnrollCommand") {
				found = true
			}
			return !found
		})
	}
	return found
}

// sendsToEnrolledTerminal is a text that directs someone to the enrolled
// terminal; saying a shell does not descend from it states a cause.
func sendsToEnrolledTerminal(text string) bool {
	text = strings.NewReplacer("descend from the enrolled terminal", "", "descends from the enrolled terminal", "").Replace(text)
	for _, phrase := range []string{"at the enrolled terminal", "at their enrolled terminal", "from the enrolled terminal", "use the enrolled terminal",
		"at a real enrolled terminal", "at the terminal enrolled on this machine", "--by at the enrolled terminal"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// The session stop refusal is two lines: why, in plain words, and the one
// command with the person's name filled in, never NAME when a name is known.
func TestSessionStopRefusalIsTwoLinesWithTheNameFilledIn(t *testing.T) {
	t.Parallel()
	got := sessionStopRefusal("", "Wido", errors.New(humanauthority.OutcomeTerminalMissing))
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || strings.Contains(got, "NAME") || strings.HasPrefix(lines[0], "session stop refused") ||
		!strings.Contains(lines[0], "only a person can stop a session quietly") || !strings.HasPrefix(lines[1], "run: metasystem system enroll --name Wido") {
		t.Fatalf("session stop refusal:\n%s", got)
	}
	for _, line := range lines {
		if len([]rune(line)) > 100 {
			t.Fatalf("line longer than the page: %q", line)
		}
	}
	agent := sessionStopRefusal("", "Wido", errors.New(humanauthority.OutcomeAgent))
	if !strings.Contains(agent, "run: metasystem session stop --by Wido") {
		t.Fatalf("an agent's shell is told the same command, in a terminal the person opened:\n%s", agent)
	}
}
