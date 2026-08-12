package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The custodian verdict table (go-production-grade Phase 1, B1), one focused
// test per row. Custodian binds the kernel prober directly, so the rows are
// driven through real processes and the fixture file — the same two sources
// production uses.

func selfIdentity(t *testing.T) (pid int64, start int64, tag string) {
	t.Helper()
	pid = int64(os.Getpid())
	exact, state, err := (KernelProber{}).Probe(pid)
	if err != nil || state != Alive {
		t.Fatalf("cannot probe self: %v %v", state, err)
	}
	if !exact.ArgvKnown || len(exact.Argv) == 0 {
		t.Fatalf("self argv must be readable, got known=%v argv=%v", exact.ArgvKnown, exact.Argv)
	}
	return pid, exact.StartedAt.Unix(), exact.Argv[0]
}

func writeFixture(t *testing.T, entries map[string]any) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.json")
	data, _ := json.Marshal(entries)
	os.WriteFile(path, data, 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", path)
}

// Row 1: kernel death always vetoes the fixture.
func TestCustodianRow1KernelDeathVetoesFixture(t *testing.T) {
	child := exec.Command("true")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := int64(child.Process.Pid)
	child.Wait() // reaped: the pid is provably gone
	writeFixture(t, map[string]any{
		fmt.Sprint(pid): map[string]any{"pidStartedAt": int64(123), "command": "anything"},
	})
	if got := Custodian(pid, 123, ""); got != Dead {
		t.Fatalf("dead pid with fixture entry: want Dead, got %v", got)
	}
}

// Row 2: a fixture entry is authoritative for a live pid.
func TestCustodianRow2FixtureAuthoritative(t *testing.T) {
	pid, start, _ := selfIdentity(t)
	writeFixture(t, map[string]any{
		fmt.Sprint(pid): map[string]any{"pidStartedAt": start, "command": "runner --tag the-tag"},
	})
	if got := Custodian(pid, start, "the-tag"); got != Alive {
		t.Fatalf("matching fixture: want Alive, got %v", got)
	}
	if got := Custodian(pid, start+7, "the-tag"); got != Dead {
		t.Fatalf("fixture start mismatch: want Dead, got %v", got)
	}
	if got := Custodian(pid, start, "absent-tag"); got != Dead {
		t.Fatalf("tag absent from fixture command: want Dead, got %v", got)
	}
}

// Row 3: no fixture entry and the kernel probe cannot determine anything.
func TestCustodianRow3ProbeErrorIsUnknown(t *testing.T) {
	if got := Custodian(0, 0, "any"); got != Unknown {
		t.Fatalf("invalid pid probes to error: want Unknown, got %v", got)
	}
}

// Row 4: probe succeeded, start time differs — a recycled pid is a stranger.
func TestCustodianRow4StartMismatchIsDead(t *testing.T) {
	pid, start, _ := selfIdentity(t)
	if got := Custodian(pid, start+1, ""); got != Dead {
		t.Fatalf("start mismatch: want Dead, got %v", got)
	}
}

// Row 5 — the B1 correction: a tag is expected but the argv was unreadable.
// Absence of evidence is Unknown, never Dead. The subject is a real process
// whose probe succeeds with ArgvKnown=false: on darwin an unprivileged test
// cannot read launchd's argv; on linux a kernel thread has an empty cmdline.
func TestCustodianRow5UnreadableArgvIsUnknown(t *testing.T) {
	var subject Exact
	found := false
	for _, pid := range []int64{1, 2} {
		exact, state, err := (KernelProber{}).Probe(pid)
		if err == nil && state == Alive && !exact.ArgvKnown {
			subject, found = exact, true
			break
		}
	}
	if !found {
		t.Skip("no probe-alive process with unreadable argv on this host (running privileged?)")
	}
	if got := Custodian(subject.Pid, subject.StartedAt.Unix(), "some-tag"); got != Unknown {
		t.Fatalf("unreadable argv with expected tag: want Unknown (B1), got %v", got)
	}
	// Without a tag the same process is simply Alive: no argv evidence needed.
	if got := Custodian(subject.Pid, subject.StartedAt.Unix(), ""); got != Alive {
		t.Fatalf("unreadable argv without tag: want Alive, got %v", got)
	}
}

// Row 6: argv readable and the tag is genuinely absent — a stranger.
func TestCustodianRow6ReadableArgvWithoutTagIsDead(t *testing.T) {
	pid, start, _ := selfIdentity(t)
	if got := Custodian(pid, start, "tag-that-cannot-appear-a6f8e2"); got != Dead {
		t.Fatalf("readable argv, absent tag: want Dead, got %v", got)
	}
}

// Row 7: everything matches.
func TestCustodianRow7Alive(t *testing.T) {
	pid, start, tag := selfIdentity(t)
	if got := Custodian(pid, start, tag); got != Alive {
		t.Fatalf("matching custodian: want Alive, got %v", got)
	}
	if got := Custodian(pid, start, ""); got != Alive {
		t.Fatalf("no tag expected: want Alive, got %v", got)
	}
}
