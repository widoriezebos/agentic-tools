package deploy

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var boundA0, boundA, boundB, boundC = strings.Repeat("0", 40), strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)

type boundBed struct {
	t                *testing.T
	runner           *Runner
	parents          map[string]string
	tips             []string
	fetches, kills   int
	calls            []string
	active           Response
	failBuild        bool
	entered, release chan struct{}
}

func newBoundBed(t *testing.T) *boundBed {
	t.Helper()
	b := &boundBed{t: t, parents: map[string]string{boundA0: "", boundA: boundA0, boundB: boundA, boundC: boundA0}, tips: []string{boundB, boundB}}
	clock := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	b.runner = &Runner{Dir: t.TempDir(), Project: "fixture", Installation: ".", Commit: boundA, By: "Wido", Now: func() time.Time { clock = clock.Add(time.Second); return clock }, Transport: b.start}
	b.runner.Git = Git{FetchMain: func() (string, error) {
		b.fetches++
		if len(b.tips) == 0 {
			return "", errors.New("unexpected fetch")
		}
		tip := b.tips[0]
		b.tips = b.tips[1:]
		return tip, nil
	}, Contains: b.contains, AddTree: fakeTree([]string{"fake"}), RemoveTree: os.RemoveAll}
	return b
}

func (b *boundBed) contains(commit, descendant string) (bool, error) {
	if _, ok := b.parents[commit]; !ok {
		return false, errors.New("unknown commit")
	}
	for at := descendant; at != ""; {
		parent, ok := b.parents[at]
		if !ok {
			return false, errors.New("unknown descendant")
		}
		if at == commit {
			return true, nil
		}
		at = parent
	}
	return false, nil
}

func boundArtifact(commit string) Response {
	return Response{Outcome: OutcomeActive, Version: "v-" + commit, Artifact: "art-" + commit, Digest: "d-" + commit}
}

func (b *boundBed) seed(commit string) {
	b.t.Helper()
	b.active = boundArtifact(commit)
	line := Line{Kind: KindDeploy, Commit: commit, By: "fixture", Outcome: OutcomeActive, Version: b.active.Version, Artifact: b.active.Artifact, Digest: b.active.Digest}
	if err := appendLine(b.runner.Dir, line); err != nil {
		b.t.Fatal(err)
	}
}

func (b *boundBed) start(command *exec.Cmd) (Process, error) {
	var request Request
	if err := json.NewDecoder(command.Stdin).Decode(&request); err != nil {
		return Process{}, err
	}
	b.calls = append(b.calls, request.Operation+" "+request.Commit)
	return Process{PID: 999999999, Kill: func() { b.kills++ }, Wait: func() int {
		response := boundArtifact(request.Commit)
		switch request.Operation {
		case "version":
			response = b.active
			if response.Outcome == "" {
				response.Outcome = OutcomeNone
			}
		case "build":
			if b.entered != nil {
				close(b.entered)
				<-b.release
			}
			response.Outcome = "built"
			if b.failBuild {
				response = Response{Outcome: "failed", Reason: "build failed"}
			}
		case "activate":
			b.active = response
		default:
			b.t.Errorf("unexpected operation %q", request.Operation)
		}
		if err := json.NewEncoder(command.Stdout).Encode(response); err != nil {
			b.t.Error(err)
		}
		if response.Outcome == "failed" {
			return 1
		}
		return 0
	}}, nil
}

func (b *boundBed) run(outcome string) Report {
	b.t.Helper()
	report, err := b.runner.Run()
	if err != nil || report.Outcome != outcome {
		b.t.Fatalf("run = %+v, %v; want %s", report, err, outcome)
	}
	if b.kills != 0 {
		b.t.Fatalf("fake process killed %d times", b.kills)
	}
	return report
}

func (b *boundBed) wantCalls(want ...string) {
	b.t.Helper()
	if !reflect.DeepEqual(b.calls, want) {
		b.t.Fatalf("calls = %v; want %v", b.calls, want)
	}
}

