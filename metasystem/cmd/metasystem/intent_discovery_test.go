package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// TestIntentPublicDiscovery: a bare call and --help are the same short,
// successful orientation over the objects; an object lists its actions;
// hidden options stay parseable but are never shown or suggested; an
// unknown word is answered with a public form, never an engine family.
func TestIntentPublicDiscovery(t *testing.T) {
	t.Parallel()
	registered := families()
	code, root, problem := runCLIHelp(nil, registered)
	if code != 0 || problem != "" {
		t.Fatalf("bare = %d %q", code, problem)
	}
	if lines := strings.Count(root, "\n"); lines > 50 {
		t.Errorf("the root page is %d lines; it is an orientation, not an inventory", lines)
	}
	for _, leak := range []string{"internal", "FAMILY", "usage: metasystem <family>", "goal list", "fetch", "run-loop"} {
		if strings.Contains(root, leak) {
			t.Errorf("the root page mentions %q", leak)
		}
	}
	for _, want := range []string{"Plan:", "Deliver:", "Run:", "Practice:", "metasystem OBJECT ACTION --help", "metasystem work review my-goal", "metasystem work land my-goal"} {
		if !strings.Contains(root, want) {
			t.Errorf("the root page lacks %q", want)
		}
	}
	for index, group := range intentGroups {
		heading := strings.Index(root, group.heading+":")
		if heading < 0 {
			t.Fatalf("the root page has no %s group", group.heading)
		}
		for _, object := range group.objects {
			at := strings.Index(root, "\n  "+object+" ")
			next := len(root)
			if index+1 < len(intentGroups) {
				next = strings.Index(root, intentGroups[index+1].heading+":")
			}
			if at < heading || at > next {
				t.Errorf("%s is not listed under %s", object, group.heading)
			}
		}
	}
	// Settings validation is discoverable from the settings object.
	if _, text, _ := runCLIHelp([]string{"settings"}, registered); !strings.Contains(text, "check ") {
		t.Errorf("metasystem settings does not name settings check: %q", text)
	}
	// Hidden options: parseable, absent from help and suggestions.
	approve, _ := findIntentCommand("goal approve")
	var page bytes.Buffer
	writeIntentCommandHelp(&page, approve)
	for _, hidden := range []string{"--fixture-human-authority", "--lineage", "--approved-ref"} {
		if strings.Contains(page.String(), hidden) {
			t.Errorf("goal approve help shows %s", hidden)
		}
	}
	if _, problem := parseIntentArgs(approve, []string{"g", "--fixture-human-authority", "--lineage", "L"}); problem != nil {
		t.Errorf("hidden options no longer parse: %s", problem.summary)
	}
	if _, problem := parseIntentArgs(approve, []string{"g", "--fixture-human"}); problem == nil || strings.Contains(problem.summary, "fixture-human-authority") || strings.Contains(problem.summary, "--lineage") {
		t.Errorf("a near miss suggests a hidden option: %+v", problem)
	}
	if _, problem := parseIntentArgs(approve, []string{"g", "--budgt", "norm"}); problem == nil || !strings.Contains(strings.Join(problem.next, " "), "metasystem goal approve g --budget norm") {
		t.Errorf("a near-miss option is not corrected in the object-action spelling: %+v", problem)
	}
	code, out, problem := runCLIHelp([]string{"wrok"}, registered)
	if code != 2 || out != "" || !strings.Contains(problem, "did you mean: metasystem work") || strings.Contains(problem, "internal") {
		t.Errorf("unknown word = %d %q %q", code, out, problem)
	}
}

// TestRealPartnerPublicCatalogue: the Partner's kit tool, given the real
// catalogue, names public actions once each, with their usage, and never an
// engine family, an entry or a doubled executable name.
func TestRealPartnerPublicCatalogue(t *testing.T) {
	t.Parallel()
	readers := uitools.Readers{Kit: uitools.Kit{Commands: commandCatalogue}}
	build := readers.Answer(uitools.OpKit, uitools.Args{"topic": "work build"}).Text()
	if !strings.Contains(build, "metasystem work build") || !strings.Contains(build, "metasystem work build G [--work NAME] --brief FILE --check COMMAND...") {
		t.Errorf("kit build = %q", build)
	}
	status := readers.Answer(uitools.OpKit, uitools.Args{"topic": "metasystem system status"}).Text()
	for _, want := range []string{"metasystem system status", "MetaSystem administration:"} {
		if !strings.Contains(status, want) {
			t.Errorf("kit system status omits %q: %q", want, status)
		}
	}
	for _, command := range commandCatalogue()[0].Verbs {
		if command.Name == "work land" && (command.Scope != "mixed" || len(command.AdministrationUsage) == 0 || len(command.Usage) == 0) {
			t.Errorf("Partner lost work land forms or scope: %+v", command)
		}
		if strings.HasPrefix(command.Name, "settings ") && command.Scope != "administration" {
			t.Errorf("Partner lost settings scope: %+v", command)
		}
	}
	for _, topic := range []string{"build", "review", "goal", "launch", "fetch"} {
		text := readers.Answer(uitools.OpKit, uitools.Args{"topic": topic}).Text()
		for _, leak := range []string{"metasystem metasystem", "metasystem goal fetch", "metasystem launch ", "internal"} {
			if strings.Contains(text, leak) {
				t.Errorf("kit %s mentions %q: %q", topic, leak, text)
			}
		}
	}
}
