package diskstore

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// --verbose gives every released item, kept line and stray its own line,
// where the default groups a repeated finding into one line with its count.
func TestReportVerboseLinesListEveryItem(t *testing.T) {
	t.Parallel()
	report := Report{Schema: ReportSchema, Kind: "checkout", Name: "/repo", At: testNow, Mode: ModeApply,
		Actions: []Line{
			{Class: "context handoffs", Path: "/repo/h/0", Reason: "context handoff removed"},
			{Class: "context handoffs", Path: "/repo/h/1", Reason: "context handoff removed"},
		},
		Kept: []Line{
			{Class: "stores", Path: "/k/0", Reason: "owner live", Command: "metasystem disk show"},
			{Class: "stores", Path: "/k/1", Reason: "owner live", Command: "metasystem disk show"},
		},
		Strays: []Item{
			{Path: "/tmp/metasystem-a", Bytes: 3 << 20, IdleSecs: 90000, Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --strays"}},
			{Path: "/tmp/metasystem-b", Bytes: 5 << 20, IdleSecs: 90000, Verdict: Verdict{Decision: Keep, Reason: "engine-prefixed entry no store owns", Command: "metasystem disk clean --strays"}},
		},
	}
	grouped := []string{
		"checkout /repo: apply pass at 2026-09-28T12:00:00Z",
		"  released: 2 items (context handoff removed) (e.g. /repo/h/0, /repo/h/1)",
		"  kept: 2 items: owner live; run metasystem disk show (e.g. /k/0, /k/1)",
		"  strays: 2 items, 8.0 MiB: engine-prefixed entry no store owns; run metasystem disk clean --strays (largest: /tmp/metasystem-b 5.0 MiB, /tmp/metasystem-a 3.0 MiB)",
	}
	if got := report.Lines(); strings.Join(got, "\n") != strings.Join(grouped, "\n") {
		t.Fatalf("report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(grouped, "\n"))
	}
	verbose := []string{
		"checkout /repo: apply pass at 2026-09-28T12:00:00Z",
		"  released: /repo/h/0 (context handoff removed)",
		"  released: /repo/h/1 (context handoff removed)",
		"  kept: /k/0: owner live; run metasystem disk show",
		"  kept: /k/1: owner live; run metasystem disk show",
		"  stray: /tmp/metasystem-a, 3.0 MiB, idle 25h0m0s: engine-prefixed entry no store owns; run metasystem disk clean --strays",
		"  stray: /tmp/metasystem-b, 5.0 MiB, idle 25h0m0s: engine-prefixed entry no store owns; run metasystem disk clean --strays",
	}
	if got := report.VerboseLines(); strings.Join(got, "\n") != strings.Join(verbose, "\n") {
		t.Fatalf("verbose report lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(verbose, "\n"))
	}
}

// personOutcomes is a stray act's outcomes in the order the plan listed
// them: done and kept interleaved, two findings repeated.
func personOutcomes() []PersonOutcome {
	inUse := func(path string, pid string, bytes int64) PersonOutcome {
		return PersonOutcome{Path: path, Reason: "in use by pid " + pid + " (uid 501, go)", Command: "metasystem disk clean --strays once pid " + pid + " has ended",
			Finding: "in use by a live process", FindingCommand: "metasystem disk clean --strays once each holder has ended (--verbose names them)", Bytes: bytes}
	}
	removed := func(path string, bytes int64) PersonOutcome {
		return PersonOutcome{Path: path, Done: true, Reason: "removed", Finding: "removed", Bytes: bytes}
	}
	replaced := "metasystem disk clean --preview, then --strays with the new plan"
	return []PersonOutcome{
		inUse("/tmp/e", "7", 10),
		removed("/tmp/a", 3<<20),
		{Path: "/tmp/g", Reason: "replaced since the preview", Command: replaced, Finding: "replaced since the preview", FindingCommand: replaced},
		removed("/tmp/b", 1<<20),
		{Path: "/tmp/d", Done: true, Reason: "already gone", Finding: "already gone", Bytes: 99},
		inUse("/tmp/f", "8", 20),
		removed("/tmp/c", 2<<20),
		{Path: "/tmp/i", Done: true, Reason: "released", Finding: "released", Bytes: 512},
	}
}

// A person's act prints one line per finding, done before kept, with the
// count, the total and the largest three; a kept finding names the command
// that settles all of its items.
func TestOutcomeLinesGroupByFinding(t *testing.T) {
	t.Parallel()
	want := []string{
		"  removed: 3 strays, 6.0 MiB (largest: /tmp/a 3.0 MiB, /tmp/c 2.0 MiB, /tmp/b 1.0 MiB)",
		"  already gone: /tmp/d, 99 B",
		"  released: /tmp/i, 512 B",
		"  kept: 2 strays, 30 B: in use by a live process; run metasystem disk clean --strays once each holder has ended (--verbose names them) (largest: /tmp/f 20 B, /tmp/e 10 B)",
		"  kept: /tmp/g: replaced since the preview; run metasystem disk clean --preview, then --strays with the new plan",
	}
	if got := OutcomeLines("strays", personOutcomes(), false); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("outcome lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// --verbose prints each outcome on its own line in the plan's order, with
// the item's own reason and command (the holder's pid, not the finding).
func TestOutcomeLinesVerboseKeepsEachItemsOwnReason(t *testing.T) {
	t.Parallel()
	want := []string{
		"kept /tmp/e: in use by pid 7 (uid 501, go); run metasystem disk clean --strays once pid 7 has ended",
		"removed: /tmp/a",
		"kept /tmp/g: replaced since the preview; run metasystem disk clean --preview, then --strays with the new plan",
		"removed: /tmp/b",
		"already gone: /tmp/d",
		"kept /tmp/f: in use by pid 8 (uid 501, go); run metasystem disk clean --strays once pid 8 has ended",
		"removed: /tmp/c",
		"released: /tmp/i",
	}
	if got := OutcomeLines("strays", personOutcomes(), true); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("verbose outcome lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Freed counts what the act itself removed or released: an item already
// gone and a kept item free nothing.
func TestFreedCountsOnlyWhatTheActRemoved(t *testing.T) {
	t.Parallel()
	if got, want := Freed(personOutcomes()), int64(6<<20+512); got != want {
		t.Fatalf("freed %d bytes, want %d", got, want)
	}
	if got := Freed(nil); got != 0 {
		t.Fatalf("freed %d bytes by no act", got)
	}
}

// A removal stopped at a permission names the entry that refused and sends
// the person to its owner, since running again fails the same way; any
// other stop is finished by the next run.
func TestRemovalStoppedNamesTheRefusingEntry(t *testing.T) {
	t.Parallel()
	denied := &fs.PathError{Op: "unlinkat", Path: "/tmp/metasystem-x/locked/file", Err: fs.ErrPermission}
	reason, command := removalStopped("/tmp/metasystem-x", denied)
	if !strings.HasPrefix(reason, "removal stopped: unlinkat /tmp/metasystem-x/locked/file: permission denied; ") || !strings.Contains(reason, "another user's or flagged immutable") {
		t.Fatalf("permission reason = %q", reason)
	}
	if command != "ls -ld /tmp/metasystem-x/locked/file to see its owner, and remove it as that owner" {
		t.Fatalf("permission command = %q", command)
	}
	reason, command = removalStopped("/tmp/metasystem-x", fs.ErrPermission)
	if command != "ls -ld /tmp/metasystem-x to see its owner, and remove it as that owner" || !strings.HasPrefix(reason, "removal stopped: permission denied; ") {
		t.Fatalf("bare permission = %q, %q", reason, command)
	}
	reason, command = removalStopped("/tmp/metasystem-x", errors.New("walk cut short"))
	if reason != "removal stopped: walk cut short" || command != "metasystem disk clean --strays again" {
		t.Fatalf("other stop = %q, %q", reason, command)
	}
}
