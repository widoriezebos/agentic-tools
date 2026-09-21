package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv(identity.FixtureCustodianEnv) == "1" || os.Getenv("GOAL_STEERING_HELPER_ROOT") != "" {
		os.Exit(testenv.Main(m))
	}
	root, err := os.MkdirTemp("", "metasystem-goal-test-admission.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create goal test admission root:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "write goal test admission authority:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}
	if err := os.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(root, "host-admission")); err != nil {
		fmt.Fprintln(os.Stderr, "set goal test admission directory:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}
	if err := os.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root); err != nil {
		fmt.Fprintln(os.Stderr, "set goal test admission authority:", err)
		_ = os.RemoveAll(root)
		os.Exit(2)
	}
	code := testenv.Main(m,
		testenv.Declare("METASYSTEM_PROOF_ADMISSION_TEST_DIR"),
		testenv.Declare("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT"))
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintln(os.Stderr, "remove goal test admission root:", err)
	}
	os.Exit(code)
}

func testEnvironment(base []string, entries ...string) []string {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, _, _ := strings.Cut(entry, "=")
		values[name] = entry
	}
	environment := make([]string, 0, len(base)+len(entries))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if replacement, ok := values[name]; ok {
			environment = append(environment, replacement)
			delete(values, name)
			continue
		}
		environment = append(environment, entry)
	}
	for _, entry := range entries {
		name, _, _ := strings.Cut(entry, "=")
		if replacement, ok := values[name]; ok {
			environment = append(environment, replacement)
			delete(values, name)
		}
	}
	return environment
}
