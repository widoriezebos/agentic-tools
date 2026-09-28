package steward

// Ported from scripts/agents/health-fixtures.sh (verb redesign U7b part 3).
// The shell bed armed a real steward runner and drove `internal health`,
// `steward tick` and `internal health acknowledge-alert` against a scratch
// repository while it stopped, killed and restarted real processes. This bed
// keeps the same durable evidence and asserts the same verdicts, but every
// process is a fact in a per-test prober, every clock is an explicit instant,
// and every notification goes through the production delivery resolver with a
// per-test configuration read and command runner. No Git, no sleeps.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// healthBedFreshRoles are the roles the shell bed required on its healthy
// line. Every other role in healthRoleOrder is held alive by the bed's
// evaluator because its owner (the goal ledger, spend, trunk red, ...) is
// proved by its own tests and needs Git to evaluate for real.
var healthBedFreshRoles = []HealthRole{
	RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher, RoleCensusFreshness,
	RoleNarratorFreshness, RoleSessionMain, RoleHookFreshness, RoleStopHookDuration,
	RoleContext, RoleClaimedGoalBudget, RoleStopCapabilityEpoch, RoleNonterminalJobs,
	RoleCapabilitySnapshots,
}

type healthBed struct {
	t           *testing.T
	root        string
	generation  int
	tickSeconds int
	probe       healthProbe
	runner      identity.Ref
	owner       identity.Ref
	watcher     identity.Ref
	main        identity.Ref
	base        time.Time
	notify      notificationDependencies
	sink        string
	platform    []string
}

func healthBedRef(pid int64) identity.Ref {
	return identity.Ref{Pid: pid, StartedAtSec: 1_700_000_000 + pid, StartTicks: pid * 10, BootID: "health-bed-boot"}
}

func (b *healthBed) alive(ref identity.Ref) {
	b.probe[ref.Pid] = struct {
		exact identity.Exact
		state identity.Liveness
		err   error
	}{exact: identity.Exact{Pid: ref.Pid, StartedAt: time.Unix(ref.StartedAtSec, 0), StartTicks: ref.StartTicks, BootID: ref.BootID}, state: identity.Alive}
}

func (b *healthBed) dead(ref identity.Ref) {
	b.probe[ref.Pid] = struct {
		exact identity.Exact
		state identity.Liveness
		err   error
	}{state: identity.Dead}
}

func (b *healthBed) writeJSON(relative string, value any) {
	b.t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		b.t.Fatal(err)
	}
	b.writeFile(relative, append(data, '\n'))
}

func (b *healthBed) writeFile(relative string, data []byte) {
	b.t.Helper()
	path := filepath.Join(b.root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// newHealthBed builds the armed world the shell bed reached after `steward
// arm`: an enrolled installation, a resident runner whose tick and narrator
// passes succeeded, a live supervision owner and watcher, a fresh census, an
// announced session main and one completed Stop without a duration.
func newHealthBed(t *testing.T, enrollment, configuredCommand string) *healthBed {
	t.Helper()
	root := canonicalPath(t.TempDir())
	b := &healthBed{
		t: t, root: root, generation: 3, tickSeconds: 1, probe: healthProbe{},
		runner: healthBedRef(51001), owner: healthBedRef(51002), watcher: healthBedRef(51003), main: healthBedRef(51004),
		base: time.Date(2026, 9, 20, 10, 0, 0, 500_000_000, time.UTC),
		sink: filepath.Join(t.TempDir(), "alerts.log"),
	}
	for _, ref := range []identity.Ref{b.runner, b.owner, b.watcher, b.main} {
		b.alive(ref)
	}
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=none\nwatch.stale-min=20\nwatch.interval-sec=60\ncapability.snapshot-max-age-days=30\n"))
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: b.generation, InstallPath: "/fixture/metasystem",
		MintedAt: b.base.Add(-time.Hour).Format(time.RFC3339), Enrollment: enrollment,
	}); err != nil {
		t.Fatal(err)
	}
	b.writeJSON("artifacts/agents/supervision/state.json", map[string]any{
		"generation": 1, "intervalSec": 60,
		"owner":      map[string]any{"pid": b.owner.Pid, "pidStartedAt": b.owner.StartedAtSec, "instanceTag": "health-owner"},
		"components": map[string]any{"watcher": map[string]any{"pid": b.watcher.Pid, "pidStartedAt": b.watcher.StartedAtSec, "instanceTag": "health-watcher"}},
	})
	b.writeJSON("artifacts/agents/supervision/lock.d/owner.json", map[string]any{
		"pid": b.owner.Pid, "pidStartedAt": b.owner.StartedAtSec, "instanceTag": "health-owner",
	})
	b.census(b.base)
	b.writeJSON("artifacts/agents/mains/main-fixture.json", map[string]any{
		"sessionId": "fixture", "mainId": "fixture", "pid": b.main.Pid, "pidStartedAt": b.main.StartedAtSec, "runtime": "fake",
	})
	b.componentSuccess("repo-watcher", 1, b.watcher, b.base)
	b.startRunner(b.runner, b.base)
	b.componentSuccess("narrator", b.generation, b.runner, b.base)
	recordStopCompletion(t, root, nil, "EMITTED")

	b.notify = notificationDependencies{
		configuredCommand: func(got string) ([]byte, error) {
			if got != root {
				t.Errorf("notification configuration read for %q, want %q", got, root)
			}
			if configuredCommand == "" {
				return nil, errors.New("metasystem.steward.notify-command is unset")
			}
			return []byte(configuredCommand + "\n"), nil
		},
		platform: "darwin",
		commandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name == "osascript" {
				b.platform = append(b.platform, strings.Join(args, " "))
				return exec.CommandContext(ctx, "/usr/bin/false")
			}
			return exec.CommandContext(ctx, name, args...)
		},
	}
	t.Cleanup(func() {
		if len(b.platform) != 0 {
			t.Errorf("the bed invoked the platform notifier: %q", b.platform)
		}
	})
	return b
}

