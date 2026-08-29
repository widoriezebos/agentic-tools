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
