package gaterun

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type probeAnswer struct {
	exact identity.Exact
	live  identity.Liveness
	err   error
}

type weightProber map[int64]probeAnswer

func (p weightProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	answer, ok := p[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return answer.exact, answer.live, answer.err
}

func alive(pid, second, ticks int64, boot string) probeAnswer {
	return probeAnswer{exact: identity.Exact{Pid: pid, StartedAt: time.Unix(second, 0), StartTicks: ticks, BootID: boot}, live: identity.Alive}
}

func fixedClock(t *testing.T, at *time.Time) {
	t.Helper()
	prior := weightNow
	weightNow = func() time.Time { return at.UTC() }
	t.Cleanup(func() { weightNow = prior })
}

func add(t *testing.T, root, commit, row string) WeightState {
	t.Helper()
	state, _, err := WeightAdd(root, commit, []byte(row), "", 60)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func openCheckpoint(t *testing.T, root, run, subject, envelope string, pid int64, prober identity.Prober) CheckpointResult {
	t.Helper()
	result, err := WeightCheckpointOpen(root, CheckpointRequest{
		RunID: run, Subject: subject, RunnerPID: pid, RepairDestination: envelope,
	}, prober)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLandingWeightUsesLandingProjectionAndNULPaths(t *testing.T) {
	rows := strings.Join([]string{
		"10\t2\tinternal/covenant/covenant.go",
		"40\t0\tinternal/covenant/covenant_test.go",
		"6\t1\tscripts/validate-metasystem.sh",
		"9\t0\tplans/goals.md",
		"1\t1\tdocs/white space\n$meta;name.md",
	}, "\x00") + "\x00"
	weight, err := LandingWeight([]byte(rows), "")
	if err != nil {
		t.Fatal(err)
	}
	if weight != 6 {
		t.Fatalf("LANDING projection weight = %d, want 6", weight)
	}
	nested, err := LandingWeight([]byte("4\t0\tmetasystem/plans/goals.md\x001\t0\tmetasystem/docs/x.md\x001\t0\tbenchmark/extra.go\x00"), "metasystem/")
	if err != nil || nested != 4 {
		t.Fatalf("nested projection weight = %d, %v; want 4", nested, err)
	}
}

func TestStableSiblingLockSurvivesStateRename(t *testing.T) {
	root := t.TempDir()
	lock, err := acquireWeightLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	if err := os.WriteFile(weightPath(root)+".new", []byte(`{"sinceUtc":"2026-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(weightPath(root)+".new", weightPath(root)); err != nil {
		t.Fatal(err)
	}
	other, err := os.OpenFile(WeightLockPath(root), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := unix.Flock(int(other.Fd()), unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("sibling lock stopped excluding after state rename: %v", err)
	}
	if WeightLockPath(root) == weightPath(root) {
		t.Fatal("state inode was used as the lock")
	}
}

func TestWeightLockHelperProcess(t *testing.T) {
	if os.Getenv("METASYSTEM_WEIGHT_LOCK_HELPER") != "1" {
		return
	}
	root := os.Getenv("METASYSTEM_WEIGHT_LOCK_ROOT")
	commit := os.Getenv("METASYSTEM_WEIGHT_LOCK_COMMIT")
	if root == "" || commit == "" {
		t.Fatal("weight lock helper environment is incomplete")
	}
	if signal := os.Getenv("METASYSTEM_WEIGHT_LOCK_SIGNAL"); signal != "" {
		release := os.Getenv("METASYSTEM_WEIGHT_LOCK_RELEASE")
		priorWriter := writeWeightState
		writeWeightState = func(root string, state WeightState) error {
			if err := os.WriteFile(signal, []byte("locked\n"), 0o600); err != nil {
				return err
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(release); err == nil {
					break
				}
				if time.Now().After(deadline) {
					return errors.New("helper timed out waiting for release")
				}
				time.Sleep(10 * time.Millisecond)
			}
			return priorWriter(root, state)
		}
	}
	if _, _, err := WeightAdd(root, commit, []byte("1\t0\tdocs/a.md\x00"), "", 60); err != nil {
		t.Fatal(err)
	}
}

func TestStableSiblingLockSerializesSeparateProcesses(t *testing.T) {
	root := t.TempDir()
	signal := filepath.Join(root, "holder-locked")
	release := filepath.Join(root, "release-holder")
	startHelper := func(commit string, hold bool, output *strings.Builder) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=^TestWeightLockHelperProcess$")
		command.Env = append(os.Environ(),
			"METASYSTEM_WEIGHT_LOCK_HELPER=1",
			"METASYSTEM_WEIGHT_LOCK_ROOT="+root,
			"METASYSTEM_WEIGHT_LOCK_COMMIT="+commit,
		)
		if hold {
			command.Env = append(command.Env,
				"METASYSTEM_WEIGHT_LOCK_SIGNAL="+signal,
				"METASYSTEM_WEIGHT_LOCK_RELEASE="+release,
			)
		}
		command.Stdout, command.Stderr = output, output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		return command
	}
	var holderOutput, followerOutput strings.Builder
	holder := startHelper("holder", true, &holderOutput)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(signal); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = holder.Process.Kill()
			t.Fatalf("holder never acquired the cross-process lock:\n%s", holderOutput.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	follower := startHelper("follower", false, &followerOutput)
	followerDone := make(chan error, 1)
	go func() { followerDone <- follower.Wait() }()
	select {
	case err := <-followerDone:
		_ = os.WriteFile(release, []byte("release\n"), 0o600)
		_ = holder.Wait()
		t.Fatalf("second process crossed a held flock: %v\n%s", err, followerOutput.String())
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.WriteFile(release, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatalf("holder failed: %v\n%s", err, holderOutput.String())
	}
	if err := <-followerDone; err != nil {
		t.Fatalf("follower failed after release: %v\n%s", err, followerOutput.String())
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Generation != 2 || state.Landings != 2 || state.Accumulated != 2 {
		t.Fatalf("serialized process additions interleaved or were lost: %+v %v", state, err)
	}
}

func TestCheckpointResetPreservesConcurrentAddsAndProvenance(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	state := add(t, root, "subject", "10\t2\tinternal/a.go\x00")
	if state.Generation != 1 || state.Accumulated != 3 || state.Landings != 1 {
		t.Fatalf("initial add: %+v", state)
	}
	prober := weightProber{100: alive(100, 1000, 55, "boot-a")}
	checkpoint := openCheckpoint(t, root, "run-1", "subject", envelope, 100, prober)
	if checkpoint.Checkpoint.OpenedGeneration != 2 || checkpoint.Checkpoint.Runner.StartTicks != 55 {
		t.Fatalf("checkpoint identity/generation incomplete: %+v", checkpoint)
	}
	now = now.Add(time.Minute)
	state = add(t, root, "newer", "0\t0\tplans/goals.md\x00")
	post := state.PostCheckpointSinceUTC
	if post != now.Format(time.RFC3339) || state.Generation != 3 || state.LastCommit != "newer" {
		t.Fatalf("zero-weight post-checkpoint add not recorded: %+v", state)
	}
	now = now.Add(time.Minute)
	state = add(t, root, "newest", "1\t0\tdocs/after.md\x00")
	if state.PostCheckpointSinceUTC != post {
		t.Fatalf("later add replaced first post-checkpoint time: %+v", state)
	}
	state, report, err := WeightReset(root, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Accumulated != 1 || state.Landings != 2 || state.SinceUTC != post {
		t.Fatalf("concurrent additions were not preserved: %+v", state)
	}
	if state.LastCommit != "newest" || report.Subject != "subject" || report.LastCommit != "newest" {
		t.Fatalf("reset confused subject with newest landing: state=%+v report=%+v", state, report)
	}
	if state.Generation != 6 || report.ResetGeneration != 5 {
		t.Fatalf("reset mutations did not each advance one generation: state=%+v report=%+v", state, report)
	}
	if _, err := os.Stat(filepath.Join(envelope, "reset.json")); err != nil {
		t.Fatal(err)
	}
	staleState, _, err := WeightReset(root, "run-1")
	if !errors.Is(err, ErrStaleCheckpoint) {
		t.Fatalf("consumed checkpoint reset did not refuse stale: %v", err)
	}
	if staleState.Generation != state.Generation || staleState.Accumulated != state.Accumulated {
		t.Fatalf("stale reset mutated state: before=%+v after=%+v", state, staleState)
	}
}

func TestFullResetRestartsWindowWithoutChangingLastCommit(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "landing", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "run", "subject", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	now = now.Add(time.Hour)
	state, _, err := WeightReset(root, "run")
	if err != nil {
		t.Fatal(err)
	}
	if state.Accumulated != 0 || state.Landings != 0 || state.SinceUTC != now.Format(time.RFC3339) || state.LastCommit != "landing" {
		t.Fatalf("full reset violated state ownership: %+v", state)
	}
}

func TestCheckpointLivenessAndSupersession(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	prober := weightProber{10: alive(10, 100, 9, "boot"), 20: alive(20, 200, 10, "boot")}
	openCheckpoint(t, root, "first", "one", t.TempDir(), 10, prober)
	add(t, root, "two", "1\t0\tdocs/b.md\x00")
	if _, err := WeightCheckpointOpen(root, CheckpointRequest{RunID: "second", Subject: "one", RunnerPID: 20, RepairDestination: t.TempDir()}, prober); !errors.Is(err, ErrCheckpointLive) {
		t.Fatalf("live runner did not block second checkpoint: %v", err)
	}
	prober[10] = probeAnswer{live: identity.Unknown, err: errors.New("permission denied")}
	if _, err := WeightCheckpointOpen(root, CheckpointRequest{RunID: "second", Subject: "one", RunnerPID: 20, RepairDestination: t.TempDir()}, prober); !errors.Is(err, ErrCheckpointUnknown) {
		t.Fatalf("unknown runner authorized supersession: %v", err)
	}
	// The pid exists but its start identity differs: AliveRef treats reuse as
	// the recorded runner being dead.
	prober[10] = alive(10, 101, 11, "boot")
	result, err := WeightCheckpointOpen(root, CheckpointRequest{RunID: "second", Subject: "one", RunnerPID: 20, RepairDestination: t.TempDir()}, prober)
	if err != nil {
		t.Fatal(err)
	}
	if result.Superseded == nil || result.Superseded.RunID != "first" || result.State.Accumulated != 2 {
		t.Fatalf("dead-runner supersession lost transition or weight: %+v", result)
	}
	if result.State.Generation != 4 || result.State.PostCheckpointSinceUTC != "" {
		t.Fatalf("supersession did not make one clean checkpoint mutation: %+v", result.State)
	}
}

func TestAbandonmentRetiresCheckpointTimestampAndPreservesWeight(t *testing.T) {
	root, firstEnvelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 13, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	prober := weightProber{1: alive(1, 10, 0, ""), 2: alive(2, 20, 0, "")}
	openCheckpoint(t, root, "red", "one", firstEnvelope, 1, prober)
	now = now.Add(time.Minute)
	add(t, root, "two", "1\t0\tdocs/b.md\x00")
	result, err := WeightAbandon(root, "red", "validation-red", false)
	if err != nil || !result.AppendixPublished {
		t.Fatalf("abandonment did not publish: %+v %v", result, err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Accumulated != 2 || state.Checkpoint != nil || state.PostCheckpointSinceUTC != "" {
		t.Fatalf("abandonment changed weight or left checkpoint lifetime state: %+v %v", state, err)
	}
	if state.Generation != 5 {
		t.Fatalf("abandonment generation = %d, want 5", state.Generation)
	}
	second := openCheckpoint(t, root, "next", "two", t.TempDir(), 2, prober)
	if second.State.PostCheckpointSinceUTC != "" {
		t.Fatalf("later checkpoint inherited prior lifetime timestamp: %+v", second.State)
	}
}

func TestBestEffortAbandonmentClearsAfterAppendixFailure(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "copy-failed", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorPublisher := publishWeightAppendix
	publishWeightAppendix = func(string, any) error { return errors.New("injected copy destination failure") }
	result, err := WeightAbandon(root, "copy-failed", "evidence-copy-failed", true)
	publishWeightAppendix = priorPublisher
	if err != nil || result.AppendixPublished || result.AppendixError == "" {
		t.Fatalf("best-effort abandonment result = %+v, %v", result, err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Checkpoint != nil || state.Accumulated != 1 || state.Generation != 4 {
		t.Fatalf("best-effort abandonment did not preserve and unblock: %+v %v", state, err)
	}
}

func TestAbandonmentStateWriteFailureLeavesOpenCheckpointAndNoTerminalAppendix(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "red", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorWriter := writeWeightState
	writeWeightState = func(string, WeightState) error { return errors.New("injected abandonment state write") }
	_, err := WeightAbandon(root, "red", "validation-red", false)
	writeWeightState = priorWriter
	if err == nil {
		t.Fatal("injected abandonment state failure passed")
	}
	if _, err := os.Stat(filepath.Join(envelope, "abandoned.json")); !os.IsNotExist(err) {
		t.Fatalf("terminal appendix appeared before the state transition: %v", err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Checkpoint == nil || state.Checkpoint.RunID != "red" || state.PendingAbandon != nil {
		t.Fatalf("failed abandonment did not leave one OPEN story: %+v %v", state, err)
	}
}

func TestAbandonmentAppendixFailureLeavesRepairableTerminalState(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "red", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorPublisher := publishWeightAppendix
	publishWeightAppendix = func(string, any) error { return errors.New("injected abandonment appendix failure") }
	result, err := WeightAbandon(root, "red", "validation-red", false)
	publishWeightAppendix = priorPublisher
	if err == nil || result.AppendixPublished || result.AppendixError == "" {
		t.Fatalf("appendix failure result = %+v, %v", result, err)
	}
	data, readErr := os.ReadFile(weightPath(root))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var persisted WeightState
	if err := json.Unmarshal(data, &persisted); err != nil || persisted.Checkpoint != nil || persisted.PendingAbandon == nil {
		t.Fatalf("appendix failure did not persist one terminal repair story: %+v %v", persisted, err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Checkpoint != nil || state.PendingAbandon != nil {
		t.Fatalf("abandonment read-side repair failed: %+v %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(envelope, "abandoned.json")); err != nil {
		t.Fatal(err)
	}
}

func TestPublishedAbandonmentWithCleanupFailureHasOneTerminalStory(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	prober := weightProber{1: alive(1, 10, 0, ""), 2: alive(2, 20, 0, "")}
	openCheckpoint(t, root, "red", "one", envelope, 1, prober)
	priorWriter := writeWeightState
	writes := 0
	writeWeightState = func(root string, state WeightState) error {
		writes++
		if writes == 2 {
			return errors.New("injected abandonment cleanup write")
		}
		return priorWriter(root, state)
	}
	result, err := WeightAbandon(root, "red", "validation-red", false)
	writeWeightState = priorWriter
	if err == nil || !result.AppendixPublished {
		t.Fatalf("cleanup failure result = %+v, %v", result, err)
	}
	data, readErr := os.ReadFile(weightPath(root))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var persisted WeightState
	if err := json.Unmarshal(data, &persisted); err != nil || persisted.Checkpoint != nil || persisted.PendingAbandon == nil {
		t.Fatalf("published abandonment lost its terminal repair record: %+v %v", persisted, err)
	}
	if _, err := os.Stat(filepath.Join(envelope, "abandoned.json")); err != nil {
		t.Fatal(err)
	}
	next, err := WeightCheckpointOpen(root, CheckpointRequest{
		RunID: "next", Subject: "one", RunnerPID: 2, RepairDestination: t.TempDir(),
	}, prober)
	if err != nil || next.State.PendingAbandon != nil || next.Checkpoint.RunID != "next" {
		t.Fatalf("new checkpoint did not finish terminal repair first: %+v %v", next, err)
	}
}

func TestAddDuringPendingAbandonmentRoundTrips(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "red", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorWriter := writeWeightState
	writes := 0
	writeWeightState = func(root string, state WeightState) error {
		writes++
		if writes == 2 {
			return errors.New("injected abandonment cleanup write")
		}
		return priorWriter(root, state)
	}
	result, err := WeightAbandon(root, "red", "validation-red", false)
	writeWeightState = priorWriter
	if err == nil || !result.AppendixPublished {
		t.Fatalf("cleanup failure result = %+v, %v", result, err)
	}

	added, _, err := WeightAdd(root, "two", []byte("1\t0\tdocs/b.md\x00"), "", 60)
	if err != nil || added.PendingAbandon == nil || added.Accumulated != 2 || added.Landings != 2 || added.Generation != 4 {
		t.Fatalf("add did not fold around pending abandonment: %+v %v", added, err)
	}
	loaded, err := loadWeightLocked(root, weightNow())
	if err != nil || loaded.PendingAbandon == nil || loaded.Accumulated != 2 || loaded.Landings != 2 || loaded.LastCommit != "two" {
		t.Fatalf("folded pending abandonment did not round-trip: %+v %v", loaded, err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.PendingAbandon != nil || state.Accumulated != 2 || state.Landings != 2 || state.LastCommit != "two" {
		t.Fatalf("read-side repair lost the folded landing: %+v %v", state, err)
	}
}

func TestMalformedStateRefusesUnchanged(t *testing.T) {
	root := t.TempDir()
	path := weightPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"accumulated":`)
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WeightCheck(root, 60); err == nil {
		t.Fatal("malformed state read as zero")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("malformed state changed: %q %v", got, err)
	}
}

func TestSyntacticallyValidCorruptStateRefusesUnchanged(t *testing.T) {
	now := "2026-08-25T13:00:00Z"
	tests := []struct {
		name  string
		state WeightState
	}{
		{
			name: "positive accumulator without a landing",
			state: WeightState{
				Generation: 1, Accumulated: 1, SinceUTC: now,
			},
		},
		{
			name: "checkpoint accumulator without checkpoint landings",
			state: WeightState{
				Generation: 2, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
				Checkpoint: &WeightCheckpoint{
					RunID: "run", Subject: "one", OpenedGeneration: 2, Accumulated: 1,
					OpenedAtUTC: now, Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: t.TempDir(),
				},
			},
		},
		{
			name: "checkpoint generation and landing provenance disagree",
			state: WeightState{
				Generation: 4, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "two", PostCheckpointSinceUTC: now,
				Checkpoint: &WeightCheckpoint{
					RunID: "run", Subject: "one", OpenedGeneration: 2, Accumulated: 1, Landings: 1,
					OpenedAtUTC: now, Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: t.TempDir(),
				},
			},
		},
		{
			name: "post-checkpoint timestamp without a later landing",
			state: WeightState{
				Generation: 2, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one", PostCheckpointSinceUTC: now,
				Checkpoint: &WeightCheckpoint{
					RunID: "run", Subject: "one", OpenedGeneration: 2, Accumulated: 1, Landings: 1,
					OpenedAtUTC: now, Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: t.TempDir(),
				},
			},
		},
		{
			name: "relative checkpoint repair destination",
			state: WeightState{
				Generation: 2, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
				Checkpoint: &WeightCheckpoint{
					RunID: "run", Subject: "one", OpenedGeneration: 2, Accumulated: 1, Landings: 1,
					OpenedAtUTC: now, Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: "relative",
				},
			},
		},
		{
			name: "pending reset remainder accumulator without landings",
			state: WeightState{
				Generation: 3, Accumulated: 1, SinceUTC: now, LastCommit: "one",
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 2, ResetGeneration: 3,
						ResetAtUTC: now, CheckpointAccumulated: 1, CheckpointLandings: 1,
						RemainingAccumulated: 1, RemainingSinceUTC: now, LastCommit: "one",
					},
				},
			},
		},
		{
			name: "pending reset generation and landing provenance disagree",
			state: WeightState{
				Generation: 6, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "two",
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 2, ResetGeneration: 3,
						ResetAtUTC: now, CheckpointAccumulated: 1, CheckpointLandings: 1,
						RemainingSinceUTC: now, LastCommit: "one",
					},
				},
			},
		},
		{
			name: "pending reset counts exceed state",
			state: WeightState{
				Generation: 5, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 3, ResetGeneration: 5,
						ResetAtUTC: now, CheckpointAccumulated: 1, CheckpointLandings: 1,
						RemainingAccumulated: 2, RemainingLandings: 1, RemainingSinceUTC: now,
					},
				},
			},
		},
		{
			name: "pending reset counts do not equal state",
			state: WeightState{
				Generation: 5, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "two",
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 3, ResetGeneration: 5,
						ResetAtUTC: now, CheckpointAccumulated: 1, CheckpointLandings: 1,
						RemainingAccumulated: 1, RemainingLandings: 1, RemainingSinceUTC: now, LastCommit: "one",
					},
				},
			},
		},
		{
			name: "pending reset generation predates checkpoint",
			state: WeightState{
				Generation: 5, Accumulated: 0, Landings: 0, SinceUTC: now,
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 5, ResetGeneration: 4,
						ResetAtUTC: now, RemainingSinceUTC: now,
					},
				},
			},
		},
		{
			name: "pending abandonment weight without landings",
			state: WeightState{
				Generation: 3, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
				PendingAbandon: &PendingAbandon{
					Destination: t.TempDir(),
					Result: AbandonResult{
						RunID: "red", Subject: "one", Reason: "red", AbandonedAtUTC: now,
						Generation: 3, WeightPreserved: 1, AppendixPublished: true,
					},
				},
			},
		},
		{
			name: "pending abandonment claims unpublished result",
			state: WeightState{
				Generation: 3, Accumulated: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
				PendingAbandon: &PendingAbandon{
					Destination: t.TempDir(),
					Result: AbandonResult{
						RunID: "red", Subject: "one", Reason: "red", AbandonedAtUTC: now,
						Generation: 3, WeightPreserved: 1, LandingsPreserved: 1,
					},
				},
			},
		},
		{
			name: "pending abandonment counts do not equal state",
			state: WeightState{
				Generation: 3, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "two",
				PendingAbandon: &PendingAbandon{
					Destination: t.TempDir(),
					Result: AbandonResult{
						RunID: "red", Subject: "one", Reason: "red", AbandonedAtUTC: now,
						Generation: 3, WeightPreserved: 1, LandingsPreserved: 1, AppendixPublished: true,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := weightPath(root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(test.state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := WeightCheck(root, 60); err == nil {
				t.Fatal("materially corrupt state was accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(want) {
				t.Fatalf("refused state changed: %q %v", got, err)
			}
		})
	}
}

func TestValidCrossFieldAccumulatorAndTerminalProvenance(t *testing.T) {
	now := "2026-08-25T13:00:00Z"
	tests := []struct {
		name  string
		state WeightState
	}{
		{
			name: "zero-weight landing",
			state: WeightState{
				Generation: 1, Landings: 1, SinceUTC: now, LastCommit: "one",
			},
		},
		{
			name: "open checkpoint with one later landing",
			state: WeightState{
				Generation: 3, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "two", PostCheckpointSinceUTC: now,
				Checkpoint: &WeightCheckpoint{
					RunID: "run", Subject: "one", OpenedGeneration: 2, Accumulated: 1, Landings: 1,
					OpenedAtUTC: now, Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: t.TempDir(),
				},
			},
		},
		{
			name: "pending reset with two later landings",
			state: WeightState{
				Generation: 5, Accumulated: 2, Landings: 2, SinceUTC: now, LastCommit: "three",
				PendingReset: &PendingReset{
					Destination: t.TempDir(),
					Result: ResetResult{
						RunID: "run", Subject: "one", CheckpointGeneration: 2, ResetGeneration: 3,
						ResetAtUTC: now, CheckpointAccumulated: 1, CheckpointLandings: 1,
						RemainingSinceUTC: now, LastCommit: "one",
					},
				},
			},
		},
		{
			name: "pending abandonment with two later landings",
			state: WeightState{
				Generation: 5, Accumulated: 3, Landings: 3, SinceUTC: now, LastCommit: "three",
				PendingAbandon: &PendingAbandon{
					Destination: t.TempDir(),
					Result: AbandonResult{
						RunID: "red", Subject: "one", Reason: "red", AbandonedAtUTC: now,
						Generation: 3, WeightPreserved: 1, LandingsPreserved: 1, AppendixPublished: true,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateWeightState(test.state); err != nil {
				t.Fatalf("valid state refused: %v", err)
			}
		})
	}
}

func TestResetWriteFailureLeavesCheckpointOpen(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "run", "one", t.TempDir(), 1, weightProber{1: alive(1, 10, 0, "")})
	priorWriter := writeWeightState
	writeWeightState = func(string, WeightState) error { return errors.New("injected reset write") }
	_, _, err := WeightReset(root, "run")
	writeWeightState = priorWriter
	if err == nil {
		t.Fatal("injected reset write passed")
	}
	state, _, checkErr := WeightCheck(root, 60)
	if checkErr != nil || state.Checkpoint == nil || state.Accumulated != 1 {
		t.Fatalf("failed reset changed disk state: %+v %v", state, checkErr)
	}
}

func TestDurabilityUnknownResetStateReportsFailureAndRepairs(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "run", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})

	priorWriter := writeWeightState
	injected := false
	writeWeightState = func(root string, state WeightState) error {
		if err := priorWriter(root, state); err != nil {
			return err
		}
		if !injected {
			injected = true
			return errors.New("injected post-rename durability uncertainty")
		}
		return nil
	}
	t.Cleanup(func() { writeWeightState = priorWriter })

	if _, _, err := WeightReset(root, "run"); err == nil {
		t.Fatal("durability-unknown reset reported success")
	}
	writeWeightState = priorWriter

	persisted, err := loadWeightLocked(root, weightNow())
	if err != nil || persisted.Checkpoint != nil || persisted.PendingReset == nil || persisted.Accumulated != 0 {
		t.Fatalf("published durability-unknown state is not a valid pending reset: %+v %v", persisted, err)
	}
	state, _, err := WeightCheck(root, 60)
	if err != nil || state.Checkpoint != nil || state.PendingReset != nil || state.Accumulated != 0 {
		t.Fatalf("read-side repair did not reconcile the durability-unknown reset: %+v %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(envelope, "reset.json")); err != nil {
		t.Fatalf("read-side repair did not publish reset.json: %v", err)
	}
}

func TestMissingResetAppendixRepairsOnReadWithoutSecondSubtraction(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "run", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorPublisher := publishWeightAppendix
	publishWeightAppendix = func(string, any) error { return errors.New("injected appendix failure") }
	state, _, err := WeightReset(root, "run")
	if state.PendingReset == nil {
		t.Fatalf("failed appendix lost replay data: %+v", state)
	}
	var pending *ResetAppendixPendingError
	if !errors.As(err, &pending) {
		t.Fatalf("wrong reset failure: %v", err)
	}
	if _, _, repairErr := WeightCheck(root, 60); repairErr == nil {
		t.Fatal("failed read-side repair did not report")
	}
	publishWeightAppendix = priorPublisher
	state, _, err = WeightAdd(root, "two", []byte("1\t0\tdocs/b.md\x00"), "", 60)
	if err != nil || state.PendingReset == nil || state.Accumulated != 1 {
		t.Fatalf("landing overwrote pending reset replay data: %+v %v", state, err)
	}
	state, _, err = WeightCheck(root, 60)
	if err != nil || state.PendingReset != nil || state.Accumulated != 1 || state.LastCommit != "two" {
		t.Fatalf("read-side repair failed or subtracted again: %+v %v", state, err)
	}
	if state.Generation != 5 {
		t.Fatalf("repair cleanup generation = %d, want 5", state.Generation)
	}
	generation := state.Generation
	state, _, err = WeightCheck(root, 60)
	if err != nil || state.Generation != generation || state.Accumulated != 1 {
		t.Fatalf("repeated repair was not idempotent: %+v %v", state, err)
	}
}

func TestConflictingResetAppendixLeavesRepairForRetry(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	openCheckpoint(t, root, "run", "one", envelope, 1, weightProber{1: alive(1, 10, 0, "")})
	priorPublisher := publishWeightAppendix
	publishWeightAppendix = func(string, any) error { return errors.New("injected first publication failure") }
	if _, _, err := WeightReset(root, "run"); err == nil {
		t.Fatal("fixture reset unexpectedly published")
	}
	publishWeightAppendix = priorPublisher
	appendix := filepath.Join(envelope, "reset.json")
	if err := os.WriteFile(appendix, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WeightCheck(root, 60); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflicting reset facts were accepted: %v", err)
	}
	data, err := os.ReadFile(weightPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var state WeightState
	if err := json.Unmarshal(data, &state); err != nil || state.PendingReset == nil {
		t.Fatalf("failed repair discarded replay data: %+v %v", state, err)
	}
	if err := os.Remove(appendix); err != nil {
		t.Fatal(err)
	}
	state, _, err = WeightCheck(root, 60)
	if err != nil || state.PendingReset != nil {
		t.Fatalf("later retry did not repair: %+v %v", state, err)
	}
}

func TestPublishedAppendixWithPendingCleanupRepairsBeforeNextCheckpoint(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 16, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	add(t, root, "one", "1\t0\tdocs/a.md\x00")
	prober := weightProber{1: alive(1, 10, 0, ""), 2: alive(2, 20, 0, "")}
	openCheckpoint(t, root, "run", "one", envelope, 1, prober)
	priorWriter := writeWeightState
	writes := 0
	writeWeightState = func(root string, state WeightState) error {
		writes++
		if writes == 2 {
			return errors.New("injected pending cleanup write")
		}
		return priorWriter(root, state)
	}
	state, _, err := WeightReset(root, "run")
	writeWeightState = priorWriter
	if state.PendingReset == nil || err == nil {
		t.Fatalf("cleanup failure did not retain repair record: %+v %v", state, err)
	}
	result, err := WeightCheckpointOpen(root, CheckpointRequest{RunID: "next", Subject: "one", RunnerPID: 2, RepairDestination: t.TempDir()}, prober)
	if err != nil || result.State.PendingReset != nil || result.Checkpoint.RunID != "next" {
		t.Fatalf("new checkpoint overwrote repair before completing it: %+v %v", result, err)
	}
}

func TestPartialResetWithoutPostTimestampFallsBackToResetTime(t *testing.T) {
	root, envelope := t.TempDir(), t.TempDir()
	now := time.Date(2026, 8, 25, 17, 0, 0, 0, time.UTC)
	fixedClock(t, &now)
	state := WeightState{
		Generation: 4, Accumulated: 2, Landings: 2, SinceUTC: now.Add(-time.Hour).Format(time.RFC3339), LastCommit: "newer",
		Checkpoint: &WeightCheckpoint{
			RunID: "legacy", Subject: "subject", OpenedGeneration: 3, Accumulated: 1, Landings: 1,
			OpenedAtUTC: now.Add(-30 * time.Minute).Format(time.RFC3339), Runner: RunnerIdentity{PID: 1, StartedAtSec: 1}, RepairDestination: envelope,
		},
	}
	data, _ := json.Marshal(state)
	if err := os.MkdirAll(filepath.Dir(weightPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(weightPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	reset, _, err := WeightReset(root, "legacy")
	if err != nil || reset.SinceUTC != now.Format(time.RFC3339) || reset.Accumulated != 1 {
		t.Fatalf("fallback reset time not adopted: %+v %v", reset, err)
	}
}
