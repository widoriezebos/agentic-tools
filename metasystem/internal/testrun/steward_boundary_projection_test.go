package testrun

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const boundaryLandingRef = "refs/remotes/origin/main"

type boundaryProbe struct{ calls int }

func (probe *boundaryProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	probe.calls++
	return identity.Exact{}, identity.Dead, nil
}

type boundaryCompletion struct {
	outcome steward.ReArmOutcome
	err     error
}

type boundaryLane struct {
	t                                                     *testing.T
	root, binary, source, tip, head                       string
	paths                                                 []string
	dirty, fetchFailure, comparisonFailure, launchFailure bool
	builds, starts, runners, comparisons                  int
	modeOnly                                              bool
	clock                                                 *scheduledRearmClock
	probe                                                 boundaryProbe
	ops                                                   steward.RearmOperations
	release                                               chan struct{}
	completed                                             chan boundaryCompletion
}

func newBoundaryLane(t *testing.T, paths ...string) *boundaryLane {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lane := &boundaryLane{t: t, root: root, source: strings.Repeat("a", 40), tip: strings.Repeat("b", 40), paths: paths, clock: newScheduledRearmClock()}
	lane.head = lane.source
	lane.binary = filepath.Join(root, "bin", "metasystem")
	lane.writeBinary(lane.source)
	data, err := os.ReadFile(lane.binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{
		RepoIdentity: root, InstallPath: lane.binary, InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(data)),
		Generation: 7, EngineBuild: lane.source, MintedAt: lane.clock.Now().Format("2006-01-02T15:04:05Z07:00"),
		Enrollment: steward.EnrollmentFixture, MintedBy: "human-terminal", HumanWitnessedGeneration: 7,
	}); err != nil {
		t.Fatal(err)
	}
	// A saved announcement still needs a process observation before replacement.
	rec, err := json.Marshal(steward.RunnerRecord{Pid: 1234, StartTicks: 12, BootID: "fixture-boot"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.json"), rec, 0o600); err != nil {
		t.Fatal(err)
	}
	lane.ops = steward.RearmOperations{Clock: lane.clock, Git: lane.git, Prober: &lane.probe,
		LaunchRunner: func(root string, pinned *steward.EnrolledBinary, lineage string) (steward.RunnerRecord, error) {
			lane.runners++
			if root != lane.root || pinned.BuildStamp() != lane.tip {
				return steward.RunnerRecord{}, errors.New("runner did not receive authenticated rebuilt bytes")
			}
			return steward.RunnerRecord{Pid: 4321}, nil
		},
	}
	return lane
}

func (lane *boundaryLane) writeBinary(stamp string) {
	lane.t.Helper()
	if err := os.MkdirAll(filepath.Dir(lane.binary), 0o700); err != nil {
		lane.t.Fatal(err)
	}
	// The installed fixture carries a real linked-stamp record. It is never executed.
	if err := testexec.WriteFile(lane.binary, []byte(enginebuild.StampRecord(stamp)), 0o700); err != nil {
		lane.t.Fatal(err)
	}
}

func (lane *boundaryLane) git(_ context.Context, root string, args ...string) ([]byte, error) {
	if root != lane.root {
		return nil, fmt.Errorf("unexpected repository %s", root)
	}
	answer := func(value string) ([]byte, error) { return []byte(value), nil }
	switch strings.Join(args, " ") {
	case "config --local --no-includes --get metasystem.steward.landing-ref":
		return answer(boundaryLandingRef)
	case "config --get metasystem.steward.rearm-resolve-seconds":
		return answer("20")
	case "config --get metasystem.steward.notify-command":
		return answer("fixture-notify")
	case "fetch --progress origin main":
		if lane.fetchFailure {
			return nil, errors.New("fixture remote unavailable")
		}
		return answer("")
	case "rev-parse --verify " + boundaryLandingRef + "^{commit}", "rev-parse --verify --quiet " + boundaryLandingRef + "^{commit}":
		return answer(lane.tip)
	case "rev-parse --verify HEAD^{commit}":
		return answer(lane.head)
	case "rev-parse --verify --quiet " + lane.source + "^{commit}":
		return answer(lane.source)
	case "rev-parse --verify --quiet " + lane.tip + "^{commit}":
		return answer(lane.tip)
	case "rev-parse --git-common-dir", "rev-parse --git-dir":
		return answer(".git")
	case "merge-base --is-ancestor " + lane.source + " " + lane.tip,
		"merge-base --is-ancestor " + lane.tip + " " + boundaryLandingRef,
		"merge-base --is-ancestor " + lane.head + " " + lane.tip:
		return answer("")
	case "diff --name-only --no-renames --no-relative -z HEAD --":
		if lane.dirty {
			return answer("internal/local.go\x00")
		}
		return answer("")
	case "ls-files --others --exclude-standard --full-name -z":
		return answer("")
	case "diff --name-only --no-renames --no-relative -z " + lane.head + " " + lane.tip + " --":
		return answer(strings.Join(lane.paths, "\x00"))
	}
	if len(args) == 11 && args[0] == "diff-tree" && args[9] == lane.source && args[10] == lane.tip {
		lane.comparisons++
		if lane.comparisonFailure {
			return nil, errors.New("fixture projection object unreadable")
		}
		var raw strings.Builder
		for _, path := range lane.paths {
			mode, newID := "100644", strings.Repeat("2", 40)
			if lane.modeOnly {
				mode, newID = "100755", strings.Repeat("1", 40)
			}
			fmt.Fprintf(&raw, ":100644 %s %s %s M\x00%s\x00", mode, strings.Repeat("1", 40), newID, path)
		}
		return answer(raw.String())
	}
	return nil, fmt.Errorf("unexpected git call: %q", args)
}

