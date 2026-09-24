package proofrun

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func proofSurvivorRef(pid, token int64) identity.Ref {
	if runtime.GOOS == "linux" {
		return identity.Ref{Pid: pid, StartTicks: token, BootID: "fixture-boot"}
	}
	return identity.Ref{Pid: pid, StartedAtSec: token, StartedAtUnixMicro: token * 1_000_000}
}

func proofSurvivorProcess(pid, token int64) census.Process {
	process := census.Process{Pid: pid, PPID: 1, PGID: pid, Started: token, StartedExactMicro: token * 1_000_000, Argv: "/bin/sh", Alive: true}
	if runtime.GOOS == "linux" {
		process.StartTicks, process.BootID = token, "fixture-boot"
	}
	return process
}

func proofSurvivorTag(t *testing.T, key identity.FixtureKey) string {
	t.Helper()
	encoded, err := identity.EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return identity.FixtureOwnerEnv + "=" + encoded
}

func launcherSurvivorRows(t *testing.T) ([]census.Process, int64, int64) {
	t.Helper()
	base := int64(os.Getpid()) * 20
	deadOwner := proofSurvivorRef(base+1, 11)
	liveOwner := proofSurvivorRef(base+2, 12)
	fresh := proofSurvivorProcess(base+10, 210)
	fresh.Environ = []string{proofSurvivorTag(t, identity.FixtureKey{Owner: deadOwner, Test: "TestFresh", Nonce: "00000001"})}
	live := proofSurvivorProcess(base+11, 211)
	live.Environ = []string{proofSurvivorTag(t, identity.FixtureKey{Owner: liveOwner, Test: "TestLive", Nonce: "00000002"})}
	unreadable := proofSurvivorProcess(base+12, 212)
	unreadable.PGID, unreadable.Unreadable = fresh.Pid, true
	old := proofSurvivorProcess(base+13, 100)
	old.Environ = []string{proofSurvivorTag(t, identity.FixtureKey{Owner: deadOwner, Test: "TestOld", Nonce: "00000003"})}
	liveOwnerRow := proofSurvivorProcess(liveOwner.Pid, 12)
	return []census.Process{fresh, live, unreadable, old, liveOwnerRow}, fresh.Pid, old.Pid
}

