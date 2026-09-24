package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// The tick integration of seat presence, driven through RunTick itself
// against a bare repository in this test's own temporary directory: one tick
// under a runner context publishes, and the manual tick verb's own
// TickConfig — the one with no runner context — publishes nothing and leaves
// the ref where the resident runner put it.

func seatTickBed(t *testing.T) (root, remote string) {
	t.Helper()
	root = gitRepoWithCurrentGoal(t)
	remote = filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	seatGit(t, remote, "init", "-q", "--bare", "-b", "main")
	seatGit(t, root, "config", "user.name", "fixture")
	seatGit(t, root, "config", "user.email", "fixture@example.invalid")
	seatGit(t, root, "config", "metasystem.goal.machine", "m1e")
	seatGit(t, root, "remote", "add", "origin", remote)

	// An armed generation, which is what separates a publishing tick from one
	// that must never overwrite the armed runner's record.
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: absolute, Generation: 4, InstallPath: "/bin/true",
		MintedAt: "2026-09-24T09:00:00Z", EngineBuild: "3f9c1e2",
	}); err != nil {
		t.Fatal(err)
	}
	return root, remote
}

func TestOneTickPublishesPresenceAndTheManualTickLeavesItAlone(t *testing.T) {
	t.Parallel()
	root, remote := seatTickBed(t)
	runner := &seat.RunnerContext{RepoIdentity: root, Generation: 4, Engine: "3f9c1e2", ArmedLineage: "lineage-7", TickSeconds: 600}

	resident, err := RunTick(root, TickConfig{Now: seatFixtureClock, Runner: runner}, fakeCensus{})
	if err != nil {
		t.Fatalf("resident tick: %v", err)
	}
	if resident.SeatPresence.Outcome != seat.OutcomePublished {
		t.Fatalf("seat presence report = %+v", resident.SeatPresence)
	}

	// The component's own attempt is recorded beside every other component's.
	evidence, err := os.ReadFile(ComponentEvidencePath(root, "seat-presence"))
	if err != nil {
		t.Fatalf("component record: %v", err)
	}
	if !strings.Contains(string(evidence), `"outcome": "PASS_COMPLETE"`) &&
		!strings.Contains(string(evidence), `"outcome":"PASS_COMPLETE"`) {
		t.Fatalf("component record = %s", evidence)
	}

	// The ref is at the remote, and it carries the record this tick composed.
	published := seatGit(t, remote, "rev-parse", "refs/metasystem/presence/m1e")
	if published == "" {
		t.Fatal("the tick published no ref at the remote")
	}
	file := seatGit(t, remote, "cat-file", "blob", published+":presence.json")
	record, err := seat.ParseRecord([]byte(file))
	if err != nil {
		t.Fatalf("the published record did not parse: %v\n%s", err, file)
	}
	if record.Machine != "m1e" || record.Generation != 4 || record.ArmedLineage != "lineage-7" ||
		record.TickAt != seat.FormatTime(seatFixtureClock) {
		t.Fatalf("published record = %+v", record)
	}

	// The publication state and the health line agree with it.
	state, readable, err := seat.LoadPublicationState(root)
	if err != nil || !readable || state.LastOutcome != seat.OutcomePublished || state.Rung != 1 {
		t.Fatalf("publication state = %+v readable %t err %v", state, readable, err)
	}
	verdict := checkSeatPresence(root, seatFixtureClock.Add(2*time.Minute))
	if verdict.Status != HealthAlive || !strings.Contains(verdict.Reason, "on rung 1, a metasystem ref") {
		t.Fatalf("health verdict = %+v", verdict)
	}

	// The manual verb calls the same RunTick with no runner context, and the
	// ref must not move: a command process publishes no presence.
	manual, err := RunTick(root, TickConfig{Now: seatFixtureClock.Add(10 * time.Minute)}, fakeCensus{})
	if err != nil {
		t.Fatalf("manual tick: %v", err)
	}
	if manual.SeatPresence.Outcome != seat.OutcomeSkipped || manual.SeatPresence.Reason != seat.SkipManualTick {
		t.Fatalf("manual tick report = %+v", manual.SeatPresence)
	}
	if after := seatGit(t, remote, "rev-parse", "refs/metasystem/presence/m1e"); after != published {
		t.Fatalf("the manual tick moved the ref from %s to %s", published, after)
	}
	unchanged, _, err := seat.LoadPublicationState(root)
	if err != nil || unchanged.LastAttemptAt != state.LastAttemptAt {
		t.Fatalf("the manual tick wrote publication state: %+v (%v)", unchanged, err)
	}
}