func (lane *boundaryLane) boundary() bool {
	lane.t.Helper()
	deps := stewardBoundaryDependencies(context.Background(), lane.root, "", func() (bool, error) { return true, nil }, lane.ops)
	deps.forward = func(tip string) error { lane.head = tip; return nil }
	deps.rebuild = func() error { lane.builds++; lane.writeBinary(lane.head); return nil }
	deps.start = func() error {
		lane.starts++
		if lane.launchFailure {
			return errors.New("fixture detached up launch failed")
		}
		release := make(chan struct{})
		completed := make(chan boundaryCompletion, 1)
		done := make(chan struct{})
		lane.release, lane.completed = release, completed
		lane.t.Cleanup(func() {
			select {
			case <-release:
			default:
				close(release)
			}
			<-done
		})
		go func() {
			defer close(done)
			<-release
			outcome, err := lane.ops.ReArmRebuiltEngine(lane.root, lane.root, lane.binary)
			completed <- boundaryCompletion{outcome, err}
		}()
		return nil
	}
	replaced, err := rearmStewardWithDependencies(lane.root, deps)
	if err != nil {
		lane.t.Fatal(err)
	}
	return replaced
}

func (lane *boundaryLane) enrollment() steward.InstallIdentity {
	lane.t.Helper()
	enrolled, err := steward.VerifyIdentity(steward.RepoIdentityPath(lane.root), lane.root)
	if err != nil {
		lane.t.Fatal(err)
	}
	return enrolled
}

func (lane *boundaryLane) log() string {
	lane.t.Helper()
	data, err := os.ReadFile(filepath.Join(lane.root, "artifacts", "agents", "steward", "runner.log"))
	if err != nil && !os.IsNotExist(err) {
		lane.t.Fatal(err)
	}
	return string(data)
}

func (lane *boundaryLane) assertUnchanged(diagnostic string) {
	lane.t.Helper()
	if lane.builds != 0 || lane.starts != 0 || lane.runners != 0 || lane.enrollment().Generation != 7 {
		lane.t.Fatalf("unexpected replacement: builds=%d up=%d runners=%d enrollment=%+v", lane.builds, lane.starts, lane.runners, lane.enrollment())
	}
	pending, err := steward.PendingNotifications(lane.root)
	if err != nil || len(pending) != 0 {
		lane.t.Fatalf("notification on skipped replacement: %v %v", pending, err)
	}
	log := lane.log()
	if diagnostic == "" && strings.Contains(log, "engine refresh:") || diagnostic != "" && !strings.Contains(log, diagnostic) {
		lane.t.Fatalf("diagnostic %q: log=%q", diagnostic, log)
	}
	if deferred := steward.RearmDeferredLine(lane.root); deferred != "" {
		lane.t.Fatalf("skip retained false deferral: %s", deferred)
	}
}

func (lane *boundaryLane) finishRearm() {
	lane.t.Helper()
	if lane.enrollment().Generation != 7 {
		lane.t.Fatal("generation minted before detached child ran")
	}
	close(lane.release)
	result := <-lane.completed
	if result.err != nil {
		lane.t.Fatalf("detached re-arm: %+v %v", result.outcome, result.err)
	}
	enrolled := lane.enrollment()
	if enrolled.Generation != 8 || enrolled.LandedCommit != lane.tip || enrolled.LandingRef != boundaryLandingRef || enrolled.MintedBy != "machine-rebuild" || enrolled.MintedAt != lane.clock.Now().Format("2006-01-02T15:04:05Z07:00") {
		lane.t.Fatalf("production generation publication: %+v", enrolled)
	}
	pending, err := steward.PendingNotifications(lane.root)
	if err != nil || len(pending) != 1 || pending[0].Nonce != "engine-rearm-generation-8" {
		lane.t.Fatalf("generation notice: %v %v", pending, err)
	}
	if lane.runners != 1 || lane.probe.calls == 0 || steward.RearmDeferredLine(lane.root) != "" {
		lane.t.Fatalf("runner/defer publication: launches=%d probes=%d deferred=%s", lane.runners, lane.probe.calls, steward.RearmDeferredLine(lane.root))
	}
}