func (b *healthBed) census(at time.Time) {
	b.writeJSON("artifacts/agents/supervision/last-census.json", map[string]any{
		"schemaVersion": 2, "writer": "watch-background-jobs.sh", "verdict": "SUCCESS",
		"completedAtEpoch": at.Unix(), "intervalSec": 60, "generation": 1, "fingerprint": "fixture",
		"counts": map[string]any{}, "inventory": []any{}, "diagnostics": []any{}, "errors": []any{},
	})
}

func (b *healthBed) componentSuccess(component string, generation int, process identity.Ref, at time.Time) {
	b.t.Helper()
	attempt, err := beginComponentAttempt(b.root, component, generation, process, at)
	if err != nil {
		b.t.Fatal(err)
	}
	if _, err := completeComponentAttempt(b.root, component, generation, attempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "health bed pass", nil, at); err != nil {
		b.t.Fatal(err)
	}
}

// startRunner records what arm and restart leave behind: the resident runner
// record and a generation-bound tick success by that exact process.
func (b *healthBed) startRunner(process identity.Ref, at time.Time) {
	b.t.Helper()
	b.runner = process
	b.alive(process)
	if err := writeJSONAtomic(runnerRecordPath(b.root), RunnerRecord{
		Pid: process.Pid, PidStartedAt: process.StartedAtSec, StartTicks: process.StartTicks, BootID: process.BootID,
		StartedAt: at.Format(time.RFC3339),
	}); err != nil {
		b.t.Fatal(err)
	}
	b.componentSuccess("steward-tick", b.generation, process, at)
}

func (b *healthBed) evaluate(repoRoot, metasystemRoot string, now time.Time, _ identity.Prober, currentHook bool) ([]RoleVerdict, SpendObservation) {
	state, stateErr := readHealthObject(filepath.Join(repoRoot, "artifacts", "agents", "supervision", "state.json"))
	cadence := func(string) int { return b.tickSeconds }
	real := map[HealthRole]func() RoleVerdict{
		RoleStewardRunner:       func() RoleVerdict { return checkStewardRunnerWithCadence(repoRoot, now, b.probe, cadence) },
		RoleSupervisionOwner:    func() RoleVerdict { return checkSupervisionOwner(repoRoot, b.probe) },
		RoleRepoWatcher:         func() RoleVerdict { return checkRepoWatcher(repoRoot, now, state, stateErr, b.probe) },
		RoleCensusFreshness:     func() RoleVerdict { return checkCensusFreshness(repoRoot, now, state, stateErr) },
		RoleNarratorFreshness:   func() RoleVerdict { return checkNarratorFreshnessWithCadence(repoRoot, now, cadence) },
		RoleRetroDebt:           func() RoleVerdict { return checkRetroDebt(repoRoot) },
		RoleSessionMain:         func() RoleVerdict { return checkSessionMain(repoRoot, b.probe) },
		RoleHookFreshness:       func() RoleVerdict { return checkHookFreshnessAt(repoRoot, now, currentHook) },
		RoleStopHookDuration:    func() RoleVerdict { return checkStopHookDuration(repoRoot) },
		RoleNonterminalJobs:     func() RoleVerdict { return checkNonterminalJobs(repoRoot, b.probe) },
		RoleCapabilitySnapshots: func() RoleVerdict { return checkCapabilitySnapshots(repoRoot, metasystemRoot, now) },
	}
	roles := make([]RoleVerdict, 0, len(healthRoleOrder))
	for _, role := range healthRoleOrder {
		if check, ok := real[role]; ok {
			roles = append(roles, check())
			continue
		}
		roles = append(roles, roleAlive(role, "held alive by the health bed"))
	}
	return roles, SpendObservation{Valid: true, Crossings: []SpendCrossing{}}
}

