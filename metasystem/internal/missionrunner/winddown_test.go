package missionrunner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

// spawnTaggedGroup starts a real process group whose leader's argv carries
// the tag in the janitor's tagged-hold positional shape. termImmune leaders
// ignore SIGTERM so only the kill-through path can end them.
type taggedTestGroup struct {
	cmd          *exec.Cmd
	waited       <-chan struct{}
	lifetimeRead *os.File
}

func spawnTaggedGroup(t *testing.T, tag string, termImmune bool) *taggedTestGroup {
	t.Helper()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	lifetimeRead, lifetimeWrite, err := os.Pipe()
	if err != nil {
		readyRead.Close()
		readyWrite.Close()
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		readyRead.Close()
		readyWrite.Close()
		lifetimeRead.Close()
		lifetimeWrite.Close()
		t.Fatal(err)
	}
	// The trailing no-op keeps Bash from replacing the positioned, tagged
	// leader with its final external command. Its child writes readiness only
	// after inheriting the leader's TERM disposition. Both inherit the lifetime
	// writer, so EOF proves the whole group has exited; the release pipe keeps
	// the child blocked without making its lifetime depend on elapsed time.
	script := `bash -c 'printf x >&3; exec 3>&-; IFS= read -r _ <&5' & wait; :`
	if termImmune {
		script = `trap "" TERM; bash -c 'printf x >&3; exec 3>&-; IFS= read -r _ <&5' & wait; :`
	}
	cmd := exec.Command("bash", "-c", script, "metasystem", "util", "hold", "--tag", tag)
	cmd.ExtraFiles = []*os.File{readyWrite, lifetimeWrite, releaseRead}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		readyRead.Close()
		readyWrite.Close()
		lifetimeRead.Close()
		lifetimeWrite.Close()
		releaseRead.Close()
		releaseWrite.Close()
		t.Fatal(err)
	}
	leaderPID := cmd.Process.Pid
	readyWrite.Close()
	lifetimeWrite.Close()
	releaseRead.Close()
	// Reap concurrently: an unreaped leader is a zombie that keeps its
	// group technically alive after the kill, which is this harness's
	// artifact, not the wind-down's failure (the same rule the older
	// TestTerminateGroup records).
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	t.Cleanup(func() {
		releaseWrite.Close()
		<-waited
		_, _ = io.Copy(io.Discard, lifetimeRead)
		lifetimeRead.Close()
		readyRead.Close()
	})
	ready := []byte{0}
	if _, err := io.ReadFull(readyRead, ready); err != nil || ready[0] != 'x' {
		t.Fatalf("tagged test owner pid %d did not publish readiness: byte=%q err=%v", leaderPID, ready, err)
	}
	if !taggedGroupLeader(leaderPID, tag) {
		t.Fatalf("ready tagged test owner pid %d was not a positioned group leader", leaderPID)
	}
	return &taggedTestGroup{cmd: cmd, waited: waited, lifetimeRead: lifetimeRead}
}

func TestTaggedGroupCleanupReleasesBothTermVariants(t *testing.T) {
	t.Parallel()
	for _, termImmune := range []bool{false, true} {
		name := "TERM-honouring"
		if termImmune {
			name = "TERM-immune"
		}
		var group *taggedTestGroup
		if !t.Run(name, func(t *testing.T) {
			tag := fmt.Sprintf("metasystem-job-cleanup-%d-%t", os.Getpid(), termImmune)
			group = spawnTaggedGroup(t, tag, termImmune)
		}) {
			continue
		}
		select {
		case <-group.waited:
		default:
			t.Errorf("cleanup returned before the %s group leader was reaped", name)
		}
	}
}

func taggedGroupLeader(pid int, tag string) bool {
	exact, state, err := identity.KernelProber{}.Probe(int64(pid))
	if err != nil || state != identity.Alive || !exact.ArgvKnown {
		return false
	}
	pgid, err := syscall.Getpgid(pid)
	_, positioned := janitor.MatchShape(janitor.DefaultShapes(), exact.Argv, tag)
	return err == nil && pgid == pid && positioned
}

func waitGroupDead(group *taggedTestGroup) {
	<-group.waited
	_, _ = io.Copy(io.Discard, group.lifetimeRead)
}

func censusHoldsGroup(pgid int, observed census.TaggedProcessCensus) bool {
	for _, process := range observed.Tagged {
		if process.Universe == census.ProcessUniverseSignalable && process.PGID == int64(pgid) {
			return true
		}
	}
	for _, process := range observed.Indeterminate {
		if process.Universe == census.ProcessUniverseSignalable &&
			process.PGID == int64(pgid) {
			return true
		}
	}
	return false
}

type liveWindDownClassification string

const (
	liveWindDownLeaked            liveWindDownClassification = "leaked"
	liveWindDownAbandonedToCensus liveWindDownClassification = "abandoned-to-census"
	// Three immediate observations let transient process-table or identity
	// failures clear without making custody proof depend on elapsed time.
	censusHandoffProofAttempts = 3
)

func classifyLiveWindDown(pgid int, windDownErr error, scan func() census.TaggedProcessCensus) (liveWindDownClassification, census.TaggedProcessCensus) {
	if windDownErr != nil {
		return liveWindDownLeaked, census.TaggedProcessCensus{}
	}
	var observed census.TaggedProcessCensus
	for attempt := 0; attempt < censusHandoffProofAttempts; attempt++ {
		observed = scan()
		if censusHoldsGroup(pgid, observed) {
			return liveWindDownAbandonedToCensus, observed
		}
	}
	return liveWindDownLeaked, observed
}

