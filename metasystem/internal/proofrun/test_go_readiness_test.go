package proofrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const readinessPrefix = "example.invalid/readiness/"

func readinessPartition(packages, names string) goTestPartition {
	p := goTestPartition{Names: strings.Fields(names)}
	for _, pkg := range strings.Fields(packages) {
		p.Packages = append(p.Packages, readinessPrefix+pkg)
	}
	return p
}

type readinessWait struct {
	predecessor int
	pkg         string
}
type readinessResult struct {
	outcome             supervisorOutcome
	closeErr, launchErr error
	output              synchronizedBuffer
}
type readinessRun struct {
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	launched []chan struct{}
	waiting  []chan readinessWait
	log      string
	result   readinessResult
	expected []NativeTestIdentity
}

// A direct partition run owns its candidate until every shard and waiter joins.
func startReadinessRun(t *testing.T, parent context.Context, partitions []goTestPartition, extra map[string]testSnapshotEntry, event func(int, goEvent) bool) *readinessRun {
	t.Helper()
	files := map[string]testSnapshotEntry{"go.mod": testSnapshotFile("module example.invalid/readiness\n\ngo 1.27\n", 0644)}
	var expected []NativeTestIdentity
	seen := map[string]bool{}
	for _, partition := range partitions {
		for _, pkg := range partition.Packages {
			path := strings.TrimPrefix(pkg, readinessPrefix)
			files[path+"/pkg.go"] = testSnapshotFile("package pkg\n", 0644)
			files[path+"/pkg_test.go"] = testSnapshotFile("package pkg\nimport \"testing\"\nfunc TestOne(t *testing.T) { t.Parallel() }\nfunc TestTwo(t *testing.T) { t.Parallel() }\nfunc TestThree(t *testing.T) { t.Parallel() }\n", 0644)
			names := partition.Names
			entry, replacedTests := extra[path+"/pkg_test.go"]
			if len(names) == 0 || replacedTests && !bytes.Contains(entry.data, []byte("func TestOne")) {
				names = []string{goPackageBuildIdentity}
			}
			for _, name := range names {
				if key := pkg + "/" + name; !seen[key] {
					seen[key] = true
					expected = append(expected, NativeTestIdentity{Classname: pkg, Name: name})
				}
			}
		}
	}
	for path, entry := range extra {
		if entry.mode == 0 {
			delete(files, path)
		} else {
			files[path] = entry
		}
	}
	root := t.TempDir()
	snapshot := newTestSnapshotFactory(t, root, strings.Repeat("a", 40), files, 1)
	candidate, err := snapshot.open(root, snapshot.tree)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(withTestWorkerPool(parent, 2))
	run := &readinessRun{ctx: ctx, cancel: cancel, done: make(chan struct{}), log: filepath.Join(root, "group.log"), expected: expected}
	for range partitions {
		run.launched = append(run.launched, make(chan struct{}))
		run.waiting = append(run.waiting, make(chan readinessWait, len(partitions)+1))
	}
	ctx = context.WithValue(ctx, goShardLifecycleHooksKey{}, goShardLifecycleHooks{
		waitReadiness: func(index, predecessor int, pkg string) { run.waiting[index] <- readinessWait{predecessor, pkg} },
		wrapOutput: func(index int, writer io.Writer) io.Writer {
			close(run.launched[index])
			return &readinessEventWriter{Writer: writer, event: func(e goEvent) bool { return event == nil || event(index, e) }}
		},
	})
	t.Cleanup(func() {
		cancel()
		<-run.done
		if err := candidate.Close(); err != nil {
			t.Error(err)
		}
	})
	go func() {
		run.result.outcome, run.result.closeErr, _, _, run.result.launchErr = runGoPartitions(ctx, TestRunRequest{Workers: 2}, testpolicy.Group{ID: "readiness", Adapter: "go"}, candidate.Workspace().Dir,
			append(gittree.ScrubbedEnviron(), "GOFLAGS=-mod=readonly -buildvcs=false"), expected, partitions, supervisorLimits{}, 20*time.Millisecond, run.log, &run.result.output, goCacheFacts{})
		close(run.done)
	}()
	return run
}

