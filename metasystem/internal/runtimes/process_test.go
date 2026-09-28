package runtimes

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The supervisor argv a launcher builds is the argv the recognizer shapes
// name: entry word, runtime, verb, then the flags in order.
func TestSupervisorArgsBuildsEngineArgv(t *testing.T) {
	t.Parallel()
	got := SupervisorArgs("codex", SupervisorDispatch, "--root", "/r", SupervisorTagFlag, "tag-1")
	want := []string{"delegate-supervisor", "codex", "dispatch", "--root", "/r", "--instance-tag", "tag-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupervisorArgs = %v, want %v", got, want)
	}
	if bare := SupervisorArgs("claude", SupervisorHostTurn); !reflect.DeepEqual(bare, []string{"delegate-supervisor", "claude", "start-turn"}) {
		t.Fatalf("SupervisorArgs without flags = %v", bare)
	}
}

// A delegate runtime gets a dispatch and a follow-up shape, a host runtime a
// start-turn shape, and a runtime that is neither gets none.
func TestRuntimeSupervisorShapesPerRole(t *testing.T) {
	t.Parallel()
	both := RuntimeSupervisorShapes("acme", true, true)
	wantBoth := []SupervisorShape{
		{Name: "adapter-supervisor-acme-dispatch", Includes: []string{"delegate-supervisor", "acme", "dispatch"}, TagFlag: "--instance-tag"},
		{Name: "adapter-supervisor-acme-follow-up", Includes: []string{"delegate-supervisor", "acme", "follow-up"}, TagFlag: "--instance-tag"},
		{Name: "host-acme-start-turn", Includes: []string{"delegate-supervisor", "acme", "start-turn"}, TagFlag: "--instance-tag"},
	}
	if !reflect.DeepEqual(both, wantBoth) {
		t.Fatalf("delegate+host shapes = %#v, want %#v", both, wantBoth)
	}
	if got := RuntimeSupervisorShapes("acme", false, true); len(got) != 1 || got[0].Name != "host-acme-start-turn" {
		t.Fatalf("host-only shapes = %#v", got)
	}
	if got := RuntimeSupervisorShapes("acme", true, false); len(got) != 2 || got[0].Name != "adapter-supervisor-acme-dispatch" {
		t.Fatalf("delegate-only shapes = %#v", got)
	}
	if got := RuntimeSupervisorShapes("acme", false, false); len(got) != 0 {
		t.Fatalf("neither role must yield no shapes, got %#v", got)
	}
}

// The shipped universe yields three shapes per runtime (every shipped
// runtime is both a delegate and a host), and every shape's words match the
// argv SupervisorArgs builds for it.
func TestSupervisorShapesCoverEveryShippedRuntime(t *testing.T) {
	t.Parallel()
	shapes := SupervisorShapes()
	byName := map[string]SupervisorShape{}
	for _, s := range shapes {
		byName[s.Name] = s
	}
	for _, name := range []string{"claude", "codex", "devin", "fake"} {
		for _, verb := range []string{SupervisorDispatch, SupervisorFollowUp} {
			s, ok := byName["adapter-supervisor-"+name+"-"+verb]
			if !ok {
				t.Fatalf("missing %s %s shape in %v", name, verb, shapes)
			}
			if !reflect.DeepEqual(s.Includes, SupervisorArgs(name, verb)) {
				t.Fatalf("shape %s includes %v, launcher argv %v", s.Name, s.Includes, SupervisorArgs(name, verb))
			}
		}
		if _, ok := byName["host-"+name+"-start-turn"]; !ok {
			t.Fatalf("missing %s host shape", name)
		}
	}
	if len(shapes) != 3*len(declarations) {
		t.Fatalf("got %d shapes for %d runtimes", len(shapes), len(declarations))
	}
}

