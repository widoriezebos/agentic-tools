package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakePathInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (info fakePathInfo) Name() string       { return info.name }
func (info fakePathInfo) Size() int64        { return info.size }
func (info fakePathInfo) Mode() fs.FileMode  { return info.mode }
func (info fakePathInfo) ModTime() time.Time { return info.modTime }
func (info fakePathInfo) IsDir() bool        { return info.mode.IsDir() }
func (info fakePathInfo) Sys() any           { return nil }

type pathWaitClock struct {
	now    time.Time
	boot   time.Duration
	sleeps []time.Duration
}

func newPathWaitClock() *pathWaitClock {
	return &pathWaitClock{now: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC), boot: time.Hour}
}

func (clock *pathWaitClock) options(stat PathStat) WaitOptions {
	return WaitOptions{
		Now:       func() time.Time { return clock.now },
		BootClock: func() (string, time.Duration, error) { return "boot-path", clock.boot, nil },
		Sleep: func(_ context.Context, duration time.Duration) error {
			clock.sleeps = append(clock.sleeps, duration)
			clock.now = clock.now.Add(duration)
			clock.boot += duration
			return nil
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: func(ctx context.Context, selector WaitSelector, _ WaiterTarget, _ string) (SourceObservation, error) {
			return ObservePath(ctx, selector, stat)
		},
		EmitEvent: func(string, string, string, map[string]string) error { return nil },
	}
}

func pathWaitRequest(path, until string, timeout time.Duration) WaitRequest {
	selector := WaitSelector{Kind: "path", TargetID: PathWaitTargetID(path), Path: path, Until: until}
	return WaitRequest{Selector: selector, Owner: mainCaller, RuntimeSession: mainCaller.SessionId, Timeout: timeout}
}

func TestWaitPathAbsentEndsWhenThePathGoesAway(t *testing.T) {
	t.Run("removal ends the wait", func(t *testing.T) {
		path := "/tmp/metasystem-path-absent"
		clock := newPathWaitClock()
		observations := 0
		stat := func(string) (os.FileInfo, error) {
			observations++
			if observations < 3 {
				return fakePathInfo{name: filepath.Base(path), size: 12, mode: 0o600, modTime: clock.now}, nil
			}
			return nil, fs.ErrNotExist
		}
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
			context.Background(), pathWaitRequest(path, "absent", time.Minute), clock.options(stat),
		)
		if result.ExitCode != ExitGreen || result.SourceOutcome != "absent" || result.SourceEvidence != path+":absent" {
			t.Fatalf("removed path result = %+v", result)
		}
		if observations != 3 || len(clock.sleeps) != 1 {
			t.Fatalf("observations=%d sleeps=%v", observations, clock.sleeps)
		}
	})

	t.Run("stat errors remain pending", func(t *testing.T) {
		path := "/tmp/metasystem-path-permission"
		clock := newPathWaitClock()
		statErr := errors.New("permission denied by fixture")
		observations := 0
		stat := func(string) (os.FileInfo, error) {
			observations++
			return nil, statErr
		}
		root := t.TempDir()
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(
			context.Background(), pathWaitRequest(path, "absent", 2*time.Second), clock.options(stat),
		)
		row, _, rowErr := LoadWaiterByID(root, result.WaitID)
		if result.ExitCode != ExitWaitDeadline || result.SourceOutcome != "wait-deadline" {
			t.Fatalf("stat error ended the wait = %+v", result)
		}
		if rowErr != nil || !strings.Contains(row.LastPollError, statErr.Error()) || row.LastPollAt == "" {
			t.Fatalf("stat error was not retained: row=%+v err=%v", row, rowErr)
		}
		if observations != 3 {
			t.Fatalf("stat-error observations = %d, want initial, loop, and final observations", observations)
		}
	})
}