func (b *boundBed) files() map[string]string {
	b.t.Helper()
	result := map[string]string{}
	for _, name := range []string{"deploys.jsonl", "pending.json", "run.json"} {
		data, err := os.ReadFile(filepath.Join(b.runner.Dir, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			b.t.Fatal(err)
		}
		if err == nil {
			result[name] = string(data)
		}
	}
	return result
}

func TestBoundRunDeploysOnlyItsCommitForward(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tip-moves", "tip-moves-after"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundBed(t)
			b.seed(boundA0)
			if name == "tip-moves-after" {
				b.tips = []string{boundA, boundB}
			}
			report := b.run(RunDeployed)
			b.wantCalls("version "+boundA0, "build "+boundA, "activate "+boundA, "version "+boundA)
			if b.fetches != 1 {
				t.Fatalf("fetches = %d; want 1", b.fetches)
			}
			if got := outcomes(report.Lines); got != "deploy:"+boundA+":active" {
				t.Fatalf("attempts = %s", got)
			}
		})
	}
	for _, name := range []string{"covered", "equal"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundBed(t)
			active := boundB
			if name == "equal" {
				active = boundA
			}
			b.seed(active)
			before := b.files()
			report := b.run(RunCurrent)
			b.wantCalls("version " + active)
			want := ""
			if name == "covered" {
				want = "already covered by " + Short(boundB)
			}
			if report.Reason != want || !reflect.DeepEqual(before, b.files()) || len(report.Lines) != 0 {
				t.Fatalf("current = %+v; record changed: %t", report, !reflect.DeepEqual(before, b.files()))
			}
		})
	}
	t.Run("first", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.run(RunDeployed)
		b.wantCalls("version "+boundA, "build "+boundA, "activate "+boundA, "version "+boundA)
	})
	t.Run("rows", func(t *testing.T) {
		t.Parallel()
		for _, row := range []string{"unmapped", "pending", "run", "side-by-side", "off-main", "nil-contains", "main-ancestry", "covered-ancestry", "after-ancestry"} {
			t.Run(row, func(t *testing.T) {
				t.Parallel()
				b := newBoundBed(t)
				b.seed(boundA0)
				reason := ""
				expectedCalls := 0
				switch row {
				case "unmapped":
					b.active = boundArtifact(boundC)
					reason = "record never saw"
					expectedCalls = 1
				case "pending", "run":
					if err := os.WriteFile(filepath.Join(b.runner.Dir, row+".json"), []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
					reason = row + ".json"
				case "side-by-side":
					b.seed(boundC)
					reason = "not after the active"
					expectedCalls = 1
				case "off-main":
					b.tips = []string{boundC}
					reason = "not on origin's main"
				case "nil-contains":
					b.runner.Git.Contains = nil
					reason = "ancestry can't be read"
				default:
					pair := [2]string{boundA, boundB}
					reason = "ancestry to origin's main"
					if row == "covered-ancestry" {
						pair = [2]string{boundA, boundA0}
						reason = "ancestry to the active"
						expectedCalls = 1
					}
					if row == "after-ancestry" {
						pair = [2]string{boundA0, boundA}
						reason = "ancestry to the active"
						expectedCalls = 1
					}
					b.runner.Git.Contains = func(c, d string) (bool, error) {
						if [2]string{c, d} == pair {
							return false, errors.New("ancestry unreadable")
						}
						return b.contains(c, d)
					}
				}
				before := b.files()
				report := b.run(RunSkipped)
				if !strings.Contains(report.Reason, reason) {
					t.Fatalf("reason = %q; want %q", report.Reason, reason)
				}
				if len(b.calls) != expectedCalls {
					t.Fatalf("calls = %v; want %d version calls only", b.calls, expectedCalls)
				}
				for _, call := range b.calls {
					if !strings.HasPrefix(call, "version ") {
						t.Fatalf("unsafe attempt: %s", call)
					}
				}
				if !reflect.DeepEqual(before, b.files()) {
					t.Fatalf("unsafe run changed records: before %v, after %v", before, b.files())
				}
			})
		}
	})
	t.Run("plain-ignores-run-json", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.runner.Commit = ""
		b.tips = []string{boundB, boundB, boundB}
		if err := os.WriteFile(activePath(b.runner.Dir), []byte("{"), 0600); err != nil {
			t.Fatal(err)
		}
		b.run(RunDeployed)
		b.wantCalls("version "+boundB, "build "+boundB, "activate "+boundB, "version "+boundB)
	})
	t.Run("paused", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		if err := writePause(b.runner.Dir, Pause{By: "Wido"}); err != nil {
			t.Fatal(err)
		}
		b.run(RunPaused)
		b.wantCalls()
		if b.fetches != 0 {
			t.Fatal("paused run fetched")
		}
	})
	t.Run("busy", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.entered = make(chan struct{})
		b.release = make(chan struct{})
		type result struct {
			report Report
			err    error
		}
		done := make(chan result, 1)
		go func() { report, err := b.runner.Run(); done <- result{report, err} }()
		<-b.entered
		second := newBoundBed(t)
		second.runner.Dir = b.runner.Dir
		report := second.run(RunBusy)
		second.wantCalls()
		if second.fetches != 0 || report.Holder == nil || report.Holder.Operation != "build" || report.Holder.Commit != boundA || report.Holder.PID != 999999999 || report.Holder.AdapterAlive() {
			t.Errorf("busy = %+v, fetches %d", report, second.fetches)
		}
		close(b.release)
		finished := <-done
		if finished.err != nil || finished.report.Outcome != RunDeployed || b.fetches != 1 {
			t.Fatalf("holder = %+v, fetches %d", finished, b.fetches)
		}
	})
	t.Run("retry-after-failure", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.failBuild = true
		first := b.run(RunFailed)
		if len(first.Lines) != 1 || first.Lines[0].Outcome != OutcomeBuildFailed {
			t.Fatalf("failed attempt = %+v", first)
		}
		b.failBuild = false
		b.calls = nil
		second := b.run(RunDeployed)
		b.wantCalls("version "+boundA, "build "+boundA, "activate "+boundA, "version "+boundA)
		lines, err := Lines(b.runner.Dir)
		if err != nil || len(lines) != 2 || len(second.Lines) != 1 || lines[1].By != "Wido" {
			t.Fatalf("retry record = %+v, %v", lines, err)
		}
	})
}

