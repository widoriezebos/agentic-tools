package proofrun

import (
	"os"
	"testing"
	"time"

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
	os.Exit(testenv.Main(m, declarations...))
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