// The shared exclusions are regexes that keep the supervision hook and the
// supervisor process (never the CLI) out of a signature.
func TestSharedExcludesMatchHookAndSupervisor(t *testing.T) {
	t.Parallel()
	ex := SharedExcludes()
	if len(ex) != 2 {
		t.Fatalf("SharedExcludes = %v", ex)
	}
	for _, e := range ex {
		if strings.HasPrefix(e, "exclude ") {
			t.Fatalf("shared exclude %q keeps the declaration keyword", e)
		}
	}
	hook := regexp.MustCompile(ex[0])
	sup := regexp.MustCompile(ex[1])
	if !hook.MatchString("/bin/bash /x/scripts/supervision-hook.sh pre") {
		t.Fatal("hook exclude does not match the supervision hook")
	}
	for _, argv := range []string{
		"/opt/bin/metasystem delegate-supervisor codex dispatch --root /r",
		"metasystem internal delegate-supervisor claude start-turn",
	} {
		if !sup.MatchString(argv) {
			t.Fatalf("supervisor exclude misses %q", argv)
		}
	}
	for _, argv := range []string{"codex exec -c x", "metasystem delegate-supervisorx", "notmetasystem delegate-supervisor"} {
		if sup.MatchString(argv) {
			t.Fatalf("supervisor exclude wrongly matches %q", argv)
		}
	}
}

// Every shipped adapter runtime has a signature whose lines are match or
// exclude declarations, ending in a newline, and carrying the supervisor
// exclusion; the signature recognizes its own CLI.
func TestSignatureTextShippedRuntimes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"claude": "/usr/local/bin/claude -p hello",
		"codex":  "codex exec -c metasystem_instance_tag=t",
		"devin":  "devin-delegate-acp --serve",
		"fake":   "/x/metasystem-fake-agent run",
	}
	for name, argv := range cases {
		text, err := SignatureText(name)
		if err != nil {
			t.Fatalf("SignatureText(%s): %v", name, err)
		}
		if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
			t.Fatalf("%s signature not newline-terminated once: %q", name, text)
		}
		matched := false
		for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			kind, pattern, ok := strings.Cut(line, " ")
			if !ok || (kind != "match" && kind != "exclude") {
				t.Fatalf("%s signature line %q is not a declaration", name, line)
			}
			re := regexp.MustCompile(pattern)
			if kind == "match" && re.MatchString(argv) {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("%s signature does not match its own CLI argv %q", name, argv)
		}
		if !strings.Contains(text, supervisorExclude) {
			t.Fatalf("%s signature lacks the supervisor exclusion", name)
		}
	}
}

func TestSignatureTextRefusesUndeclaredRuntime(t *testing.T) {
	t.Parallel()
	if text, err := SignatureText("nosuch"); err == nil || text != "" || !strings.Contains(err.Error(), `"nosuch" declares no delegate signature`) {
		t.Fatalf("SignatureText(nosuch) = %q, %v", text, err)
	}
}

// A declared runtime without an adapter, and an adapter runtime with no
// signature lines, both refuse. Not parallel: it swaps the universe.
func TestSignatureTextAndConfigRefuseWithoutAdapterOrLines(t *testing.T) {
	restore := OverrideForTest([]Declaration{
		{Name: "hostonly", HasHostLauncher: true},
		{Name: "bare", HasAdapter: true},
	})
	defer restore()
	for _, name := range []string{"hostonly", "bare"} {
		if _, err := SignatureText(name); err == nil {
			t.Fatalf("SignatureText(%s) must refuse", name)
		}
	}
	if paths, ok := LocalConfigPaths("hostonly"); ok || paths != nil {
		t.Fatalf("LocalConfigPaths(hostonly) = %v, %v", paths, ok)
	}
	if paths, ok := LocalConfigPaths("bare"); !ok || len(paths) != 0 {
		t.Fatalf("LocalConfigPaths(bare) = %v, %v; an adapter runtime with no files reports an empty set", paths, ok)
	}
	if _, ok := Lookup("claude"); ok {
		t.Fatal("override did not replace the universe")
	}
}

