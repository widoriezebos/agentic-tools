package delegation_test

// Ported from dispatch-fixtures.sh cluster e (lines 4401-4911): the escalation
// approval ladder, mission-context resolution, the status exit codes and the
// reaper's mission fence ask. Git is stubbed (newBed); the scenarios whose
// owners read Git live in port_p7_integration_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
)

// p7RosterBed is a stubbed-Git bed whose configuration resolves a fake
// roster (fake:fake-model, no model tiers) and whose implementer role files
// exist, so a dispatch reaches the roster and mission decisions.
func p7RosterBed(t *testing.T) *bed {
	t.Helper()
	b := newBed(t)
	b.writeFile("metasystem.conf", dispatchBedConfig+"evidence.root="+filepath.Join(b.root, "evidence")+"\n")
	return b
}

func (b *bed) requireNoJobRecords() {
	b.t.Helper()
	records, _ := filepath.Glob(filepath.Join(b.root, "artifacts", "agents", "jobs", "*.json"))
	if len(records) != 0 {
		b.t.Fatalf("a refused dispatch left job records: %v", records)
	}
}

// no-tier-model-override (fixture line 4541): an override the roster cannot
// rank refuses with the corrective actions before any job exists.
func TestP7ModelOverrideWithoutTiersRefusesBeforeAnyJob(t *testing.T) {
	t.Parallel()
	b := p7RosterBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	result := b.runEnv(delegation.Env{DelegateInternal: true, RecordOutcome: true, FixtureHazard: "MECHANICAL"},
		"dispatch", "--role", "implementer", "--brief", brief, "--model", "fake-escalated", "--job-id", "no-tier-model")
	requireExit(t, result, 1, b.stderr.String())
	stderr := b.stderr.String()
	for _, want := range []string{
		"dispatch escalation refused: fake:fake-escalated is requested, the roster gives fake:fake-model, and no model tiers rank them",
		"configure model.tier.* to rank both",
		"add fake:fake-escalated to a signed envelope.dispatch-allow mission contract",
		"--approve-escalation",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr %q lacks %q", stderr, want)
		}
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
		t.Fatalf("outcome %v", outcome)
	}
	b.requireNoJobRecords()
}

// escalation-non-tty (fixture line 4543): --approve-escalation needs both
// ends interactive, and refuses before the roster is read.
func TestP7ApproveEscalationRequiresAnInteractiveTTY(t *testing.T) {
	t.Parallel()
	b := p7RosterBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	for _, tty := range []struct{ stdin, stderr bool }{{false, false}, {true, false}, {false, true}} {
		result := b.life.Run(t.Context(), delegation.Request{
			Invocation: delegation.Invocation{CallerPid: int64(os.Getpid())},
			Env:        delegation.Env{DelegateInternal: true, RecordOutcome: true, FixtureHazard: "MECHANICAL"},
			LockTag:    "delegation-test-lock-tag", Stderr: &b.stderr,
			Stdin: strings.NewReader("APPROVE Someone\n"), StdinTTY: tty.stdin, StderrTTY: tty.stderr,
		}, []string{"dispatch", "--role", "implementer", "--brief", brief, "--model", "fake-escalated", "--approve-escalation", "--job-id", "escalation-non-tty"})
		requireExit(t, result, 1, b.stderr.String())
		if !strings.Contains(b.stderr.String(), "--approve-escalation needs an interactive terminal") {
			t.Fatalf("tty %+v: stderr %q", tty, b.stderr.String())
		}
		b.stderr.Reset()
	}
	b.requireNoJobRecords()
}

// runTTY runs one dispatch whose stdin and stderr are interactive terminals
// answering with answer.
func (b *bed) runTTY(answer string, args ...string) delegation.Result {
	b.t.Helper()
	b.stderr.Reset()
	return b.life.Run(b.t.Context(), delegation.Request{
		Invocation: delegation.Invocation{CallerPid: int64(os.Getpid())},
		Env:        delegation.Env{DelegateInternal: true, RecordOutcome: true, FixtureHazard: "MECHANICAL"},
		LockTag:    "delegation-test-lock-tag", Stderr: &b.stderr,
		Stdin: strings.NewReader(answer), StdinTTY: true, StderrTTY: true,
	}, args)
}

