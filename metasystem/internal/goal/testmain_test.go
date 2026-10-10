package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	humanauthority.EnrollmentLocation = func(root string) (string, string, error) {
		return filepath.Join(root, "artifacts", "agents", "authority", "human-terminal.json"), root, nil
	}
	os.Exit(testenv.Main(m))
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
