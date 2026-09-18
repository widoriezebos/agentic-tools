package census

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func survivorRef(pid, token int64) identity.Ref {
	if runtime.GOOS == "linux" {
		return identity.Ref{Pid: pid, StartTicks: token, BootID: "fixture-boot"}
	}
	return identity.Ref{Pid: pid, StartedAtSec: token, StartedAtUnixMicro: token * 1_000_000}
}

func survivorProcess(pid, token int64) Process {
	process := Process{Pid: pid, PPID: 1, PGID: pid, Started: token, StartedExactMicro: token * 1_000_000, Argv: "/bin/sh", Alive: true}
	if runtime.GOOS == "linux" {
		process.StartTicks, process.BootID = token, "fixture-boot"
	}
	return process
}

func survivorTag(t *testing.T, key identity.FixtureKey) string {
	t.Helper()
	encoded, err := identity.EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return identity.FixtureOwnerEnv + "=" + encoded
}

type unknownFixtureOwner struct {
	identity.Prober
	pid int64
}

func (prober unknownFixtureOwner) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == prober.pid {
		return identity.Exact{}, identity.Unknown, errors.New("fixture owner unreadable")
	}
	return prober.Prober.Probe(pid)
}

func TestCensusReapsOnlyDeadOwnedSurvivors(t *testing.T) {
	base := int64(os.Getpid()) * 10
	deadOwner := survivorRef(base+1, 101)
	liveOwner := survivorRef(base+2, 102)
	unknownOwner := survivorRef(base+3, 103)
	deadKey := identity.FixtureKey{Owner: deadOwner, Test: "TestDead", Nonce: "00000001"}
	liveKey := identity.FixtureKey{Owner: liveOwner, Test: "TestLive", Nonce: "00000002"}
	recordedKey := identity.FixtureKey{Owner: deadOwner, Test: "TestRecorded", Nonce: "00000003"}
	unknownKey := identity.FixtureKey{Owner: unknownOwner, Test: "TestUnknown", Nonce: "00000004"}

	dead := survivorProcess(base+10, 110)
	dead.Environ = []string{survivorTag(t, deadKey)}
	live := survivorProcess(base+11, 111)
	live.Environ = []string{survivorTag(t, liveKey)}
	unowned := survivorProcess(base+12, 112)
	unowned.Exe = filepath.Join(t.TempDir(), "go-tmp", "TestOther99", "unowned")
	recordDirectory, err := os.MkdirTemp(t.TempDir(), "TestRecorded")
	if err != nil {
		t.Fatal(err)
	}
	encodedRecord, err := identity.EncodeKey(recordedKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recordDirectory, "fixture-owner"), []byte(encodedRecord+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorded := survivorProcess(base+13, 113)
	recorded.Exe = filepath.Join(recordDirectory, "bin", "child")
	unreadable := survivorProcess(base+14, 114)
	unreadable.PGID, unreadable.Unreadable = dead.Pid, true
	unknown := survivorProcess(base+15, 115)
	unknown.Environ = []string{survivorTag(t, unknownKey)}
	liveOwnerRow := survivorProcess(liveOwner.Pid, 102)
	rows := []Process{dead, live, unowned, recorded, unreadable, unknown, liveOwnerRow}
	table := FixtureProcessProber(rows)
	prober := unknownFixtureOwner{Prober: table, pid: unknownOwner.Pid}

	var signals []int64
	survivors, err := ReapFixtureSurvivors(prober, rows, FixtureSurvivorSelection{}, func(pid int, signal syscall.Signal) error {
		if signal != syscall.SIGKILL {
			t.Fatalf("signal = %v, want KILL", signal)
		}
		signals = append(signals, int64(pid))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[0] != dead.Pid || signals[1] != recorded.Pid {
		t.Fatalf("signals = %v, want KILL for dead-owned pids %d and %d", signals, dead.Pid, recorded.Pid)
	}
	lines := make([]string, len(survivors))
	for index, survivor := range survivors {
		lines[index] = FixtureSurvivorLine(prober, survivor)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "unowned-in-cache pid=") || !strings.Contains(joined, "fixture-survivor? pid=") {
		t.Fatalf("survivor lines did not preserve classes: %s", joined)
	}
	for _, survivor := range survivors {
		if survivor.Ref.Pid == live.Pid {
			t.Fatalf("live-owned process was named: %s", joined)
		}
	}

	if got, err := ScanFixtureSurvivors(table, rows, FixtureSurvivorSelection{Owner: &liveOwner}); err == nil || got != nil {
		t.Fatalf("live owner scan = %#v, %v; want refusal", got, err)
	}
	if got, err := ScanFixtureSurvivors(prober, rows, FixtureSurvivorSelection{Owner: &unknownOwner}); err == nil || got != nil {
		t.Fatalf("unknown owner scan = %#v, %v; want refusal", got, err)
	}
	keyed, err := ScanFixtureSurvivors(table, rows, FixtureSurvivorSelection{Key: &deadKey})
	if err != nil {
		t.Fatal(err)
	}
	for _, survivor := range keyed {
		if survivor.Class == identity.FixtureSurvivorCertain {
			encoded, encodeErr := identity.EncodeKey(survivor.Key)
			want, _ := identity.EncodeKey(deadKey)
			if encodeErr != nil || encoded != want {
				t.Fatalf("key scan widened to %#v", survivor.Key)
			}
		}
	}
}

func TestFixtureSurvivorScanExcludesItsOwnProcess(t *testing.T) {
	self := int64(os.Getpid())
	owner := survivorRef(self+1, 201)
	key := identity.FixtureKey{Owner: owner, Test: "TestSelf", Nonce: "00000005"}
	process := survivorProcess(self, 202)
	process.Environ = []string{survivorTag(t, key)}
	rows := []Process{process}
	prober := FixtureProcessProber(rows)

	lines, certain, err := FixtureSurvivorLines(prober, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 || certain {
		t.Fatalf("scan listed its own process: lines=%v certain=%t", lines, certain)
	}

	var signals []int
	survivors, err := ReapFixtureSurvivors(prober, rows, FixtureSurvivorSelection{Key: &key}, func(pid int, signal syscall.Signal) error {
		signals = append(signals, pid)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(survivors) != 0 || len(signals) != 0 {
		t.Fatalf("reap saw own-process survivors %#v and signals %v, want neither", survivors, signals)
	}
}
