package adapter

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- typed-usage assertion ---

func usageRecord(t *testing.T, usage string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "job.json")
	writeFile(t, path, fmt.Sprintf(`{"usage": %s}`, usage))
	return path
}

func TestSelftestUsageCheck(t *testing.T) {
	cases := []struct {
		name     string
		usage    string
		expected string
		wantErr  string
	}{
		{"native tokens pass", `{"availability": "native", "inputTokens": 10, "outputTokens": 3}`, "native", ""},
		{"native without tokens", `{"availability": "native"}`, "native", "numeric inputTokens"},
		{"native availability mismatch", `{"availability": "unavailable"}`, "native", `availability is "unavailable"`},
		{"unavailable pass", `{"availability": "unavailable"}`, "unavailable", ""},
		{"metered by provider units", `{"availability": "unavailable", "providerUnits": {"name": "acu", "value": 4.5}}`, "metered", ""},
		{"metered by native tokens", `{"availability": "native", "inputTokens": 10, "outputTokens": 3}`, "metered", ""},
		{"metered tokens must be native", `{"availability": "unavailable", "inputTokens": 10, "outputTokens": 3}`, "metered", "native availability"},
		{"metered by nothing", `{"availability": "unavailable"}`, "metered", "neither token counts nor named provider units"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := SelftestUsageCheck(usageRecord(t, testCase.usage), testCase.expected)
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("err = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestSelftestUsageCheckRequiresTypedUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.json")
	writeFile(t, path, `{"usage": null}`)
	if err := SelftestUsageCheck(path, "native"); err == nil || !strings.Contains(err.Error(), "no typed usage") {
		t.Fatalf("err = %v", err)
	}
}

// --- envelope declaration read ---

func TestSelftestEnvelopeDeclaration(t *testing.T) {
	dir := t.TempDir()
	// No snapshot at all: mapped, the stricter reading.
	if got := SelftestEnvelopeDeclaration(dir, "rt", "network"); got != "mapped" {
		t.Fatalf("empty dir: %q", got)
	}
	writeFile(t, filepath.Join(dir, "rt-1.0-aaa-20260801-001.json"),
		`{"envelopeEnforcement": {"writeRoots": "mapped", "readRoots": "mapped", "network": "mapped"}}`)
	writeFile(t, filepath.Join(dir, "rt-1.0-aaa-20260802-001.json"),
		`{"envelopeEnforcement": {"writeRoots": "mapped", "readRoots": "mapped", "network": "notEnforced"}}`)
	// Another runtime's snapshot must not shadow this runtime's newest.
	writeFile(t, filepath.Join(dir, "zz-9.9-zzz-20260809-001.json"),
		`{"envelopeEnforcement": {"network": "mapped"}}`)
	if got := SelftestEnvelopeDeclaration(dir, "rt", "network"); got != "notEnforced" {
		t.Fatalf("newest snapshot: %q", got)
	}
	// An unparseable newer file falls back to the newest parseable snapshot.
	writeFile(t, filepath.Join(dir, "rt-1.0-aaa-20260803-001.json"), "not json")
	if got := SelftestEnvelopeDeclaration(dir, "rt", "network"); got != "notEnforced" {
		t.Fatalf("unparseable newest: %q", got)
	}
	// A field the snapshot never declared reads as mapped.
	if got := SelftestEnvelopeDeclaration(dir, "rt", "somethingElse"); got != "mapped" {
		t.Fatalf("undeclared field: %q", got)
	}
}

// --- pass record ---

func TestWriteSelftestRecordStatesOnlyWhatWasProven(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()

	path := filepath.Join(t.TempDir(), "pass.json")
	if err := WriteSelftestRecord(path, "codex", "job-1", "metered", nil, "mapped", "notEnforced"); err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, path)
	if record["passedAt"] != "2026-08-10T12:00:00Z" || record["runtime"] != "codex" || record["job"] != "job-1" {
		t.Fatalf("record = %v", record)
	}
	proven := fmt.Sprintf("%v", record["provenBehaviorally"])
	for _, tag := range []string{"dispatch", "forbidden-write-denied", "usage-metered"} {
		if !strings.Contains(proven, tag) {
			t.Fatalf("proven misses %q: %s", tag, proven)
		}
	}
	// The unenforced network field must never read as behaviourally denied.
	if strings.Contains(proven, "denied-network") {
		t.Fatalf("overclaimed network denial: %s", proven)
	}
	envelope := record["permissionEnvelopeEvidence"].(map[string]any)
	behaviorally := envelope["behaviorallyProven"].(map[string]any)
	if behaviorally["writeRoots"] != "forbidden-write-denied" {
		t.Fatalf("writeRoots evidence = %v", behaviorally["writeRoots"])
	}
	if !strings.Contains(behaviorally["network"].(string), "not-enforced") {
		t.Fatalf("network evidence = %v", behaviorally["network"])
	}
	if !strings.Contains(record["usageNote"].(string), "provider units") {
		t.Fatalf("usageNote = %v", record["usageNote"])
	}
}

func TestWriteSelftestRecordNativeUsageAndDevinChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pass.json")
	if err := WriteSelftestRecord(path, "devin", "job-2", "native", []string{"documented-exit-status-observation", "symlinked-skill-discovery"}, "notEnforced", "mapped"); err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, path)
	proven := fmt.Sprintf("%v", record["provenBehaviorally"])
	for _, tag := range []string{"usage-extraction", "denied-network", "symlinked-skill-discovery"} {
		if !strings.Contains(proven, tag) {
			t.Fatalf("proven misses %q: %s", tag, proven)
		}
	}
	if strings.Contains(proven, "forbidden-write-denied") {
		t.Fatalf("overclaimed write denial: %s", proven)
	}
	if _, present := record["usageNote"]; present {
		t.Fatal("native usage needs no note")
	}
}

