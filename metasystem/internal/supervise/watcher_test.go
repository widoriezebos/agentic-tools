package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// stageCheckout builds a minimal metasystem checkout the watcher can census: a
// fake-runtime config, the signature adapter, the fingerprint inputs, and a
// present supervision state.json. It returns the checkout root.
func stageCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string, mode os.FileMode) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	// The runtime and its signature adapter drive classification; the marker
	// matches no real process, so the live scan classifies nothing in scope.
	write("metasystem.conf", "metasystem.runtimes=fake\n", 0o644)
	write("scripts/agents/adapters/fake.sh",
		"#!/usr/bin/env bash\ncase \"$1\" in signature) echo 'match metasystem-fake-runtime-marker' ;; esac\n", 0o755)
	// The remaining fingerprint inputs need only exist and be readable.
	for _, rel := range []string{
		"scripts/agents/arm-supervision.sh",
		"scripts/agents/dispatch.sh",
		"bin/metasystem",
		"scripts/agents/adapters/runtime-common.sh",
		"scripts/watch-background-jobs.sh",
	} {
		write(rel, "# fingerprint input placeholder\n", 0o644)
	}
	// A present, complete supervision state.json.
	state := map[string]any{
		"schemaVersion": 1,
		"generation":    1,
		"owner":         map[string]any{"pid": 1, "pidStartedAt": 1, "instanceTag": "owner-tag"},
		"components": map[string]any{
			"watcher": map[string]any{"pid": 1, "pidStartedAt": 1, "instanceTag": "watcher-tag"},
			"reaper":  map[string]any{"pid": 1, "pidStartedAt": 1, "instanceTag": "reaper-tag"},
		},
		"intervalSec": 5,
	}
	encoded, _ := json.MarshalIndent(state, "", "  ")
	write(filepath.Join("artifacts", "agents", "supervision", "state.json"), string(encoded)+"\n", 0o644)
	return root
}

func productionWatcher(t *testing.T, root string) WatcherConfig {
	t.Helper()
	return WatcherConfig{
		SupervisionDir: SupervisionDir(root),
		Interval:       5,
		IntervalMS:     5000,
		BudgetPercent:  50,
		Fingerprint:    func() (string, error) { return census.Fingerprint(root, root) },
		Census: func(fingerprint string, now time.Time) (census.Verdict, error) {
			return census.RunProductionCensus(root, root, fingerprint, 5, now)
		},
		Now: func() time.Time { return time.Now().UTC() },
	}
}

// The watcher pass, run against a real checkout, publishes a schema-2 census
// verdict AND the component beats — the two liveness signals the owner reads.
func TestWatcherPassPublishesVerdictAndHeartbeat(t *testing.T) {
	root := stageCheckout(t)
	cfg := productionWatcher(t, root)

	if err := cfg.WatcherPass(); err != nil {
		t.Fatalf("watcher pass: %v", err)
	}
	self := identity.Ref{Pid: int64(os.Getpid()), StartedAtSec: 42}
	heartbeatPath := filepath.Join(SupervisionDir(root), "watcher.heartbeat.json")
	if err := WriteHeartbeat(heartbeatPath, "watcher", self, "watcher-tag", 5, 230); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	// The verdict.
	verdictPath := filepath.Join(SupervisionDir(root), censusVerdictFile)
	data, err := os.ReadFile(verdictPath)
	if err != nil {
		t.Fatalf("no census verdict written: %v", err)
	}
	var verdict census.Verdict
	if err := json.Unmarshal(data, &verdict); err != nil {
		t.Fatalf("verdict is not valid json: %v", err)
	}
	if verdict.SchemaVersion != 2 || verdict.Writer != "watch-background-jobs.sh" {
		t.Fatalf("bad verdict envelope: %+v", verdict)
	}
	if !hex64.MatchString(verdict.Fingerprint) {
		t.Fatalf("verdict fingerprint is not a real digest, so the scan path did not run: %q", verdict.Fingerprint)
	}
	if verdict.Counts == nil || verdict.Inventory == nil {
		t.Fatal("verdict collections must be non-nil")
	}

	// The heartbeat.
	beatData, err := os.ReadFile(heartbeatPath)
	if err != nil {
		t.Fatalf("no heartbeat written: %v", err)
	}
	var beat map[string]any
	if err := json.Unmarshal(beatData, &beat); err != nil {
		t.Fatalf("heartbeat is not valid json: %v", err)
	}
	if int64(beat["pid"].(float64)) != self.Pid {
		t.Fatalf("heartbeat names the wrong pid: %v", beat["pid"])
	}
	if beat["observedAtEpoch"] == nil {
		t.Fatal("heartbeat carries no observedAtEpoch")
	}
}

