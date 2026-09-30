package dispatch

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle, the dispatch_fails/dispatch_succeeds gate cases: dispatch
// refuses absent, stale, generation-stale, failed and fingerprint-mismatched
// census verdicts, with one capped freshness window and one refusal shape.
// The clock is injected; no census runs.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const supCGateRepo = "/fixture/gate-repo"
const supCGateArm = "metasystem system start"

var supCGateNow = time.Unix(1786000000, 0)

type supCGate struct {
	dir     string
	state   string
	verdict string
}

func supCNewGate(t *testing.T, generation int) supCGate {
	t.Helper()
	dir := t.TempDir()
	gate := supCGate{dir: dir, state: filepath.Join(dir, "state.json"), verdict: filepath.Join(dir, "last-census.json")}
	gate.writeState(t, generation)
	return gate
}

func (gate supCGate) writeState(t *testing.T, generation int) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"generation": generation,
		"owner": map[string]any{"pid": 71001, "pidStartedAt": 1, "instanceTag": "owner-t"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gate.state, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// set is the bed's set_gate_census: a verdict aged age seconds at the gate
// clock, with the given interval, verdict label, fingerprint and generation,
// attesting the current state bytes.
func (gate supCGate) set(t *testing.T, age, interval int64, label, fingerprint string, generation int) {
	t.Helper()
	state, err := os.ReadFile(gate.state)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(state)
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "writer": "watch-background-jobs.sh", "verdict": label,
		"completedAtEpoch": supCGateNow.Unix() - age, "intervalSec": interval, "fingerprint": fingerprint,
		"counts": map[string]any{"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0}, "inventory": []any{},
		"diagnostics": []any{}, "errors": []any{}, "generation": generation, "stateDigest": hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gate.verdict, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (gate supCGate) check(fingerprint string) error {
	return CensusFresh(gate.verdict, gate.state, supCGateArm, supCGateRepo, fingerprint, supCGateNow)
}

var supCAgePattern = regexp.MustCompile(` [0-9]+s (old|ago) `)

// supCAssertStaleShape is the bed's assert_stale_shape: every stale refusal
// names the age, the window, the retry remedy and the re-arm remedy.
func supCAssertStaleShape(t *testing.T, err error, window string) {
	t.Helper()
	if err == nil {
		t.Fatal("stale census was accepted")
	}
	message := err.Error()
	if !strings.Contains(message, "nothing was dispatched") || !supCAgePattern.MatchString(message) ||
		!strings.Contains(message, "(limit "+window+"s)") || !strings.Contains(message, "try again in a moment") ||
		!strings.Contains(message, "\nif the machinery is stopped, run: metasystem system start --repo "+supCGateRepo) {
		t.Fatalf("stale census refusal did not carry the common diagnostic shape: %q", message)
	}
}

// Freshness is one capped window: inside proceeds; the exact boundary and one
// second past refuse with age, window and both remedies; a configured
// interval above the cap still refuses at 180 seconds.
func TestSupCCensusGateFreshnessWindow(t *testing.T) {
	t.Parallel()
	gate := supCNewGate(t, 4)
	gate.set(t, 0, 10, "SUCCESS", "fp", 4)
	if err := gate.check("fp"); err != nil {
		t.Fatalf("inside-census-window refused: %v", err)
	}
	gate.set(t, 20, 10, "SUCCESS", "fp", 4)
	supCAssertStaleShape(t, gate.check("fp"), "20")
	gate.set(t, 21, 10, "SUCCESS", "fp", 4)
	supCAssertStaleShape(t, gate.check("fp"), "20")
	gate.set(t, 180, 200, "SUCCESS", "fp", 4)
	supCAssertStaleShape(t, gate.check("fp"), "180")
	gate.set(t, 179, 200, "SUCCESS", "fp", 4)
	if err := gate.check("fp"); err != nil {
		t.Fatalf("a verdict inside the capped window refused: %v", err)
	}
}

// A census from the prior arming generation refuses with the same shape and
// names both generations; a CENSUS-FAILED verdict and a fingerprint mismatch
// refuse by name.
func TestSupCCensusGateGenerationFailedAndFingerprint(t *testing.T) {
	t.Parallel()
	gate := supCNewGate(t, 4)
	gate.set(t, 0, 10, "SUCCESS", "fp", 4)
	gate.writeState(t, 5)
	err := gate.check("fp")
	supCAssertStaleShape(t, err, "20")
	if !strings.Contains(armingDetail(err), "censusGeneration=4") || !strings.Contains(armingDetail(err), "armedGeneration=5") {
		t.Fatalf("generation-stale refusal did not name both generations: %v", err)
	}
	var armingWindow ArmingWindowError
	if !errors.As(err, &armingWindow) {
		t.Fatalf("generation-stale refusal is not the typed arming-window transient: %T", err)
	}

	gate.writeState(t, 4)
	gate.set(t, 0, 10, "CENSUS-FAILED", "fp", 4)
	if err := gate.check("fp"); err == nil || !strings.Contains(err.Error(), "CENSUS-FAILED") {
		t.Fatalf("failed census = %v", err)
	}
	gate.set(t, 0, 10, "SUCCESS", "wrong", 4)
	if err := gate.check("fp"); err == nil || !strings.Contains(err.Error(), "fingerprint does not match") {
		t.Fatalf("fingerprint census = %v", err)
	}
}

// S4-1 and S4-10: the job record's ownership identity carries the census join
// key pidStartedAt beside pid, the host-turn contract documents it, and every
// cross-component owner asset the census and the hooks depend on ships.
func TestSupCJobRecordCarriesTheCensusJoinKey(t *testing.T) {
	t.Parallel()
	fields := exactIdentityFields(identity.Ref{Pid: 4242, StartedAtSec: 1786000000})
	if fields["pid"] != int64(4242) || fields["pidStartedAt"] != int64(1786000000) {
		t.Fatalf("S4-1: job ownership fields = %v", fields)
	}
	if !ownershipPatchFields["pidStartedAt"] {
		t.Fatal("S4-1: the ownership patch does not carry pidStartedAt")
	}
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "orchestration.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(docs), "pidStartedAt") {
		t.Fatal("S4-1/S4-10: the host-turn contract does not document pidStartedAt")
	}
	for _, asset := range []string{
		"internal/delegation/lifecycle.go", "internal/hooks/runtime_hook.go",
		"internal/runtimes/enforcement/claude-code-hooks.json", "internal/runtimes/enforcement/codex-hooks.json",
		"internal/runtimes/enforcement/devin-hooks.json",
	} {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(asset))); err != nil {
			t.Fatalf("S4-10: cross-component owner asset is missing: %s (%v)", asset, err)
		}
	}
}

// armingDetail is an arming-window refusal's counters (what --verbose adds).
func armingDetail(err error) string {
	var window ArmingWindowError
	if errors.As(err, &window) {
		return window.Detail()
	}
	return ""
}
