package steward

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

var seatFixtureClock = time.Date(2026, 9, 24, 9, 40, 0, 0, time.UTC)

// seatPresenceRoot is a checkout with no machine nickname and no remote: the
// component reaches no network from it, which is exactly what a behaviour
// test wants.
func seatPresenceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "steward"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestATickWithNoRunnerContextPublishesNoPresence(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	report := RunSeatPresence(root, nil, 4, seatFixtureClock)
	if report.Outcome != seat.OutcomeSkipped || report.Reason != seat.SkipManualTick {
		t.Fatalf("report = %+v; want the manual-tick skip", report)
	}
	// A manual tick writes no publication state and so changes no verdict,
	// whatever else about the checkout would have stopped a publish.
	if _, readable, err := seat.LoadPublicationState(root); err != nil {
		t.Fatal(err)
	} else if readable {
		t.Fatal("a manual tick wrote publication state")
	}
}

func TestAManualTickSkipsEvenWhereTheMachineIsEnrolled(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	seatEnroll(t, root, "m1e")
	report := RunSeatPresence(root, nil, 4, seatFixtureClock)
	if report.Outcome != seat.OutcomeSkipped || report.Reason != seat.SkipManualTick {
		t.Fatalf("report = %+v; want the manual-tick skip", report)
	}
	if _, readable, _ := seat.LoadPublicationState(root); readable {
		t.Fatal("a manual tick wrote publication state")
	}
}

func TestAnUnarmedTickNeverOverwritesTheArmedRunnersRecord(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	seatEnroll(t, root, "m1e")
	runner := &seat.RunnerContext{RepoIdentity: "repo-a", Generation: 0, Engine: "3f9c1e2", ArmedLineage: seat.NoLease, TickSeconds: 600}
	report := RunSeatPresence(root, runner, 0, seatFixtureClock)
	if report.Outcome != seat.OutcomeSkipped || report.Reason != seat.SkipUnarmed {
		t.Fatalf("report = %+v; want the unarmed skip", report)
	}
	state, readable, err := seat.LoadPublicationState(root)
	if err != nil || !readable {
		t.Fatalf("publication state = readable %t err %v", readable, err)
	}
	if seat.SkippedFor(state) != seat.SkipUnarmed {
		t.Fatalf("state = %+v", state)
	}
}

func TestPresencePublishesInLocalModeAndRecordsItsRung(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	seatGit(t, root, "init", "-q", "-b", "main")
	seatGit(t, root, "config", "user.name", "fixture")
	seatGit(t, root, "config", "user.email", "fixture@example.invalid")
	seatGit(t, root, "config", "metasystem.goal.machine", "m1e")
	seatGit(t, root, "config", "goal.sync-remote", "local")

	runner := &seat.RunnerContext{RepoIdentity: "repo-a", Generation: 4, Engine: "3f9c1e2", ArmedLineage: "lineage-7", TickSeconds: 600}
	report := RunSeatPresence(root, runner, 4, seatFixtureClock)
	if report.Outcome != seat.OutcomePublished || report.Rung != int(seat.RungMetasystemRef) {
		t.Fatalf("report = %+v", report)
	}
	state, readable, err := seat.LoadPublicationState(root)
	if err != nil || !readable {
		t.Fatalf("publication state = readable %t err %v", readable, err)
	}
	if state.LastOutcome != seat.OutcomePublished || state.Rung != 1 || state.TickSeconds != 600 {
		t.Fatalf("state = %+v", state)
	}
	verdict := checkSeatPresence(root, seatFixtureClock.Add(time.Minute))
	if verdict.Status != HealthAlive || !strings.Contains(verdict.Reason, "on rung 1") {
		t.Fatalf("health = %+v", verdict)
	}
	// A local seat sees itself, which is the whole point of LocalMode.
	standings := seatStandings(t, root)
	if standings.Machines["m1e"].Standing != seat.Reachable {
		t.Fatalf("standings = %+v", standings.Machines)
	}
}

