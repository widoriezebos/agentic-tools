package missionrunner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

// spawnTaggedGroup starts a real process group whose leader's argv carries
// the tag in the janitor's tagged-hold positional shape. termImmune leaders
// ignore SIGTERM so only the kill-through path can end them.
func spawnTaggedGroup(t *testing.T, tag string, termImmune bool) *exec.Cmd {
	t.Helper()
	script := "sleep 30"
	if termImmune {
		script = `trap "" TERM; sleep 30`
	}
	cmd := exec.Command("bash", "-c", script, "metasystem", "util", "hold", "--tag", tag)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap concurrently: an unreaped leader is a zombie that keeps its
	// group technically alive after the kill, which is this harness's
	// artifact, not the wind-down's failure (the same rule the older
	// TestTerminateGroup records).
	go func() { _ = cmd.Wait() }()
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	return cmd
}

func waitGroupDead(pgid int, patience time.Duration) bool {
	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) {
		if !groupAlive(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !groupAlive(pgid)
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

func TestTerminateGroupKillsThroughATermImmuneOwnedGroup(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown"}
	tag := fmt.Sprintf("metasystem-job-winddown-%d", os.Getpid())
	cmd := spawnTaggedGroup(t, tag, true)
	pgid := cmd.Process.Pid
	if err := engine.terminateGroup(pgid, tag, false); err != nil {
		t.Fatalf("kill-through wind-down failed: %v", err)
	}
	if !waitGroupDead(pgid, 3*time.Second) {
		t.Fatal("a TERM-immune owned group must die through the SIGKILL path")
	}
}

func TestTerminateGroupNeverSignalsAForeignGroup(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-winddown"}
	foreign := exec.Command("sleep", "30")
	foreign.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-foreign.Process.Pid, syscall.SIGKILL)
		_, _ = foreign.Process.Wait()
	})
	tag := fmt.Sprintf("metasystem-job-foreign-%d", os.Getpid())
	if err := engine.terminateGroup(foreign.Process.Pid, tag, false); err != nil {
		t.Fatalf("foreign wind-down must skip without error: %v", err)
	}
	if !groupAlive(foreign.Process.Pid) {
		t.Fatal("a group without the positioned tag was signaled")
	}
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
		cmd := spawnTaggedGroup(t, tag, cycle%2 == 1)
		pgid := cmd.Process.Pid
		windDownErr := engine.terminateGroup(pgid, tag, false)
		if waitGroupDead(pgid, 3*time.Second) {
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