func TestLauncherReapsSurvivorsIntoTheVerdict(t *testing.T) {
	rows, freshPID, oldPID := launcherSurvivorRows(t)
	prober := census.FixtureProcessProber(rows)
	var signals []int64
	var verdict bytes.Buffer
	failed := reapFixtureSurvivors(prober, rows, func(pid int, signal syscall.Signal) error {
		if signal != syscall.SIGKILL {
			t.Fatalf("signal = %s, want killed", signal)
		}
		signals = append(signals, int64(pid))
		return nil
	}, fixtureAttemptOwnership{startedAt: time.Unix(200, 0)}, &verdict)
	if !failed || len(signals) != 2 || signals[0] != freshPID || signals[1] != oldPID {
		t.Fatalf("reap failed=%t signals=%v, want KILL for %d and %d", failed, signals, freshPID, oldPID)
	}
	lines := verdict.String()
	if strings.Count(lines, "fixture-survivor pid=") != 2 || strings.Count(lines, "fixture-survivor? pid=") != 1 {
		t.Fatalf("survivor verdict = %q", lines)
	}
	oldLine := ""
	for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
		if strings.Contains(line, "pid="+strconvInt(oldPID)+" ") {
			oldLine = line
		}
	}
	if !strings.HasSuffix(oldLine, " stale") {
		t.Fatalf("older survivor line = %q, verdict=%q", oldLine, lines)
	}
	if strings.Contains(lines, "pid="+strconvInt(rows[1].Pid)+" ") {
		t.Fatalf("live-owned process was named: %q", lines)
	}

	signals, verdict = nil, bytes.Buffer{}
	withoutFresh := append([]census.Process(nil), rows[1:]...)
	if failed := reapFixtureSurvivors(census.FixtureProcessProber(withoutFresh), withoutFresh, func(pid int, _ syscall.Signal) error {
		signals = append(signals, int64(pid))
		return nil
	}, fixtureAttemptOwnership{startedAt: time.Unix(200, 0)}, &verdict); failed || len(signals) != 1 || signals[0] != oldPID {
		t.Fatalf("old-only reap failed=%t signals=%v verdict=%q", failed, signals, verdict.String())
	}

	t.Run("LaunchSuite", func(t *testing.T) {
		root := t.TempDir()
		conf := filepath.Join(root, "metasystem.conf")
		if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		watchdog := filepath.Join(root, "watchdog.sh")
		writeExecutable(t, watchdog, "#!/usr/bin/env bash\ndone_path=\nwhile (($#)); do if [[ $1 == --done ]]; then done_path=$2; shift 2; else shift; fi; done\nwhile [[ ! -e $done_path ]]; do sleep 0.01; done\n")
		processFile := filepath.Join(root, "processes.json")
		t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processFile)
		fresh := rows[0]
		fresh.Started, fresh.StartedExactMicro = time.Now().Add(time.Hour).Unix(), time.Now().Add(time.Hour).UnixMicro()
		writeProofProcessTable(t, processFile, []census.Process{fresh})
		var recorded []int
		logPath := filepath.Join(root, "suite.log")
		launch := func(suite, launchLog string) int {
			return LaunchSuite(LaunchOptions{Suite: suite, Root: root, ConfPath: conf,
				ProgressPath: filepath.Join(root, suite+".progress.jsonl"), LogPath: launchLog, Banner: "fixture banner",
				Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
				Poll: 5 * time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
				WatchdogExecutable: watchdog, Command: []string{"sh", "-c", "exit 0"},
				Signal: func(pid int, signal syscall.Signal) error { recorded = append(recorded, pid); return nil }})
		}
		if result := launch("fixture-survivor", logPath); result != 1 || len(recorded) != 1 || int64(recorded[0]) != fresh.Pid {
			t.Fatalf("survivor launch result=%d signals=%v", result, recorded)
		}
		log, err := os.ReadFile(logPath)
		if err != nil || !strings.Contains(string(log), "fixture-survivor pid="+strconvInt(fresh.Pid)) {
			t.Fatalf("survivor launch log=%q err=%v", log, err)
		}
		recorded = nil
		writeProofProcessTable(t, processFile, nil)
		emptyLog := filepath.Join(root, "empty.log")
		if result := launch("fixture-empty", emptyLog); result != 0 || len(recorded) != 0 {
			t.Fatalf("empty launch result=%d signals=%v", result, recorded)
		}
		if log, err := os.ReadFile(emptyLog); err != nil || strings.Contains(string(log), "fixture-survivor") {
			t.Fatalf("empty launch log=%q err=%v", log, err)
		}
		if err := os.WriteFile(processFile, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		unreadableLog := filepath.Join(root, "unreadable.log")
		if result := launch("fixture-unreadable", unreadableLog); result != 0 || len(recorded) != 0 {
			t.Fatalf("unreadable launch result=%d signals=%v", result, recorded)
		}
		log, err = os.ReadFile(unreadableLog)
		if err != nil || strings.Count(string(log), "fixture survivor table is unreadable") != 1 {
			t.Fatalf("unreadable launch log=%q err=%v", log, err)
		}
	})
}

