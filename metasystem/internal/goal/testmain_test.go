package goal

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
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