func (b *healthBed) deliver(root, message string) error {
	return deliverWithDependencies(root, message, b.notify)
}

// health is `metasystem internal health --repo`: one durable observation and
// a silent alert-episode update (the command never submits a notification).
func (b *healthBed) health(now time.Time) HealthVerdict {
	b.t.Helper()
	verdict, err := observeHealthWithEvaluation(b.root, now, b.probe, b.evaluate)
	if err != nil {
		b.t.Fatalf("health evidence is unknown: %v", err)
	}
	view := verdict
	view.ShouldAlert = false
	if _, err := updateAlertEpisodesWith(b.root, view, verdict.Line(), now, b.deliver); err != nil {
		b.t.Fatalf("alert episode state is unknown: %v", err)
	}
	return verdict
}

// tick is the health end of `metasystem steward tick`: the observation, the
// narration, and the alert episode that may submit a notification.
func (b *healthBed) tick(now time.Time) HealthVerdict {
	b.t.Helper()
	var result TickResult
	if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, now, tickHealthDependencies{
		evaluate: b.evaluate, now: func() time.Time { return now }, deliver: b.deliver,
	}); err != nil {
		b.t.Fatalf("tick failed: %v", err)
	}
	return result.Health
}

func (b *healthBed) requireHealthy(label string, verdict HealthVerdict) {
	b.t.Helper()
	if verdict.ExitCode() != 0 {
		b.t.Fatalf("%s: health exit %d, want 0: %s", label, verdict.ExitCode(), verdict.Line())
	}
}

func (b *healthBed) healthyBaseline() {
	b.t.Helper()
	verdict := b.health(b.base.Add(time.Millisecond))
	b.requireHealthy("armed repository", verdict)
	line := verdict.Line()
	for _, role := range healthBedFreshRoles {
		if !strings.Contains(line, string(role)+"=alive") {
			b.t.Fatalf("healthy line omitted %s: %s", role, line)
		}
	}
	if !strings.Contains(line, "stop-hook-duration=alive (the last Stop carried no measurement)") {
		b.t.Fatalf("healthy line did not explain its unmeasured Stop duration: %s", line)
	}
}

func (b *healthBed) episodes() []AlertEpisode {
	b.t.Helper()
	episodes, err := AlertEpisodes(b.root)
	if err != nil {
		b.t.Fatal(err)
	}
	return episodes
}

func (b *healthBed) activeEpisodes(digest string) []AlertEpisode {
	var active []AlertEpisode
	for _, episode := range b.episodes() {
		if !episode.Cleared && episode.Digest == digest {
			active = append(active, episode)
		}
	}
	return active
}

func (b *healthBed) recordedDigest() string {
	b.t.Helper()
	record, err := loadHealthRecord(HealthRecordPath(b.root))
	if err != nil {
		b.t.Fatalf("health record unreadable: %v", err)
	}
	return record.Verdict.FindingDigest
}