func TestWaitPathPresentEndsWhenTheFileAppears(t *testing.T) {
	path := "/tmp/metasystem-path-present"
	modTime := time.Date(2026, 9, 17, 9, 59, 0, 123, time.UTC)
	t.Run("empty regular file remains pending", func(t *testing.T) {
		clock := newPathWaitClock()
		observations := 0
		stat := func(string) (os.FileInfo, error) {
			observations++
			switch observations {
			case 1:
				return nil, fs.ErrNotExist
			case 2:
				return fakePathInfo{name: filepath.Base(path), mode: 0o600, modTime: modTime}, nil
			default:
				return fakePathInfo{name: filepath.Base(path), size: 7, mode: 0o600, modTime: modTime}, nil
			}
		}
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
			context.Background(), pathWaitRequest(path, "present", time.Minute), clock.options(stat),
		)
		wantEvidence := path + ":size=7:mtime=" + modTime.Format(time.RFC3339Nano)
		if result.ExitCode != ExitGreen || result.SourceOutcome != "present" || result.SourceEvidence != wantEvidence {
			t.Fatalf("non-empty path result = %+v", result)
		}
		if observations != 3 || len(clock.sleeps) != 1 {
			t.Fatalf("empty file did not remain pending: observations=%d sleeps=%v", observations, clock.sleeps)
		}
	})

	t.Run("directory is present", func(t *testing.T) {
		clock := newPathWaitClock()
		options := clock.options(func(string) (os.FileInfo, error) {
			return fakePathInfo{name: filepath.Base(path), size: 96, mode: fs.ModeDir | 0o700, modTime: modTime}, nil
		})
		options.Sleep = func(context.Context, time.Duration) error {
			t.Fatal("a present directory waited")
			return nil
		}
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
			context.Background(), pathWaitRequest(path, "present", time.Minute), options,
		)
		if result.ExitCode != ExitGreen || result.SourceOutcome != "present" {
			t.Fatalf("directory result = %+v", result)
		}
	})
}

func TestWaitPathEndsAtItsDeadline(t *testing.T) {
	path := "/tmp/metasystem-path-deadline"
	clock := newPathWaitClock()
	observations := 0
	options := clock.options(func(string) (os.FileInfo, error) {
		observations++
		return nil, fs.ErrNotExist
	})
	result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
		context.Background(), pathWaitRequest(path, "present", 3*time.Second), options,
	)
	if result.ExitCode != ExitWaitDeadline || result.SourceOutcome != "wait-deadline" || result.Reason != "this wait reached its deadline" {
		t.Fatalf("deadline result = %+v", result)
	}
	if len(clock.sleeps) != 1 || clock.sleeps[0] != 3*time.Second {
		t.Fatalf("deadline sleeps = %v", clock.sleeps)
	}
	if observations != 3 {
		t.Fatalf("deadline observations = %d, want initial, loop, and final observations", observations)
	}
}

func TestWaitPathSeesAChangeDuringItsFinalSleep(t *testing.T) {
	modTime := time.Date(2026, 9, 17, 9, 59, 0, 123, time.UTC)
	for _, until := range []string{"present", "absent"} {
		t.Run(until, func(t *testing.T) {
			path := "/tmp/metasystem-path-final-" + until
			clock := newPathWaitClock()
			ready := false
			observations := 0
			options := clock.options(func(string) (os.FileInfo, error) {
				observations++
				if ready == (until == "present") {
					return fakePathInfo{name: filepath.Base(path), size: 7, mode: 0o600, modTime: modTime}, nil
				}
				return nil, fs.ErrNotExist
			})
			options.Sleep = func(_ context.Context, duration time.Duration) error {
				clock.sleeps = append(clock.sleeps, duration)
				clock.now = clock.now.Add(duration)
				clock.boot += duration
				ready = true
				return nil
			}

			result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
				context.Background(), pathWaitRequest(path, until, 3*time.Second), options,
			)
			wantEvidence := path + ":absent"
			if until == "present" {
				wantEvidence = path + ":size=7:mtime=" + modTime.Format(time.RFC3339Nano)
			}
			if result.ExitCode != ExitGreen || result.SourceOutcome != until || result.SourceEvidence != wantEvidence {
				t.Fatalf("final observation result = %+v", result)
			}
			if observations != 3 || len(clock.sleeps) != 1 || clock.sleeps[0] != 3*time.Second {
				t.Fatalf("observations=%d sleeps=%v", observations, clock.sleeps)
			}
		})
	}
}

