package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type startLockProcesses map[int64]identity.Exact

func (p startLockProcesses) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, exists := p[pid]
	if !exists {
		return identity.Exact{}, identity.Dead, nil
	}
	if exact.Pid == 0 {
		return identity.Exact{}, identity.Unknown, errors.New("process unreadable")
	}
	return exact, identity.Alive, nil
}

func (p startLockProcesses) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	return p.Probe(pid)
}

func (p startLockProcesses) ReadArgv(pid int64) ([]string, bool) {
	return p[pid].Argv, p[pid].ArgvKnown
}

func TestAStartsCapLockHoldsAgainstAnotherStart(t *testing.T) {
	t.Parallel()
	const pid int64 = 101
	holder := identity.Exact{Pid: pid, StartTicks: 707949, BootID: "boot-aaaa", StartedAt: time.Unix(500, 0),
		Argv: []string{"metasystem", "system", "start"}, ArgvKnown: true}
	processes := startLockProcesses{pid: holder}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(SupervisionDir(root), "cap-authority.lock.d", "owner.json")
	now := time.Unix(1000, 0)
	startA := startLockClaim{pid: pid, now: func() time.Time { return now }, sleep: func(d time.Duration) { now = now.Add(d) }, reader: processes}
	startB := startA
	startB.pid = 202
	contender := holder
	contender.Pid = startB.pid
	processes[startB.pid] = contender

	claim := func(t *testing.T) func() error {
		t.Helper()
		start := now
		release, err := startA.acquire(root, 1000)
		if err != nil || now != start {
			t.Fatalf("claim took %s, error %v", now.Sub(start), err)
		}
		return release
	}
	release := func(t *testing.T, f func() error) {
		t.Helper()
		if err := f(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owner survived release: %v", err)
		}
	}
	busy := func(t *testing.T, owner int64, operation string) {
		t.Helper()
		start := now
		_, err := startB.acquire(root, 1000)
		if !errors.Is(err, ErrCapAuthorityBusy) || !strings.Contains(err.Error(), fmt.Sprintf("process %d (%s)", owner, operation)) || now.Sub(start) != 10*time.Second {
			t.Fatalf("waited %s, refusal %v", now.Sub(start), err)
		}
	}

	for _, shift := range []int64{-4, -1, 1, 4} {
		processes[pid] = holder
		free := claim(t)
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var record struct {
			Pid        int64
			Tag        string `json:"instanceTag"`
			Process    identity.Ref
			AcquiredAt time.Time
		}
		want := identity.Ref{Pid: pid, StartTicks: holder.StartTicks, BootID: holder.BootID}
		if err := json.Unmarshal(data, &record); err != nil || record.Pid != pid || record.Process != want || record.Tag != "metasystem system start" || !record.AcquiredAt.Equal(now.Truncate(time.Second)) {
			t.Fatalf("unexpected claim %s, decode %v", data, err)
		}
		shifted := holder
		shifted.StartedAt = shifted.StartedAt.Add(time.Duration(shift) * time.Second)
		shifted.Argv = []string{"renamed"}
		processes[pid] = shifted
		busy(t, pid, "metasystem system start")
		delete(processes, pid)
		release(t, free)
	}

	for _, change := range []string{"start ticks", "boot"} {
		processes[pid] = holder
		claim(t) // The crash leaves the real writer's record behind.
		next := holder
		if change == "start ticks" {
			next.StartTicks++
		} else {
			next.BootID = "boot-bbbb"
		}
		processes[pid] = next
		release(t, claim(t))
	}
	processes[pid] = holder
	other := pid + 1
	dir := filepath.Dir(file)
	for _, tc := range []struct {
		name, tag string
		exact     identity.Exact
		wantBusy  bool
	}{
		{"command matches", "metasystem system start", identity.Exact{Pid: other, Argv: holder.Argv, ArgvKnown: true}, true},
		{"command differs", "metasystem system start", identity.Exact{Pid: other, Argv: []string{"another"}, ArgvKnown: true}, false},
		{"literal matches", "metasystem up", identity.Exact{Pid: other, Argv: []string{"metasystem", "up"}, ArgvKnown: true}, true},
		{"literal differs", "metasystem up", identity.Exact{Pid: other, Argv: holder.Argv, ArgvKnown: true}, false},
		{"argv unreadable", "metasystem up", identity.Exact{Pid: other}, true},
		{"unknown", "metasystem up", identity.Exact{}, true},
		{"dead", "metasystem up", identity.Exact{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processes[other] = tc.exact
			if tc.name == "dead" {
				delete(processes, other)
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			data := fmt.Sprintf("{\"pid\":%d,\"instanceTag\":%q}", other, tc.tag)
			if err := os.WriteFile(file, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.wantBusy {
				busy(t, other, tc.tag)
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
			} else {
				release(t, claim(t))
			}
		})
	}
	for _, state := range []string{"dead", "unknown"} {
		delete(processes, pid)
		if state == "unknown" {
			processes[pid] = identity.Exact{}
		}
		if _, err := startA.acquire(root, 1000); err == nil {
			t.Fatalf("%s claimant acquired the lock", state)
		}
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s claimant published an owner: %v", state, err)
		}
	}
}