func TestBoundLaneHoldsForRecordedFailures(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{OutcomeBuildFailed, OutcomeActivateFailed, OutcomeVerifyFailed, OutcomeVersionFailed, OutcomeStopped} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newBoundBed(t)
			b.runner.Automatic = true
			if err := appendLine(b.runner.Dir, Line{Kind: KindDeploy, Commit: boundA, Outcome: outcome}); err != nil {
				t.Fatal(err)
			}
			report, err := b.runner.Run()
			if err != nil || report.Outcome != RunSkipped || report.Reason != "its earlier attempt ended "+outcome {
				t.Fatalf("recorded failure = %+v, %v", report, err)
			}
			b.wantCalls()
		})
	}
	t.Run("another-commit", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.runner.Automatic = true
		if err := appendLine(b.runner.Dir, Line{Kind: KindDeploy, Commit: boundB, Outcome: OutcomeStopped}); err != nil {
			t.Fatal(err)
		}
		report, err := b.runner.Run()
		if err != nil || report.Outcome != RunDeployed {
			t.Fatalf("other commit failure = %+v, %v", report, err)
		}
		b.wantCalls("version "+boundA, "build "+boundA, "activate "+boundA, "version "+boundA)
	})
	t.Run("unreadable-record", func(t *testing.T) {
		t.Parallel()
		b := newBoundBed(t)
		b.runner.Automatic = true
		if err := os.WriteFile(recordPath(b.runner.Dir), []byte("{\n"), 0600); err != nil {
			t.Fatal(err)
		}
		report, err := b.runner.Run()
		if err != nil || report.Outcome != RunSkipped || !strings.Contains(report.Reason, "not a deploy") {
			t.Fatalf("unreadable record = %+v, %v", report, err)
		}
		b.wantCalls()
	})
}