func TestWaitPathFinalObservationIsOnlyForPathWaits(t *testing.T) {
	clock := newPathWaitClock()
	observations := 0
	options := clock.options(nil)
	options.Observe = func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
		observations++
		return SourceObservation{
			Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"},
			Outcome: "running", Evidence: "run:r:g1",
		}, nil
	}
	result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
		context.Background(),
		WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: 3 * time.Second},
		options,
	)
	if result.ExitCode != ExitWaitDeadline || observations != 2 {
		t.Fatalf("result=%+v observations=%d, want no observation at the deadline", result, observations)
	}
}

func TestWaitPathWaitsOnlyThroughTheInjectedClock(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "waiter_path.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{"Sleep": true, "After": true, "NewTimer": true, "NewTicker": true}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !forbidden[selector.Sel.Name] {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		if ok && owner.Name == "time" {
			t.Errorf("path source calls forbidden wall-time primitive time.%s", selector.Sel.Name)
		}
		return true
	})

	path := "/tmp/metasystem-path-clock"
	clock := newPathWaitClock()
	observations := 0
	stat := func(string) (os.FileInfo, error) {
		observations++
		if observations < 4 {
			return nil, fs.ErrNotExist
		}
		return fakePathInfo{name: filepath.Base(path), size: 1, mode: 0o600, modTime: clock.now}, nil
	}
	observationStarted := make(chan struct{})
	releaseObservation := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(releaseObservation)
		}
	}()
	options := clock.options(stat)
	first := true
	options.Observe = func(_ context.Context, selector WaitSelector, _ WaiterTarget, _ string) (SourceObservation, error) {
		if first {
			first = false
			close(observationStarted)
			<-releaseObservation
		}
		// The waiter owns its production operation deadline. This controlled
		// semantic-clock proof uses only the test's outer termination context.
		return ObservePath(t.Context(), selector, stat)
	}
	root := t.TempDir()
	resultReady := make(chan WaitResult, 1)
	go func() {
		resultReady <- (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(
			t.Context(), pathWaitRequest(path, "present", time.Minute), options,
		)
	}()
	<-observationStarted
	select {
	case result := <-resultReady:
		t.Fatalf("path wait returned before delayed observation release: %+v", result)
	default:
	}
	close(releaseObservation)
	released = true
	result := <-resultReady
	if result.ExitCode != ExitGreen || observations != 4 {
		t.Fatalf("fake-clock result=%+v observations=%d", result, observations)
	}
	if len(clock.sleeps) != 2 || clock.sleeps[0] != 10*time.Second || clock.sleeps[1] != 10*time.Second {
		t.Fatalf("unrecorded or unexpected pauses: %v", clock.sleeps)
	}
}