func scanTaggedGroup(tag string) census.TaggedProcessCensus {
	return census.ScanTaggedProcesses(tag, census.TaggedScanDependencies{
		MatchesTag: func(argv []string, wanted string) bool {
			_, matched := janitor.MatchShape(janitor.DefaultShapes(), argv, wanted)
			return matched
		},
	})
}

func TestClassifyLiveWindDownDistinguishesLeakFromLawfulCensusHandoff(t *testing.T) {
	const pgid = int64(700)
	tagged := census.TaggedProcessCensus{Tagged: []census.TaggedProcess{{PGID: pgid}}}
	exactIndeterminate := census.TaggedProcessCensus{Indeterminate: []census.IndeterminateProcess{{
		PGID: pgid, Universe: census.ProcessUniverseSignalable,
	}}}
	rows := []struct {
		name         string
		windDownErr  error
		observations []census.TaggedProcessCensus
		want         liveWindDownClassification
		wantScans    int
	}{
		{
			name: "transient census failures clear into lawful custody",
			observations: []census.TaggedProcessCensus{
				{EnumerationError: "process table temporarily unavailable"},
				{Indeterminate: []census.IndeterminateProcess{{PID: pgid, Universe: census.ProcessUniverseSignalable}}},
				tagged,
			},
			want: liveWindDownAbandonedToCensus, wantScans: 3,
		},
		{
			name:         "exact indeterminate group remains in census custody",
			observations: []census.TaggedProcessCensus{exactIndeterminate},
			want:         liveWindDownAbandonedToCensus, wantScans: 1,
		},
		{
			name: "live group outside census custody is a real leak",
			observations: []census.TaggedProcessCensus{
				{Tagged: []census.TaggedProcess{{PGID: pgid + 1}}},
				{Tagged: []census.TaggedProcess{{PGID: pgid + 1}}},
				{Tagged: []census.TaggedProcess{{PGID: pgid + 1}}},
			},
			want: liveWindDownLeaked, wantScans: 3,
		},
		{
			name: "exhausted census failures cannot prove custody",
			observations: []census.TaggedProcessCensus{
				{EnumerationError: "process table unavailable on first attempt"},
				{EnumerationError: "process table unavailable on second attempt"},
				{EnumerationError: "process table unavailable on third attempt"},
			},
			want: liveWindDownLeaked, wantScans: 3,
		},
		{
			name:         "kill-through error is a leak despite census visibility",
			windDownErr:  fmt.Errorf("group survived SIGKILL"),
			observations: []census.TaggedProcessCensus{tagged},
			want:         liveWindDownLeaked, wantScans: 0,
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			scans := 0
			classification, _ := classifyLiveWindDown(int(pgid), row.windDownErr, func() census.TaggedProcessCensus {
				observation := row.observations[scans]
				scans++
				return observation
			})
			if classification != row.want || scans != row.wantScans {
				t.Fatalf("classification = %s after %d census scans, want %s after %d",
					classification, scans, row.want, row.wantScans)
			}
		})
	}
}

// TestTerminateGroupLeaksNoGroupsUnderCompression is the wind-down's wiring
// proof: real tagged groups, TERM-honouring and TERM-immune, through the
// real probes and signals. The ladder's verdicts themselves are proven on
// the fake kernel (winddown_fake_test.go) with no wall time.
func TestTerminateGroupLeaksNoGroupsUnderCompression(t *testing.T) {
	// Repeated wind-downs under an aggressive compression scale may leave an
	// unprovable group to the census, but may not lose a live group outside
	// that custody.
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "20") // scale 50
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown-scale"}
	leaked := 0
	abandonedToCensus := 0
	for cycle := 0; cycle < 4; cycle++ {
		tag := fmt.Sprintf("metasystem-job-scale-%d-%d", os.Getpid(), cycle)
		group := spawnTaggedGroup(t, tag, cycle%2 == 1)
		pgid := group.cmd.Process.Pid
		termination, windDownErr := engine.terminateGroup(pgid, tag, false)
		// A wind-down that reports a leak is one, whatever the group does
		// afterwards; only a clean wind-down earns the wait for its death.
		if windDownErr == nil && termination != TerminationAlreadyGone {
			waitGroupDead(group)
			continue
		}
		classification, observed := classifyLiveWindDown(pgid, windDownErr, func() census.TaggedProcessCensus {
			return scanTaggedGroup(tag)
		})
		if classification == liveWindDownAbandonedToCensus {
			abandonedToCensus++
			continue
		}
		leaked++
		t.Logf("group %d remained alive outside census custody after wind-down: %v; census enumeration error %q, tagged %d, unknown within the signalable universe %d",
			pgid, windDownErr, observed.EnumerationError, len(observed.Tagged), observed.UnknownWithinUniverse())
	}
	if leaked != 0 {
		t.Fatalf("compressed wind-down leaked %d of 4 groups; %d live groups remained in census custody",
			leaked, abandonedToCensus)
	}
}

func TestGroupOwnershipFixtureFallbackStaysExact(t *testing.T) {
	// A zero grant must refuse the fixture path entirely; the kernel
	// tri-state alone decides.
	var grant fixtureauth.GroupOwnershipGrant
	if got := groupOwnership(1, "metasystem-job-x", grant); got == "OWNED" {
		t.Fatalf("pgid 1 with a zero grant = %s", got)
	}
}