// Local config paths are the declared checkout-relative files, returned as
// a copy the caller may not use to mutate the declaration.
func TestLocalConfigPaths(t *testing.T) {
	t.Parallel()
	got, ok := LocalConfigPaths("devin")
	if !ok || !reflect.DeepEqual(got, []string{".devin/config.json", ".devin/config.local.json", ".devin/hooks.v1.json"}) {
		t.Fatalf("LocalConfigPaths(devin) = %v, %v", got, ok)
	}
	got[0] = "mutated"
	again, _ := LocalConfigPaths("devin")
	if again[0] != ".devin/config.json" {
		t.Fatal("LocalConfigPaths returned the shared slice")
	}
	if got, ok := LocalConfigPaths("fake"); !ok || len(got) != 0 {
		t.Fatalf("LocalConfigPaths(fake) = %v, %v", got, ok)
	}
	if got, ok := LocalConfigPaths("nosuch"); ok || got != nil {
		t.Fatalf("LocalConfigPaths(nosuch) = %v, %v", got, ok)
	}
	for _, name := range []string{"claude", "codex"} {
		paths, ok := LocalConfigPaths(name)
		if !ok || len(paths) == 0 {
			t.Fatalf("LocalConfigPaths(%s) = %v, %v", name, paths, ok)
		}
		for _, p := range paths {
			if !cleanRelative(p) {
				t.Fatalf("%s config path %q is not clean-relative", name, p)
			}
		}
	}
}

// The enforcement map is emitted in canonical field order with the
// declared values; a runtime that declares none (fake) or is unknown has no
// map.
func TestEnforcementMapJSON(t *testing.T) {
	t.Parallel()
	got, ok := EnforcementMapJSON("codex")
	if !ok || got != `{"writeRoots":"mapped","readRoots":"notEnforced","network":"mapped"}` {
		t.Fatalf("EnforcementMapJSON(codex) = %q, %v", got, ok)
	}
	got, ok = EnforcementMapJSON("devin")
	if !ok || got != `{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}` {
		t.Fatalf("EnforcementMapJSON(devin) = %q, %v", got, ok)
	}
	if got, ok := EnforcementMapJSON("fake"); ok || got != "" {
		t.Fatalf("EnforcementMapJSON(fake) = %q, %v", got, ok)
	}
	if got, ok := EnforcementMapJSON("nosuch"); ok || got != "" {
		t.Fatalf("EnforcementMapJSON(nosuch) = %q, %v", got, ok)
	}
}

// CLI invocation shapes name the tag placement per runtime and are
// returned as copies; an undeclared runtime has none.
func TestCLIInvocations(t *testing.T) {
	t.Parallel()
	codex := CLIInvocations("codex")
	if len(codex) != 1 || !reflect.DeepEqual(codex[0].Includes, []string{"codex", "exec"}) ||
		codex[0].TagFlag != "-c" || codex[0].TagPrefix != "metasystem_instance_tag=" || codex[0].TagPathBase {
		t.Fatalf("CLIInvocations(codex) = %#v", codex)
	}
	devin := CLIInvocations("devin")
	if len(devin) != 1 || devin[0].TagFlag != "--config" || !devin[0].TagPathBase {
		t.Fatalf("CLIInvocations(devin) = %#v", devin)
	}
	claude := CLIInvocations("claude")
	if len(claude) != 1 || claude[0].TagFlag != "--name" || claude[0].TagPrefix != "" {
		t.Fatalf("CLIInvocations(claude) = %#v", claude)
	}
	claude[0].TagFlag = "mutated"
	if CLIInvocations("claude")[0].TagFlag != "--name" {
		t.Fatal("CLIInvocations returned the shared slice")
	}
	if got := CLIInvocations("fake"); len(got) != 0 {
		t.Fatalf("CLIInvocations(fake) = %#v", got)
	}
}

// With no adoption default declared the answer is empty, not a guess.
func TestAdoptionDefaultEmptyWithoutDefault(t *testing.T) {
	restore := OverrideForTest([]Declaration{{Name: "solo", TailoringPriority: 1}})
	defer restore()
	if got := AdoptionDefault(); got != "" {
		t.Fatalf("AdoptionDefault() = %q, want empty", got)
	}
}
