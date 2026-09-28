package adapter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The runtime's identity read and probe are required inputs, and a failing
// identity read aborts before any delegate child is spawned.
func TestSelftestRunRequiresIdentityAndProbe(t *testing.T) {
	t.Parallel()
	root := stageSelftestFixture(t, "mapped", "mapped")
	base := SelftestParams{
		Root: root, Runtime: "stub", RunIdentity: stubAdapterStep(root), RunProbe: stubAdapterStep(root),
		Status: stubStatus(root), returnCheck: acceptSelftestReturns, Usage: "native", TurnCeilingSec: 10,
	}
	for name, edit := range map[string]func(*SelftestParams){
		"no identity": func(p *SelftestParams) { p.RunIdentity = nil },
		"no probe":    func(p *SelftestParams) { p.RunProbe = nil },
	} {
		p := base
		edit(&p)
		err := SelftestRun(p, "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "requires the runtime's identity and probe") {
			t.Errorf("%s: SelftestRun = %v, want the named refusal", name, err)
		}
	}
	identityFailure := errors.New("identity read failed")
	p := base
	p.RunIdentity = func() error { return identityFailure }
	if err := SelftestRun(p, "stub-model", &strings.Builder{}); !errors.Is(err, identityFailure) {
		t.Fatalf("identity failure = %v, want it returned", err)
	}
	if jobs, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json")); len(jobs) != 0 {
		t.Fatalf("a refused self-test dispatched jobs: %v", jobs)
	}
}

// Engine replaces ROOT/bin/metasystem for every delegate child, and ExtraEnv
// reaches each dispatch and follow-up: a recording engine sees those legs carry the extra
// assignment and the self-test marker, then hands off to the stub engine.
func TestSelftestRunUsesTheEngineAndPassesExtraEnv(t *testing.T) {
	t.Parallel()
	root := stageSelftestFixture(t, "mapped", "mapped")
	log := filepath.Join(t.TempDir(), "engine.log")
	engine := filepath.Join(t.TempDir(), "recording-engine")
	script := "#!/usr/bin/env bash\nprintf '%s %s %s\\n' \"$3\" \"${SELFTEST_EXTRA:-missing}\" \"${METASYSTEM_DELEGATE_SELFTEST_INTERNAL:-0}\" >>" +
		shellQuote(log) + "\nexec " + shellQuote(filepath.Join(root, "bin", "metasystem")) + " \"$@\"\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	p := SelftestParams{
		Root: root, Runtime: "stub", RunIdentity: stubAdapterStep(root), RunProbe: stubAdapterStep(root),
		Status: stubStatus(root), returnCheck: acceptSelftestReturns, Usage: "native", TurnCeilingSec: 10,
		Engine: engine, ExtraEnv: []string{"SELFTEST_EXTRA=critic-model"},
	}
	if err := SelftestRun(p, "stub-model", &out); err != nil {
		t.Fatalf("selftest through the recording engine failed: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the configured engine never ran: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	seen := map[string]bool{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("unexpected engine log line: %q", line)
		}
		seen[fields[0]] = true
		if fields[0] == "--cancel" {
			continue // the cancel verb carries no model override
		}
		if fields[1] != "critic-model" {
			t.Fatalf("a delegate child lacked the extra environment: %q", line)
		}
		// Only the self-test dispatch legs carry the internal marker; the
		// follow-up rides the ordinary delegate path.
		if wantMarker := fields[0] == "--adapter-selftest"; (fields[2] == "1") != wantMarker {
			t.Fatalf("self-test marker on %q: %q", fields[0], line)
		}
	}
	for _, leg := range []string{"--adapter-selftest", "--follow-up", "--cancel"} {
		if !seen[leg] {
			t.Fatalf("the engine never ran the %s leg: %q", leg, lines)
		}
	}
}

// A missing engine is a hard failure at the first delegate child, never a
// silent fall-back to the checkout's own binary.
func TestSelftestRunMissingEngineFails(t *testing.T) {
	t.Parallel()
	root := stageSelftestFixture(t, "mapped", "mapped")
	p := SelftestParams{
		Root: root, Runtime: "stub", RunIdentity: stubAdapterStep(root), RunProbe: stubAdapterStep(root),
		Status: stubStatus(root), returnCheck: acceptSelftestReturns, Usage: "native", TurnCeilingSec: 10,
		Engine: filepath.Join(t.TempDir(), "no-such-engine"),
	}
	if err := SelftestRun(p, "stub-model", &strings.Builder{}); err == nil {
		t.Fatal("a missing engine ran the self-test")
	}
	if jobs, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json")); len(jobs) != 0 {
		t.Fatalf("the checkout's own engine ran despite the configured one: %v", jobs)
	}
}

func TestSelftestStatusWithoutALifecycleIsUnknown(t *testing.T) {
	t.Parallel()
	reaped := false
	if got := (SelftestParams{}).dispatchStatus("job-1"); got != "" {
		t.Fatalf("status without a lifecycle = %q, want empty", got)
	}
	(SelftestParams{}).reapJob("job-1") // no lifecycle: nothing to reap, no panic
	(SelftestParams{Reap: func(job string) { reaped = job == "job-1" }}).reapJob("job-1")
	if !reaped {
		t.Fatal("the lifecycle reap was not called with the job")
	}
}
