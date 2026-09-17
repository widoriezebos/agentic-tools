// Package enginecause owns the closed set of policy-engine refusal outcomes.
package enginecause

import (
	"fmt"
	"strings"
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
		if item.Key == key && item.Value != "" {
			values = append(values, item.Value)
		}
	}
	return values
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
	{Token: "fast-forward-blocked", Commands: 5, remedy: func(f []Fact) []string {
		return append([]string{"git status --short"}, rearm(f)...)
	}},
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
	return Rendered{Remedy: strings.Join(commands, " && "), Commands: cause.Commands}, nil
}

// Refuse renders one complete policy-engine refusal.
func Refuse(token string, facts []Fact, detail string) error {
	rendered, err := Render(token, facts)
	if err != nil {
		return err
	}
	var observed strings.Builder
	for _, item := range facts {
		value := item.Value
		if item.Path {
			value = quoted(value)
		}
		fmt.Fprintf(&observed, " %s=%s", item.Key, value)
	}
	return fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: cause=%s%s: %s; run: %s", token, observed.String(), detail, rendered.Remedy)
}