func TestAFailedFetchStopsThePublishAndLeavesTheOtherCheckoutsRecord(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	seatGit(t, remote, "init", "-q", "--bare", "-b", "main")
	seatGit(t, root, "init", "-q", "-b", "main")
	seatGit(t, root, "config", "user.name", "fixture")
	seatGit(t, root, "config", "user.email", "fixture@example.invalid")
	seatGit(t, root, "config", "metasystem.goal.machine", "m1e")
	seatGit(t, root, "remote", "add", "origin", remote)

	// Another checkout is already publishing under this nickname, and its
	// record is newer than the one this tick would compose.
	foreign := seat.Record{
		PresenceSchema: seat.RecordSchema, Machine: "m1e", RepoIdentity: "another-checkout",
		Generation: 9, Engine: "8a01d77", ArmedLineage: seat.NoLease, TickSeconds: 600,
		TickAt: seat.FormatTime(seatFixtureClock.Add(time.Minute)),
	}
	file, err := foreign.Encode()
	if err != nil {
		t.Fatal(err)
	}
	publisher := seat.Git{Root: root, Remote: "origin"}
	held, err := publisher.Publish(seat.Write{
		Ref: seat.MetasystemNamespace + "/m1e", Message: seat.CommitMessage(foreign), File: file, Force: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The remote goes away before this tick runs, so the copy this machine
	// holds cannot be current.
	seatGit(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	runner := &seat.RunnerContext{RepoIdentity: "repo-a", Generation: 4, Engine: "3f9c1e2", ArmedLineage: seat.NoLease, TickSeconds: 600}
	report := RunSeatPresence(root, runner, 4, seatFixtureClock)
	if report.Outcome != seat.OutcomeFailed {
		t.Fatalf("report = %+v; a publish on a copy that could not be refreshed must fail", report)
	}
	if !strings.Contains(report.Detail, "fetch failed") {
		t.Fatalf("report detail = %q; it must name the transport failure", report.Detail)
	}
	if still := seatGit(t, remote, "rev-parse", seat.MetasystemNamespace+"/m1e"); still != held {
		t.Fatalf("the other checkout's record was overwritten: %s is now %s", held, still)
	}
}

func TestTheTransitionQueuesBeforeItPersists(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	// A previous read that saw m1c reachable, and a fetched copy in which it
	// has gone silent. The baseline is already written, so the change is news.
	if err := seat.SaveStandings(root, seat.StandingsState{
		ReadAt: seat.FormatTime(seatFixtureClock.Add(-time.Hour)),
		Machines: map[string]seat.Observation{
			"m1c": {Standing: seat.Reachable, Since: seat.FormatTime(seatFixtureClock.Add(-8 * time.Hour))},
		},
	}); err != nil {
		t.Fatal(err)
	}
	copied := seat.Copy{Records: map[string]seat.Record{
		"m1c": {PresenceSchema: 1, Machine: "m1c", RepoIdentity: "repo-c", Generation: 3,
			Engine: "8a01d77", ArmedLineage: seat.NoLease, TickSeconds: 600,
			TickAt: seat.FormatTime(seatFixtureClock.Add(-6 * time.Hour))},
	}}
	notified, err := noticeSeatStandings(root, "m1e", copied, seatFixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	if len(notified) != 1 || !strings.HasPrefix(notified[0], "seat-presence-m1c-unreachable-") {
		t.Fatalf("notified = %v", notified)
	}
	pending, err := PendingNotifications(root)
	if err != nil {
		t.Fatal(err)
	}
	var message string
	for _, note := range pending {
		if note.Nonce == notified[0] {
			message = note.Message
		}
	}
	if !strings.Contains(message, "m1c has been unreachable since") {
		t.Fatalf("pending = %+v", pending)
	}
	// The standings are persisted only after the notification is durable, and
	// they carry the frozen since the nonce was built from.
	standings := seatStandings(t, root)
	if standings.Machines["m1c"].Standing != seat.Unreachable {
		t.Fatalf("standings = %+v", standings.Machines)
	}
	// The same standing at the next pass is not news again.
	again, err := noticeSeatStandings(root, "m1e", copied, seatFixtureClock.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("an unchanged standing notified again: %v", again)
	}
}

func TestTheFirstReadPersistsABaselineAndNotifiesNothing(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	copied := seat.Copy{Records: map[string]seat.Record{
		"m1c": {PresenceSchema: 1, Machine: "m1c", RepoIdentity: "repo-c", Generation: 3,
			Engine: "8a01d77", ArmedLineage: seat.NoLease, TickSeconds: 600,
			TickAt: seat.FormatTime(seatFixtureClock.Add(-6 * time.Hour))},
	}}
	notified, err := noticeSeatStandings(root, "m1e", copied, seatFixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	if len(notified) != 0 {
		t.Fatalf("the baseline read notified %v", notified)
	}
	if standings := seatStandings(t, root); standings.Machines["m1c"].Standing != seat.Unreachable {
		t.Fatalf("the baseline was not persisted: %+v", standings)
	}
}

func TestTheSeatPresenceRoleEscalatesOnlyThroughConsecutiveFailures(t *testing.T) {
	t.Parallel()
	root := seatPresenceRoot(t)
	dead := roleDead(RoleSeatPresence, "presence not published since 2026-09-24T03:00:00Z: the remote refused", seat.RemedyRemote)
	state := HealthObservationState{}
	for observation := 1; observation <= 3; observation++ {
		verdict := applyHealthObservation(root, state, []RoleVerdict{dead}, seatFixtureClock.Add(time.Duration(observation)*time.Minute))
		state = verdict.State
		role := verdict.Roles[0]
		if role.ConsecutiveFailures != observation {
			t.Fatalf("observation %d counted %d failures", observation, role.ConsecutiveFailures)
		}
		if role.FailureEscalation == NoLawfulRemedy {
			t.Fatalf("observation %d escalated as having no lawful remedy; the next tick republishes", observation)
		}
		if verdict.Aggregate != "unhealthy" {
			t.Fatalf("observation %d aggregate = %q", observation, verdict.Aggregate)
		}
	}
	alive := roleAlive(RoleSeatPresence, "presence published 2 min ago on rung 1, a metasystem ref")
	recovered := applyHealthObservation(root, state, []RoleVerdict{alive}, seatFixtureClock.Add(4*time.Minute))
	if recovered.Roles[0].ConsecutiveFailures != 0 || recovered.Aggregate != "healthy" {
		t.Fatalf("recovery = %+v", recovered.Roles[0])
	}
}

func TestTheSeatPresenceRoleIsInTheClosedSchema(t *testing.T) {
	t.Parallel()
	if !KnownHealthRole(RoleSeatPresence) {
		t.Fatal("seat-presence is not in the closed health schema")
	}
	var afterLedgerAttention bool
	for _, role := range healthRoleOrder {
		if role == RoleLedgerAttention {
			afterLedgerAttention = true
			continue
		}
		if afterLedgerAttention {
			if role != RoleSeatPresence {
				t.Fatalf("seat-presence does not follow ledger-attention; %s does", role)
			}
			break
		}
	}
}

func seatEnroll(t *testing.T, root, machine string) {
	t.Helper()
	seatGit(t, root, "init", "-q", "-b", "main")
	seatGit(t, root, "config", "user.name", "fixture")
	seatGit(t, root, "config", "user.email", "fixture@example.invalid")
	seatGit(t, root, "config", "metasystem.goal.machine", machine)
	seatGit(t, root, "config", "goal.sync-remote", "local")
}

// seatGit drives git in one fixture checkout; the test owns the directory it
// names and the system configuration is kept out.
func seatGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func seatStandings(t *testing.T, root string) seat.StandingsState {
	t.Helper()
	data, err := os.ReadFile(seat.StandingsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var state seat.StandingsState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}
