package proofrun

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	declarations := []testenv.Declaration{}
	if proofrunSubprocessHelper() || os.Getenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE") == "1" {
		declarations = testenv.DeclareInheritedControls()
	}
	if err := os.Unsetenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE"); err != nil {
		panic(err)
	}
	os.Exit(testenv.Main(m, declarations...))
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
