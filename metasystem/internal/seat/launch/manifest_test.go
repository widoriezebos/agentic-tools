package launch

// The manifest is the runtimes' declared contract: exactly these files, no
// more, no fewer (formerly the config-manifest scenario of
// scripts/agents/second-session-fixtures.sh, which diffed the adapter
// scripts' local-config-paths against this literal).

import (
	"strings"
	"testing"
)

func TestTheManifestIsTheAdaptersDeclaredContract(t *testing.T) {
	t.Parallel()
	declared := []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".codex/config.toml",
		".devin/config.json",
		".devin/config.local.json",
		".devin/hooks.v1.json",
	}
	if strings.Join(declared, "\n") != strings.Join(LocalConfigPaths, "\n") {
		t.Fatalf("the declared paths are\n%s\nand the registry yields\n%s",
			strings.Join(declared, "\n"), strings.Join(LocalConfigPaths, "\n"))
	}
	if Manifest() != strings.Join(declared, "\n")+"\n" {
		t.Fatalf("manifest = %q", Manifest())
	}
}
