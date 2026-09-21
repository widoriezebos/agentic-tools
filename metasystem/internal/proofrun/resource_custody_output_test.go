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
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("custody fixture launcher did not drain during cleanup")
		}
	})
	note := "section over-cap passed its 300ms cap"
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	for {
		data, _ := os.ReadFile(publicPath)
		if bytes.Contains(data, []byte(note)) {
			break
		}
		select {
		case result := <-finished:
			t.Fatalf("suite ended before its live cap note: status=%d output=%s", result, data)
		case <-ticker.C:
		case <-deadline.C:
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

type extendingSpoolWriter struct {
	file          *os.File
	keepAppending atomic.Bool
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
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(output.release) })
		_ = os.WriteFile(continuePath, nil, 0o600)
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("gated suite launcher did not settle")
		}
	})
	select {
	case <-output.first:
	case <-time.After(5 * time.Second):
		t.Fatal("suite first output was not observed")
	}
	if err := os.WriteFile(continuePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The launcher writes .done only after the direct child has exited and
	// exec.Cmd.Wait has completed. Keep the first public write gated until then.
	waitCustodyFile(t, logPath+".done", 5*time.Second)
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
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("borrowed custody launcher did not settle")
		}
	})
	waitCustodyFile(t, pidPath, 5*time.Second)
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
	feedback := &extendingSpoolWriter{file: appendFile}
	feedback.keepAppending.Store(true)
	done := make(chan struct{})
	close(done)
	finished := make(chan error, 1)
	go func() {
		finished <- followCustodySpool(path, &lockedWriter{writers: []io.Writer{feedback, log}}, log, done)
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		feedback.keepAppending.Store(false)
		<-finished
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
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("large-output launcher did not settle after exact child stop")
		}
	})
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(complete); err == nil {
			break
		}
		select {
		case result := <-finished:
			t.Fatalf("suite ended before writing all output: status=%d", result)
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("suite stdout pipe stopped draining after public writer failed")
		}
	}
	select {
	case result := <-finished:
		if result == 0 {
			t.Fatal("failed public output produced a green launch")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("large-output launch did not settle after child completion")
	}
	logData, err := os.ReadFile(logPath)
	if err != nil || !bytes.Contains(logData, []byte("noisy-last-line")) {
		t.Fatalf("durable suite log lost the final child line: err=%v bytes=%d", err, len(logData))
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	next, err := AcquireHostResources(ctx, root, conf, "heavy", nil)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	next, err := AcquireHostResources(ctx, root, conf, "heavy", nil)
	if err != nil {
		t.Fatalf("public writer failure left capacity held: %v", err)
	}
	defer next.Close()
	t.Run("prior regular done marker", func(t *testing.T) {
		doneLog := filepath.Join(root, "done-race.log")
		var publicOut, publicErr bytes.Buffer
		status := LaunchSuite(LaunchOptions{
			Suite: "done-race", Root: root, ConfPath: conf,
			ProgressPath: filepath.Join(root, "done-race.progress.jsonl"), LogPath: doneLog,
			Banner: "done race fixture", Silence: 10 * time.Second, SectionCap: 10 * time.Second,
			EvidenceTimeout: 5 * time.Second, EvidenceMax: 1024, Poll: 20 * time.Millisecond,
			TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: engine,
			// A regular marker exists when the launcher publishes completion,
			// forcing the same exclusive-create order as custodian-first settlement.
			Command:           []string{"sh", "-c", `: > "$1"`, "sh", doneLog + ".done"},
			HostResourceFiles: next.Files(), Output: &publicOut, ErrorOutput: &publicErr,
		})
		if status != 0 {
			t.Fatalf("completed managed suite failed after prior regular done marker: status=%d stdout=%s stderr=%s", status, publicOut.String(), publicErr.String())
		}
		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
		released, err := AcquireHostResources(ctx, root, conf, "heavy", nil)
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
