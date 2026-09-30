// Package enginecause owns the closed set of policy-engine refusal outcomes.
package enginecause

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Fact is one observed value carried by a refusal. Paths are shell-quoted
// when rendered so a refusal never turns an unusual checkout name into an
// unsafe command.
type Fact struct {
	Key, Value string
	Path       bool
}

// Value records an ordinary diagnostic fact.
func Value(key, value string) Fact { return Fact{Key: key, Value: value} }

// Path records a path-valued diagnostic fact.
func Path(key, value string) Fact { return Fact{Key: key, Value: value, Path: true} }

var appendOnlyLedgers = []string{
	"memory/receipts.log",
	"records/narrator-digest.log",
}

// IsAppendOnlyLedger reports whether path is one of the ledgers whose local
// append must be saved and reconciled across a landed fast-forward.
func IsAppendOnlyLedger(path string) bool {
	for _, ledger := range appendOnlyLedgers {
		if path == ledger {
			return true
		}
	}
	return false
}

// Cause is one operator outcome. Kind is the value of the "fact" fact when
// one token has branches that need different operator actions.
type Cause struct {
	Token, Kind string
	Commands    int
	sample      []Fact
	remedy      func(facts []Fact) []string
}

// Rendered is the selected outcome's remedy and declared command count.
type Rendered struct {
	Remedy   string
	Commands int
}

// TokenEngineUnavailable is the cause for an enrolled engine path or
// descriptor that cannot be executed or hashed.
const TokenEngineUnavailable = "engine-unavailable"

func fact(facts []Fact, key, fallback string) string {
	for _, item := range facts {
		if item.Key == key && item.Value != "" {
			return item.Value
		}
	}
	return fallback
}

func factValues(facts []Fact, key string) []string {
	var values []string
	for _, item := range facts {
		if item.Key == key && item.Value != "" && (!item.Path || printablePath(item.Value)) {
			values = append(values, item.Value)
		}
	}
	return values
}

