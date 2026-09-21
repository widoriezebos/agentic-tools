package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var declaredContextCostCandidateEngine string

// A package test starts without the engine and installation selectors of its
// caller. Tests that exercise a selected engine add it to the one child
// process that consumes it. The opt-in context-cost proof is the only test
// whose candidate is a package-level input; its proof flag declares that
// input before the ambient selector is removed.
func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "locate command test executable:", err)
		os.Exit(2)
	}
	if filepath.Base(os.Args[0]) == proofrun.TestHostLoadCommandName("0") {
		os.Args[0] = executable
	} else {
		arguments := append([]string{proofrun.TestHostLoadCommandName("0")}, os.Args[1:]...)
		if err := syscall.Exec(executable, arguments, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "replace command test process:", err)
			os.Exit(2)
		}
	}
	// The managed resource worker re-execs this test binary directly. Its
	// custody verbs must reach the command dispatcher even when the caller's
	// fixture-only GO_WANT flag was removed from the inherited environment.
	resourceCustodyCommand := len(os.Args) > 2 && os.Args[1] == "proof-run" &&
		(os.Args[2] == "custody-exec" || os.Args[2] == "watchdog")
	if resourceCustodyCommand || os.Getenv("GO_WANT_BATCH_E2E_COMMAND") == "1" && len(os.Args) > 1 && os.Args[1][0] != '-' {
		os.Exit(dispatch(os.Args[1:]))
	}
	if os.Getenv("METASYSTEM_CONTEXT_COST_PROOF") == "1" {
		declaredContextCostCandidateEngine = os.Getenv("METASYSTEM_BIN")
	}
	os.Exit(testenv.Main(m))
}

func TestGoTestHarnessRejectsHostileInheritedEngine(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "called")
	hostileRoot := filepath.Join(root, "hostile-installation")
	hostileEngine := filepath.Join(hostileRoot, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(hostileEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	hostile := `#!/bin/sh
printf 'called\n' >"$HOSTILE_ENGINE_MARKER"
case "${1:-} ${2:-}" in
  "lease classify") printf '%s\n' '{"class":"MAIN"}'; exit 0 ;;
  "json get") printf '%s\n' MAIN; exit 0 ;;
esac
exit 73
`
	if err := testexec.WriteFile(hostileEngine, []byte(hostile), 0o755); err != nil {
		t.Fatal(err)
	}

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(testBinary,
		"-test.run=^(TestFreshInitializationUsesHumanGitCommitThenRealMigration|TestInheritedEngineAndInstallationSelectorsAreAbsent)$",
		"-test.count=1",
	)
	command.Env = testenv.WithoutInheritedControls(os.Environ())
	command.Env = append(command.Env,
		"GO_WANT_HOSTILE_ENGINE_INHERITANCE_HELPER=1",
		"HOSTILE_ENGINE_MARKER="+marker,
		"HOSTILE_REGISTRY_HOME="+hostileRoot,
		"METASYSTEM_ALLOW_NEW_PLAN=1",
		"METASYSTEM_BIN="+hostileEngine,
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_NONCE=hostile",
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT="+hostileRoot,
		"METASYSTEM_DELEGATE_ROOT="+hostileRoot,
		"METASYSTEM_GATE_WITNESS_ACTIVE=1",
		"METASYSTEM_GATE_WITNESS_ROOT="+hostileRoot,
		"METASYSTEM_GUARD_PROBE=hostile",
		"METASYSTEM_HARNESS_ROOT="+hostileRoot,
		"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT="+hostileRoot,
		"METASYSTEM_HOOK_DELEGATE_JOB=inherited-from-a-bed",
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT="+hostileRoot,
		"METASYSTEM_PROOF_ATTEMPT=hostile",
		"METASYSTEM_PROOF_AUTH_BIN="+hostileEngine,
		"METASYSTEM_PROOF_CONTROL_ROOT="+hostileRoot,
		"METASYSTEM_PROOF_RUN_ROOT="+hostileRoot,
		"METASYSTEM_SUITE_PROGRESS_ACTIVE=1",
		"METASYSTEM_SUITE_PROGRESS_ROOT="+hostileRoot,
		"METASYSTEM_SUPERVISION_REGISTRY_HOME="+hostileRoot,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("test subprocess inherited a hostile engine or installation selector: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile inherited engine was called: %v\n%s", err, output)
	}
}

func TestGoTestHarnessKeepsDeclaredContextCostEngine(t *testing.T) {
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	declared := filepath.Join(t.TempDir(), "declared-context-cost-engine")
	command := exec.Command(testBinary,
		"-test.run=^TestDeclaredContextCostEngineIsCaptured$",
		"-test.count=1",
	)
	command.Env = testenv.WithoutInheritedControls(os.Environ())
	command.Env = append(command.Env,
		"GO_WANT_DECLARED_CONTEXT_COST_ENGINE_HELPER=1",
		"EXPECTED_CONTEXT_COST_ENGINE="+declared,
		"METASYSTEM_CONTEXT_COST_PROOF=1",
		"METASYSTEM_BIN="+declared,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("declared context-cost engine was not captured: %v\n%s", err, output)
	}
}

func TestDeclaredContextCostEngineIsCaptured(t *testing.T) {
	if os.Getenv("GO_WANT_DECLARED_CONTEXT_COST_ENGINE_HELPER") != "1" {
		return
	}
	want := os.Getenv("EXPECTED_CONTEXT_COST_ENGINE")
	if declaredContextCostCandidateEngine != want {
		t.Fatalf("declared context-cost engine = %q, want %q", declaredContextCostCandidateEngine, want)
	}
	if inherited := os.Getenv("METASYSTEM_BIN"); inherited != "" {
		t.Fatalf("declared context-cost engine remained ambient as %q", inherited)
	}
}

func TestInheritedEngineAndInstallationSelectorsAreAbsent(t *testing.T) {
	if os.Getenv("GO_WANT_HOSTILE_ENGINE_INHERITANCE_HELPER") != "1" {
		return
	}
	for _, name := range []string{
		"METASYSTEM_ALLOW_NEW_PLAN",
		"METASYSTEM_BIN",
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_NONCE",
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT",
		"METASYSTEM_DELEGATE_ROOT",
		"METASYSTEM_GATE_WITNESS_ACTIVE",
		"METASYSTEM_GATE_WITNESS_ROOT",
		"METASYSTEM_GUARD_PROBE",
		"METASYSTEM_HARNESS_ROOT",
		"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT",
		"METASYSTEM_HOOK_DELEGATE_JOB",
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT",
		"METASYSTEM_PROOF_ATTEMPT",
		"METASYSTEM_PROOF_AUTH_BIN",
		"METASYSTEM_PROOF_CONTROL_ROOT",
		"METASYSTEM_PROOF_RUN_ROOT",
		"METASYSTEM_SUITE_PROGRESS_ACTIVE",
		"METASYSTEM_SUITE_PROGRESS_ROOT",
	} {
		if value := os.Getenv(name); value != "" {
			t.Errorf("%s survived package test setup as %q", name, value)
		}
	}
	if got, hostile := os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), os.Getenv("HOSTILE_REGISTRY_HOME"); got == "" || got == hostile || !filepath.IsAbs(got) {
		t.Errorf("METASYSTEM_SUPERVISION_REGISTRY_HOME = %q, want an isolated absolute directory other than %q", got, hostile)
	}
}
