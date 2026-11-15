package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// landingRunBed is a registered lane whose landing agent's keeper is the
// production keeper on a real launch manager whose supervisor is a stand-in:
// a start records a running landing launch and no process runs.
func landingRunBed(t *testing.T) (*laneVerbBed, launch.Store) {
	t.Helper()
	bed := newLaneVerbBed(t)
	bed.repoTop = func(string) (string, error) { return bed.landingA, nil }
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 {
		t.Fatalf("set = %d %q %q", code, stdout, stderr)
	}
	module := filepath.Join(bed.landingA, "metasystem")
	for name, content := range map[string]string{"go.mod": "module fixture\n", "metasystem.conf": "metasystem.template=true\n", "metasystem.conf.local": "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\n"} {
		if err := os.WriteFile(filepath.Join(module, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude", ProjectsRoot: filepath.Join(t.TempDir(), "projects")}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return laneTestNow }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return bed.home, nil })}
	bed.plainProve = plain.ProveSeams{Now: func() time.Time { return laneTestNow },
		Git: func(_ string, args ...string) (string, error) {
			if args[0] == "rev-parse" && !strings.Contains(args[len(args)-1], "MERGE_HEAD") {
				return "main", nil
			}
			return "", nil
		},
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
	}
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.proofEffects = bed.plainProve
		agent.machine = func(string) (string, error) { return "landing", nil }
		agent.manager = func() *launch.Manager { return manager }
		agent.now = func() time.Time { return laneTestNow }
		agent.nonce = func() (string, error) { return "0011223344556677", nil }
	})
	bed.ownerProbe = agent.probe
	bed.keeper = func(home, root string) lane.AgentKeeper {
		keeper := newLandingAgentKeeper(root, home, agent)
		keeper.Sources.Reasons = func(string) ([]string, error) { return bed.wake, nil }
		return keeper
	}
	// landing run runs its step in the lane checkout; from anywhere else
	// it hands off to the lane's own engine.
	bed.cwd = bed.landingA
	return bed, store
}

func TestLandingRunContinuesTheRecordedDelegatedBatch(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"start", "person policy", "provider", "helm", "batch", "hold time", "hold fingerprint", "hold launch", "pause", "continue", "continue provider", "continue pause", "handoff"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			if name != "person policy" {
				helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\nproof.trunk-every=1h\nproof.full=fixture-proof\n"), 0600))
			}
			b.barrenKeeper()
			b.keeper.Observe = nil
			state := lane.AgentState{Barren: 2, BarrenAt: b.now.Add(-time.Minute).Format(time.RFC3339Nano),
				BarrenFingerprint: "unchanged", BarrenLaunches: []string{"first", "last"}}
			data, err := json.Marshal(state)
			helmMust(t, err, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
			seams := b.owners.landing.plainProve
			seams.Lane = func() (lane.Record, error) { return b.record, nil }
			helmMust(t, plain.RecordBarrenStop(b.lane, state, lane.AgentStatePath(b.home), b.now.Add(-time.Minute), seams))
			helmMust(t, plain.SyncPolicyQuestion(b.lane, b.owners.landing.machine, b.now, seams))
			absent := exec.Command("/bin/sh", "-c", "exit 128").Run()
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(root string, args ...string) (string, error) {
				if args[len(args)-1] == "MERGE_HEAD^{commit}" {
					return "", absent
				}
				return git(root, args...)
			}
			b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
				return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "G1", Act: "lane-recovery", Decision: "D1", Session: "main-1"}, at)
			}
			var seen lane.AgentRun
			var batch *plain.Batch
			provider := func(string) (string, error) { return "provider is held", nil }
			b.owners.landing.runKeeper = func(keeper lane.AgentKeeper) lane.AgentRun {
				batch, err = plain.ReadBatch(b.lane)
				helmMust(t, err)
				if batch == nil || batch.Person != nil || batch.Delegated == nil {
					t.Fatalf("delegated continuation lost its authority: batch=%+v", batch)
				}
				switch name {
				case "provider", "continue provider":
					keeper.ProviderHold = provider
				case "pause", "continue pause":
					_, err := lane.SetPauseBecause(b.home, "Wido", "maintenance", b.now)
					helmMust(t, err)
				case "hold time", "hold fingerprint", "hold launch":
					changed := state
					if name == "hold time" {
						changed.BarrenAt = b.now.Format(time.RFC3339Nano)
					}
					if name == "hold fingerprint" {
						changed.BarrenFingerprint = "different"
					}
					if name == "hold launch" {
						changed.BarrenLaunches = []string{"other"}
					}
					data, err := json.Marshal(changed)
					helmMust(t, err, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
					keeper.Fingerprint = func(string) (string, error) { return changed.BarrenFingerprint, nil }
				}
				if strings.HasPrefix(name, "continue") {
					keeper.PersonAct = true
					keeper.PersonSelection = func(lane.Record) bool { return true }
					keeper.Continuation = func(lane.Record) string { return "person-batch" }
					seen = plain.ContinueDelegated(b.lane, b.record, keeper)
				} else {
					seen = keeper.Run()
				}
				return seen
			}
			if name == "helm" {
				b.owners.landing.helm = func(string) helm.State { return helm.State{Active: true} }
			}
			cwd := b.lane
			if name == "handoff" {
				cwd = b.seat
				binary := filepath.Join(b.lane, "bin", "metasystem")
				helmMust(t, os.MkdirAll(filepath.Dir(binary), 0700))
				helmMust(t, testexec.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0700))
			}
			code, out := b.run(t, cwd, "landing", "run", "--goals", "a,b")
			saved, err := plain.ReadBatch(b.lane)
			helmMust(t, err)
			if saved == nil || saved.Delegated == nil || saved.Person != nil {
				t.Fatalf("selection missing: %d %s %+v", code, out, saved)
			}
			switch name {
			case "helm":
				if code == 0 || b.starts != 0 || !strings.Contains(out, "at the helm") {
					t.Fatalf("helm bypassed: %d %s starts=%d", code, out, b.starts)
				}
			case "handoff":
				if code != 0 || b.starts != 0 || !strings.Contains(out, "landing run --batch "+saved.ID) {
					t.Fatalf("handoff lost batch: %d %s", code, out)
				}
			case "provider", "continue provider":
				if b.starts != 0 || seen.Outcome != lane.AgentHeld || !strings.Contains(out, "provider is held") {
					t.Fatalf("provider bypassed: %d %s %+v", code, out, seen)
				}
			case "pause", "continue pause":
				if b.starts != 0 || seen.Outcome != lane.AgentPaused {
					t.Fatalf("pause bypassed: %d %s %+v", code, out, seen)
				}
			case "hold time", "hold fingerprint", "hold launch":
				if b.starts != 0 || seen.Outcome != lane.AgentHeld {
					t.Fatalf("different episode cleared: %d %s %+v", code, out, seen)
				}
			case "person policy":
				if b.starts != 0 || seen.Outcome != lane.AgentHeld || !strings.Contains(out, "person's answer") {
					t.Fatalf("person selection policy bypassed: %d %s %+v", code, out, seen)
				}
			default:
				if code != 0 || b.starts != 1 || seen.Outcome != lane.AgentStarted {
					t.Fatalf("delegated batch not started: %d %s %+v starts=%d", code, out, seen, b.starts)
				}
				if name == "batch" {
					code, out = b.run(t, b.lane, "landing", "run", "--batch", saved.ID)
					if code != 0 || b.starts != 2 {
						t.Fatalf("recorded batch refused: %d %s starts=%d", code, out, b.starts)
					}
				}
			}
		})
	}
}