// A fingerprint that cannot be computed still publishes a well-formed verdict,
// labelled CENSUS-FAILED with a FINGERPRINT-FAILED digest, so the failure is
// visible rather than a stale success.
func TestWatcherPassCensusFailedOnFingerprintError(t *testing.T) {
	dir := t.TempDir()
	cfg := WatcherConfig{
		SupervisionDir: dir,
		Interval:       5,
		IntervalMS:     5000,
		BudgetPercent:  50,
		Fingerprint:    func() (string, error) { return "", fmt.Errorf("inputs unreadable") },
		Census: func(string, time.Time) (census.Verdict, error) {
			t.Fatal("census must not run when the fingerprint fails")
			return census.Verdict{}, nil
		},
		Now: func() time.Time { return time.Unix(1786000000, 0).UTC() },
	}
	if err := cfg.WatcherPass(); err != nil {
		t.Fatalf("watcher pass: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, censusVerdictFile))
	if err != nil {
		t.Fatalf("no verdict written: %v", err)
	}
	var verdict census.Verdict
	if err := json.Unmarshal(data, &verdict); err != nil {
		t.Fatal(err)
	}
	if verdict.Verdict != "CENSUS-FAILED" || verdict.Fingerprint != "FINGERPRINT-FAILED" {
		t.Fatalf("expected a CENSUS-FAILED/FINGERPRINT-FAILED verdict, got %+v", verdict)
	}
	if len(verdict.Errors) == 0 {
		t.Fatal("a failed fingerprint must record the reason")
	}
}

// A scan slower than the whole interval is warned as the serious defect.
func TestWatcherPassWarnsWhenScanExceedsInterval(t *testing.T) {
	dir := t.TempDir()
	var warnings []string
	cfg := WatcherConfig{
		SupervisionDir: dir,
		Interval:       5,
		IntervalMS:     1, // any real scan duration exceeds a 1ms interval
		BudgetPercent:  50,
		Fingerprint:    func() (string, error) { return "deadbeef", nil },
		Census: func(fingerprint string, now time.Time) (census.Verdict, error) {
			time.Sleep(5 * time.Millisecond)
			return censusFailedVerdict(fingerprint, 5, "stub", now), nil
		},
		Now:  func() time.Time { return time.Unix(1786000000, 0).UTC() },
		Warn: func(message string) { warnings = append(warnings, message) },
	}
	if err := cfg.WatcherPass(); err != nil {
		t.Fatalf("watcher pass: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one CENSUS-SLOW warning, got %v", warnings)
	}
	if want := "defect=scan-exceeds-interval"; !strings.Contains(warnings[0], want) {
		t.Fatalf("warning %q lacks %q", warnings[0], want)
	}
}

// scanSeq is the census actor's monotonic "attempt" marker for attempt-based
// fixture patience (records/patience/patience-attempts.md). It must (a) advance by one per
// successful publish, (b) stay monotonic across FRESH WatcherConfigs — the
// fixtures drive census as repeated one-shot watcher-pass processes, so the
// counter cannot live in the process; it is seeded from the published file —
// and (c) be present on all three verdict paths (success, census-error,
// fingerprint-failure), whose stamping is centralised in publish().
func TestWatcherPassScanSeqMonotonicAcrossFreshConfigs(t *testing.T) {
	root := stageCheckout(t)
	verdictPath := filepath.Join(SupervisionDir(root), censusVerdictFile)
	readSeq := func() int64 {
		data, err := os.ReadFile(verdictPath)
		if err != nil {
			t.Fatalf("no verdict: %v", err)
		}
		var v census.Verdict
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("verdict not json: %v", err)
		}
		return v.ScanSeq
	}
	// Each pass rebuilds the config, standing in for a fresh one-shot process.
	for want := int64(1); want <= 3; want++ {
		if err := productionWatcher(t, root).WatcherPass(); err != nil {
			t.Fatalf("pass %d: %v", want, err)
		}
		if got := readSeq(); got != want {
			t.Fatalf("scanSeq did not advance monotonically across fresh configs: pass %d got %d want %d", want, got, want)
		}
	}
}

func TestWatcherPassScanSeqOnAllVerdictPaths(t *testing.T) {
	root := stageCheckout(t)
	verdictPath := filepath.Join(SupervisionDir(root), censusVerdictFile)
	readVerdict := func() census.Verdict {
		data, err := os.ReadFile(verdictPath)
		if err != nil {
			t.Fatalf("no verdict: %v", err)
		}
		var v census.Verdict
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("verdict not json: %v", err)
		}
		return v
	}
	base := productionWatcher(t, root)

	// Path 1: the normal path — Census returns without error (the verdict LABEL
	// depends on what the staged checkout scans; the code path is what matters).
	if err := base.WatcherPass(); err != nil {
		t.Fatalf("normal pass: %v", err)
	}
	if v := readVerdict(); v.ScanSeq != 1 {
		t.Fatalf("normal path: scanSeq=%d (want 1)", v.ScanSeq)
	}

	// Path 2: census scan error -> CENSUS-FAILED, seq continues from the file.
	censusErr := base
	censusErr.Census = func(string, time.Time) (census.Verdict, error) {
		return census.Verdict{}, fmt.Errorf("scan blew up")
	}
	if err := censusErr.WatcherPass(); err != nil {
		t.Fatalf("census-error pass: %v", err)
	}
	if v := readVerdict(); v.Verdict != "CENSUS-FAILED" || v.ScanSeq != 2 {
		t.Fatalf("census-error path: verdict=%s scanSeq=%d (want CENSUS-FAILED, 2)", v.Verdict, v.ScanSeq)
	}

	// Path 3: fingerprint error -> FINGERPRINT-FAILED (the early-return path).
	fpErr := base
	fpErr.Fingerprint = func() (string, error) { return "", fmt.Errorf("fingerprint blew up") }
	if err := fpErr.WatcherPass(); err != nil {
		t.Fatalf("fingerprint-error pass: %v", err)
	}
	// The fingerprint-failure early-return path stamps "FINGERPRINT-FAILED" into
	// the Fingerprint field (the label stays CENSUS-FAILED); scanSeq==3 proves
	// publish() stamped it even on this path that returns before the common block.
	if v := readVerdict(); v.Fingerprint != "FINGERPRINT-FAILED" || v.ScanSeq != 3 {
		t.Fatalf("fingerprint-error path (early return): fingerprint=%s scanSeq=%d (want FINGERPRINT-FAILED, 3)", v.Fingerprint, v.ScanSeq)
	}
}

func TestLastPublishedScanSeqFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last-census.json")
	if got := lastPublishedScanSeq(path); got != 0 {
		t.Fatalf("absent file: got %d want 0", got)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastPublishedScanSeq(path); got != 0 {
		t.Fatalf("garbage file: got %d want 0", got)
	}
	if err := os.WriteFile(path, []byte(`{"scanSeq":41}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastPublishedScanSeq(path); got != 41 {
		t.Fatalf("valid file: got %d want 41", got)
	}
	// A negative (corrupt) marker is garbage, not a count: restart from 0.
	if err := os.WriteFile(path, []byte(`{"scanSeq":-100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastPublishedScanSeq(path); got != 0 {
		t.Fatalf("negative file: got %d want 0", got)
	}
}
