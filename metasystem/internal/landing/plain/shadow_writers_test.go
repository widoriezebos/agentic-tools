package plain

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestShadowWriterTimesPreserveBatchAndDetachedProof(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	home := shadowHome(t, b.checkout, b.install)
	registered, _, err := lane.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lane.RecordPath(home))
	if err != nil {
		t.Fatal(err)
	}
	now := bedNow
	s := b.seams
	s.Incidents = shadowSeams().Incidents
	s.Now = func() time.Time { return now }
	s.Lane = func() (lane.Record, error) { r, _, err := lane.Read(home); return r, err }
	s.Launch = func(_ []string, _ string, log string) (int64, error) {
		if err := os.MkdirAll(filepath.Dir(log), 0700); err != nil {
			return 0, err
		}
		return int64(os.Getpid()), os.WriteFile(log, nil, 0600)
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	git := s.Git
	s.Git = func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "cat-file", "fetch":
			return "", nil
		case "merge-base":
			if args[3] == "parent" && args[2] != "parent" {
				return "", notAncestor
			}
			return "", nil
		case "rev-list":
			if slices.Contains(args, "--parents") {
				return "commit parent tip", nil
			}
			return "tip", nil
		}
		return git(dir, args...)
	}
	batch := &Batch{ID: "selected", Lane: registered, Base: "parent", State: BatchPrepared, CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Selector: PolicyValue{Value: "auto"}, Open: true, Members: []GoalSHA{{Goal: "A", SHA: "tip"}}}
	if _, _, err := HandIn(b.install, Line{Goal: "A", SHA: "tip", At: batch.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	running, _, err := Start(b.install, b.checkout, s)
	if err != nil {
		t.Fatal(err)
	}
	check := func(wantWord string, want time.Time) {
		t.Helper()
		if _, err := RefreshShadow(home, now, "fixture", shadowSeams()); err != nil {
			t.Fatal(err)
		}
		r, _, err := state.Read(Dir(b.install))
		if err != nil || r.Batch.Execution.Word != wantWord || r.Batch.Execution.Since.Observed || !r.Batch.Execution.Since.At.Equal(want) {
			t.Fatalf("batch writer time: %+v %v", r.Batch, err)
		}
	}
	check(BatchRunning, now)
	now = now.Add(time.Minute)
	if _, err := lane.SetPause(home, "person", now); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := lane.ClearPauseAt(home, "person", now); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(lane.RecordPath(home))
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("resume rewrote registration: %v", err)
	}
	if _, err := checkBatchLocked(b.install, b.checkout, "commit", "parent", false, false, s); err != nil {
		t.Fatalf("resume voided batch: %v", err)
	}
	if running.Admission == nil || running.Admission.Lane == nil || *running.Admission.Lane != registered {
		t.Fatalf("proof did not save registration: %+v", running.Admission)
	}
	check(BatchRunning, bedNow)
	r, _, _ := state.Read(Dir(b.install))
	if r.Lane.Word != "running" || r.Lane.Since.Observed || !r.Lane.Since.At.Equal(now) || r.Lane.By != "person" {
		t.Fatalf("resume time: %+v", r.Lane)
	}
	result, err := Run(b.install, b.checkout, "printf 'LANDING-CHECKED\t0\n'", running.Attempt, io.Discard, s)
	if err != nil || result.Result != Green {
		t.Fatalf("resume voided detached admission: %+v %v", result, err)
	}
	now = now.Add(time.Minute)
	if err := appendLine(queuePath(b.install), Line{Goal: "A", SHA: "tip", Outcome: StateReturned, At: now.Format(time.RFC3339Nano), Reason: "returned"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectBatch(b.install, b.checkout, registered, s); err != nil {
		t.Fatal(err)
	}
	check(BatchClosed, now)
}

func TestShadowKeeperTransitionDoesNotDrift(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	if _, err := RefreshShadow(b.home, bedNow, "fixture", s); err != nil {
		t.Fatal(err)
	}
	pauseAt := bedNow.Add(time.Minute)
	if _, err := lane.SetPause(b.home, "person", pauseAt); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshShadow(b.home, pauseAt, "keeper", s); err != nil {
		t.Fatal(err)
	}
	for _, event := range shadowEvents(t, b.install) {
		if event.Kind == "drift" {
			t.Fatalf("transition became drift: %+v", event)
		}
	}
	// A hand edit still leaves one unexplained change for the keeper.
	data := []byte(`{"by":"person","at":"2026-10-01T12:00:00Z","reason":"hand edit"}`)
	if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshShadow(b.home, pauseAt.Add(time.Minute), "keeper", s); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range shadowEvents(t, b.install) {
		if event.Kind == "drift" {
			count++
			if !strings.Contains(event.Evidence, "lane.") {
				t.Fatal(event)
			}
		}
	}
	if count != 1 {
		t.Fatalf("handwritten resume drift events=%d", count)
	}
}

func TestLandingStewardReportDayDrift(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	for _, event := range []state.Event{
		{Kind: "drift", By: "keeper", At: b.now.Add(-2 * time.Hour), Evidence: "lane.word: running → paused"},
		{Kind: "drift", By: "keeper", At: b.now.Add(-2 * time.Hour), Evidence: "lane.since.at: " + b.now.Add(-time.Hour).String() + " → " + b.now.String()},
		{Kind: "drift", By: "keeper", At: b.now.Add(-24 * time.Hour), Evidence: "yesterday"},
		{Kind: "drift", By: "keeper", At: b.now.Add(time.Hour), Evidence: "future"},
		{Kind: "drift", By: "landing stop", At: b.now, Evidence: "published stop"},
		{Kind: "drift", At: b.now, Evidence: "unknown publisher"},
		{Kind: "rebuild", At: b.now, Evidence: "rebuilt"},
	} {
		b.linesAppend("events.jsonl", event)
	}
	b.report(b.now)
	text := b.text()
	if !strings.Contains(text, "### drift\n- "+b.now.Add(-2*time.Hour).Local().Format("15:04")+": lane.word: running → paused") || strings.Contains(text, "yesterday") || strings.Contains(text, "future") || strings.Contains(text, "rebuilt") || strings.Contains(text, "published stop") || strings.Contains(text, "unknown publisher") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "lane.since.at: "+b.now.Add(-time.Hour).Local().Format("2006-01-02 15:04 MST")+" → "+b.now.Local().Format("2006-01-02 15:04 MST")) {
		t.Fatal("report kept machine clocks: " + text)
	}
}

func TestShadowBatchCloseTimes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"accounted", "push", "drain"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := shadowSeams()
			now := bedNow.Add(time.Minute)
			s.Now = func() time.Time { return now }
			batch := &Batch{ID: "selection", Lane: b.record, Base: "main", State: BatchRunning}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			switch path {
			case "accounted":
				if _, err := SelectBatch(b.install, b.checkout, b.record, s); err != nil {
					t.Fatal(err)
				}
			case "push":
				if err := completePush(b.install, b.checkout, batch, "before", "main", "tree", now, s); err != nil {
					t.Fatal(err)
				}
			case "drain":
				drainFixture(t, b.install)
				if _, err := AdvanceDrain(b.install, b.checkout, s); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := RefreshShadow(b.home, now, "fixture", s); err != nil {
				t.Fatal(err)
			}
			r, _, err := state.Read(Dir(b.install))
			if err != nil || r.Batch.ID != "" {
				t.Fatalf("%s empty closed batch remained: %+v %v", path, r.Batch, err)
			}
			closed, err := ReadBatch(b.install)
			if err != nil || closed == nil || closed.State != BatchClosed || closed.ClosedAt != now.Format(time.RFC3339Nano) {
				t.Fatalf("%s raw close clock: %+v %v", path, closed, err)
			}
			closedAt := closed.ClosedAt
			if err := completePush(b.install, b.checkout, closed, "before", "main", "tree", now.Add(time.Minute), s); err != nil {
				t.Fatal(err)
			}
			again, err := ReadBatch(b.install)
			if err != nil || again.ClosedAt != closedAt {
				t.Fatalf("confirmation retimed closure: %+v %v", again, err)
			}
		})
	}
}