// escalation-declined (fixture lines 4545-4550): the prompt shows the facts
// a human approves; anything but APPROVE <name> declines with the corrective
// action and creates no job.
func TestP7DeclinedEscalationNamesTheCorrectiveActionAndCreatesNoJob(t *testing.T) {
	t.Parallel()
	b := p7RosterBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	for _, answer := range []string{"NO\n", "APPROVE \n", "APPROVE  padded\n", ""} {
		result := b.runTTY(answer, "dispatch", "--role", "implementer", "--brief", brief, "--model", "fake-escalated", "--approve-escalation", "--job-id", "escalation-declined")
		requireExit(t, result, 1, b.stderr.String())
		stderr := b.stderr.String()
		if !strings.Contains(stderr, "escalation approval declined") {
			t.Fatalf("answer %q: stderr %q", answer, stderr)
		}
		for _, fact := range []string{"Roster resolution: fake:fake-model\n", "Requested pair: fake:fake-escalated\n",
			"Cost direction: unranked (model tiers absent; overrides always escalate)\n", "Type APPROVE <name> to confirm: "} {
			if !strings.Contains(stderr, fact) {
				t.Fatalf("the prompt did not display %q: %q", fact, stderr)
			}
		}
	}
	if _, err := os.Stat(b.recordPath("escalation-declined")); !os.IsNotExist(err) {
		t.Fatalf("a declined escalation created its job: %v", err)
	}
	b.requireNoJobRecords()
}

// A pair that needs no escalation refuses a superfluous approval flag.
func TestP7ApprovalFlagWithoutAnEscalationIsRefused(t *testing.T) {
	t.Parallel()
	b := p7RosterBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	result := b.runTTY("APPROVE Someone\n", "dispatch", "--role", "implementer", "--brief", brief, "--approve-escalation", "--job-id", "needless")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "--approve-escalation is not needed") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	b.requireNoJobRecords()
}

// missing-mission-lease and ambiguous-mission (fixture lines 4827-4828): a
// mission with no live lease and a mission context that disagrees with its
// inherited scope both refuse before any job exists.
func TestP7MissionContextRefusals(t *testing.T) {
	t.Parallel()
	b := p7RosterBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	lease := filepath.Join(b.root, "artifacts", "agents", "missions", "mission-alpha", "lease.json")
	base := delegation.Env{DelegateInternal: true, RecordOutcome: true, FixtureHazard: "MECHANICAL"}
	for _, tc := range []struct {
		name string
		env  func(delegation.Env) delegation.Env
		args []string
		want string
	}{
		{"missing lease", func(e delegation.Env) delegation.Env { return e },
			[]string{"--job-id", "missing-mission", "--mission", "missing"},
			"mission missing does not have a live, matching lease"},
		{"flag disagrees with inherited", func(e delegation.Env) delegation.Env {
			e.MissionID, e.MissionLease = "mission-alpha", lease
			return e
		}, []string{"--job-id", "mission-ambiguous", "--mission", "another"},
			"--mission names another mission than the one this process runs in"},
		{"half an inherited context", func(e delegation.Env) delegation.Env {
			e.MissionID = "mission-alpha"
			return e
		}, []string{"--job-id", "mission-half"},
			"the inherited mission context is incomplete"},
		{"a turn without a mission", func(e delegation.Env) delegation.Env {
			e.MissionTurn = "t1"
			return e
		}, []string{"--job-id", "mission-turn-only"},
			"the inherited mission context names a runner turn but no mission"},
	} {
		argv := append([]string{"dispatch", "--role", "implementer", "--brief", brief}, tc.args...)
		result := b.runEnv(tc.env(base), argv...)
		requireExit(t, result, 1, b.stderr.String())
		if !strings.Contains(b.stderr.String(), tc.want) {
			t.Fatalf("%s: stderr %q, want %q", tc.name, b.stderr.String(), tc.want)
		}
	}
	if _, err := os.Stat(filepath.Join(b.root, "artifacts", "agents", "missions", "missing")); !os.IsNotExist(err) {
		t.Fatalf("a refused mission left mission state behind: %v", err)
	}
	b.requireNoJobRecords()
}