func landingLaunches(t *testing.T, store launch.Store) []launch.Record {
	t.Helper()
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

// landing run with queued work and no agent alive starts one landing agent
// through the keeper and names it; a repeat while it is alive is success
// that starts no second one (R-129).
func TestLandingRunStartsTheAgentOnceForQueuedWork(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	bed.wake = []string{"queued"}
	code, stdout, stderr := bed.run(t, "landing", "run")
	launches := landingLaunches(t, store)
	if code != 0 || len(launches) != 1 || launches[0].Kind != launch.LandingKind {
		t.Fatalf("landing run = %d %q %q, launches %+v; want one landing launch", code, stdout, stderr, launches)
	}
	if !strings.Contains(oneSpaced(stdout), "started the landing agent "+launches[0].ID+" for queued work") {
		t.Fatalf("landing run said %q; want the started session %s named, for queued work", stdout, launches[0].ID)
	}
	code, stdout, stderr = bed.run(t, "landing", "run", "--json")
	var result struct {
		Outcome string
		Data    struct{ Outcome, Launch string }
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &result) != nil || result.Outcome != string(intentUnchanged) || result.Data.Outcome != string(lane.AgentRunning) || result.Data.Launch != launches[0].ID {
		t.Fatalf("landing run again = %d %q %q; want unchanged, the running agent named", code, stdout, stderr)
	}
	if launches := landingLaunches(t, store); len(launches) != 1 {
		t.Fatalf("a repeat started a second landing agent: %d launches", len(launches))
	}
	code, stdout, _ = bed.run(t, "landing", "run")
	if code != 0 || !strings.Contains(stdout, "nothing to do; it is at work") {
		t.Fatalf("landing run while alive = %d %q; want nothing to do", code, stdout)
	}
}

// The started line names each of the lane's wake reasons in words.
func TestLandingRunSaysEachWakeReasonInWords(t *testing.T) {
	t.Parallel()
	if got := wakeWords([]string{"queued", "proof-finished"}); got != "queued work and a finished test run" {
		t.Fatalf("wake words = %q", got)
	}
}

// landing run with an empty queue starts nothing and says why.
func TestLandingRunWithAnEmptyQueueHasNothingToDo(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	code, stdout, stderr := bed.run(t, "landing", "run")
	if code != 0 || !strings.Contains(stdout, "nothing to do; the lane is empty") {
		t.Fatalf("landing run of an empty lane = %d %q %q", code, stdout, stderr)
	}
	if launches := landingLaunches(t, store); len(launches) != 0 {
		t.Fatalf("an empty lane started a landing agent: %+v", launches)
	}
}

// landing run on a paused lane is refused with landing start as line 2,
// and starts nothing.
func TestLandingRunOnAPausedLaneIsRefused(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	bed.wake = []string{"queued"}
	if code, _, stderr := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 {
		t.Fatalf("stop = %d %q", code, stderr)
	}
	code, stdout, stderr := bed.run(t, "landing", "run")
	if code == 0 || !strings.Contains(stderr, "stopped by Wido") || !strings.Contains(stderr, "metasystem landing start") {
		t.Fatalf("landing run of a paused lane = %d %q %q; want refused with landing start", code, stdout, stderr)
	}
	if launches := landingLaunches(t, store); len(launches) != 0 {
		t.Fatalf("a paused lane started a landing agent: %+v", launches)
	}
}

// landing run while the lane checkout is at the helm is refused, as the
// steward skips the keeper there, with helm return as line 2.
func TestLandingRunAtTheHelmIsRefused(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	bed.wake = []string{"queued"}
	bed.helmed = true
	code, stdout, stderr := bed.run(t, "landing", "run")
	if code == 0 || !strings.Contains(stderr, "at the helm") || !strings.Contains(oneSpaced(stderr), "metasystem helm return") {
		t.Fatalf("landing run at the helm = %d %q %q; want refused with helm return", code, stdout, stderr)
	}
	_, stdout, stderr = bed.run(t, "landing", "run", "--json")
	var recorded struct{ Next *intentNext }
	if err := json.Unmarshal([]byte(stdout+stderr), &recorded); err != nil || recorded.Next == nil || !strings.Contains(recorded.Next.Reason, humanauthority.PersonActRemedy(shellCommand([]string{"metasystem", "helm", "return", "--repo", bed.landingA}))) {
		t.Fatalf("the record lost the enrolled terminal requirement: %s %s", stdout, stderr)
	}
	if launches := landingLaunches(t, store); len(launches) != 0 {
		t.Fatalf("a helmed lane started a landing agent: %+v", launches)
	}
}

// landing run from a checkout that is not the lane's hands off to the lane
// installation's own engine, run in the lane checkout, so the lane's engine
// supervises its landing agent; --json passes through and its JSON result
// is read. Without that engine it is refused with the build that makes it.
func TestLandingRunFromAnotherCheckoutExecsTheLanesEngine(t *testing.T) {
	t.Parallel()
	bed, store := landingRunBed(t)
	bed.cwd = bed.landingB
	binary := filepath.Join(bed.landingA, "metasystem", "bin", "metasystem")
	code, stdout, stderr := bed.run(t, "landing", "run")
	if code == 0 || !strings.Contains(oneSpaced(stderr), "cd "+filepath.Join(bed.landingA, "metasystem")+" &&") || !strings.Contains(oneSpaced(stderr), "go run ./cmd/devgate build") {
		t.Fatalf("landing run without the lane's engine = %d %q %q; want refused with its build", code, stdout, stderr)
	}
	record := filepath.Join(t.TempDir(), "argv")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s|%s\\n' \"$(pwd -P)\" \"$*\" > '" + record + "'\n" +
		"printf '%s\\n' '{\"schemaVersion\":1,\"verb\":\"landing run\",\"targets\":[],\"outcome\":\"confirmed\",\"summary\":\"started\",\"data\":{\"outcome\":\"started\",\"launch\":\"landing-from-the-lane\"}}'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = bed.run(t, "landing", "run", "--json")
	var result struct {
		Outcome string
		Data    struct{ Outcome, Launch string }
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &result) != nil || result.Data.Launch != "landing-from-the-lane" {
		t.Fatalf("landing run --json from another checkout = %d %q %q; want the lane engine's result", code, stdout, stderr)
	}
	seen, err := os.ReadFile(record)
	if err != nil || strings.TrimSpace(string(seen)) != bed.landingA+"|landing run --json" {
		t.Fatalf("the lane's engine ran as %q %v; want landing run --json in %s", seen, err, bed.landingA)
	}
	if launches := landingLaunches(t, store); len(launches) != 0 {
		t.Fatalf("the calling engine launched itself: %+v", launches)
	}
}
