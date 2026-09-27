package census

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The host-ancestry signature set is the registry's adoptable population,
// not the checkout's execution roster; no adapter script is consulted.
func TestAllHostSignaturesIgnoreExecutionRoster(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configured, err := signaturesFor(root, "")
	if err != nil || len(configured) != 1 || configured[0].Runtime != "claude" {
		t.Fatalf("configured signatures = %+v, %v; want claude only", configured, err)
	}
	all, err := signaturesFor(root, "", true)
	if err != nil || len(all) != len(runtimes.Adoptable()) || len(all) < 3 {
		t.Fatalf("all-host signatures = %d, %v; want the adoptable population %v", len(all), err, runtimes.Adoptable())
	}
	if got := Runtime("devin --project x", all); got != "devin" {
		t.Fatalf("Devin outside roster was not discoverable: %q", got)
	}
	if got := Runtime("devin --project x", configured); got != "" {
		t.Fatalf("the configured roster must not reach Devin: %q", got)
	}
}
