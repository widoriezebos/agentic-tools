package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// TestIntentPublicDiscovery: a bare call and --help are the same short,
// successful orientation; topics list whole areas; compatibility-only
// options stay parseable but are never shown or suggested; an unknown word
// is answered with a public command, never an engine family.
func TestIntentPublicDiscovery(t *testing.T) {
	t.Parallel()
	registered := families()
	code, root, problem := runCLIHelp(nil, registered)
	if code != 0 || problem != "" {
		t.Fatalf("bare = %d %q", code, problem)
	}
	if lines := strings.Count(root, "\n"); lines > 45 {
		t.Errorf("the root page is %d lines; it is an orientation, not an inventory", lines)
	}
	for _, leak := range []string{"internal", "FAMILY", "usage: metasystem <family>", "goal list"} {
		if strings.Contains(root, leak) {
			t.Errorf("the root page mentions %q", leak)
		}
	}
	for _, want := range []string{"help goals", "help work", "help questions", "help operations", "help all", "metasystem review my-goal", "metasystem land my-goal"} {
		if !strings.Contains(root, want) {
			t.Errorf("the root page lacks %q", want)
		}
	}
	// goals is a command and a topic: the command help carries the topic.
	_, goals, _ := runCLIHelp([]string{"help", "goals"}, registered)
	if !strings.HasPrefix(goals, "goals - ") || !strings.Contains(goals, "Plan and steer goals") || !strings.Contains(goals, "metasystem split") {
		t.Errorf("help goals = %q", goals)
	}
	// Settings validation is discoverable from both check and settings.
	for _, page := range []string{"check", "settings"} {
		if _, text, _ := runCLIHelp([]string{"help", page}, registered); !strings.Contains(text, "check settings") {
			t.Errorf("help %s does not name check settings: %q", page, text)
		}
	}
	// Raw unit references remain compatible but ordinary discovery names work.
	for _, topic := range []string{"work", "status", "all"} {
		if _, text, _ := runCLIHelp([]string{"help", topic}, registered); strings.Contains(text, "metasystem status unit") {
			t.Errorf("help %s advertises an internal work reference: %q", topic, text)
		}
	}
	// Hidden options: parseable, absent from help and suggestions.
	approve, _ := findIntentCommand("approve")
	var page bytes.Buffer
	writeIntentCommandHelp(&page, approve)
	for _, hidden := range []string{"--fixture-human-authority", "--lineage", "--approved-ref"} {
		if strings.Contains(page.String(), hidden) {
			t.Errorf("approve help shows %s", hidden)
		}
	}
	if _, problem := parseIntentArgs(approve, []string{"g", "--fixture-human-authority", "--lineage", "L"}); problem != nil {
		t.Errorf("hidden options no longer parse: %s", problem.summary)
	}
	if _, problem := parseIntentArgs(approve, []string{"g", "--fixture-human"}); problem == nil || strings.Contains(problem.summary, "fixture-human-authority") || strings.Contains(problem.summary, "--lineage") {
		t.Errorf("a near miss suggests a hidden option: %+v", problem)
	}
	code, out, problem := runCLIHelp([]string{"biuld"}, registered)
	if code != 2 || out != "" || !strings.Contains(problem, "did you mean: metasystem build") || strings.Contains(problem, "internal") {
		t.Errorf("unknown word = %d %q %q", code, out, problem)
	}
	// Compatibility spellings keep routing and say what replaced them.
	for name, current := range map[string]string{"doctor": "metasystem check", "red": "metasystem incidents", "ready": "metasystem land G --queue-only"} {
		command, ok := findIntentCommand(name)
		var help bytes.Buffer
		writeIntentCommandHelp(&help, command)
		if !ok || !command.compatibility || !strings.Contains(help.String(), current) {
			t.Errorf("%s = %t %t %q", name, ok, command.compatibility, help.String())
		}
	}
}

// TestRealPartnerPublicCatalogue: the Partner's kit tool, given the real
// catalogue, names public commands once each, with their usage, and never an
// engine family or a doubled executable name.
func TestRealPartnerPublicCatalogue(t *testing.T) {
	t.Parallel()
	readers := uitools.Readers{Kit: uitools.Kit{Commands: commandCatalogue}}
	build := readers.Answer(uitools.OpKit, uitools.Args{"topic": "build"}).Text()
	if !strings.Contains(build, "metasystem build") || !strings.Contains(build, "metasystem build G [--work NAME] --brief FILE --check COMMAND...") {
		t.Errorf("kit build = %q", build)
	}
	for _, topic := range []string{"build", "review", "goal", "launch"} {
		text := readers.Answer(uitools.OpKit, uitools.Args{"topic": topic}).Text()
		for _, leak := range []string{"metasystem metasystem", "metasystem goal ", "metasystem launch ", "internal"} {
			if strings.Contains(text, leak) {
				t.Errorf("kit %s mentions %q: %q", topic, leak, text)
			}
		}
	}
}