// --- one-shot listener ---

func TestSelftestListenerAnswersExactlyOneRequest(t *testing.T) {
	dir := t.TempDir()
	portFile := filepath.Join(dir, "port")
	requestLog := filepath.Join(dir, "requested")
	done := make(chan error, 1)
	go func() { done <- SelftestListener(portFile, requestLog, 5*time.Second) }()

	var port string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(portFile); err == nil && len(data) > 0 {
			port = string(data)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if port == "" {
		t.Fatal("port file never appeared")
	}

	connection, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("GET /nonce-123 HTTP/1.1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 256)
	n, err := connection.Read(reply)
	if err != nil || !strings.Contains(string(reply[:n]), "200 OK") {
		t.Fatalf("reply = %q, %v", reply[:n], err)
	}
	if err := <-done; err != nil {
		t.Fatalf("listener: %v", err)
	}
	logged, err := os.ReadFile(requestLog)
	if err != nil || !strings.Contains(string(logged), "GET /nonce-123") {
		t.Fatalf("request log = %q, %v", logged, err)
	}
}

func TestSelftestListenerTimesOutQuietlyWithoutARequest(t *testing.T) {
	dir := t.TempDir()
	requestLog := filepath.Join(dir, "requested")
	if err := SelftestListener(filepath.Join(dir, "port"), requestLog, 50*time.Millisecond); err != nil {
		t.Fatalf("timeout should be quiet success: %v", err)
	}
	// No request means no log: the log's existence is the tripwire.
	if _, err := os.Stat(requestLog); !os.IsNotExist(err) {
		t.Fatal("request log must not exist without a request")
	}
}

// adapter-host-registry-3: newest means capturedAt, never filename order —
// after a CLI upgrade the old version's snapshots sort lexically LAST.
func TestSelftestEnvelopeDeclarationPicksByCapturedAt(t *testing.T) {
	dir := t.TempDir()
	// Lexically LAST but chronologically OLD (version 1.9 sorts after 1.10).
	writeFile(t, filepath.Join(dir, "codex-1.9-aaaa-20260101-001.json"),
		`{"runtime":"codex","capturedAt":"2026-01-01T00:00:00Z","envelopeEnforcement":{"network":"notEnforced"}}`)
	// Lexically FIRST but the actual newest.
	writeFile(t, filepath.Join(dir, "codex-1.10-zzzz-20260813-001.json"),
		`{"runtime":"codex","capturedAt":"2026-08-13T00:00:00Z","envelopeEnforcement":{"network":"mapped"}}`)
	// A different runtime extending the prefix must not shadow.
	writeFile(t, filepath.Join(dir, "codex-next-9.9-ffff-20270101-001.json"),
		`{"runtime":"codex-next","capturedAt":"2027-01-01T00:00:00Z","envelopeEnforcement":{"network":"notEnforced"}}`)

	if got := SelftestEnvelopeDeclaration(dir, "codex", "network"); got != "mapped" {
		t.Fatalf("declaration = %q; the stale or foreign snapshot won", got)
	}
}
