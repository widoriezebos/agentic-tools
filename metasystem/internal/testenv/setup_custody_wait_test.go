package testenv

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

type setupCustodyEvidence struct {
	registry                string
	owner, custodian, child identity.Ref
	prober                  identity.Prober
}

// wait reports a stalled custody event once, while preserving the test's
// deadline reserve. Without a binary deadline it gives up after ten minutes.
func (e setupCustodyEvidence) wait(t interface {
	AwaitTB
	Name() string
}, what string, observed func() bool, pause func(), remaining func() (time.Duration, bool), now func() time.Time, diagnostic io.Writer) {
	t.Helper()
	if now == nil {
		now = time.Now
	}
	if diagnostic == nil {
		diagnostic = os.Stderr
	}
	started, emitted := now(), false
	for !observed() {
		elapsed := now().Sub(started)
		if !emitted && elapsed >= 2*time.Minute {
			fmt.Fprintf(diagnostic, "testenv: still waiting test=%s pending=%s; %s\n", t.Name(), what, e.describe())
			emitted = true
		}
		left, bounded := remaining()
		if bounded && left <= awaitReserve {
			t.Fatalf("still awaiting %s with %s left before the test binary's deadline; %s", what, left.Round(time.Millisecond), e.describe())
			return
		}
		if !bounded && elapsed >= 10*time.Minute {
			t.Fatalf("still awaiting %s after %s without a test binary deadline; %s", what, elapsed, e.describe())
			return
		}
		pause()
	}
}

type setupCustodyRecorder struct {
	awaitRecorder
	name string
}

func (r *setupCustodyRecorder) Name() string { return r.name }

func (e setupCustodyEvidence) describe() string {
	var report, pending bytes.Buffer
	settled := homeSettled(e.registry, &pending)
	fmt.Fprintf(&report, "registry=%s owner=%+v custodian=%+v child=%+v settled=%t condition=%s\n",
		e.registry, e.owner, e.custodian, e.child, settled, strings.TrimSpace(pending.String()))
	for _, process := range []struct {
		name string
		ref  identity.Ref
	}{{"child", e.child}, {"custodian", e.custodian}} {
		exact, state, err := e.prober.Probe(process.ref.Pid)
		fmt.Fprintf(&report, "%s pid=%d state=%s same-identity=%t zombie=%t probe=%v\n",
			process.name, process.ref.Pid, state, identity.SameIdentity(exact, process.ref), exact.Zombie, err)
	}
	paths := []string{e.registry + ".custodian-" + strconv.FormatInt(e.owner.Pid, 10) + ".log",
		e.registry + ".fixture-refs-" + strconv.FormatInt(e.owner.Pid, 10)}
	// An inherited helper may have registered another custodian in this home.
	// Include its files too when they, rather than the primary pair, are pending.
	entries, err := os.ReadDir(filepath.Dir(e.registry))
	if err != nil {
		fmt.Fprintf(&report, "list sidecar evidence: %v\n", err)
	}
	for _, entry := range entries {
		stem, _, ok := fixtureCustodianSidecar(entry.Name())
		path := filepath.Join(filepath.Dir(e.registry), entry.Name())
		if ok && stem == filepath.Base(e.registry) && path != paths[0] && path != paths[1] {
			paths = append(paths, path)
		}
	}
	for _, path := range paths {
		fmt.Fprintf(&report, "%s: %s\n", path, setupCustodyFileEvidence(path))
	}
	return report.String()
}

// Evidence remains bounded and cannot block on a FIFO or follow a symlink.
func setupCustodyFileEvidence(path string) string {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return err.Error()
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err.Error()
	}
	if !info.Mode().IsRegular() {
		return "not a regular file"
	}
	offset := max(0, info.Size()-fixtureCustodianLogReportLimit)
	tail := make([]byte, info.Size()-offset)
	n, err := file.ReadAt(tail, offset)
	if err != nil && err != io.EOF {
		return err.Error()
	}
	return fmt.Sprintf("last %d of %d bytes:\n%s", n, info.Size(), tail[:n])
}

func newSetupCustodyEvidence(t *testing.T) setupCustodyEvidence {
	t.Helper()
	home := deadRegistryHomeUnder(t, t.TempDir())
	owner := identity.Ref{Pid: 4242, StartedAtSec: 100}
	child := identity.Exact{Pid: 4343, StartedAt: time.Unix(100, 0)}
	custodian := identity.Ref{Pid: 4444, StartedAtSec: 100}
	checkTestenv(t, os.WriteFile(home.path+".fixture-refs-4242", []byte("synthetic setup-child record\n"), 0o600))
	checkTestenv(t, os.WriteFile(home.path+".custodian-4242.log", []byte("synthetic custodian: cleanup failed\n"), 0o600))
	return setupCustodyEvidence{registry: home.path, owner: owner, child: child.Ref(), custodian: custodian,
		prober: exitProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
			exact := child
			exact.Pid = pid
			return exact, identity.Alive, nil
		})}
}