func TestForeignFixtureSurvivorDoesNotFailProofAttempt(t *testing.T) {
	t.Parallel()

	base := int64(os.Getpid()) * 30
	attemptOwner := proofSurvivorRef(base+1, 11)
	foreignOwner := proofSurvivorRef(base+2, 12)
	foreign := proofSurvivorProcess(base+10, 210)
	foreign.Environ = []string{proofSurvivorTag(t, identity.FixtureKey{Owner: foreignOwner, Test: "TestForeign", Nonce: "00000001"})}

	var signals []int64
	var verdict bytes.Buffer
	failed := reapFixtureSurvivors(census.FixtureProcessProber([]census.Process{foreign}), []census.Process{foreign}, func(pid int, _ syscall.Signal) error {
		signals = append(signals, int64(pid))
		return nil
	}, fixtureAttemptOwnership{owner: attemptOwner, attemptID: "attempt-a", startedAt: time.Unix(200, 0)}, &verdict)
	if failed || len(signals) != 0 || !strings.Contains(verdict.String(), "fixture-survivor pid="+strconvInt(foreign.Pid)) ||
		!strings.Contains(verdict.String(), " foreign") {
		t.Fatalf("foreign survivor failed=%t signals=%v verdict=%q", failed, signals, verdict.String())
	}

	owned := foreign
	owned.Pid++
	owned.Argv += " " + identity.FixtureAttemptEnv + "=attempt-a"
	signals, verdict = nil, bytes.Buffer{}
	failed = reapFixtureSurvivors(census.FixtureProcessProber([]census.Process{owned}), []census.Process{owned}, func(pid int, _ syscall.Signal) error {
		signals = append(signals, int64(pid))
		return nil
	}, fixtureAttemptOwnership{owner: attemptOwner, attemptID: "attempt-a", startedAt: time.Unix(200, 0)}, &verdict)
	if !failed || len(signals) != 1 || signals[0] != owned.Pid || strings.Contains(verdict.String(), " foreign") {
		t.Fatalf("owned survivor failed=%t signals=%v verdict=%q", failed, signals, verdict.String())
	}
}

func TestFixtureSurvivorOwnedByAttemptCanonicalRef(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux exact references carry the redundant-seconds representation under test")
	}

	owner := identity.Ref{Pid: 4101, StartedAtSec: 17, StartTicks: 701, BootID: "boot-a"}
	survivor := identity.FixtureSurvivor{
		Ref: identity.Ref{Pid: 4201, StartTicks: 801, BootID: "boot-a"},
		Key: identity.FixtureKey{Owner: identity.Ref{Pid: owner.Pid, StartTicks: owner.StartTicks, BootID: owner.BootID}},
	}
	process := proofSurvivorProcess(survivor.Ref.Pid, survivor.Ref.StartTicks)
	process.BootID = survivor.Ref.BootID
	prober := census.FixtureProcessProber([]census.Process{process})

	tests := []struct {
		name  string
		owner identity.Ref
		want  bool
	}{
		{name: "redundant seconds differ", owner: owner, want: true},
		{name: "pid differs", owner: identity.Ref{Pid: owner.Pid + 1, StartTicks: owner.StartTicks, BootID: owner.BootID}},
		{name: "ticks differ", owner: identity.Ref{Pid: owner.Pid, StartTicks: owner.StartTicks + 1, BootID: owner.BootID}},
		{name: "boot differs", owner: identity.Ref{Pid: owner.Pid, StartTicks: owner.StartTicks, BootID: "boot-b"}},
		{name: "survivor owner invalid", owner: owner},
		{name: "attempt owner invalid", owner: identity.Ref{Pid: owner.Pid}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := survivor
			if test.name == "survivor owner invalid" {
				candidate.Key.Owner = identity.Ref{Pid: owner.Pid}
			}
			if got := fixtureSurvivorOwnedByAttempt(prober, candidate, fixtureAttemptOwnership{owner: test.owner}); got != test.want {
				t.Fatalf("fixtureSurvivorOwnedByAttempt() = %t, want %t for survivor owner %+v and attempt owner %+v",
					got, test.want, candidate.Key.Owner, test.owner)
			}
		})
	}
}

func writeProofProcessTable(t *testing.T, path string, rows []census.Process) {
	t.Helper()
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func strconvInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
