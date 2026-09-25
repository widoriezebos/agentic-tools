package launch

// The manifest is a copy of a contract, so the copy is checked.
//
// scripts/agents/second-session-fixtures.sh declares the adapters'
// local-config-paths as a literal and diffs the adapters against it. This verb
// runs no adapter, so it carries its own copy of that literal — and this reads
// the script's and compares, so a new adapter's local file cannot reach
// second-session.sh's manifest and quietly miss every machine this verb
// launches.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTheManifestIsTheAdaptersDeclaredContract(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "agents", "second-session-fixtures.sh"))
	if err != nil {
		t.Fatalf("the second-session fixtures could not be read: %v", err)
	}
	declared := declaredPaths(string(script))
	if len(declared) == 0 {
		t.Fatal("the second-session fixtures declare no local-config paths; the contract this copy tracks has moved")
	}
	if strings.Join(declared, "\n") != strings.Join(LocalConfigPaths, "\n") {
		t.Fatalf("the declared paths are\n%s\nand this package carries\n%s",
			strings.Join(declared, "\n"), strings.Join(LocalConfigPaths, "\n"))
	}
	if Manifest() != strings.Join(LocalConfigPaths, "\n")+"\n" {
		t.Fatalf("manifest = %q", Manifest())
	}
}

// declaredPaths reads the quoted paths out of the fixture's expected-paths
// block: the printf whose redirection names expected-paths.
func declaredPaths(script string) []string {
	block := regexp.MustCompile(`(?s)printf '%s\\n' \\\n(.*?)>"\$tmp/expected-paths"`).FindStringSubmatch(script)
	if block == nil {
		return nil
	}
	found := []string{}
	for _, quoted := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(block[1], -1) {
		found = append(found, quoted[1])
	}
	return found
}
