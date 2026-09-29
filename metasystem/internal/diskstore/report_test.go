package diskstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The report golden (R10): every section a person reads, in order, with
// the reason and the command of every kept and pending item.
func TestReportGolden(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "machine", Name: "machine", At: testNow, Mode: ModeApply,
		Volumes: []Volume{{Path: "/Users/wido", FreeBytes: 277 << 20, FloorBytes: 50 << 30, BelowFloor: true}},
		Floor: &FloorReport{Active: true, MinAge: "1h0m0s", Trim: "the caches are trimmed to their caps by the steward's cache step (Part A) each cycle; the floor's half-cap trim is not built yet",
			Consumers: []Consumer{{Path: "/Users/wido/evidence/gocache-x", Kind: "evidence root", Bytes: 60 << 30, Measured: true, AgeSecs: 7200,
				Use: "no live process has it open", Command: "a person removes it once nothing needs it: rm -rf -- '/Users/wido/evidence/gocache-x'"}},
			Remedy: "metasystem disk clean --preview"},
		Classes: []ClassReport{{Name: "context handoffs", Items: 3, Released: 2}, {Name: "tmpdir strays", Items: 1, Bytes: 4 << 20}},
		Actions: []Line{{Class: "context handoffs", Path: "/repo/artifacts/agents/context/handoffs/6800000000000001", Reason: "context handoff removed"}},
		Kept: []Line{{Class: "stores", Path: "/repo-g1", Reason: "in use by pid 4242 (uid 501, vim notes.txt)",
			Command: "metasystem disk clean --release 01ARZ3NDEKTSV4RRFFQ69G5FAV once pid 4242 has ended"}},
		Pending: []Line{{Class: "context handoffs", Path: "/repo/artifacts/agents/context/handoffs/6800000000000002",
			Reason: "steward arbitration is held; the next pass retries", Command: "metasystem disk clean"}},
		Strays: []Item{{Path: "/var/folders/T/metasystem-audit.x", Bytes: 4 << 20, IdleSecs: 90000,
			Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --preview, then metasystem disk clean --strays"}}},
		Foreign: []Item{{Path: "/var/folders/T/tmp.*", Verdict: Verdict{Reason: "1370 bare tmp.* entries: not engine-named, never touched"}}},
		Backlog: []string{"usage call sessions: 2 item(s) left for the next pass"},
		Census:  &UseCensus{Taken: true, Unreadable: []CensusGap{{Pid: 7, UID: 0, Command: "launchd", Reason: "descriptor list unreadable"}}},
		Notes:   []string{"settings conflict: evidence.machine-cap-gib 50 (m1e), 80 (m1b): 80 in force (the largest)"},
	}
	report.Health = healthOf(report)
	want := []string{
		"machine: apply pass at 2026-09-28T12:00:00Z",
		"  free: 277.0 MiB on /Users/wido, BELOW the floor of 50.0 GiB",
		"  floor mode: ageing lowered to 1h0m0s; the caches are trimmed to their caps by the steward's cache step (Part A) each cycle; the floor's half-cap trim is not built yet",
		"  consumer: /Users/wido/evidence/gocache-x (evidence root): 60.0 GiB, last written 2h0m0s ago, no live process has it open; a person removes it once nothing needs it: rm -rf -- '/Users/wido/evidence/gocache-x'",
		"  context handoffs: 3 item(s), 2 released",
		"  tmpdir strays: 1 item(s), 4.0 MiB",
		"  released: /repo/artifacts/agents/context/handoffs/6800000000000001 (context handoff removed)",
		"  kept: /repo-g1: in use by pid 4242 (uid 501, vim notes.txt); run metasystem disk clean --release 01ARZ3NDEKTSV4RRFFQ69G5FAV once pid 4242 has ended",
		"  pending: /repo/artifacts/agents/context/handoffs/6800000000000002: steward arbitration is held; the next pass retries; run metasystem disk clean",
		"  stray: /var/folders/T/metasystem-audit.x, 4.0 MiB, idle 25h0m0s: engine-prefixed entry no store owns; run metasystem disk clean --preview, then metasystem disk clean --strays",
		"  not the engine's: /var/folders/T/tmp.* (1370 bare tmp.* entries: not engine-named, never touched)",
		"  backlog: usage call sessions: 2 item(s) left for the next pass",
		"  use census incomplete: pid 7 (uid 0, launchd): descriptor list unreadable",
		"  settings conflict: evidence.machine-cap-gib 50 (m1e), 80 (m1b): 80 in force (the largest)",
	}
	if got := report.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if report.Health.Status != HealthAttention || !strings.Contains(report.Health.Reason, "277.0 MiB free, below the floor of 50.0 GiB") {
		t.Fatalf("health = %+v", report.Health)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeJSON(path, report); err != nil {
		t.Fatal(err)
	}
	read, err := ReadReport(path)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(read)
	first, _ := json.Marshal(report)
	if string(again) != string(first) {
		t.Fatalf("the report does not round-trip:\n%s\n%s", first, again)
	}
	if err := os.WriteFile(path, []byte(`{"schema":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReport(path); err == nil {
		t.Fatal("a foreign report was read")
	}
}
