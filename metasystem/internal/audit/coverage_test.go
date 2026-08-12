package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleOutput = `ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/adapter	(cached)	coverage: 85.9% of statements
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch	1.2s	coverage: 66.8% of statements
?   	github.com/widoriezebos/agentic-tools/metasystem/internal/empty	[no test files]
`

const modulePrefix = "github.com/widoriezebos/agentic-tools/metasystem/"

func testBaseline(t *testing.T, floors map[string]float64) *CoverageBaseline {
	t.Helper()
	return &CoverageBaseline{Floors: floors, Exempt: map[string]string{"cmd/metasystem": "fixture-covered"}}
}

func TestParseCoverage(t *testing.T) {
	measured := ParseCoverage(sampleOutput, modulePrefix)
	if len(measured) != 2 || measured["internal/adapter"] != 85.9 || measured["internal/dispatch"] != 66.8 {
		t.Fatalf("parse wrong: %v", measured)
	}
}

// The ratchet can fail — a deliberately lowered number is caught (the
// prove-the-check-can-fail requirement of production-grade Phase 0c).
func TestCheckCoverageFailsOnDrop(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9, "internal/dispatch": 66.8})
	measured := map[string]float64{"internal/adapter": 85.9, "internal/dispatch": 60.0}
	violations := CheckCoverage(baseline, measured)
	if len(violations) != 1 || !strings.Contains(violations[0], "internal/dispatch coverage 60.0% is below") {
		t.Fatalf("drop not caught: %v", violations)
	}
}

func TestCheckCoveragePassesAtFloor(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9})
	if violations := CheckCoverage(baseline, map[string]float64{"internal/adapter": 85.9}); len(violations) != 0 {
		t.Fatalf("floor equality must pass: %v", violations)
	}
}

// A new package without a floor fails until registered; exempt packages do
// not; a floored package that vanishes from the measurement fails — losing
// sight never passes.
func TestCheckCoverageClosedWorld(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9})
	violations := CheckCoverage(baseline, map[string]float64{
		"internal/adapter": 86.0, "internal/newpkg": 99.0, "cmd/metasystem": 3.5,
	})
	if len(violations) != 1 || !strings.Contains(violations[0], "internal/newpkg") {
		t.Fatalf("unregistered package not caught: %v", violations)
	}
	violations = CheckCoverage(baseline, map[string]float64{})
	if len(violations) != 1 || !strings.Contains(violations[0], "was not measured") {
		t.Fatalf("vanished package not caught: %v", violations)
	}
}

func TestReadCoverageBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ratchet.json")
	if err := os.WriteFile(path, []byte(`{"floors":{"internal/x":50.0},"exempt":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := ReadCoverageBaseline(path)
	if err != nil || baseline.Floors["internal/x"] != 50.0 {
		t.Fatalf("read failed: %v %v", err, baseline)
	}
	for name, bad := range map[string]string{
		"empty":      `{"floors":{}}`,
		"range":      `{"floors":{"internal/x":140}}`,
		"unparsable": `nope`,
	} {
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCoverageBaseline(path); err == nil {
			t.Fatalf("%s baseline accepted", name)
		}
	}
}