func TestHealthBedDirectVerdicts(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.healthyBaseline()

	// A measured slow Stop is a direct unhealthy verdict; restoring the real
	// hook record restores health.
	hookPath := ComponentEvidencePath(b.root, "supervision-hook")
	original, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(original, &record); err != nil {
		t.Fatal(err)
	}
	record["lastStopElapsedSec"] = 20
	slow, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, slow, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict := b.health(b.base.Add(2 * time.Millisecond))
	line := verdict.Line()
	if verdict.ExitCode() != 1 || !strings.Contains(line, "stop-hook-duration=dead") ||
		!strings.Contains(line, "the last Stop took 20s of the 60s budget") {
		t.Fatalf("slow Stop verdict did not name its twenty seconds and sixty-second budget: exit %d %s", verdict.ExitCode(), line)
	}
	if err := os.WriteFile(hookPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	b.requireHealthy("restored Stop record", b.health(b.base.Add(3*time.Millisecond)))

	// Exit 2 comes from a malformed job record.
	jobPath := filepath.Join(b.root, "artifacts", "agents", "jobs", "unknown-job.json")
	b.writeFile("artifacts/agents/jobs/unknown-job.json", []byte(`{"jobId":"unknown-job"}`+"\n"))
	verdict = b.health(b.base.Add(4 * time.Millisecond))
	if verdict.ExitCode() != 2 || !strings.Contains(verdict.Line(), "nonterminal-jobs=unknown") {
		t.Fatalf("unknown verdict did not name the malformed job: exit %d %s", verdict.ExitCode(), verdict.Line())
	}
	if err := os.Remove(jobPath); err != nil {
		t.Fatal(err)
	}
	b.requireHealthy("unknown recovery", b.health(b.base.Add(5*time.Millisecond)))
}

func TestHealthBedNarratorRecovery(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.healthyBaseline()

	// The resident loop stops producing; the narrator's last success keeps
	// its sub-second precision and becomes stale at exactly two producer
	// intervals, not a nanosecond later.
	narrator, err := loadComponentEvidence(ComponentEvidencePath(b.root, "narrator"))
	if err != nil {
		t.Fatal(err)
	}
	anchor := narrator.LastSuccess
	if anchor.Nanosecond() == 0 {
		t.Fatalf("the narrator anchor lost its sub-second precision: %s", anchor.Format(time.RFC3339Nano))
	}
	boundary := anchor.Add(time.Duration(2*b.tickSeconds) * time.Second)
	before := b.health(boundary.Add(-time.Nanosecond))
	if !strings.Contains(before.Line(), "narrator-freshness=alive") {
		t.Fatalf("narrator was stale before two producer intervals: %s", before.Line())
	}
	stalled := b.health(boundary)
	if stalled.ExitCode() != 1 || !strings.Contains(stalled.Line(), "narrator-freshness=dead") {
		t.Fatalf("narrator was not stale at exactly two producer intervals: exit %d %s", stalled.ExitCode(), stalled.Line())
	}
	if !strings.Contains(stalled.Line(), "metasystem session start --repo") {
		t.Fatalf("stale narrator omitted the up remedy: %s", stalled.Line())
	}

	// The focused restart replaces the runner, whose passes produce again.
	recovered := boundary.Add(time.Second)
	b.startRunner(healthBedRef(51005), recovered)
	b.componentSuccess("narrator", b.generation, b.runner, recovered)
	b.requireHealthy("the stale narrator's focused restart", b.health(recovered.Add(time.Millisecond)))
}

func TestHealthBedAlertEpisode(t *testing.T) {
	t.Parallel()
	sink := filepath.Join(t.TempDir(), "configured-alerts.log")
	b := newHealthBed(t, EnrollmentFixture, `printf '%s\n' "$STEWARD_MESSAGE" >> `+sink)
	b.sink = sink
	runHealthBedAlertEpisode(t, b, sink)
	if data, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "steward", "notifications.log")); err == nil && len(data) != 0 {
		t.Fatalf("configured notifier did not win over fixture-local delivery: %q", data)
	}
}