type readinessEventWriter struct {
	io.Writer
	mu      sync.Mutex
	pending []byte
	event   func(goEvent) bool
}

func (w *readinessEventWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, data...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			break
		}
		line := w.pending[:end+1]
		var e goEvent
		keep := json.Unmarshal(line, &e) != nil || w.event(e)
		if keep {
			if _, err := w.Writer.Write(line); err != nil {
				return 0, err
			}
			if e.Action == "start" {
				// This observation follows the readiness parser's consumption of start.
				e.Action = "observed-start"
				w.event(e)
			}
		}
		w.pending = w.pending[end+1:]
	}
	return len(data), nil
}

func awaitReadinessSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	testenv.Await(t, "readiness signal", func() bool {
		select {
		case <-signal:
			return true
		case <-ctx.Done():
			t.Fatalf("readiness observation: %v", ctx.Err())
		default:
		}
		return false
	})
}
func (run *readinessRun) gate(t *testing.T, index, predecessor int, pkg string) {
	t.Helper()
	var wait readinessWait
	testenv.Await(t, "unresolved package readiness gate", func() bool {
		select {
		case wait = <-run.waiting[index]:
			return true
		case <-run.launched[index]:
			t.Fatalf("shard %d launched before reaching its readiness gate", index)
		case <-run.done:
			t.Fatalf("run ended before readiness gate: %+v", &run.result)
		case <-run.ctx.Done():
			t.Fatal(run.ctx.Err())
		default:
		}
		pool := run.ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
		if _, waiters := testWorkerPoolState(pool); waiters != 0 {
			t.Fatal("a shard queued for a worker before reaching its readiness gate")
		}
		return false
	})
	if wait.predecessor != predecessor || wait.pkg != readinessPrefix+pkg {
		t.Fatalf("shard %d waited for %+v", index, wait)
	}
}
func (run *readinessRun) blocked(t *testing.T, indexes ...int) {
	t.Helper()
	for _, index := range indexes {
		select {
		case <-run.launched[index]:
			t.Fatalf("shard %d launched before package readiness", index)
		default:
		}
	}
}
func (run *readinessRun) join(t *testing.T) {
	t.Helper()
	awaitReadinessSignal(t, run.ctx, run.done)
	if run.result.launchErr != nil || run.result.closeErr != nil || run.result.outcome.WaitErr != nil {
		t.Fatalf("partition run: %+v close=%v launch=%v\n%s", run.result.outcome, run.result.closeErr, run.result.launchErr, run.result.output.String())
	}
	assertWorkerPoolFullyAvailable(t, run.ctx, 2)
}
func (run *readinessRun) finish(t *testing.T) {
	t.Helper()
	run.join(t)
	_, missing, unexpected, complete := parseGoJSON(run.result.output.Bytes(), run.expected)
	if !complete {
		t.Fatalf("incomplete successful run: missing=%+v unexpected=%+v", missing, unexpected)
	}
}

func holdReadinessEvent(ctx context.Context, entered, release chan struct{}) {
	close(entered)
	select {
	case <-release:
	case <-ctx.Done():
	}
}
func (run *readinessRun) header(t *testing.T, index int, want string) {
	t.Helper()
	line := strings.SplitN(readFileText(fmt.Sprintf("%s.shard-%d.log", strings.TrimSuffix(run.log, ".log"), index+1)), "\n", 2)[0]
	if !strings.Contains(line, want) {
		t.Fatalf("shard %d readiness header=%q want %q", index, line, want)
	}
}

