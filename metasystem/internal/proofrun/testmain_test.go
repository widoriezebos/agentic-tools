package proofrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	installDeterministicTestLoadReaders()
	if err := os.Unsetenv(TestHostLoadEnvironment); err != nil {
		panic(err)
	}
	declarations := []testenv.Declaration{}
	if proofrunSubprocessHelper() || os.Getenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE") == "1" {
		declarations = testenv.DeclareInheritedControls()
	}
	if err := os.Unsetenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE"); err != nil {
		panic(err)
	}
	// This exact real-process custody helper is owned by its parent test,
	// which holds exact refs and a bounded cleanup. Giving the helper a second
	// testenv fixture custodian would kill the product custodian when the
	// launcher is deliberately killed, obscuring the behavior under test.
	if hostResourceCustodyHelperInvocation() || hostResourceNestedCustodyHelperInvocation() {
		os.Exit(m.Run())
	}
	// Independent test binaries have independent proof roots. Give their host
	// admission guard the same isolation; tests exercising real contention
	// explicitly replace this directory with their shared fixture directory.
	admissionRoot := ""
	if len(declarations) == 0 && os.Getenv(identity.FixtureCustodianEnv) != "1" {
		var err error
		admissionRoot, err = os.MkdirTemp("", "metasystem-proofrun-admission.")
		if err != nil {
			panic(err)
		}
		hostAdmissionDirectoryForTest = filepath.Join(admissionRoot, "host-admission")
	}
	code := testenv.Main(m, declarations...)
	if admissionRoot != "" {
		if err := os.RemoveAll(admissionRoot); err != nil {
			fmt.Fprintln(os.Stderr, "remove test host admission directory:", err)
		}
	}
	os.Exit(code)
}

func hostResourceCustodyHelperInvocation() bool {
	args := os.Args
	if os.Getenv("METASYSTEM_HOST_CUSTODY_HELPER") != "1" || len(args) != 10 ||
		args[1] != "-test.run=^TestGLEHostResourceCustodyProcessHelper$" || args[2] != "--" || args[3] != "launcher" {
		return false
	}
	root := args[4]
	relative, err := filepath.Rel(os.TempDir(), root)
	return err == nil && filepath.IsAbs(root) && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && fixtureauth.FixtureModeRoot(root)
}

func hostResourceNestedCustodyHelperInvocation() bool {
	args := os.Args
	if os.Getenv("METASYSTEM_NESTED_CUSTODY_HELPER") != "1" || len(args) != 7 ||
		args[1] != "-test.run=^TestHostResourceNestedCustodySubprocess$" || args[2] != "--" ||
		(args[3] != "launcher" && args[3] != "worker") {
		return false
	}
	root := args[4]
	relative, err := filepath.Rel(os.TempDir(), root)
	return err == nil && filepath.IsAbs(root) && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && fixtureauth.FixtureModeRoot(root)
}

type deadTestProber struct{}

func (deadTestProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Dead, nil
}

func installDeterministicTestLoadReaders() {
	loadSeams = loadReaders{
		host: func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.UTC().Format(time.RFC3339Nano), Available: true, Cores: 18}
		},
		launchers: func(int64) (int, bool) { return 0, true },
		nested:    func(int64) (bool, bool) { return false, true },
		prober:    deadTestProber{},
		pids:      func() ([]int64, error) { return nil, nil },
		parent:    func(int64) (int64, bool) { return 0, false },
	}
}

// useRealLoadReaders is the only opt-in from package tests to the machine's
// load and process census. Tests that do not call it stay host-independent.
func useRealLoadReaders(t *testing.T) {
	t.Helper()
	previous := loadSeams
	loadSeams = realLoadReaders()
	t.Cleanup(func() { loadSeams = previous })
}

func proofrunSubprocessHelper() bool {
	for _, name := range []string{
		"GO_WANT_COVERAGE_SCRIPT_HELPER",
		"GO_WANT_LEGACY_PROOF_WORKER",
		"GO_WANT_PROOF_ENTRYPOINT_HELPER",
		"GO_WANT_SUPERVISOR_HELPER",
		"GO_WANT_VERSION_IDENTITY_HELPER",
		"GO_WANT_WITNESS_GATE_HELPER",
	} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}
