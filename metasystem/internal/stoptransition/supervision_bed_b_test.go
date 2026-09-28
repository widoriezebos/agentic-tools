package stoptransition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// supBFamily is one per-test family whose items stay live until the
// transition stops them. prime hands each inventoried item to the real family
// owner whose Stop renders the outcome.
type supBFamily struct {
	name    string
	items   []Item
	stopped map[string]bool
	prime   func(Item)
	stop    func(Item) (Outcome, error)
}

func (f *supBFamily) Name() string { return f.name }

func (f *supBFamily) Inventory() ([]Item, error) {
	var live []Item
	for _, item := range f.items {
		if f.stopped[item.Key] {
			continue
		}
		if f.prime != nil {
			f.prime(item)
		}
		live = append(live, item)
	}
	return live, nil
}

func (f *supBFamily) Stop(item Item) (Outcome, error) {
	if f.stopped == nil {
		f.stopped = map[string]bool{}
	}
	f.stopped[item.Key] = true
	return f.stop(item)
}

type supBBed struct {
	transition *Transition
	jobPath    string
}

// supBStopEverythingBed is the stop-everything checkout from the supervision
// bed: one held job, one wrapped run, the steward runner, and the complete
// supervision set. The job, steward and supervision outcomes are rendered by
// the real family owners; only their process effects are injected.
func supBStopEverythingBed(t *testing.T) supBBed {
	t.Helper()
	root := t.TempDir()
	checkout := "/checkout"
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	jobPath := filepath.Join(root, "artifacts", "agents", "jobs", "stop-fixture-job.json")
	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, []byte(`{"jobId":"stop-fixture-job","status":"running","pid":4101,"pidStartedAt":4100,"pgid":4101,"role":"design-critic"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	job := newJobFamily(LocalConfig{Root: root, Checkout: checkout})
	var cancelled []string
	job.config.CancelJob = func(id string) (string, error) {
		cancelled = append(cancelled, id)
		record, err := dispatch.ReadRecordObject(jobPath)
		if err != nil {
			return "", err
		}
		record["status"] = "cancelled"
		data, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		return "", os.WriteFile(jobPath, data, 0o644)
	}
	t.Cleanup(func() {
		if len(cancelled) > 1 {
			t.Errorf("job cancel path ran %d times, want once: %v", len(cancelled), cancelled)
		}
	})

	pid, pgid := int64(4201), int64(4201)
	run := &supBFamily{
		name: "run",
		items: []Item{{Key: "run:stop-fixture-run", StatusLine: "run stop-fixture-run running pid 4201 pgid 4201 wrapped: running",
			Survivor: stopfence.Survivor{Component: "run", ID: "stop-fixture-run", Pid: pid, PidStartedAt: 4200}}},
		stop: func(item Item) (Outcome, error) {
			line, complete, err := runOutcomeLine(runpkg.StopOutcome{
				RunID: "stop-fixture-run", InitialStatus: runpkg.StatusRunning, Status: runpkg.StatusEndedUnknown,
				Custody: runpkg.CustodyWrapped, PID: pid, PGID: pgid, Signal: runpkg.StopSignalTerm,
				Result: runpkg.StopResultStopped, Reason: "TERM",
			}, false)
			return Outcome{Line: line, Complete: complete, Survivor: item.Survivor}, err
		},
	}

	stewardOwner := newStewardFamily(root)
	disarms := 0
	stewardOwner.disarm = func(string) (steward.RunnerStopOutcome, error) {
		disarms++
		return steward.RunnerStopOutcome{Signal: "term", Result: "stopped"}, nil
	}
	stewardItem := Item{Key: "steward:runner", StatusLine: "steward-runner pid 4301 started 4300: running",
		Survivor: stopfence.Survivor{Component: "steward-runner", Pid: 4301, PidStartedAt: 4300}}
	stewardFamily := &supBFamily{
		name: "steward", items: []Item{stewardItem},
		prime: func(Item) {
			stewardOwner.items["steward:runner"] = steward.RunnerRecord{Pid: 4301, PidStartedAt: 4300}
		},
		stop: stewardOwner.Stop,
	}

	config := LocalConfig{Root: root, Installation: root, Checkout: checkout}
	supervision := newSupervisionFamily(config, &processSnapshot{})
	inventory := []supervise.InventoryItem{
		{Component: "supervision-owner", Identity: identity.Ref{Pid: 4401, StartedAtSec: 4400}, Tag: "owner-tag", Generation: 3},
		{Component: "repo-watcher", Identity: identity.Ref{Pid: 4402, StartedAtSec: 4400}, Tag: "owner-tag-watcher-3", Generation: 3},
		{Component: "job-reaper", Identity: identity.Ref{Pid: 4403, StartedAtSec: 4400}, Tag: "owner-tag-reaper-3", Generation: 3},
		{Component: "landing-batch-owner", Identity: identity.Ref{Pid: 4404, StartedAtSec: 4400}, Tag: "owner-tag-landing-owner-3", Generation: 3},
	}
	shutdowns := 0
	supervision.shutdown = func(string, string, string, string, int) (supervise.ShutdownReport, error) {
		shutdowns++
		report := supervise.ShutdownReport{}
		for index, current := range inventory {
			outcome := supervise.ComponentOutcome{Component: current.Component, Identity: current.Identity, Tag: current.Tag, Generation: current.Generation}
			if index == 0 {
				outcome.Signal, outcome.Result, outcome.Reason = supervise.ShutdownSignalTerm, supervise.ShutdownStopped, "shutdown"
			} else {
				outcome.Signal, outcome.Result, outcome.Reason = supervise.ShutdownSignalNone, supervise.ShutdownAlreadyGone, "by the owner"
			}
			report.Outcomes = append(report.Outcomes, outcome)
		}
		return report, nil
	}
	var supervisionItems []Item
	for _, current := range inventory {
		line := current.Component + " pid " + strconv.FormatInt(current.Identity.Pid, 10)
		if current.Component == "supervision-owner" {
			line += " tag owner-tag generation 3"
		}
		supervisionItems = append(supervisionItems, Item{
			Key: "supervision:" + current.Component + ":3:" + strconv.FormatInt(current.Identity.Pid, 10), StatusLine: line + ": running",
			Survivor: stopfence.Survivor{Component: current.Component, Pid: current.Identity.Pid, PidStartedAt: current.Identity.StartedAtSec, Tag: current.Tag},
		})
	}
	supervisionFamily := &supBFamily{
		name: "supervision", items: supervisionItems,
		prime: func(item Item) {
			for _, current := range inventory {
				if item.Key == "supervision:"+current.Component+":3:"+strconv.FormatInt(current.Identity.Pid, 10) {
					supervision.items[item.Key] = current
				}
			}
			supervision.failureAnchor = supervisionItems[len(supervisionItems)-1].Key
		},
		stop: supervision.Stop,
	}
	t.Cleanup(func() {
		if shutdowns > 1 || disarms > 1 {
			t.Errorf("supervision shutdown ran %d times and steward disarm %d times; want at most once each", shutdowns, disarms)
		}
	})

	return supBBed{jobPath: jobPath, transition: &Transition{
		Root: root, Checkout: checkout,
		Families: []Family{job, run, stewardFamily, supervisionFamily},
		Prober:   fakeProber{10: identity.Alive}, Now: func() time.Time { return now },
		Sleep: func(time.Duration) { t.Fatal("the stop-everything transition waited on a clock") },
		Self:  func() (identity.Ref, error) { return identity.Ref{Pid: 10, StartedAtSec: 20}, nil },
	}}
}

func supBFileDigests(t *testing.T, root string) map[string]string {
	t.Helper()
	digests := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == stopfence.TransitionPath(root) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		digests[path] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digests
}

func supBIdentities(lines []string, verdict func(string) (string, bool)) []string {
	var identities []string
	for _, line := range lines {
		if identity, ok := verdict(line); ok {
			identities = append(identities, identity)
		}
	}
	sort.Strings(identities)
	return identities
}

// TestSupBStopEverythingStatusStopAndSecondStopPages ports the
// stop-everything and status-is-live scenarios of supervision-fixtures part B:
// the exact live status page, the exact stop page in family order with each
// family's verdict grammar, the terminal job record, the completed fence, the
// status and stop pages naming the same identities, the stopped status page,
// and a second stop that finds nothing and changes nothing (U-idem: unchanged).
func TestSupBStopEverythingStatusStopAndSecondStopPages(t *testing.T) {
	t.Parallel()
	bed := supBStopEverythingBed(t)
	transition := bed.transition

	status, err := transition.Status()
	wantStatus := []string{
		"checkout /checkout",
		"job stop-fixture-job running pid 4101 pgid 4101 role design-critic: running",
		"run stop-fixture-run running pid 4201 pgid 4201 wrapped: running",
		"steward-runner pid 4301 started 4300: running",
		"supervision-owner pid 4401 tag owner-tag generation 3: running",
		"repo-watcher pid 4402: running",
		"job-reaper pid 4403: running",
		"landing-batch-owner pid 4404: running",
	}
	if err != nil || status.ExitCode != 0 || strings.Join(status.Lines, "\n") != strings.Join(wantStatus, "\n") {
		t.Fatalf("live status = %#v err=%v\nwant:\n%s", status, err, strings.Join(wantStatus, "\n"))
	}

	stop, err := transition.Stop()
	wantStop := []string{
		"checkout /checkout",
		"job stop-fixture-job running pid 4101 pgid 4101 role design-critic: cancelled (cancel path, TERM)",
		"run stop-fixture-run running pid 4201 pgid 4201 wrapped: concluded ended-unknown (TERM)",
		"steward-runner pid 4301 started 4300: stopped (TERM)",
		"narrator: stopped with the steward runner",
		"supervision-owner pid 4401 tag owner-tag generation 3: stopped (TERM, exited reason=shutdown)",
		"repo-watcher pid 4402: already gone (by the owner)",
		"job-reaper pid 4403: already gone (by the owner)",
		"landing-batch-owner pid 4404: already gone (by the owner)",
		"stopped /checkout; start again: metasystem system start --repo /checkout",
	}
	if err != nil || stop.ExitCode != 0 || strings.Join(stop.Lines, "\n") != strings.Join(wantStop, "\n") {
		t.Fatalf("stop page = %#v err=%v\nwant:\n%s", stop, err, strings.Join(wantStop, "\n"))
	}
	record, err := dispatch.ReadRecordObject(bed.jobPath)
	if err != nil || record["status"] != "cancelled" {
		t.Fatalf("stopped job record = %v err=%v, want status cancelled", record, err)
	}
	fence, err := stopfence.Read(transition.Root)
	if err != nil || fence.State != stopfence.StateClosed || fence.Phase != stopfence.PhaseStopped || fence.Generation != 1 ||
		fence.By.Verb != "stop" || fence.By.Pid != 10 || len(fence.NotStopped) != 0 || fence.ChangedAt != "2026-09-27T12:00:00Z" {
		t.Fatalf("stop fence = %#v err=%v, want closed stopped generation 1 by stop pid 10", fence, err)
	}

	// status-is-live: the identities status printed are exactly the ones stop
	// acted on; only the verdict text differs.
	processFamilies := []string{"steward-runner ", "supervision-owner ", "repo-watcher ", "job-reaper ", "landing-batch-owner "}
	processLine := func(line string) bool {
		for _, prefix := range processFamilies {
			if strings.HasPrefix(line, prefix) {
				return true
			}
		}
		return false
	}
	statusIdentities := supBIdentities(status.Lines, func(line string) (string, bool) {
		return strings.TrimSuffix(line, ": running"), processLine(line)
	})
	stopIdentities := supBIdentities(stop.Lines, func(line string) (string, bool) {
		at := strings.Index(line, ": ")
		if at < 0 || !processLine(line) {
			return "", false
		}
		return line[:at], true
	})
	if strings.Join(statusIdentities, "\n") != strings.Join(stopIdentities, "\n") || len(statusIdentities) != 5 {
		t.Fatalf("status identities %q differ from stop identities %q", statusIdentities, stopIdentities)
	}

	after, err := transition.Status()
	wantAfter := "checkout /checkout\nnothing is running\nstopped since 2026-09-27T12:00:00Z by stop pid 10; start again: metasystem system start --repo /checkout"
	if err != nil || after.ExitCode != 0 || strings.Join(after.Lines, "\n") != wantAfter {
		t.Fatalf("stopped status = %#v err=%v, want %q", after, err, wantAfter)
	}

	before := supBFileDigests(t, transition.Root)
	second, err := transition.Stop()
	// U-idem: a stop of a stopped checkout is a repeat whose effect holds:
	// success, unchanged, the stop it already carries named.
	wantSecond := "MetaSystem is already stopped for /checkout (since 2026-09-27T12:00:00Z); nothing is running; start again: metasystem system start --repo /checkout"
	if err != nil || second.ExitCode != 0 || !second.Unchanged || strings.Join(second.Lines, "\n") != wantSecond {
		t.Fatalf("second stop = %#v err=%v, want %q", second, err, wantSecond)
	}
	afterSecond := supBFileDigests(t, transition.Root)
	if len(before) != len(afterSecond) {
		t.Fatalf("the second stop changed the file set outside the fence record: before=%v after=%v", before, afterSecond)
	}
	for path, digest := range before {
		if afterSecond[path] != digest {
			t.Fatalf("the second stop changed %s outside the fence record", path)
		}
	}
}