func TestGoPartitionsReadinessChainLeavesWorkersForOtherPackages(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(withTestWorkerPool(t.Context(), 2))
	defer cancel()
	entered := []chan struct{}{make(chan struct{}), make(chan struct{})}
	release := []chan struct{}{make(chan struct{}), make(chan struct{})}
	run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("a", "TestOne"), readinessPartition("a", "TestTwo"), readinessPartition("a", "TestThree")}, nil, func(index int, e goEvent) bool {
		if index < 2 && e.Action == "run" {
			holdReadinessEvent(ctx, entered[index], release[index])
		}
		return true
	})
	run.gate(t, 1, 0, "a")
	testenv.Await(t, "producer held before its first run event", func() bool {
		pool := run.ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
		if available, _ := testWorkerPoolState(pool); available == 0 {
			t.Fatal("readiness wait consumed the spare worker before the producer could run")
		}
		select {
		case <-entered[0]:
			return true
		default:
			return false
		}
	})
	run.blocked(t, 1, 2)
	if pool := run.ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool); pool != nil {
		if available, _ := testWorkerPoolState(pool); available != 1 {
			t.Fatalf("readiness wait held a worker: available=%d", available)
		}
	}
	unrelated := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("b", "TestOne")}, nil, nil)
	awaitReadinessSignal(t, unrelated.ctx, unrelated.launched[0])
	awaitReadinessSignal(t, unrelated.ctx, unrelated.done)
	run.blocked(t, 1, 2)
	close(release[0])
	awaitReadinessSignal(t, run.ctx, entered[1])
	run.gate(t, 2, 1, "a")
	run.blocked(t, 2)
	close(release[1])
	run.finish(t)
	unrelated.finish(t)
	run.header(t, 0, "no wait")
	run.header(t, 1, "shard 1 of "+readinessPrefix+"a to begin running")
	run.header(t, 2, "shard 2 of "+readinessPrefix+"a to begin running")
}

func TestGoPartitionsReadinessResolvesPackagesIndependently(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held, release := make(chan struct{}), make(chan struct{})
	run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("a b", "TestOne"), readinessPartition("a", "TestTwo"), readinessPartition("b", "TestTwo")}, nil, func(index int, e goEvent) bool {
		if index == 0 && e.Package == readinessPrefix+"b" && e.Action == "run" {
			holdReadinessEvent(ctx, held, release)
		}
		return true
	})
	run.gate(t, 1, 0, "a")
	run.gate(t, 2, 0, "b")
	awaitReadinessSignal(t, run.ctx, held)
	awaitReadinessSignal(t, run.ctx, run.launched[1])
	run.blocked(t, 2)
	// Opus read F-1: a partition-wide release would let shard 2 leave its readiness wait and queue for a
	// worker while package b is still held; blocked-at-launch alone cannot tell the two apart on a two-worker pool.
	if _, waiters := testWorkerPoolState(run.ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)); waiters != 0 {
		t.Fatalf("shard 2 left its readiness wait while package b was held: %d worker waiters", waiters)
	}
	close(release)
	run.finish(t)
}

func TestGoPartitionsReadinessWaitsForDistinctPredecessors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held := []chan struct{}{make(chan struct{}), make(chan struct{})}
	release := []chan struct{}{make(chan struct{}), make(chan struct{})}
	run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("a", "TestOne"), readinessPartition("b", "TestOne"), readinessPartition("a b", "TestTwo")}, nil, func(index int, e goEvent) bool {
		if index < 2 && e.Action == "run" {
			holdReadinessEvent(ctx, held[index], release[index])
		}
		return true
	})
	run.gate(t, 2, 0, "a")
	for _, signal := range held {
		awaitReadinessSignal(t, run.ctx, signal)
	}
	run.blocked(t, 2)
	close(release[0])
	run.gate(t, 2, 1, "b")
	run.blocked(t, 2)
	close(release[1])
	run.finish(t)
	run.header(t, 2, "shard 1 of "+readinessPrefix+"a to begin running; shard 2 of "+readinessPrefix+"b to begin running")
}

