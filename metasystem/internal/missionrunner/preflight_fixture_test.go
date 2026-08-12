package missionrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The armed-preflight fixture (the design recorded in the plan): a real
// git checkout with frozen instruments, a contract sealed and signed
// in-test, a bare origin carrying the signed bytes, live tagged holder
// processes behind fabricated-but-honest supervision facts, and a stub
// armer whose fingerprint agrees at seal and preflight by construction —
// the exact recipe scripts/agents/mission-fixtures.sh uses, ported.

const fixtureContract = `# Intent

Reach the declared score with frozen instruments.

# Non-goals

Do not publish or deploy.

# Initial streams

Keep one stream active when the other needs a reserved decision.

` + "```mission" + `
gate.command=scripts/gate.sh
gate.ref=instruments
gate.paths=scripts/gate.sh
truth.paths=truth/*.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.audit.command=scripts/gate.sh
guard.audit.floor=1
guard.audit.noise=0
guard.cadence=1
ledger.cycle-budget=3
ledger.no-gain-budget=2
fence.wall-clock-hours=2
fence.cycles=3
fence.jobs=4
fence.concurrency=2
fence.job-cap-min=30
host.runtime=fake
host.model=fake-model
host.turn-cap-min=30
stream.primary=Reach the acceptance score.
envelope.dependencies=jq
exposure=EUR:10
` + "```" + `
`

func fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// spawnTaggedHold starts a live process whose argv carries the tag, and
// returns its pid and kernel start time. The caller's cleanup kills it.
func spawnTaggedHold(t *testing.T, tag string) (int, int64) {
	t.Helper()
	cmd := exec.Command("bash", "-c", "exec -a "+tag+" sleep 120")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	exact, state, err := identity.KernelProber{}.Probe(int64(cmd.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("hold not probeable: %v %v", state, err)
	}
	if !exact.ArgvKnown || !strings.Contains(strings.Join(exact.Argv, " "), tag) {
		t.Skipf("argv tagging not visible on this host")
	}
	return cmd.Process.Pid, exact.StartedAt.Unix()
}

// buildPreflightRoot assembles the whole launch-gate world and returns the
// engine pointed at it.
func buildPreflightRoot(t *testing.T) *Engine {
	return buildPreflightRootWithStream(t, "")
}

// buildPreflightRootWithStream appends a directive to the contract's
// primary stream text, which the prompt carries and the fake host reads.
func buildPreflightRootWithStream(t *testing.T, directive string) *Engine {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "origin.git")
	for _, dir := range []string{"scripts/agents", "truth", "plans", "docs"} {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
	}
	os.WriteFile(filepath.Join(root, "scripts", "gate.sh"),
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\n'\n"), 0o755)
	os.WriteFile(filepath.Join(root, "truth", "reference.txt"), []byte("certified truth\n"), 0o644)
	// The seal reads the pre-authorization table from project-rules; the
	// fixture carries the real repository's table verbatim.
	rules, err := os.ReadFile(filepath.Join("..", "..", "docs", "project-rules.md"))
	if err != nil {
		t.Skipf("project rules not readable from the test working directory: %v", err)
	}
	os.WriteFile(filepath.Join(root, "docs", "project-rules.md"), rules, 0o644)
	// The stub armer: ARMED for arming, a fixed fingerprint for both seal
	// and preflight — agreement by construction.
	os.WriteFile(filepath.Join(root, "scripts", "agents", "arm-supervision.sh"), []byte(
		"#!/usr/bin/env bash\nset -euo pipefail\n"+
			"if [[ ${1:-} == fingerprint ]]; then printf 'fixture-fingerprint\\n'; exit 0; fi\n"+
			"printf 'ARMED\\n'\n"), 0o755)

	fixtureGit(t, root, "init", "-q", "-b", "main")
	fixtureGit(t, root, "config", "user.name", "fixture")
	fixtureGit(t, root, "config", "user.email", "fixture@example.invalid")
	fixtureGit(t, root, "add", "scripts", "truth", "docs")
	fixtureGit(t, root, "commit", "-qm", "instruments")
	fixtureGit(t, root, "tag", "instruments")
	gitInitBare := exec.Command("git", "init", "-q", "-b", "main", "--bare", remote)
	if out, err := gitInitBare.CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	fixtureGit(t, root, "remote", "add", "origin", remote)
	fixtureGit(t, root, "push", "-qu", "origin", "main")
	fixtureGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	// Older git does not mint origin/HEAD on push; the origin verification
	// resolves through it, and the shell fixture sets it explicitly too.
	fixtureGit(t, root, "remote", "set-head", "origin", "-a")

	// The contract: written, sealed, signed, committed, pushed.
	engine := &Engine{Root: root, Mission: "alpha"}
	contractPath := engine.contractPath()
	document := fixtureContract
	if directive != "" {
		document = strings.Replace(document,
			"stream.primary=Reach the acceptance score.",
			"stream.primary=Reach the acceptance score. "+directive, 1)
	}
	os.WriteFile(contractPath, []byte(document), 0o644)
	sha, err := contract.Seal(contractPath)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	approval := "\nApproval: name=Fixture Human; date=2026-08-12; contract-sha256=" + sha + "\n"
	handle, _ := os.OpenFile(contractPath, os.O_APPEND|os.O_WRONLY, 0o644)
	handle.WriteString(approval)
	handle.Close()
	fixtureGit(t, root, "add", "plans")
	fixtureGit(t, root, "commit", "-qm", "sign mission contract")
	fixtureGit(t, root, "push", "-q", "origin", "main")

	// Supervision facts: live tagged holders behind honest state files.
	watcherPid, watcherStart := spawnTaggedHold(t, "fixture-watcher-tag")
	reaperPid, reaperStart := spawnTaggedHold(t, "fixture-reaper-tag")
	supervision := filepath.Join(root, "artifacts", "agents", "supervision")
	os.MkdirAll(supervision, 0o755)
	now := time.Now().Unix()
	watcherBeat := filepath.Join(supervision, "watcher.heartbeat.json")
	reaperBeat := filepath.Join(supervision, "reaper.heartbeat.json")
	writeDoc := func(path string, doc map[string]any) {
		if err := atomicWriteJSON(path, doc); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc(watcherBeat, map[string]any{
		"function": "watcher", "pid": watcherPid, "pidStartedAt": watcherStart, "observedAtEpoch": now,
	})
	writeDoc(reaperBeat, map[string]any{
		"function": "reaper", "pid": reaperPid, "pidStartedAt": reaperStart, "observedAtEpoch": now,
	})
	writeDoc(filepath.Join(supervision, "state.json"), map[string]any{
		"intervalSec": 60, "fingerprint": "fixture-fingerprint",
		"components": map[string]any{
			"watcher": map[string]any{"pid": watcherPid, "pidStartedAt": watcherStart,
				"instanceTag": "fixture-watcher-tag", "heartbeat": watcherBeat},
			"reaper": map[string]any{"pid": reaperPid, "pidStartedAt": reaperStart,
				"instanceTag": "fixture-reaper-tag", "heartbeat": reaperBeat},
		},
	})
	writeDoc(filepath.Join(supervision, "last-census.json"), map[string]any{
		"completedAtEpoch": now, "verdict": "SUCCESS", "fingerprint": "fixture-fingerprint",
	})
	return engine
}

// The full launch gate passes in-process: arming, seal verification,
// approval, origin, the gate command, supervision facts, the lease probe,
// and the pin landing the approved bytes into the fences.
func TestArmAndPreflightFullPass(t *testing.T) {
	engine := buildPreflightRoot(t)
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("the full launch gate refused: %v", err)
	}
	pinned, err := os.ReadFile(engine.approvedContractPath())
	if err != nil {
		t.Fatalf("approved bytes not pinned: %v", err)
	}
	if !strings.Contains(string(pinned), "Approval: name=Fixture Human") {
		t.Fatal("the pinned bytes are not the signed contract")
	}
	fences, err := readJSONDoc(engine.fencesPath())
	if err != nil {
		t.Fatalf("fences unreadable: %v", err)
	}
	sha, _ := fences["approvedContractSha256"].(string)
	if len(sha) != 64 {
		t.Fatalf("fences carry no contract sha: %v", fences["approvedContractSha256"])
	}
	// And the whole thing is idempotent-refusing: a second start steers to
	// resume, exactly as the pin ladder demands.
	if err := engine.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "already pinned; use resume") {
		t.Fatalf("second start: %v", err)
	}
}