func TestHealthBedFixtureNotification(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	installed, err := VerifyIdentity(RepoIdentityPath(b.root), b.root)
	if err != nil || installed.Enrollment != EnrollmentFixture {
		t.Fatalf("fixture-granted arm did not record fixture enrollment: %+v %v", installed, err)
	}
	log := filepath.Join(b.root, "artifacts", "agents", "steward", "notifications.log")
	runHealthBedAlertEpisode(t, b, log)
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z HEALTH unhealthy`).Match(data) {
		t.Fatalf("fixture notification log omitted its UTC timestamp or alert: %q", data)
	}
	if _, err := os.Stat(b.sink); !os.IsNotExist(err) {
		t.Fatalf("fixture default unexpectedly used a configured notifier sink: %v", err)
	}
}

func healthBedNotifications(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "HEALTH unhealthy")
}

// runHealthBedAlertEpisode is the shared escalation walk of the alert-episode
// and fixture-notification scenarios: a killed runner opens silent history,
// the fifth consecutive failure submits exactly one notification, the same
// digest never submits again, acknowledgment records its invoker, and the
// healed verdict resolves and clears the episode.
func runHealthBedAlertEpisode(t *testing.T, b *healthBed, deliveries string) {
	t.Helper()
	b.healthyBaseline()

	// Kill the resident runner. The breaker starts from a healthy reset.
	b.dead(b.runner)
	if err := os.Remove(HealthRecordPath(b.root)); err != nil {
		t.Fatal(err)
	}
	// A retained, resolved but uncleared prior finding forces every
	// assertion below to select its episode by digest.
	b.writeFile("artifacts/agents/steward/alerts/alert-aaaaaaaaaaaaaaaa-1.json", []byte(`{
  "schema": 1,
  "episodeId": "alert-aaaaaaaaaaaaaaaa-1",
  "digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "message": "fixed prior finding",
  "openedAt": "2026-09-16T00:00:00Z",
  "attempts": [],
  "transportResult": "PENDING",
  "acknowledged": false,
  "resolved": true,
  "resolvedAt": "2026-09-16T00:00:01Z",
  "cleared": false
}
`))
	clock := b.base.Add(time.Second)
	dead := b.health(clock)
	if dead.ExitCode() != 1 || !strings.Contains(dead.Line(), "steward-runner=dead") {
		t.Fatalf("dead verdict did not name the killed runner: exit %d %s", dead.ExitCode(), dead.Line())
	}
	if !strings.Contains(dead.Line(), "metasystem session start --repo") {
		t.Fatalf("dead verdict omitted the up remedy: %s", dead.Line())
	}
	initial := b.recordedDigest()
	if healthBedNotifications(t, deliveries) != 0 {
		t.Fatal("a recoverable first failure notified the human before escalation")
	}
	silent := b.activeEpisodes(initial)
	if len(silent) != 1 {
		t.Fatalf("the first failure opened %d silent history episodes, want 1", len(silent))
	}
	if len(silent[0].Attempts) != 0 || silent[0].TransportResult != TransportPending {
		t.Fatalf("the silent history episode attempted notification before escalation: %+v", silent[0])
	}

	var fifth HealthVerdict
	for observation := 2; observation <= 5; observation++ {
		clock = clock.Add(time.Second)
		fifth = b.tick(clock)
	}
	var runner RoleVerdict
	for _, role := range fifth.Roles {
		if role.Role == RoleStewardRunner {
			runner = role
		}
	}
	if runner.ConsecutiveFailures != 5 || runner.FailureEscalation != AutoHealEnded {
		t.Fatalf("failure five did not end auto-heal: %+v", runner)
	}
	failureFive := b.recordedDigest()
	active := b.activeEpisodes(failureFive)
	if len(active) != 1 {
		t.Fatalf("failure five must open one digest-keyed episode, found %d", len(active))
	}
	episode := active[0]
	if episode.TransportResult != TransportSubmitted {
		t.Fatalf("notifier exit zero was not recorded as transport submitted: %+v", episode)
	}
	if got := healthBedNotifications(t, deliveries); got != 1 {
		t.Fatalf("one episode must submit one notification, got %d", got)
	}

	clock = clock.Add(time.Second)
	b.tick(clock)
	if dedup := b.recordedDigest(); dedup != failureFive {
		t.Fatalf("dedup tick changed finding digest from %s to %s", failureFive, dedup)
	}
	if got := healthBedNotifications(t, deliveries); got != 1 {
		t.Fatalf("same digest submitted a second notification: %d", got)
	}
	if got := len(b.activeEpisodes(failureFive)); got != 1 {
		t.Fatalf("same digest opened a second active episode: %d", got)
	}

	clock = clock.Add(time.Second)
	invoker := AlertInvoker{Pid: 9001, PidStartedAt: 77, UID: os.Getuid(), ArgvDigest: evidenceDigest("health bed acknowledgment")}
	if _, err := AcknowledgeAlert(b.root, episode.EpisodeID, invoker, clock); err != nil {
		t.Fatalf("episode acknowledgment failed: %v", err)
	}
	acknowledged, err := loadAlertEpisode(alertPath(b.root, episode.EpisodeID))
	if err != nil {
		t.Fatal(err)
	}
	if !acknowledged.Acknowledged || acknowledged.AcknowledgedBy == nil || acknowledged.AcknowledgedBy.Pid != invoker.Pid {
		t.Fatalf("episode acknowledgment omitted the observed invoker: %+v", acknowledged)
	}

	// The focused restart heals health; the healthy verdict resolves and
	// clears the episode without deleting it.
	clock = clock.Add(time.Second)
	b.startRunner(healthBedRef(51006), clock)
	b.componentSuccess("narrator", b.generation, b.runner, clock)
	b.componentSuccess("repo-watcher", 1, b.watcher, clock)
	b.census(clock)
	b.requireHealthy("the killed runner's focused restart", b.health(clock.Add(time.Millisecond)))
	healed, err := loadAlertEpisode(alertPath(b.root, episode.EpisodeID))
	if err != nil {
		t.Fatal(err)
	}
	if !healed.Resolved || !healed.Cleared {
		t.Fatalf("healthy verdict did not resolve and clear the alert episode: %+v", healed)
	}
}
