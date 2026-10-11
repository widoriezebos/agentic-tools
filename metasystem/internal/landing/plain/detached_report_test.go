package plain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

const detachedReportChild = "PLAIN_TEST_DETACHED_REPORT_CHILD"

type detachedReportDiagnostics struct {
	Kind        string
	Failed      []FailedUnit
	OpenError   string
	RunError    string
	Environment string
	Durations   map[string]int64
	Packages    []PackageTiming
}

func TestDetachedProofReadsItsOwnReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := filepath.Join(dir, "proof.log")
	diagnostics := filepath.Join(dir, "diagnostics.json")
	ready := filepath.Join(dir, "ready")
	if err := os.WriteFile(log, []byte("landing environment earlier\nLANDING-FAILED\told/unit\tTestOld\nLANDING-CHECKED\t1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(ready, 0o600); err != nil {
		t.Fatal(err)
	}
	notification, err := os.OpenFile(ready, os.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer notification.Close()
	pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{
		Argv: []string{os.Args[0], "-test.run", "^TestDetachedProofReportChild$", "-test.timeout=30m"},
		Dir:  dir, Log: log,
		Env: []string{detachedReportChild + "=1", "PLAIN_TEST_REPORT_LOG=" + log,
			"PLAIN_TEST_REPORT_DIAGNOSTICS=" + diagnostics, "PLAIN_TEST_REPORT_READY=" + ready},
	})
	if err != nil {
		t.Fatal(err)
	}
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "detached proof report", Resolve: func() (int, bool, error) { return int(pid), true, nil }}})
	// The detached proof must publish its diagnostics within the launch bound.
	events := []unix.PollFd{{Fd: int32(notification.Fd()), Events: unix.POLLIN}}
	// A signal (a reaped child under load) interrupts the poll; it is simply issued again with its full bound
	// (no wall clock in a test, R-104; a handful of interruptions can only lengthen the wait, never shorten it).
	n, err := unix.Poll(events, 20_000)
	for err == unix.EINTR {
		n, err = unix.Poll(events, 20_000)
	}
	if err != nil || n != 1 || events[0].Revents&unix.POLLIN == 0 {
		data, _ := os.ReadFile(log)
		t.Fatalf("detached proof did not publish diagnostics within 20 seconds: %d %v\n%s", n, err, data)
	}
	data, err := os.ReadFile(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	var got detachedReportDiagnostics
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	wantFailed := []FailedUnit{{Unit: "p/q", Tests: []string{"TestA", "TestB"}}}
	if got.Kind != "complete" || !reflect.DeepEqual(got.Failed, wantFailed) || got.OpenError != "" || got.RunError != "" {
		t.Fatalf("detached proof report: %+v", got)
	}
	if got.Environment != "current" || !reflect.DeepEqual(got.Durations, map[string]int64{"checks": 12}) || !reflect.DeepEqual(got.Packages, []PackageTiming{{Unit: "p/q", Shard: 0, Status: "fail", MS: 12}}) {
		t.Fatalf("detached proof observations: %+v", got)
	}
}

// Signals may interrupt the wait, but cannot extend the publication deadline.
func pollDetachedReport(events []unix.PollFd, timeout time.Duration, now func() time.Time, poll func([]unix.PollFd, int) (int, error)) (int, error) {
	deadline := now().Add(timeout)
	for {
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return 0, nil
		}
		n, err := poll(events, int((remaining+time.Millisecond-1)/time.Millisecond))
		if err != unix.EINTR {
			return n, err
		}
	}
}

func TestDetachedProofReportPollRetriesInterruptedWait(t *testing.T) {
	t.Parallel()
	calls := 0
	current := time.Unix(0, 0)
	now := func() time.Time { return current }
	n, err := pollDetachedReport(nil, time.Second, now, func(_ []unix.PollFd, milliseconds int) (int, error) {
		calls++
		if want := 1000 - (calls-1)*400; milliseconds != want {
			t.Fatalf("poll timeout %d, want remaining %d milliseconds", milliseconds, want)
		}
		if calls == 1 {
			current = current.Add(400 * time.Millisecond)
			return -1, unix.EINTR
		}
		return 1, nil
	})
	if err != nil || n != 1 || calls != 2 {
		t.Fatalf("interrupted wait: n=%d calls=%d err=%v", n, calls, err)
	}
	if n, err := pollDetachedReport(nil, 0, now, func([]unix.PollFd, int) (int, error) {
		t.Fatal("expired deadline polled again")
		return 1, nil
	}); n != 0 || err != nil {
		t.Fatalf("expired wait: n=%d err=%v", n, err)
	}
	calls = 0
	n, err = pollDetachedReport(nil, time.Second, now, func([]unix.PollFd, int) (int, error) {
		calls++
		current = current.Add(time.Second)
		return -1, unix.EINTR
	})
	if n != 0 || err != nil || calls != 1 {
		t.Fatalf("interrupt extended deadline: n=%d calls=%d err=%v", n, calls, err)
	}
}

func TestDetachedProofReportChild(t *testing.T) {
	t.Parallel()
	if os.Getenv(detachedReportChild) != "1" {
		t.Skip("only runs as a detached proof child")
	}
	observed := &proofOutput{output: io.Discard}
	command := "printf 'landing planned 1\nlanding environment current\nlanding group checks failed 12\nlanding package p/q 0 fail 12\nLANDING-FAILED\tp/q\tTestA TestB\nLANDING-CHECKED\t1\n'"
	report, err := runCheck(ProveSeams{}, ".", command, Running{Log: os.Getenv("PLAIN_TEST_REPORT_LOG")}, "p/q", scopeDecision{}, os.Stdout, observed)
	got := detachedReportDiagnostics{Kind: report.kind, Failed: report.failed, Environment: observed.environment, Durations: observed.durations, Packages: observed.packages}
	if err != nil {
		got.RunError = err.Error()
	}
	if report.kind == "" {
		// Diagnose an unreadable inherited output descriptor when no report arrived.
		file, err := os.Open(os.Stdout.Name())
		if err != nil {
			got.OpenError = err.Error()
		} else {
			file.Close()
		}
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("PLAIN_TEST_REPORT_DIAGNOSTICS"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	notification, err := os.OpenFile(os.Getenv("PLAIN_TEST_REPORT_READY"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer notification.Close()
	if _, err := fmt.Fprint(notification, "ready"); err != nil {
		t.Fatal(err)
	}
}