func TestGoPartitionsReadinessSuccessfulPackageTerminals(t *testing.T) {
	t.Parallel()
	for _, terminal := range []string{"skip", "pass"} {
		t.Run(terminal, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			extra := map[string]testSnapshotEntry{"a/pkg_test.go": {}}
			if terminal == "pass" {
				extra["a/pkg_test.go"] = testSnapshotFile("package pkg\nimport (\"os\";\"testing\")\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\n", 0644)
			}
			ended, held, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("a b", "TestOne"), readinessPartition("a", "")}, extra, func(index int, e goEvent) bool {
				if index == 0 && e.Package == readinessPrefix+"a" && e.Test == "" && e.Action == terminal {
					close(ended)
				}
				if index == 0 && e.Package == readinessPrefix+"b" && e.Action == "run" {
					holdReadinessEvent(ctx, held, release)
				}
				return true
			})
			run.gate(t, 1, 0, "a")
			awaitReadinessSignal(t, run.ctx, ended)
			awaitReadinessSignal(t, run.ctx, held)
			awaitReadinessSignal(t, run.ctx, run.launched[1])
			close(release)
			run.finish(t)
			run.header(t, 1, "to end its package")
		})
	}
}

func TestGoPartitionsReadinessFailureTerminalAndExit(t *testing.T) {
	t.Parallel()
	for _, terminal := range []bool{true, false} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			held, release, after, exit := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
			partitions := []goTestPartition{readinessPartition("a", "TestOne"), readinessPartition("a", "TestTwo")}
			var extra map[string]testSnapshotEntry
			if terminal {
				partitions[0] = readinessPartition("a b", "TestOne")
				extra = map[string]testSnapshotEntry{"a/pkg_test.go": testSnapshotFile("package pkg\nimport (\"os\";\"testing\")\nfunc TestMain(m *testing.M) { os.Exit(3) }\n", 0644)}
			}
			run := startReadinessRun(t, ctx, partitions, extra, func(index int, e goEvent) bool {
				if index != 0 {
					return true
				}
				if e.Package == readinessPrefix+"a" && e.Action == "observed-start" {
					holdReadinessEvent(ctx, held, release)
				}
				if terminal {
					if e.Package == readinessPrefix+"b" && e.Action == "run" {
						holdReadinessEvent(ctx, after, exit)
					}
					return true
				}
				if e.Action == "pass" && e.Test == "" {
					holdReadinessEvent(ctx, after, exit)
				}
				return e.Action == "start" || e.Action == "output" || e.Test != "" && e.Action == "pass"
			})
			run.gate(t, 1, 0, "a")
			awaitReadinessSignal(t, run.ctx, held)
			run.blocked(t, 1)
			close(release)
			awaitReadinessSignal(t, run.ctx, after)
			if terminal {
				awaitReadinessSignal(t, run.ctx, run.launched[1])
				close(exit)
				awaitReadinessSignal(t, run.ctx, run.done)
				if run.result.launchErr != nil || run.result.outcome.WaitErr == nil {
					t.Fatalf("package failure lost: %+v", &run.result)
				}
				assertWorkerPoolFullyAvailable(t, run.ctx, 2)
				run.header(t, 1, "to end its package")
			} else {
				run.blocked(t, 1)
				close(exit)
				run.join(t)
				_, missing, unexpected, complete := parseGoJSON(run.result.output.Bytes(), run.expected)
				if complete || len(missing) != 1 || missing[0].Name != "TestOne" || len(unexpected) != 0 {
					t.Fatalf("exit without a run event hid its missing identity: missing=%+v unexpected=%+v", missing, unexpected)
				}
				run.header(t, 1, "to exit without a test event")
			}
		})
	}
}

