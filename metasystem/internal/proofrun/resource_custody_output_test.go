package proofrun

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestResourceCustodyStreamsLiveCapNoteAndRetainsSpools(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	lease, err := AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "suite.log")
	release := filepath.Join(root, "release")
	publicPath := filepath.Join(root, "public.err")
	public, err := os.Create(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	defer public.Close()
	publicStdoutPath := filepath.Join(root, "public.out")
	publicStdout, err := os.Create(publicStdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer publicStdout.Close()
	command := `printf '{"suite":"chatty","section":"over-cap","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
while [ ! -e "$2" ]; do echo growing; sleep 0.05; done
printf '{"suite":"chatty","section":"over-cap","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	finished := make(chan int, 1)
	ended := make(chan struct{})
	go func() {
		finished <- LaunchSuite(LaunchOptions{Suite: "chatty", Root: root, ConfPath: conf,
			ProgressPath: progress, LogPath: logPath, Banner: "durable watchdog fixture",
			Silence: 2 * time.Second, SectionCap: 300 * time.Millisecond,
			EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
			TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
			WatchdogExecutable: engine, Command: []string{"sh", "-c", command, "sh", progress, release},
			ExpectedSections: []string{"over-cap"}, HostResourceFiles: lease.Files(),
			Output: publicStdout, ErrorOutput: public})
		close(ended)
	}()
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		<-ended
	})
	note := "section over-cap passed its 300ms cap"
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, _ := os.ReadFile(publicPath)
		if bytes.Contains(data, []byte(note)) {
			break
		}
		select {
		case result := <-finished:
			t.Fatalf("suite ended before its live cap note: status=%d output=%s", result, data)
		case <-ticker.C:
		case <-t.Context().Done():
			t.Fatalf("live cap note missing from public output: %s", data)
		}
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if result := <-finished; result != 0 {
		t.Fatalf("released suite status=%d", result)
	}
	publicData, err := os.ReadFile(publicPath)
	if err != nil || strings.Count(string(publicData), note) != 1 {
		t.Fatalf("public cap note count=%d err=%v", strings.Count(string(publicData), note), err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil || strings.Count(string(logData), note) != 1 {
		t.Fatalf("durable log cap note count=%d err=%v", strings.Count(string(logData), note), err)
	}
	stdoutData, err := os.ReadFile(publicStdoutPath)
	if err != nil || !bytes.Contains(stdoutData, []byte("growing")) || bytes.Contains(stdoutData, []byte(note)) {
		t.Fatalf("stdout and watchdog stderr were not separate: err=%v stdout=%s", err, stdoutData)
	}
	run, err := ReadLatestProgressRun(progress)
	if err != nil || len(run.Header.LogPaths) != 5 {
		t.Fatalf("durable spool inventory=%+v err=%v", run.Header, err)
	}
	for _, path := range run.Header.LogPaths[1:] {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("spool %s: info=%v err=%v", path, info, err)
		}
	}
	stderrSpool, err := os.ReadFile(run.Header.LogPaths[2])
	if err != nil || strings.Count(string(stderrSpool), note) != 1 {
		t.Fatalf("custodian stderr spool cap note count=%d err=%v", strings.Count(string(stderrSpool), note), err)
	}
}

type refusingCustodyWriter struct{}

func (refusingCustodyWriter) Write([]byte) (int, error) {
	return 0, errors.New("public output refused")
}

type failAfterBannerWriter struct {
	mu     sync.Mutex
	writes int
}

// extendingSpoolWriter appends to the spool it is fed from, so a follower
// that reads past the final prefix is fed forever. It stops appending once it
// has been handed more than limit bytes, which a correct follower never hands
// it; the follower then ends and the test sees the overrun, with no clock.
type extendingSpoolWriter struct {
	file          *os.File
	keepAppending atomic.Bool
	limit         int64
	received      atomic.Int64
	overran       atomic.Bool
}

type gatedSuiteOutput struct {
	bytes.Buffer
	first   chan struct{}
	release chan struct{}
	once    sync.Once
}

func (writer *gatedSuiteOutput) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("suite-first-part")) {
		writer.once.Do(func() { close(writer.first) })
		<-writer.release
	}
	return writer.Buffer.Write(data)
}

func TestManagedSuiteOutputRetainsTailAfterChildWait(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	lease, err := AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	logPath := filepath.Join(root, "suite.log")
	progress := filepath.Join(root, "progress.jsonl")
	continuePath := filepath.Join(root, "continue")
	output := &gatedSuiteOutput{first: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	finished := make(chan int, 1)
	ended := make(chan struct{})
	go func() {
		finished <- LaunchSuite(LaunchOptions{Suite: "gated-tail", Root: root, ConfPath: conf,
			ProgressPath: progress, LogPath: logPath, Banner: "gated suite output",
			Silence: 3 * time.Second, SectionCap: 3 * time.Second,
			EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
			TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
			WatchdogExecutable: engine,
			Command:            []string{"sh", "-c", `printf 'suite-first-part\n'; while [ ! -e "$1" ]; do sleep 0.01; done; printf 'suite-final-tail\n'`, "sh", continuePath},
			HostResourceFiles:  lease.Files(), Output: output, ErrorOutput: io.Discard})
		close(ended)
	}()
	// Every wait below ends on an event: the barrier, the launcher returning,
	// or the test's own context. None is a wall-clock bound, which a loaded
	// host exceeds with nothing wrong.
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(output.release) })
		_ = os.WriteFile(continuePath, nil, 0o600)
		<-ended
	})
	select {
	case <-output.first:
	case <-ended:
		t.Fatal("gated suite launcher returned before its first output was observed")
	case <-t.Context().Done():
		t.Fatal("suite first output was not observed")
	}
	if err := os.WriteFile(continuePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The launcher writes .done only after the direct child has exited and
	// exec.Cmd.Wait has completed. Keep the first public write gated until then.
	waitCustodyFileWhile(t, logPath+".done", 0, ended, func() error { return errors.New("the gated suite launcher returned") })
	releaseOnce.Do(func() { close(output.release) })
	if status := <-finished; status != 0 {
		t.Fatalf("gated suite status=%d", status)
	}
	if !strings.Contains(output.String(), "suite-final-tail\n") {
		t.Fatalf("public output lost gated child tail: %q", output.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !bytes.Contains(data, []byte("suite-final-tail\n")) {
		t.Fatalf("durable log lost gated child tail: err=%v", err)
	}
}

func TestCustodiedSuiteWithoutResourceFilesDrainsGrandchildWithoutSlot(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	pidPath := filepath.Join(root, "grandchild.pid")
	release := filepath.Join(root, "release")
	logPath := filepath.Join(root, "suite.log")
	progress := filepath.Join(root, "progress.jsonl")
	var output bytes.Buffer
	finished := make(chan int, 1)
	ended := make(chan struct{})
	go func() {
		finished <- LaunchSuite(LaunchOptions{Suite: "borrowed-without-files", Root: root, ConfPath: conf,
			ProgressPath: progress, LogPath: logPath, Banner: "legacy borrowed custody",
			Silence: 3 * time.Second, SectionCap: 3 * time.Second,
			EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
			TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
			WatchdogExecutable: engine, RequireCustody: true,
			Command: []string{"sh", "-c", `sleep 60 </dev/null >/dev/null 2>&1 & printf '%s\n' "$!" >"$1"; while [ ! -e "$2" ]; do sleep 0.01; done; printf 'borrowed-suite-tail\n'`, "sh", pidPath, release},
			Output:  &output, ErrorOutput: io.Discard})
		close(ended)
	}()
	var grandchild identity.Ref
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		if grandchild.Pid > 0 {
			_ = identity.SignalExact(identity.KernelProber{}, grandchild, syscall.SIGKILL)
		}
		<-ended
	})
	// The shell creates the pid file before it writes the line; wait for the line.
	waitCustodyBarrier(t, pidPath, 0, ended, func() error { return errors.New("the borrowed custody launcher returned") },
		func() bool { return completeCustodyRecord(pidPath) })
	pidText, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(string(pidText)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("grandchild was not live during no-file custody: state=%s err=%v", state, err)
	}
	grandchild = exact.Ref()
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status := <-finished; status == 0 {
		t.Fatal("ordinary grandchild was silently accepted as a clean launch")
	}
	if state := identity.AliveRef(identity.KernelProber{}, grandchild); state == identity.Alive {
		t.Fatal("no-file custodian left its ordinary grandchild live")
	}
	if !strings.Contains(output.String(), "borrowed-suite-tail") {
		t.Fatalf("no-file suite output lost final line: %q", output.String())
	}
	run, err := ReadLatestProgressRun(progress)
	if err != nil || len(run.Header.LogPaths) != 5 {
		t.Fatalf("no-file custody did not retain both spool pairs: paths=%v err=%v", run.Header.LogPaths, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "lease-heavy-") {
			t.Fatalf("no-file custody acquired an extra host slot: %s", entry.Name())
		}
	}
	stopProgress := filepath.Join(root, "stopped.progress.jsonl")
	stopLog := filepath.Join(root, "stopped.log")
	reads := 0
	stopStatus := LaunchSuite(LaunchOptions{Suite: "borrowed-stopped-after-start", Root: root, ConfPath: conf,
		ProgressPath: stopProgress, LogPath: stopLog, Banner: "stopped borrowed custody",
		Silence: 3 * time.Second, SectionCap: 3 * time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
		TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
		WatchdogExecutable: engine, RequireCustody: true,
		Command: []string{"sh", "-c", `printf 'started-before-stop\n'; while :; do sleep 0.05; done`},
		Output:  io.Discard, ErrorOutput: io.Discard,
		FenceReader: func(string) (stopfence.Record, error) {
			reads++
			if reads == 1 {
				return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 7}, nil
			}
			return stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 8}, nil
		},
		ClaimCreator: func(string, string, int64, identity.Ref) (CreationClaim, error) {
			return &testCreationClaim{}, nil
		},
	})
	if stopStatus != 1 || reads != 2 {
		t.Fatalf("after-start stop status=%d fenceReads=%d", stopStatus, reads)
	}
	stopped, err := ReadRecord(root, "borrowed-stopped-after-start")
	if err != nil || stopped.Status != StatusDone || identity.AliveRef(identity.KernelProber{}, stopped.SuiteProcess.Ref()) == identity.Alive {
		t.Fatalf("after-start refusal left a live child: record=%+v err=%v", stopped, err)
	}
	stoppedRun, err := ReadLatestProgressRun(stopProgress)
	if err != nil || len(stoppedRun.Header.LogPaths) != 5 {
		t.Fatalf("after-start refusal did not settle durable streams: paths=%v err=%v", stoppedRun.Header.LogPaths, err)
	}
	for _, path := range stoppedRun.Header.LogPaths[1:] {
		if _, err := os.ReadFile(path); err != nil {
			t.Fatalf("after-start spool %s is unreadable: %v", path, err)
		}
	}
}

func (writer *extendingSpoolWriter) Write(data []byte) (int, error) {
	if writer.limit > 0 && writer.received.Add(int64(len(data))) > writer.limit {
		writer.overran.Store(true)
		writer.keepAppending.Store(false)
	}
	if writer.keepAppending.Load() {
		if _, err := writer.file.Write(bytes.Repeat([]byte("x"), 2*len(data))); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func TestSuiteSpoolFinalPrefixStopsSelfExtendingWriter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "suite.stdout")
	initial := bytes.Repeat([]byte("a"), 32*1024)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	appendFile, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer appendFile.Close()
	log, err := os.Create(filepath.Join(root, "suite.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	feedback := &extendingSpoolWriter{file: appendFile, limit: int64(len(initial))}
	feedback.keepAppending.Store(true)
	done := make(chan struct{})
	close(done)
	if err := followCustodySpool(path, &lockedWriter{writers: []io.Writer{feedback, log}}, log, done); err != nil {
		t.Fatal(err)
	}
	if feedback.overran.Load() {
		t.Fatal("completed suite output followed an unbounded descendant append")
	}
	content, err := os.ReadFile(log.Name())
	if err != nil || !bytes.Equal(content, initial) {
		t.Fatalf("frozen final prefix differed: bytes=%d err=%v", len(content), err)
	}
}

func (writer *failAfterBannerWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.writes++
	if writer.writes > 1 {
		return 0, errors.New("public output refused after banner")
	}
	return len(data), nil
}

func TestResourceCustodyFailedPublicSuiteOutputStillDrainsLargeChild(t *testing.T) {
	// LaunchSuite deliberately waits for the suite before joining pipe readers.
	// Cmd.Wait closes StdoutPipe, which is an expected reader end condition.
	commandWait := exec.Command("sh", "-c", "true")
	closedPipe, err := commandWait.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := commandWait.Start(); err != nil {
		t.Fatal(err)
	}
	if err := commandWait.Wait(); err != nil {
		t.Fatal(err)
	}
	closedPipeOutput := &lockedWriter{writers: []io.Writer{io.Discard}}
	var closedPipeCopy sync.WaitGroup
	closedPipeCopy.Add(1)
	copyStream(&closedPipeCopy, closedPipeOutput, closedPipe, true)
	closedPipeCopy.Wait()
	if err := closedPipeOutput.Err(); err != nil {
		t.Fatalf("successful Cmd.Wait pipe closure became an output failure: %v", err)
	}

	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	lease, err := AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "suite.log")
	complete := filepath.Join(root, "child.complete")
	command := `awk 'BEGIN { for (i=0; i<200000; i++) print "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"; print "noisy-last-line" }'
: >"$1"`
	finished := make(chan int, 1)
	ended := make(chan struct{})
	go func() {
		finished <- LaunchSuite(LaunchOptions{Suite: "noisy", Root: root, ConfPath: conf,
			ProgressPath: progress, LogPath: logPath, Banner: "large stdout fixture",
			Silence: 3 * time.Second, SectionCap: 3 * time.Second,
			EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
			TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
			WatchdogExecutable: engine, Command: []string{"sh", "-c", command, "sh", complete},
			HostResourceFiles: lease.Files(), Output: &failAfterBannerWriter{}, ErrorOutput: io.Discard})
		close(ended)
	}()
	stopSuite := func() {
		if record, err := ReadRecord(root, "noisy"); err == nil {
			_ = identity.SignalExact(identity.KernelProber{}, record.SuiteProcess.Ref(), syscall.SIGKILL)
		}
	}
	t.Cleanup(func() {
		stopSuite()
		<-ended
	})
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	childComplete := false
	var result int

waitForResult:
	for {
		if !childComplete {
			_, err := os.Stat(complete)
			childComplete = err == nil
		}
		var poll <-chan time.Time
		if !childComplete {
			poll = ticker.C
		}
		select {
		case result = <-finished:
			if !childComplete {
				if _, err := os.Stat(complete); err != nil {
					t.Fatalf("suite ended before writing all output: status=%d marker=%v", result, err)
				}
			}
			childComplete = true
			break waitForResult
		case <-poll:
		case <-t.Context().Done():
			if childComplete {
				t.Fatal("large-output launch did not settle after child completion")
			}
			t.Fatal("suite stdout pipe stopped draining after public writer failed")
		}
	}
	if result == 0 {
		t.Fatal("failed public output produced a green launch")
	}
	logData, err := os.ReadFile(logPath)
	if err != nil || !bytes.Contains(logData, []byte("noisy-last-line")) {
		t.Fatalf("durable suite log lost the final child line: err=%v bytes=%d", err, len(logData))
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireHostResources(t.Context(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatalf("large-output failure left host capacity held: %v", err)
	}
	defer next.Close()
}

func TestResourceCustodySpoolFinalDrainAndFailedPublicWriter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	spool := filepath.Join(root, "watchdog.stderr")
	content := strings.Repeat("watchdog line\n", 3000) + "unterminated final verdict"
	if err := os.WriteFile(spool, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	close(done)
	for _, failing := range []bool{false, true} {
		log, err := os.Create(filepath.Join(root, "suite-"+map[bool]string{false: "good", true: "failed"}[failing]+".log"))
		if err != nil {
			t.Fatal(err)
		}
		var public bytes.Buffer
		var output io.Writer = &public
		if failing {
			output = refusingCustodyWriter{}
		}
		err = followCustodySpool(spool, &lockedWriter{writers: []io.Writer{output, log}}, log, done)
		if closeErr := log.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if failing != (err != nil) {
			t.Fatalf("failing=%t follower error=%v", failing, err)
		}
		logData, readErr := os.ReadFile(log.Name())
		if readErr != nil || string(logData) != content {
			t.Fatalf("failing=%t final spool log bytes=%d err=%v", failing, len(logData), readErr)
		}
		if !failing && public.String() != content {
			t.Fatalf("public final drain bytes=%d", public.Len())
		}
	}
}

func TestResourceCustodyPublicWriterFailureStillDrainsAndReleasesCapacity(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	root, conf := isolatedHostResources(t)
	lease, err := AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "suite.log")
	command := `printf '{"suite":"writer-failure","section":"over-cap","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
sleep 1
printf '{"suite":"writer-failure","section":"over-cap","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	status := LaunchSuite(LaunchOptions{Suite: "writer-failure", Root: root, ConfPath: conf,
		ProgressPath: progress, LogPath: logPath, Banner: "failed public writer fixture",
		Silence: 2 * time.Second, SectionCap: 100 * time.Millisecond,
		EvidenceTimeout: time.Second, EvidenceMax: 1024 * 1024, Poll: 25 * time.Millisecond,
		TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
		WatchdogExecutable: engine, Command: []string{"sh", "-c", command, "sh", progress},
		ExpectedSections: []string{"over-cap"}, HostResourceFiles: lease.Files(),
		Output: io.Discard, ErrorOutput: refusingCustodyWriter{}})
	logData, err := os.ReadFile(logPath)
	if status == 0 || err != nil || !strings.Contains(string(logData), "section over-cap passed its 100ms cap") ||
		!strings.Contains(string(logData), "public output refused") {
		t.Fatalf("failed public sink status=%d logErr=%v log=%s", status, err, logData)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireHostResources(t.Context(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatalf("public writer failure left capacity held: %v", err)
	}
	defer next.Close()
	t.Run("prior regular done marker", func(t *testing.T) {
		doneLog := filepath.Join(root, "done-race.log")
		// A regular marker exists when the launcher publishes completion,
		// forcing the same exclusive-create order as custodian-first
		// settlement. It is placed after the suite ended, as the custodian
		// places it: a marker the suite wrote itself told the custodian the
		// suite was done while that shell still lived, and the custodian's
		// drain killed it ("native descendants survived direct worker
		// completion") whenever the shell was descheduled for a tick.
		previous := beforeLauncherDone
		t.Cleanup(func() { beforeLauncherDone = previous })
		beforeLauncherDone = func(path string) {
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
			if err != nil {
				t.Errorf("place the prior done marker: %v", err)
				return
			}
			_ = file.Close()
		}
		var publicOut, publicErr bytes.Buffer
		status := LaunchSuite(LaunchOptions{
			Suite: "done-race", Root: root, ConfPath: conf,
			ProgressPath: filepath.Join(root, "done-race.progress.jsonl"), LogPath: doneLog,
			Banner: "done race fixture", Silence: 10 * time.Second, SectionCap: 10 * time.Second,
			EvidenceTimeout: 5 * time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
			TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: engine,
			Command:           []string{"sh", "-c", ":"},
			HostResourceFiles: next.Files(), Output: &publicOut, ErrorOutput: &publicErr,
		})
		if status != 0 {
			t.Fatalf("completed managed suite failed after prior regular done marker: status=%d stdout=%s stderr=%s", status, publicOut.String(), publicErr.String())
		}
		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
		released, err := AcquireHostResources(t.Context(), root, conf, "heavy", nil)
		if err != nil {
			t.Fatalf("managed suite retained capacity: %v", err)
		}
		defer released.Close()
		for _, kind := range []string{"directory", "symlink"} {
			path := filepath.Join(root, "invalid-"+kind+".done")
			switch kind {
			case "directory":
				err = os.Mkdir(path, 0o700)
			case "symlink":
				err = os.Symlink(doneLog, path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := touchDone(path); !errors.Is(err, os.ErrExist) {
				t.Fatalf("%s done collision accepted: %v", kind, err)
			}
		}
	})
}
