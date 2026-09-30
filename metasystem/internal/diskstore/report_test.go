package diskstore

import (
	"encoding/json"
	"fmt"
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

// A finding repeated for many paths is one line with its count and at most
// three example paths, never the same message N times: the live m1e report
// printed 45 "host settings unknown" lines, one per removed fixture checkout.
func TestReportLinesGroupRepeatedFindings(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "machine", Name: "machine", At: testNow, Mode: ModeReport}
	for index := 0; index < 45; index++ {
		path := fmt.Sprintf("/private/var/folders/T/tmp.%02d/repo", index)
		report.HostUnknown = append(report.HostUnknown, fmt.Sprintf("host settings unknown: %s unreadable: state root: inspect repository path: stat %s: no such file or directory; run metasystem settings check there", path, path))
	}
	report.HostUnknown = append(report.HostUnknown, "host settings unknown: /m1b unreadable: settings of /m1b/metasystem.conf unreadable at disk.floor-gib: permission denied; run metasystem settings check there")
	for index := 0; index < 5; index++ {
		report.Pending = append(report.Pending, Line{Class: "stores", Path: fmt.Sprintf("/stores/s%d", index), Reason: "no proof for this owner kind yet", Command: "metasystem disk show"})
	}
	report.Pending = append(report.Pending, Line{Class: "stores", Path: "/stores/other", Reason: "in use by pid 7", Command: "metasystem disk show"})
	report.Notes = []string{"settings conflict: x", "settings conflict: x"}
	want := []string{
		"machine: report pass at 2026-09-28T12:00:00Z",
		"  pending: 5 items: no proof for this owner kind yet; run metasystem disk show (e.g. /stores/s0, /stores/s1, /stores/s2)",
		"  pending: /stores/other: in use by pid 7; run metasystem disk show",
		"  host settings unknown: 45 checkouts unreadable: state root: inspect repository path: stat <checkout>: no such file or directory; run metasystem settings check there (e.g. /private/var/folders/T/tmp.00/repo, /private/var/folders/T/tmp.01/repo, /private/var/folders/T/tmp.02/repo)",
		"  host settings unknown: /m1b unreadable: settings of /m1b/metasystem.conf unreadable at disk.floor-gib: permission denied; run metasystem settings check there",
		"  settings conflict: x (2 times)",
	}
	if got := report.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Strays with the same reason and command are one line with their count,
// their total size and the three largest, whatever their ages: the live
// preview printed 2330 stray lines.
func TestReportLinesGroupStrays(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "machine", Name: "machine", At: testNow, Mode: ModePreview}
	for index := 0; index < 5; index++ {
		report.Strays = append(report.Strays, Item{Path: fmt.Sprintf("/tmp/metasystem-proofrun-admission.%d", index), Bytes: int64(index+1) << 20, IdleSecs: 90000 + int64(index),
			Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --preview, then metasystem disk clean --strays"}})
	}
	for index := 0; index < 3; index++ {
		report.Strays = append(report.Strays, Item{Path: fmt.Sprintf("/tmp/metasystem-audit.%d", index), Bytes: 10, IdleSecs: int64(3600 * (index + 1)),
			Verdict: Verdict{Decision: Keep, Reason: fmt.Sprintf("engine-prefixed entry no store owns, written %dh0m0s ago: not removable before it is idle a day", index+1), Command: "metasystem disk show"}})
	}
	report.Strays = append(report.Strays, Item{Path: "/tmp/metasystem-one", Bytes: 4 << 20, IdleSecs: 90000,
		Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --strays"}})
	want := []string{
		"machine: preview pass at 2026-09-28T12:00:00Z",
		"  strays: 5 items, 15.0 MiB: engine-prefixed entry no store owns; run metasystem disk clean --preview, then metasystem disk clean --strays (largest: /tmp/metasystem-proofrun-admission.4 5.0 MiB, /tmp/metasystem-proofrun-admission.3 4.0 MiB, /tmp/metasystem-proofrun-admission.2 3.0 MiB)",
		"  strays: 3 items, 30 B: engine-prefixed entry no store owns, written less than a day ago: not removable before it is idle a day; run metasystem disk show (largest: /tmp/metasystem-audit.0 10 B, /tmp/metasystem-audit.1 10 B, /tmp/metasystem-audit.2 10 B)",
		"  stray: /tmp/metasystem-one, 4.0 MiB, idle 25h0m0s: engine-prefixed entry no store owns; run metasystem disk clean --strays",
	}
	if got := report.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Releases and planned releases with the same reason are one line each
// with their count and three examples.
func TestReportLinesGroupReleases(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "checkout", Name: "/repo", At: testNow, Mode: ModeApply}
	for index := 0; index < 4; index++ {
		report.Actions = append(report.Actions, Line{Class: "context handoffs", Path: fmt.Sprintf("/repo/h/%d", index), Reason: "context handoff removed"})
		report.Planned = append(report.Planned, Item{Class: "context handoffs", Path: fmt.Sprintf("/repo/p/%d", index), Verdict: Verdict{Decision: Release, Reason: "complete"}})
	}
	want := []string{
		"checkout /repo: apply pass at 2026-09-28T12:00:00Z",
		"  released: 4 items (context handoff removed) (e.g. /repo/h/0, /repo/h/1, /repo/h/2)",
		"  would release: 4 items (complete) (e.g. /repo/p/0, /repo/p/1, /repo/p/2)",
	}
	if got := report.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Rows are the report as a person's terminal shows it under a heading the
// caller prints: no title, no indent, counted nouns, ages as durations,
// only the classes that hold something, and an incomplete use census as a
// count, its processes named only with --verbose.
func TestReportRowsForAPerson(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "machine", Name: "machine", At: testNow, Mode: ModeApply,
		Volumes: []Volume{{Path: "/Users/wido", FreeBytes: 277 << 20, FloorBytes: 50 << 30, BelowFloor: true}},
		Floor: &FloorReport{Active: true, MinAge: "1h0m0s", Trim: "the caches are trimmed to their caps each cycle",
			Consumers: []Consumer{{Path: "/Users/wido/evidence/gocache-x", Kind: "evidence root", Bytes: 60 << 30, Measured: true, AgeSecs: 7200,
				Use: "no live process has it open", Command: "a person removes it once nothing needs it: rm -rf -- '/Users/wido/evidence/gocache-x'"}}},
		Classes: []ClassReport{{Name: "context handoffs", Items: 3, Released: 2}, {Name: "tmpdir strays", Items: 1, Bytes: 4 << 20}, {Name: "engine pins"}},
		Strays: []Item{{Path: "/var/folders/T/metasystem-audit.x", Bytes: 4 << 20, IdleSecs: 90000,
			Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --strays"}}},
		Backlog: []string{"usage call sessions: 2 item(s) left for the next pass", "unit records: 1 item(s) left for the next pass"},
		Census: &UseCensus{Taken: true, Unreadable: []CensusGap{{Pid: 7, UID: 0, Command: "launchd", Reason: "descriptor list unreadable"},
			{Pid: 8, UID: 0, Reason: "descriptor list unreadable"}}},
	}
	want := []string{
		"free: 277.0 MiB on /Users/wido, below the floor of 50.0 GiB",
		"floor mode: ageing lowered to 1h00m; the caches are trimmed to their caps each cycle",
		"consumer: /Users/wido/evidence/gocache-x (evidence root): 60.0 GiB, last written 2h00m ago, no live process has it open; a person removes it once nothing needs it: rm -rf -- '/Users/wido/evidence/gocache-x'",
		"context handoffs: 3 items, 2 released",
		"tmpdir strays: 1 item, 4.0 MiB",
		"stray: /var/folders/T/metasystem-audit.x, 4.0 MiB, idle 25h00m: engine-prefixed entry no store owns; run metasystem disk clean --strays",
		"backlog: usage call sessions: 2 items left for the next pass",
		"backlog: unit records: 1 item left for the next pass",
		"use census: 2 processes could not be inspected (--verbose names them)",
	}
	if got := report.Rows(false); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	verbose := report.Rows(true)
	if !strings.Contains(strings.Join(verbose, "\n"), "engine pins: no items") ||
		!strings.Contains(strings.Join(verbose, "\n"), "use census incomplete: pid 7 (uid 0, launchd): descriptor list unreadable; pid 8 (uid 0): descriptor list unreadable") {
		t.Fatalf("verbose rows:\n%s", strings.Join(verbose, "\n"))
	}
}
