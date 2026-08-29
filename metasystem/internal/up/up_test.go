package up

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func liveSelf(t *testing.T) (int64, int64) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read test process: %v %s", err, state)
	}
	return exact.Pid, exact.StartedAt.Unix()
}

func stageEnrollment(t *testing.T, root, binary string, generation int) steward.InstallIdentity {
	t.Helper()
	root = canonicalRuntimePath(root)
	binary = canonicalRuntimePath(binary)
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	installed := steward.InstallIdentity{
		RepoIdentity: root, Generation: generation, InstallPath: binary,
		InstallDigest: fmt.Sprintf("sha256:%x", digest[:]), MintedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), installed); err != nil {
		t.Fatal(err)
	}
	return installed
}

func TestExplicitIdentityFallbackRequiresAndVerifiesTheRecordedPair(t *testing.T) {
	pid, started := liveSelf(t)
	root := t.TempDir()
	resolved, err := resolveSessionIdentity(Options{
		Root: root, Pid: pid, StartTime: started, Session: "explicit", Runtime: "fixture-runtime",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Pid != pid || resolved.StartTime != started || resolved.Session != "explicit" ||
		resolved.Tag != "metasystem-main-fixture-runtime-explicit" ||
		resolved.Provenance.Source != "explicit-ancestry-fallback" ||
		resolved.Provenance.CallerPid != pid {
		t.Fatalf("wrong explicit identity: %+v", resolved)
	}
	if _, err := resolveSessionIdentity(Options{Root: root, Pid: pid}); err == nil || !strings.Contains(err.Error(), "passed together") {
		t.Fatalf("a partial explicit fallback was accepted: %v", err)
	}
	if _, err := resolveSessionIdentity(Options{
		Root: root, Pid: pid, StartTime: started + 1, Runtime: "fixture-runtime",
	}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a mismatched recorded fallback was accepted: %v", err)
	}
}

func TestExplicitIdentityFallbackRejectsALiveStrangerTuple(t *testing.T) {
	command := exec.Command("sleep", "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read child identity: %v %s", err, state)
	}
	_, err = resolveSessionIdentity(Options{
		Root: t.TempDir(), Pid: exact.Pid, StartTime: exact.StartedAt.Unix(),
		Runtime: "fixture-runtime", CallerPid: int64(os.Getpid()),
	})
	if err == nil || !strings.Contains(err.Error(), "not the caller or one of its ancestors") {
		t.Fatalf("a live stranger tuple granted fallback identity: %v", err)
	}
}

func TestRuntimeSignatureAbsenceFailsBeforeArmingWithTheFallbackRemedy(t *testing.T) {
	t.Setenv("METASYSTEM_AGENT_RUNTIME", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	root := t.TempDir()
	binary, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	stageEnrollment(t, root, binary, 1)
	result := ordinary(Options{Root: root, MetasystemRoot: root, Scope: root, Binary: binary, WaitScaleMilli: 1})
	if result.Outcome != "failed" || result.Failed != "session-identity" ||
		!strings.Contains(result.Components[len(result.Components)-1].Detail, "lists no metasystem.runtimes") ||
		!strings.Contains(result.Remedy, "--pid <session-pid>") {
		t.Fatalf("runtime-signature absence did not fail cleanly: %+v", result)
	}
	for _, path := range []string{
		filepath.Join(root, "artifacts", "agents", "mains"),
		filepath.Join(root, "artifacts", "agents", "supervision"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("identity refusal mutated %s: %v", path, err)
		}
	}
}

func TestOrdinaryUpRefusesDriftWithoutMintingANewGeneration(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "metasystem")
	if err := os.WriteFile(binary, []byte("accepted\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageEnrollment(t, root, binary, 7)
	if err := os.WriteFile(binary, []byte("candidate\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := ordinary(Options{Root: root, MetasystemRoot: root, Scope: root, Binary: binary, WaitScaleMilli: 1})
	if result.Outcome != "ENROLLMENT_DRIFT" || result.Failed != "accepted-engine" {
		t.Fatalf("ambient drift was not refused by name: %+v", result)
	}
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(root), canonicalRuntimePath(root))
	if err != nil || installed.Generation != 7 {
		t.Fatalf("ambient drift changed enrollment: %+v %v", installed, err)
	}
	for _, path := range []string{
		filepath.Join(root, "artifacts", "agents", "mains"),
		filepath.Join(root, "artifacts", "agents", "supervision"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("ambient drift mutated %s: %v", path, err)
		}
	}
}

func TestAdvisorRenderingNamesHolderAndWorktree(t *testing.T) {
	result := Result{
		Components: []ComponentOutcome{{Component: "checkout-lease", Outcome: "advisor", Detail: "holder=session-a"}},
		Outcome:    "advisor", Authority: "read-only", Holder: "session-a (main-1)",
		Worktree: "scripts/agents/second-session.sh",
	}
	lines := result.Lines()
	if result.ExitCode() != 0 || len(lines) != 2 ||
		lines[1] != `up outcome=advisor authority=read-only holder="session-a (main-1)" worktree="scripts/agents/second-session.sh"` {
		t.Fatalf("wrong advisor outcome: %#v", lines)
	}
}

func TestSchedulerEntryIsRecoveryOnlyAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	entry := SchedulerEntry(Options{Root: root, Scope: root, Binary: root + "/bin/metasystem"})
	if !strings.Contains(entry, "up --metasystem-root") || !strings.Contains(entry, "--recover-only --if-down") {
		t.Fatalf("scheduler entry is not restricted recovery: %s", entry)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("printing the scheduler entry wrote host or repository state: %v %+v", err, entries)
	}
}

func TestFailureNamesOneComponentAndRemedy(t *testing.T) {
	result := failure(nil, "repo-watcher", os.ErrInvalid, "rerun metasystem up")
	lines := result.Lines()
	if result.ExitCode() == 0 || len(lines) != 2 ||
		lines[1] != `up outcome=failed component=repo-watcher remedy="rerun metasystem up"` {
		t.Fatalf("wrong aggregate failure: %#v", lines)
	}
}

func TestRecoveryRequiresTheRestrictedIfDownFence(t *testing.T) {
	t.Setenv("METASYSTEM_EXECUTION_ID", "delegate-execution")
	result := Run(Options{RecoverOnly: true})
	if result.ExitCode() == 0 || result.Failed != "recovery-mode" ||
		!strings.Contains(result.Lines()[0], "--recover-only requires --if-down") {
		t.Fatalf("unfenced recovery was not refused: %+v", result)
	}
	if got := os.Getenv("METASYSTEM_EXECUTION_ID"); got != "" {
		t.Fatalf("up retained delegated execution attribution: %q", got)
	}
}

func TestRecoveryRefusesBeforeArmingWithoutAnEnrolledEngine(t *testing.T) {
	root := t.TempDir()
	result := Run(Options{
		Root: root, Scope: root, Binary: "/bin/true", RecoverOnly: true, IfDown: true,
	})
	if result.ExitCode() == 0 || result.Outcome != "ENROLLMENT_DRIFT" || result.Failed != "accepted-engine" {
		t.Fatalf("unenrolled recovery was not refused: %+v", result)
	}
	if len(result.Components) != 1 || result.Components[0].Outcome != "ENROLLMENT_DRIFT" {
		t.Fatalf("enrollment refusal was not typed: %+v", result.Components)
	}
	if _, err := os.Stat(root + "/artifacts/agents/supervision"); !os.IsNotExist(err) {
		t.Fatalf("recovery mutated supervision before enrollment proof: %v", err)
	}
}

func TestSessionDefaultsAndRecordedProcessComparison(t *testing.T) {
	t.Setenv("METASYSTEM_SESSION_ID", "session-from-environment")
	t.Setenv("METASYSTEM_INSTANCE_TAG", "tag-from-environment")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "lineage-from-environment")
	if got := sessionValue("", 41); got != "session-from-environment" {
		t.Fatalf("session environment was ignored: %q", got)
	}
	if got := sessionValue("explicit", 41); got != "explicit" {
		t.Fatalf("explicit session was replaced: %q", got)
	}
	if got := sessionTag("", "codex", "session"); got != "tag-from-environment" {
		t.Fatalf("instance tag environment was ignored: %q", got)
	}
	if got := ownerLineage(""); got != "lineage-from-environment" {
		t.Fatalf("owner lineage environment was ignored: %q", got)
	}
	if got := runtimeValue(""); got != "unknown" {
		t.Fatalf("empty runtime = %q, want unknown", got)
	}
	t.Setenv("METASYSTEM_SESSION_ID", "")
	t.Setenv("METASYSTEM_INSTANCE_TAG", "")
	if got := sessionValue("", 41); got != "session-41" {
		t.Fatalf("pid fallback session = %q", got)
	}
	if got := sessionTag("", "codex", "session 41"); got != "metasystem-main-codex-session-41" {
		t.Fatalf("default instance tag = %q", got)
	}

	legacy := census.ProcIdentity{Pid: 41, PidStartedAt: 100}
	if !sameAuthenticatedProcess(legacy, legacy) {
		t.Fatal("identical legacy process identities did not match")
	}
	otherPid := legacy
	otherPid.Pid++
	if sameAuthenticatedProcess(legacy, otherPid) {
		t.Fatal("different pids matched")
	}
	paired := census.ProcIdentity{Pid: 41, PidStartedAt: 100, PidStartTicks: 900, BootID: "boot-a"}
	if !sameAuthenticatedProcess(paired, paired) {
		t.Fatal("identical paired process identities did not match")
	}
	rebooted := paired
	rebooted.BootID = "boot-b"
	if sameAuthenticatedProcess(paired, rebooted) {
		t.Fatal("process identities from different boots matched")
	}
}

func TestCallerAncestryProofWalksParentsAndStopsOnCycles(t *testing.T) {
	prior := sessionParentPid
	parents := map[int64]int64{30: 20, 20: 10, 10: 1, 40: 40}
	sessionParentPid = func(pid int64) (int64, bool) {
		parent, ok := parents[pid]
		return parent, ok
	}
	t.Cleanup(func() { sessionParentPid = prior })
	if err := proveCallerDescendsFromTarget(30, 10); err != nil {
		t.Fatalf("ancestor was not proven: %v", err)
	}
	if err := proveCallerDescendsFromTarget(30, 99); err == nil {
		t.Fatal("unrelated target was accepted as an ancestor")
	}
	if err := proveCallerDescendsFromTarget(40, 10); err == nil {
		t.Fatal("a parent cycle granted ancestry")
	}
}

func TestWatchIntervalDefaultsAndRejectsInvalidConfiguration(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if interval, err := watchInterval(root); err != nil || interval != 60 {
		t.Fatalf("default watch interval: interval=%d err=%v", interval, err)
	}
	for _, value := range []string{"0", "not-a-number"} {
		if err := os.WriteFile(conf, []byte("watch.interval-sec="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := watchInterval(root); err == nil || !strings.Contains(err.Error(), "positive integer") {
			t.Fatalf("invalid watch interval %q was accepted: %v", value, err)
		}
	}
}

func TestArmingLogAppendsAndIgnoresUnwritableLayout(t *testing.T) {
	root := t.TempDir()
	appendArmingLog(root, "first event")
	appendArmingLog(root, "second event")
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "supervision", "arming.log"))
	if err != nil || !strings.Contains(string(data), "first event") || !strings.Contains(string(data), "second event") {
		t.Fatalf("arming log did not append both events: %q err=%v", data, err)
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	appendArmingLog(blocked, "must not panic")
}

func TestPreflightNamesMissingProductionCommands(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := preflightCommands()
	if err == nil || !strings.Contains(err.Error(), "required production commands are missing") || !strings.Contains(err.Error(), "git") {
		t.Fatalf("missing command inventory was not named: %v", err)
	}
}

func TestInvokingEnrollmentRefusesAnotherBinaryAtTheSameDigest(t *testing.T) {
	root := t.TempDir()
	accepted := filepath.Join(root, "accepted")
	candidate := filepath.Join(root, "candidate")
	for _, path := range []string{accepted, candidate} {
		if err := os.WriteFile(path, []byte("same binary bytes\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stageEnrollment(t, root, accepted, 3)
	enrolled, err := openInvokingEnrollment(Options{Root: root, Binary: candidate})
	if enrolled != nil || err == nil || !strings.Contains(err.Error(), "invoking engine") {
		t.Fatalf("another binary path used the enrollment: enrolled=%v err=%v", enrolled, err)
	}
}

func TestAdministrativeOutcomesRefuseWithoutIdentityOrHolderAuthority(t *testing.T) {
	retired := Retire(Options{Root: t.TempDir(), Pid: 41})
	if retired.Outcome != "failed" || retired.Failed != "session-identity" {
		t.Fatalf("retire accepted a partial identity: %+v", retired)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identityTable := filepath.Join(t.TempDir(), "identities.json")
	identityBody := fmt.Sprintf(`{"%d":{"terminal":false}}`, os.Getppid())
	if err := os.WriteFile(identityTable, []byte(identityBody), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identityTable)
	shutdown := Shutdown(Options{Root: root, MetasystemRoot: root, Scope: root})
	if shutdown.Outcome != "failed" || shutdown.Failed != "checkout-lease" {
		t.Fatalf("shutdown accepted a non-holder: %+v", shutdown)
	}
	for _, outcome := range []string{"failed", "recovery-partial", "ENROLLMENT_DRIFT"} {
		if code := (Result{Outcome: outcome}).ExitCode(); code != 1 {
			t.Fatalf("outcome %s exit code = %d, want 1", outcome, code)
		}
	}
}

func TestCanonicalRuntimePathResolvesSymlinkedBinary(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "engine")
	link := filepath.Join(root, "engine-link")
	if err := os.WriteFile(target, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got, want := canonicalRuntimePath(link), canonicalRuntimePath(target); got != want {
		t.Fatalf("canonical runtime path = %q, want %q", got, want)
	}
}