func TestWaitPathRecordFieldsAreOptional(t *testing.T) {
	root := t.TempDir()
	rowPath := filepath.Join(root, "legacy.json")
	legacy := `{"schemaVersion":2,"waitId":"0123456789abcdef0123456789abcdef","nonce":"fedcba9876543210fedcba9876543210","kind":"run","targetId":"run-a","selector":{"kind":"run","targetId":"run-a"},"state":"pending"}`
	if err := os.WriteFile(rowPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	row, err := readV2Waiter(rowPath)
	if err != nil || row.Selector.Path != "" || row.Selector.Until != "" {
		t.Fatalf("legacy row=%+v err=%v", row, err)
	}
	encoded, err := json.Marshal(row.Selector)
	if err != nil || strings.Contains(string(encoded), `"path"`) || strings.Contains(string(encoded), `"until"`) {
		t.Fatalf("empty path fields were serialized: %s err=%v", encoded, err)
	}
	path := "/tmp/metasystem-clean-target"
	digest := sha256.Sum256([]byte(filepath.Clean(path)))
	want := "path-" + hex.EncodeToString(digest[:])[:16]
	if got := PathWaitTargetID(path); got != want {
		t.Fatalf("target id=%q want=%q", got, want)
	}
	owner := Caller{Class: "MAIN", MainId: "main-legacy", OwnerLineage: "lineage-legacy", SessionId: "session-legacy"}
	row.OwnerDigest = OwnerDigest(owner.MainId)
	row.MainId, row.OwnerLineage, row.Session, row.RuntimeSession = owner.MainId, owner.OwnerLineage, owner.SessionId, owner.SessionId
	row.Target = WaiterTarget{Generation: 1, LaunchNonce: "legacy-nonce"}
	row.Result = &WaitResult{
		SchemaVersion: 2, WaitID: row.WaitID, Selector: row.Selector, TargetIncarnation: row.Target,
		ExitCode: ExitGreen, SourceOutcome: "green", SourceEvidence: "run:run-a:g1:legacy-nonce",
	}
	storedPath := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
	if err := writeV2Waiter(storedPath, row); err != nil {
		t.Fatal(err)
	}
	if err := writeV2Pointer(root, row, storedPath); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	result := (&Store{Root: root, Prober: waitTestProber{live: true}}).ResumeWait(context.Background(), row.WaitID, owner, owner.SessionId, 0, WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "boot-legacy", time.Hour, nil },
		Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			return SourceObservation{Incarnation: row.Target, ExitCode: ExitGreen, Outcome: "green", Evidence: row.Result.SourceEvidence}, nil
		},
		EmitEvent: func(string, string, string, map[string]string) error { return nil },
	})
	if result.ExitCode != ExitGreen || result.Mode != "replay" {
		t.Fatalf("legacy record resume = %+v", result)
	}
}

func TestWaitPathResumes(t *testing.T) {
	root := t.TempDir()
	path := "/tmp/metasystem-path-resume"
	selector := WaitSelector{Kind: "path", TargetID: PathWaitTargetID(path), Path: path, Until: "present"}
	owner := Caller{Class: "MAIN", MainId: "main-path-resume", OwnerLineage: "lineage-path-resume", SessionId: "session-path-resume"}
	waitID := "0123456789abcdef0123456789abcdef"
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	boot := time.Hour
	row := Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: "fedcba9876543210fedcba9876543210", Kind: selector.Kind, TargetID: selector.TargetID,
		OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-path",
		Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
		Selector: selector, RegisteredAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(time.Minute).Format(time.RFC3339Nano),
		DeadlineBootID: "boot-path", BootDeadlineNanos: (boot + time.Minute).Nanoseconds(), RemainingNanos: time.Minute.Nanoseconds(), State: "pending", Delivery: "blocking",
	}
	rowPath := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
	if err := writeV2Waiter(rowPath, row); err != nil {
		t.Fatal(err)
	}
	if err := writeV2Pointer(root, row, rowPath); err != nil {
		t.Fatal(err)
	}
	clock := &pathWaitClock{now: now, boot: boot}
	options := clock.options(func(string) (os.FileInfo, error) {
		return fakePathInfo{name: filepath.Base(path), size: 4, mode: 0o600, modTime: now}, nil
	})
	options.Sleep = func(context.Context, time.Duration) error {
		t.Fatal("resumed ready path waited")
		return nil
	}
	result := (&Store{Root: root, Prober: restartProber{deadPID: row.Pid}}).ResumeWait(
		context.Background(), waitID, owner, owner.SessionId, 0, options,
	)
	stored, _, storedErr := LoadWaiterByID(root, waitID)
	if result.ExitCode != ExitGreen || result.Selector != selector || storedErr != nil || stored.ResumedBy == nil || stored.State != "ready" {
		t.Fatalf("resume result=%+v row=%+v err=%v", result, stored, storedErr)
	}
}
