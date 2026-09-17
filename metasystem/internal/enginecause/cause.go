// Package enginecause owns the closed set of policy-engine refusal outcomes.
package enginecause

import (
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
	return []string{"git fetch " + quoted(remote), "git merge --ff-only " + quoted(tip), "scripts/agents/go-build.sh", "bin/metasystem up --repo " + quoted(checkout)}
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
		return []string{"cd " + quoted(checkout), "rerun the same metasystem test command with the candidate staged in this checkout"}
	}},
	{Token: "enrollment-drift", Commands: 1, remedy: func(f []Fact) []string {
		return []string{"bin/metasystem up --repo " + quoted(fact(f, "checkout", "."))}
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
		commands := []string{"bin/metasystem wait --root " + quoted(fact(f, "checkout", ".")) + " --attempt " + quoted(fact(f, "attempt", "<attempt>"))}
		return append(commands, rearm(f)...)
	}},
	{Token: "fast-forward-blocked", Commands: 9, sample: []Fact{
		Path("tracked-path", "docs/local.txt"), Path("untracked-path", "generated/local.txt"), Path("ledger-path", "memory/receipts.log"),
	}, remedy: fastForwardBlocked},
	{Token: "rebuild-failed", Commands: 2, remedy: func(f []Fact) []string {
		return []string{"scripts/agents/go-build.sh", "bin/metasystem up --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "rearm-failed", Commands: 1, remedy: func(f []Fact) []string {
		return []string{"bin/metasystem up --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "mutation-lock", Commands: 1, remedy: one("rerun the same metasystem test command after the active proof mutation ends")},
	{Token: "judgment-stalled", Commands: 1, remedy: one("rerun the same metasystem test command after the named external step can make progress")},
	{Token: "judgment-failed", Commands: 1, remedy: one("repair the named policy judgment failure, then rerun the same metasystem test command")},
	{Token: "base-moved", Commands: 1, remedy: one("rerun the same metasystem test command from the current landing ref")},
	{Token: "decision-mismatch", Commands: 1, remedy: one("re-arm the enrolled policy engine, then rerun the same metasystem test command")},
	{Token: TokenEngineUnavailable, Commands: 2, remedy: func(f []Fact) []string {
		return []string{"scripts/agents/go-build.sh", "bin/metasystem up --repo " + quoted(fact(f, "checkout", "."))}
	}},
	{Token: "child-failed", Commands: 1, remedy: one("run the named enrolled policy engine directly and repair its reported failure")},
	{Token: "child-output", Commands: 1, remedy: one("repair the named enrolled policy engine's policy output, then rerun the same metasystem test command")},
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

// Refuse renders one complete policy-engine refusal.
func Refuse(token string, facts []Fact, detail string) error {
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
	return fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: cause=%s%s: %s; run: %s", token, observed.String(), detail, rendered.Remedy)
}