// unknown-status-job and the malformed status record (fixture lines
// 4837-4854): an absent job is exit 6, an unparsable record exit 7.
func TestP7StatusExitCodesForAnAbsentAndAnUnparsableRecord(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	result := b.run("status", "--job", "absent")
	requireExit(t, result, 6, b.stderr.String())
	if len(result.Stdout) != 0 || !strings.Contains(b.stderr.String(), "status: no job record for absent") {
		t.Fatalf("stdout %q stderr %q", result.Stdout, b.stderr.String())
	}
	b.writeFile("artifacts/agents/jobs/malformed-status.json", "{malformed\n")
	result = b.run("status", "--job", "malformed-status")
	requireExit(t, result, 7, b.stderr.String())
	if len(result.Stdout) != 0 {
		t.Fatalf("a malformed record printed a status: %q", result.Stdout)
	}
}

// mission-timeout's reap (fixture lines 4763-4801): the reaper concludes a
// mission job past its cap deadline as a budget-cap timeout and raises the
// mission's batched fence ask naming job-cap-min, or wall-clock-hours when
// the mission's wall clock truncated the cap; a non-mission timeout raises
// none. The terminal mission job's usage is aggregated into the mission.
func TestP7ReapOfAMissionJobAtItsCapRaisesTheFenceAsk(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, mission, truncatedBy, reason string
		resolution                         map[string]any
	}{
		{"job cap", "mission-timeout", "", "job-cap-min", nil},
		{"wall clock truncation", "mission-wall", "wall-clock", "wall-clock-hours", nil},
		{"signed pair cap", "mission-pair", "", "cap.min.codex.gpt-5-6-sol",
			map[string]any{"rule": "contract-pair", "origin": "contract", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 60, "requestedMin": 60}},
		{"own lower cap", "mission-lower", "", "",
			map[string]any{"rule": "argument", "origin": "argument", "key": "cap.min.codex.gpt-5-6-sol", "signedMin": 180, "requestedMin": 60}},
		{"no mission", "", "", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			fields := map[string]any{"status": "running", "pid": 7601, "pgid": 7601, "sessionId": "s-1",
				"startedAt": b.doubles.Clock.Now().UTC().Format("2006-01-02T15:04:05Z"), "capMin": 60,
				"capDeadline": "2000-01-01T00:01:00Z"}
			if tc.mission != "" {
				fields["mission"] = tc.mission
				resolution := map[string]any{"capMin": 60}
				if tc.truncatedBy != "" {
					resolution["truncatedBy"] = tc.truncatedBy
				}
				for key, value := range tc.resolution {
					resolution[key] = value
				}
				fields["capResolution"] = resolution
			}
			b.writeRecord("capped-job", fields)
			b.doubles.Process.Tags[7601] = "tag-capped-job"
			b.doubles.Process.Groups[7601] = true
			b.doubles.Process.Owned[7601] = true
			requireExit(t, b.run("reap", "--job", "capped-job"), 0, b.stderr.String())
			record := b.record("capped-job")
			if record["status"] != "timeout" || record["error"] != "budget-cap" || record["groupDeathProvenAt"] == nil {
				t.Fatalf("record %v", record)
			}
			missions := filepath.Join(b.root, "artifacts", "agents", "missions")
			if tc.mission == "" {
				if _, err := os.Stat(missions); !os.IsNotExist(err) {
					t.Fatalf("a non-mission timeout touched mission state: %v", err)
				}
				return
			}
			if tc.reason == "" {
				if _, err := os.Stat(filepath.Join(missions, tc.mission, "asks", "fence-bound.json")); !os.IsNotExist(err) {
					t.Fatalf("a timeout below the signed cap raised a fence ask: %v", err)
				}
			} else {
				ask, err := os.ReadFile(filepath.Join(missions, tc.mission, "asks", "fence-bound.json"))
				if err != nil {
					t.Fatalf("the capped mission job raised no batched ask: %v; stderr %q", err, b.stderr.String())
				}
				if !strings.Contains(string(ask), "`"+tc.reason+"`") {
					t.Fatalf("the ask does not name %s: %s", tc.reason, ask)
				}
			}
			if _, err := os.Stat(filepath.Join(missions, tc.mission, "usage.json")); err != nil {
				t.Fatalf("the terminal mission job's usage was not aggregated: %v", err)
			}
		})
	}
}
