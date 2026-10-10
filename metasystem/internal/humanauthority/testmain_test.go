package humanauthority

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv("METASYSTEM_TEST_MACHINE") == "" {
		EnrollmentLocation = func(root string) (string, string, error) { return enrollmentPath(root), root, nil }
	}
	os.Exit(testenv.Main(m))
}