// The full cycle in-process: after the armed preflight, internalRun — the
// run-loop body Launch spawns — drives initializeState, ReserveCycle,
// prompt assembly, the real fake-host adapter (through the real engine
// binary), adjudication, measurement, the ledger, conclusion, and the
// anchor. Launch's own subprocess spawn is untestable in-process
// (os.Executable is the test binary), so the loop body is driven directly,
// which is exactly what the spawned process runs.
// buildFullCycleRoot extends the preflight root with the pieces a running
// turn needs: the real engine binary, the real fake-host adapter, a
// permissive prompt checker, the armed pin, and the anchor seam. The
// behavior directive, when given, rides the contract's stream text into
// the prompt, which is how the fake host selects its behavior.
func buildFullCycleRoot(t *testing.T, behavior string) *Engine {
	t.Helper()
	engine := buildPreflightRootWithStream(t, behavior)
	root := engine.Root
	binary, err := os.ReadFile(filepath.Join("..", "..", "bin", "metasystem"))
	if err != nil {
		t.Skipf("engine binary not built: %v", err)
	}
	os.MkdirAll(filepath.Join(root, "bin"), 0o755)
	if err := os.WriteFile(filepath.Join(root, "bin", "metasystem"), binary, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "hosts", "fake.sh"))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "scripts", "agents", "hosts"), 0o755)
	os.WriteFile(filepath.Join(root, "scripts", "agents", "hosts", "fake.sh"), adapter, 0o755)
	os.WriteFile(filepath.Join(root, "scripts", "assert-turn-prompt.sh"),
		[]byte("#!/usr/bin/env bash\nexit 0\n"), 0o755)
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	engine.anchorFn = func(statePath, ledgerPath, identityName string) error {
		return mission.Anchor(statePath, engine.Root, ledgerPath)
	}
	return engine
}

func TestInternalRunFullCycle(t *testing.T) {
	engine := buildFullCycleRoot(t, "")
	root := engine.Root
	_ = root

	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	if code != 0 {
		// Surface the run's own trail before any assertion: the runner log,
		// the signal, and the newest turn's record.
		if data, err := os.ReadFile(signal); err == nil {
			t.Logf("start signal: %s", data)
		}
		_, _, logPath := engine.runnerPaths()
		if data, err := os.ReadFile(logPath); err == nil {
			t.Logf("runner log tail: %s", tailOf(string(data), 800))
		}
		turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
		for _, turn := range turns {
			if data, err := os.ReadFile(turn); err == nil {
				t.Logf("%s: %s", turn, data)
			}
		}
	}

	// The mission ran real cycles: turn one exists and is terminal, the
	// state file advanced, and the ledger booked at least one cycle. The
	// exact terminal (completed, parked, budget) is the mission's business —
	// the cycle plumbing under test either carried it there or errored.
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatalf("no mission state after the run (rc=%d): %v", code, err)
	}
	status, _ := state["status"].(string)
	if status == "" || status == "running" {
		t.Fatalf("the mission did not reach a terminal: status=%q rc=%d", status, code)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatalf("no turns ran (rc=%d, status=%s)", code, status)
	}
	firstTurn, err := readJSONDoc(turns[0])
	if err != nil {
		t.Fatal(err)
	}
	turnStatus, _ := firstTurn["status"].(string)
	if turnStatus == "pending" || turnStatus == "running" {
		t.Fatalf("turn one never settled: %q", turnStatus)
	}
	ledger := filepath.Join(engine.missionDir(), "ledger.md")
	if data, err := os.ReadFile(ledger); err != nil || !strings.Contains(string(data), "Cycle 1") {
		t.Fatalf("the ledger booked nothing: %v", err)
	}
}

func tailOf(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[len(text)-n:]
}

// The fault alternations, on the same fixture: a host that exits non-zero
// drives the failed-turn conclusion; the mission still books the cycle and
// reaches a terminal rather than hanging or losing the record.
func TestInternalRunHostFailureCycle(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:exit-nonzero")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	status, _ := state["status"].(string)
	if status == "" || status == "running" {
		t.Fatalf("no terminal after host failures: %q rc=%d", status, code)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turns ran")
	}
	first, _ := readJSONDoc(turns[0])
	if outcome, _ := first["outcome"].(string); outcome != "failed" {
		t.Fatalf("turn one's outcome: %q", outcome)
	}
}