func printablePath(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func quoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func rearm(facts []Fact) []string {
	remote := fact(facts, "remote", "origin")
	tip := fact(facts, "tip", remote+"/main")
	checkout := fact(facts, "checkout", ".")
	return []string{"git fetch " + quoted(remote), "git merge --ff-only " + quoted(tip), "go run ./cmd/devgate build", "bin/metasystem session start --repo " + quoted(checkout)}
}

// Command is argv as one shell command, each word quoted where it needs it:
// the "command" fact a remedy runs.
func Command(argv ...string) string {
	words := make([]string, len(argv))
	for index, word := range argv {
		words[index] = word
		if word == "" || strings.ContainsFunc(word, func(r rune) bool {
			return !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_./=:,@+", r))
		}) {
			words[index] = quoted(word)
		}
	}
	return strings.Join(words, " ")
}

// engineCommand runs the pinned engine's own command, so its failure is
// seen first hand; without one it falls back to the words.
func engineCommand(words string) func([]Fact) []string {
	return func(facts []Fact) []string { return []string{fact(facts, "command", words)} }
}

func one(command string) func([]Fact) []string {
	return func([]Fact) []string { return []string{command} }
}

func fastForwardBlocked(facts []Fact) []string {
	tracked := factValues(facts, "tracked-path")
	untracked := factValues(facts, "untracked-path")
	ledgers := factValues(facts, "ledger-path")
	if len(tracked)+len(untracked)+len(ledgers) == 0 {
		return append([]string{"git status --short"}, rearm(facts)...)
	}

	commands := []string{}
	var saved []string
	if len(ledgers) > 0 {
		parts := make([]string, 0, len(ledgers))
		for index, path := range ledgers {
			variable := fmt.Sprintf("save%d", index+1)
			parts = append(parts, fmt.Sprintf("%s=%s.local.$(git rev-parse --short HEAD).0; while test -e \"$%s\"; do %s=${%s%%.*}.$((${%s##*.}+1)); done; cp -p -n -- %s \"$%s\"",
				variable, quoted(path), variable, variable, variable, variable, quoted(path), variable))
			saved = append(saved, fmt.Sprintf("$%s", variable))
		}
		commands = append(commands, strings.Join(parts, "; "))
		quotedLedgers := make([]string, len(ledgers))
		for index, path := range ledgers {
			quotedLedgers[index] = quoted(path)
		}
		commands = append(commands, "git restore --staged --worktree --source=HEAD -- "+strings.Join(quotedLedgers, " "))
	}
	stashPaths := append(append([]string(nil), tracked...), untracked...)
	if len(stashPaths) > 0 {
		for index := range stashPaths {
			stashPaths[index] = quoted(stashPaths[index])
		}
		includeUntracked := ""
		if len(untracked) > 0 {
			includeUntracked = "--include-untracked "
		}
		commands = append(commands, "git stash push "+includeUntracked+"-- "+strings.Join(stashPaths, " "))
	}
	commands = append(commands, rearm(facts)...)
	if len(stashPaths) > 0 {
		commands = append(commands, "git stash pop")
	}
	if len(ledgers) > 0 {
		commands = append(commands, "after the fast-forward, append only the missing saved lines from "+strings.Join(saved, " and ")+" back to "+strings.Join(ledgers, " and "))
	}
	return commands
}

// Table is the single inventory of policy-engine refusal outcomes. A generic
// token row has an empty Kind; a fact-specific row wins when present.
var Table = []Cause{
	{Token: "not-enrolled", Commands: 2, sample: []Fact{Path("linked-worktree", "/repo")}, remedy: func(f []Fact) []string {
		checkout := fact(f, "linked-worktree", fact(f, "checkout", "."))
		return []string{"cd " + quoted(checkout), "rerun the same metasystem test run command with the candidate staged in this checkout"}
	}},
	{Token: "enrollment-drift", Commands: 1, remedy: func(f []Fact) []string {
		return []string{"bin/metasystem session start --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "engine-behind-tip", Commands: 4, remedy: rearm},
	{Token: "engine-behind-tip", Kind: "fetch-failed", Commands: 4, sample: []Fact{Value("fact", "fetch-failed")}, remedy: rearm},
	{Token: "engine-behind-tip", Kind: "head-diverged", Commands: 4, sample: []Fact{Value("fact", "head-diverged")}, remedy: func(f []Fact) []string {
		commands := rearm(f)
		commands[1] = "git rebase " + quoted(fact(f, "tip", "origin/main"))
		return commands
	}},
	{Token: "engine-behind-tip", Kind: "dirty-engine-paths", Commands: 6, sample: []Fact{Value("fact", "dirty-engine-paths"), Path("path", "cmd/main.go")}, remedy: func(f []Fact) []string {
		paths := factValues(f, "path")
		if len(paths) == 0 {
			paths = []string{"."}
		}
		for index := range paths {
			paths[index] = quoted(paths[index])
		}
		commands := []string{"git stash push -- " + strings.Join(paths, " ")}
		commands = append(commands, rearm(f)...)
		return append(commands, "git stash pop")
	}},
	{Token: "engine-behind-tip", Kind: "named-delivery-tree", Commands: 4, sample: []Fact{Value("fact", "named-delivery-tree")}, remedy: rearm},
	{Token: "engine-behind-tip", Kind: "live-attempt", Commands: 5, sample: []Fact{Value("fact", "live-attempt"), Value("attempt", "proof-1")}, remedy: func(f []Fact) []string {
		commands := []string{"bin/metasystem test wait proof:" + fact(f, "attempt", "<attempt>") + " --repo " + quoted(fact(f, "checkout", "."))}
		return append(commands, rearm(f)...)
	}},
	{Token: "fast-forward-blocked", Commands: 9, sample: []Fact{
		Path("tracked-path", "docs/local.txt"), Path("untracked-path", "generated/local.txt"), Path("ledger-path", "memory/receipts.log"),
	}, remedy: fastForwardBlocked},
	{Token: "rebuild-failed", Commands: 2, remedy: func(f []Fact) []string {
		return []string{"go run ./cmd/devgate build", "bin/metasystem session start --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "rearm-failed", Commands: 1, remedy: func(f []Fact) []string {
		return []string{"bin/metasystem session start --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "mutation-lock", Commands: 1, remedy: one("rerun the same metasystem test run command after the active proof mutation ends")},
	{Token: "judgment-stalled", Commands: 1, remedy: one("rerun the same metasystem test run command after the named external step can make progress")},
	{Token: "judgment-failed", Commands: 1, remedy: one("repair the named policy judgment failure, then rerun the same metasystem test run command")},
	{Token: "base-moved", Commands: 1, remedy: one("rerun the same metasystem test run command from the current landing ref")},
	{Token: "decision-mismatch", Commands: 1, remedy: one("re-arm the enrolled policy engine, then rerun the same metasystem test run command")},
	{Token: TokenEngineUnavailable, Commands: 2, remedy: func(f []Fact) []string {
		return []string{"go run ./cmd/devgate build", "bin/metasystem session start --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "child-failed", Commands: 1, remedy: engineCommand("run the named enrolled policy engine directly and repair its reported failure")},
	{Token: "child-output", Commands: 1, remedy: engineCommand("repair the named enrolled policy engine's policy output, then rerun the same metasystem test run command")},
}

func outcome(token string, facts []Fact) (Cause, bool) {
	kind := fact(facts, "fact", "")
	var fallback Cause
	for _, cause := range Table {
		if cause.Token != token {
			continue
		}
		if cause.Kind == kind && kind != "" {
			return cause, true
		}
		if cause.Kind == "" {
			fallback = cause
		}
	}
	return fallback, fallback.Token != ""
}

// Render selects and renders the remedy for the observed outcome.
func Render(token string, facts []Fact) (Rendered, error) {
	cause, ok := outcome(token, facts)
	if !ok {
		return Rendered{}, fmt.Errorf("unknown policy-engine refusal cause %q", token)
	}
	commands := cause.remedy(facts)
	return Rendered{Remedy: strings.Join(commands, " && "), Commands: len(commands)}, nil
}

// Code is the refusal register code every policy-engine refusal carries; it
// is a detail, never the refusal's first line ("Messages a Person Reads").
const Code = "TEST_POLICY_ENGINE_REQUIRED"

// Refusal is one policy-engine refusal. Its text is the two lines a person
// reads: the plain reason the site observed, and the one command that
// resolves it. The code, the cause token and the observed facts are its
// Detail, for --verbose, --json and tests.
type Refusal struct {
	Token      string
	Facts      []Fact
	Reason     string
	Background string
	Remedy     string
	Commands   int
	observed   string
}

// Error is the refusal's two lines: the reason, then the command.
func (r *Refusal) Error() string {
	return r.Reason + "\nrun: " + r.Remedy
}

// Detail is the refusal's code, cause and observed facts, with the site's
// background when it has one.
func (r *Refusal) Detail() string {
	detail := Code + ": cause=" + r.Token + r.observed + ": " + r.Reason
	if r.Background != "" {
		detail += ": " + r.Background
	}
	return detail
}

// Detail is the detail of the policy-engine refusal inside err, or "" when
// err holds none.
func Detail(err error) string {
	var refusal *Refusal
	if errors.As(err, &refusal) {
		return refusal.Detail()
	}
	return ""
}

// Refuse renders one complete policy-engine refusal: reason is the plain
// first line a person reads.
func Refuse(token string, facts []Fact, reason string) error {
	return RefuseWith(token, facts, reason, "")
}

// RefuseWith is Refuse with background (such as a child's own output) that
// only the refusal's Detail shows.
func RefuseWith(token string, facts []Fact, reason, background string) error {
	rendered, err := Render(token, facts)
	if err != nil {
		return err
	}
	var observed strings.Builder
	unprintable := 0
	for _, item := range facts {
		value := item.Value
		if item.Path {
			if !printablePath(value) {
				unprintable++
				continue
			}
			value = quoted(value)
		}
		fmt.Fprintf(&observed, " %s=%s", item.Key, value)
	}
	if unprintable > 0 {
		fmt.Fprintf(&observed, " unprintable=%d see=git-status--porcelain-z", unprintable)
	}
	return &Refusal{Token: token, Facts: facts, Reason: reason, Background: background, Remedy: rendered.Remedy,
		Commands: rendered.Commands, observed: observed.String()}
}