func TestGoPartitionsReadinessCompileFailurePreservesEveryShard(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held, release := make(chan struct{}), make(chan struct{})
	run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("broken", "TestOne"), readinessPartition("broken healthy", "TestTwo"), readinessPartition("broken", "TestThree")}, map[string]testSnapshotEntry{"broken/pkg.go": testSnapshotFile("package pkg\nvar _ = undefinedName\n", 0644)}, func(index int, e goEvent) bool {
		if index == 0 && e.Action == "observed-start" {
			holdReadinessEvent(ctx, held, release)
		}
		return true
	})
	run.gate(t, 1, 0, "broken")
	run.gate(t, 2, 1, "broken")
	awaitReadinessSignal(t, run.ctx, held)
	run.blocked(t, 1, 2)
	close(release)
	awaitReadinessSignal(t, run.ctx, run.done)
	if run.result.launchErr != nil || run.result.outcome.WaitErr == nil {
		t.Fatalf("compile failure lost: %+v", &run.result)
	}
	for index := range run.launched {
		data := []byte(readFileText(fmt.Sprintf("%s.shard-%d.log", strings.TrimSuffix(run.log, ".log"), index+1)))
		found := false
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			var e goEvent
			if json.Unmarshal(line, &e) == nil && e.Package == readinessPrefix+"broken" && e.Action == "fail" && e.Test == "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("shard %d skipped its native broken package failure:\n%s", index, data)
		}
	}
	if !strings.Contains(run.result.output.String(), `"Action":"pass","Package":"`+readinessPrefix+`healthy","Test":"TestTwo"`) {
		t.Fatalf("healthy selected test lost:\n%s", run.result.output.String())
	}
	assertWorkerPoolFullyAvailable(t, run.ctx, 2)
}

func TestGoPartitionsReadinessCancellationJoinsWaiters(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held, release := make(chan struct{}), make(chan struct{})
	run := startReadinessRun(t, ctx, []goTestPartition{readinessPartition("a", "TestOne"), readinessPartition("a", "TestTwo")}, nil, func(index int, e goEvent) bool {
		if index == 0 && e.Action == "observed-start" {
			holdReadinessEvent(ctx, held, release)
		}
		return true
	})
	run.gate(t, 1, 0, "a")
	awaitReadinessSignal(t, run.ctx, held)
	run.blocked(t, 1)
	cancel()
	awaitReadinessSignal(t, t.Context(), run.done)
	if !errors.Is(run.result.launchErr, context.Canceled) {
		t.Fatalf("cancellation lost: %v", run.result.launchErr)
	}
	run.blocked(t, 1)
	assertWorkerPoolFullyAvailable(t, run.ctx, 2)
}

func TestGoPartitionsReadinessStreamUsesCompletePackageEvents(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"run", "pass", "skip", "fail"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			a := &goPackageReadiness{done: make(chan struct{}), pkg: "a"}
			b := &goPackageReadiness{done: make(chan struct{}), pkg: "b"}
			writer := &goReadinessWriter{writer: io.Discard, ready: map[string]*goPackageReadiness{"a": a, "b": b}}
			write := func(data []byte) {
				t.Helper()
				if _, err := writer.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			unresolved := func(ready *goPackageReadiness) {
				t.Helper()
				select {
				case <-ready.done:
					t.Fatalf("package %s resolved too early", ready.pkg)
				default:
				}
			}
			for _, event := range []goEvent{{Action: "start", Package: "a"}, {Action: "pass", Package: "a", Test: "TestOne"}, {Action: "output", Package: "a", Output: strings.Repeat("diagnostic", 8192)}} {
				data, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				write(append(data, '\n'))
				unresolved(a)
				unresolved(b)
			}
			data, err := json.Marshal(goEvent{Action: action, Package: "a"})
			if err != nil {
				t.Fatal(err)
			}
			write(data[:len(data)/2])
			unresolved(a)
			write(append(data[len(data)/2:], '\n'))
			select {
			case <-a.done:
			default:
				t.Fatalf("package did not resolve on %s", action)
			}
			unresolved(b)
			want := "end its package"
			if action == "run" {
				want = "begin running"
			}
			if a.reason != want {
				t.Fatalf("resolution=%q want %q", a.reason, want)
			}
		})
	}
}