// The artificial deadline expires after two in-process clock steps. A broken
// deadline branch fails on the next step instead of hanging the test binary.
func setupCustodyClock(t *testing.T) (func(), func() (time.Duration, bool), *int) {
	t.Helper()
	pauses := 0
	left := awaitReserve + 2*time.Nanosecond
	return func() {
		pauses++
		if pauses > 2 {
			t.Fatal("custody wait ignored the test deadline")
		}
		left -= time.Nanosecond
	}, func() (time.Duration, bool) { return left, true }, &pauses
}

func requireSetupCustodyFailure(t *testing.T, recorder *awaitRecorder, e setupCustodyEvidence, observations, pauses int, what, state string) {
	t.Helper()
	if observations != 3 || pauses != 2 || len(recorder.failures) != 1 {
		t.Fatalf("observations=%d pauses=%d failures=%q, want 3 observations, 2 clock steps and one deadline failure", observations, pauses, recorder.failures)
	}
	for _, want := range []string{what, "deadline", e.registry, "owner={Pid:4242", "custodian={Pid:4444", "child={Pid:4343",
		"child pid=4343 state=" + state, "custodian pid=4444 state=" + state, "custodian records remain", ".custodian-4242.log",
		"synthetic custodian: cleanup failed", ".fixture-refs-4242", "synthetic setup-child record"} {
		if !strings.Contains(recorder.failures[0], want) {
			t.Errorf("deadline failure lacks %q:\n%s", want, recorder.failures[0])
		}
	}
}

func TestSetupCustodyChildFailureReportsEvidenceWithinDeadline(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	pause, remaining, pauses := setupCustodyClock(t)
	recorder := &setupCustodyRecorder{name: t.Name()}
	observations := 0
	const what = "the custodian to end the setup child"
	e.wait(recorder, what, func() bool {
		observations++
		exact, state, err := e.prober.Probe(e.child.Pid)
		return fixtureExited(exact, state, err, e.child)
	}, pause, remaining, nil, nil)
	requireSetupCustodyFailure(t, &recorder.awaitRecorder, e, observations, *pauses, what, "alive")
}

func TestSetupCustodySettlementFailureReportsEvidenceWithinDeadline(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	e.prober = exitProbeFunc(func(int64) (identity.Exact, identity.Liveness, error) {
		return identity.Exact{}, identity.Dead, nil
	})
	pause, remaining, pauses := setupCustodyClock(t)
	recorder := &setupCustodyRecorder{name: t.Name()}
	observations := 0
	const what = "the custodian to settle the dead binary's registry home"
	e.wait(recorder, what, func() bool {
		observations++
		_, err := os.Lstat(e.registry)
		return os.IsNotExist(err) || HomeSettled(e.registry)
	}, pause, remaining, nil, nil)
	requireSetupCustodyFailure(t, &recorder.awaitRecorder, e, observations, *pauses, what, "dead")
}

func TestSetupCustodyWaitReturnsOnTheEvent(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	recorder := &setupCustodyRecorder{name: t.Name()}
	observed := false
	pauses := 0
	e.wait(recorder, "settlement", func() bool { return observed }, func() {
		pauses++
		if pauses > 1 {
			t.Fatal("wait missed its event")
		}
		observed = true
	}, func() (time.Duration, bool) { return awaitReserve + time.Nanosecond, true }, nil, nil)
	if pauses != 1 || len(recorder.failures) != 0 {
		t.Fatalf("pauses=%d failures=%q, want one pause and success", pauses, recorder.failures)
	}
}

func TestSetupCustodyEvidenceBoundsAndRefusesUnsafeSidecars(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	large := filepath.Join(root, "large.log")
	checkTestenv(t, os.WriteFile(large, append(bytes.Repeat([]byte("x"), fixtureCustodianLogReportLimit), []byte("tail marker")...), 0o600))
	got := setupCustodyFileEvidence(large)
	if !strings.Contains(got, "last 65536 of 65547 bytes") || !strings.HasSuffix(got, "tail marker") || len(got) > fixtureCustodianLogReportLimit+100 {
		t.Fatalf("evidence tail: length=%d prefix=%q", len(got), got[:min(100, len(got))])
	}
	link := filepath.Join(root, "link")
	checkTestenv(t, os.Symlink(large, link))
	if strings.Contains(setupCustodyFileEvidence(link), "tail marker") {
		t.Fatal("evidence followed a symlink")
	}
	fifo := filepath.Join(root, "fifo")
	checkTestenv(t, unix.Mkfifo(fifo, 0o600))
	if got := setupCustodyFileEvidence(fifo); got != "not a regular file" {
		t.Fatalf("FIFO evidence=%q", got)
	}
	e := newSetupCustodyEvidence(t)
	checkTestenv(t, os.WriteFile(e.registry+".fixture-refs-4545", []byte("another owner's synthetic record"), 0o600))
	checkTestenv(t, os.WriteFile(e.registry+".custodian-4545.log", []byte("another custodian's diagnostic"), 0o600))
	for _, want := range []string{"another owner's synthetic record", "another custodian's diagnostic"} {
		if report := e.describe(); !strings.Contains(report, want) {
			t.Fatalf("evidence omitted %q: %s", want, report)
		}
	}
}