func TestStewardBoundaryRecordsProjectionSkipsRebuild(t *testing.T) {
	t.Parallel()
	for _, preexisting := range []bool{false, true} {
		t.Run(fmt.Sprint(preexisting), func(t *testing.T) {
			t.Parallel()
			lane := newBoundaryLane(t, "plans/goals/fixture.md")
			if preexisting {
				if err := steward.NoteDeferredRearm(lane.root, lane.tip, lane.head, lane.clock.Now()); err != nil {
					t.Fatal(err)
				}
			}
			if lane.boundary() {
				t.Fatal("records-only boundary replaced the engine")
			}
			lane.assertUnchanged("")
			lane.tip = strings.Repeat("c", 40)
			if lane.boundary() {
				t.Fatal("second records-only boundary replaced the engine")
			}
			lane.assertUnchanged("")
			if lane.comparisons != 2 {
				t.Fatalf("ownership was not rechecked at each boundary: %d", lane.comparisons)
			}
		})
	}
}

func TestStewardBoundaryCodeProjectionPublishesGeneration(t *testing.T) {
	t.Parallel()
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint(mixed), func(t *testing.T) {
			t.Parallel()
			paths := []string{"internal/engine.go"}
			if mixed {
				paths = append(paths, "plans/goals/fixture.md")
			}
			lane := newBoundaryLane(t, paths...)
			if !lane.boundary() {
				t.Fatalf("code advance skipped rebuild: %s", lane.log())
			}
			if lane.builds != 1 || lane.starts != 1 {
				t.Fatalf("builds=%d up=%d", lane.builds, lane.starts)
			}
			lane.finishRearm()
		})
	}
}

func TestStewardBoundaryProjectionFailureMapping(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"nonownership", "comparison failure", "cached tip fetch failure", "authentication failure"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			lane := newBoundaryLane(t, "plans/goals/fixture.md")
			diagnostic := ""
			switch kind {
			case "nonownership":
				lane.paths = []string{"internal/engine.go"}
				lane.dirty = true
				diagnostic = "has local edits"
			case "comparison failure":
				lane.comparisonFailure = true
				diagnostic = "fixture projection object unreadable"
			case "cached tip fetch failure":
				lane.fetchFailure = true
				diagnostic = "could not be fetched"
			case "authentication failure":
				if err := os.Remove(lane.binary); err != nil {
					t.Fatal(err)
				}
				diagnostic = "open enrolled engine"
			}
			if lane.boundary() {
				t.Fatal("failed judgment replaced engine")
			}
			// Non-ownership retains the existing guarded re-arm deferral.
			if kind == "nonownership" {
				if steward.RearmDeferredLine(lane.root) == "" {
					t.Fatal("guarded rebuild lost its deferral")
				}
				if err := steward.ClearDeferredRearm(lane.root); err != nil {
					t.Fatal(err)
				}
			}
			lane.assertUnchanged(diagnostic)
			if kind == "cached tip fetch failure" && lane.comparisons != 0 {
				t.Fatal("accepted comparison against an unfetched tip")
			}
		})
	}
}

func TestStewardBoundaryRebuiltEngineRetriesFailedLaunch(t *testing.T) {
	t.Parallel()
	lane := newBoundaryLane(t, "internal/engine.go")
	lane.launchFailure = true
	if lane.boundary() || lane.builds != 1 || lane.starts != 1 || !strings.Contains(lane.log(), "fixture detached up launch failed") {
		t.Fatalf("first launch failure: builds=%d up=%d log=%q", lane.builds, lane.starts, lane.log())
	}
	if _, err := steward.OpenEnrolledBinary(lane.root); !errors.Is(err, steward.ErrEngineRebuilt) {
		t.Fatalf("fixture did not reach rebuilt recovery state: %v", err)
	}
	lane.launchFailure = false
	if !lane.boundary() {
		t.Fatalf("rebuilt recovery did not retry: %s", lane.log())
	}
	if lane.builds != 2 || lane.starts != 2 || lane.comparisons != 1 {
		t.Fatalf("retry counts: builds=%d up=%d comparisons=%d", lane.builds, lane.starts, lane.comparisons)
	}
	lane.finishRearm()
}

func TestStewardBoundaryExecutableBitProjectionSkipsRebuild(t *testing.T) {
	t.Parallel()
	lane := newBoundaryLane(t, "cmd/engine.go")
	lane.modeOnly = true
	if lane.boundary() {
		t.Fatal("executable bit changed engine ownership")
	}
	lane.assertUnchanged("")
	if lane.comparisons != 1 {
		t.Fatal("mode-only fixture bypassed projection comparison")
	}
}

func TestLandedRearmVerifiedOwnershipSurvivesFetchFailure(t *testing.T) {
	t.Parallel()
	facts := landedRearmFacts{SourceOwnsTip: true, FetchErr: errors.New("fixture remote unavailable")}
	root := t.TempDir()
	record, err := landedRearmAct(context.Background(), root, root, facts, 7)
	if err != nil || record != nil {
		t.Fatalf("verified ownership refused or re-armed after a fetch failure: record=%+v err=%v", record, err)
	}
}
