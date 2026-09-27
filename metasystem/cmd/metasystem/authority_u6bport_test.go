package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retiredLeaseFenceMarker is the implementation's retired bypass marker,
// spelled in two halves so this file never carries it whole.
const retiredLeaseFenceMarker = "METASYSTEM" + "_LEASE_FENCE"

// authority-regression WC-1 (U6b port): `job authority-check` refuses a
// positive DELEGATE classification even when the caller supplies the retired
// bypass marker, and the refusal names the caller class.
func TestAuthorityCheckRefusesADelegateEvenWithTheRetiredMarker(t *testing.T) {
	t.Setenv(retiredLeaseFenceMarker, "fixture")
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runAuthorityCheck([]string{
			"--mode", "record-writer",
			"--classification", `{"class":"DELEGATE","holder":false}`,
			"--job", "foreign-job",
		})
	})
	if code == 0 {
		t.Fatal("the retired marker bypassed a DELEGATE refusal")
	}
	if !strings.Contains(stderr, "DELEGATE") {
		t.Fatalf("the refusal did not name the caller class: %q", stderr)
	}
}

// authority-regression's source scan (U6b port): the retired marker stays out
// of the shell control-plane sources that survive the dispatcher's port.
func TestRetiredLeaseFenceMarkerStaysOutOfTheShellControlPlane(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"scripts/agents/commit.sh", "scripts/agents/evidence-gc.sh"} {
		content, err := os.ReadFile(filepath.Join("..", "..", source))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), retiredLeaseFenceMarker) {
			t.Fatalf("the retired marker resurfaced in %s", source)
		}
	}
}
