package steward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func standingFailure() RoleVerdict {
	return RoleVerdict{Role: RoleGovernedObligations, Status: HealthDead,
		Reason: "the ledger moved 43 minutes ago", Remedy: "inspect the ledger"}
}

func TestStandingBeginsOnFifthObservation(t *testing.T) {
	t.Parallel()
	bed := newHealthBed(t, EnrollmentFixture, "")
	role := standingFailure()
	var state HealthObservationState
	for observation := 1; observation <= 6; observation++ {
		verdict := applyHealthObservation(bed.root, state, []RoleVerdict{role}, bed.base.Add(time.Duration(observation)*time.Second))
		got := verdict.Roles[0]
		wantStanding := observation >= 5
		if got.Standing != wantStanding || got.Status != HealthDead || got.Reason != role.Reason {
			t.Fatalf("observation %d: role = %+v, want standing %t with its diagnostic kept", observation, got, wantStanding)
		}
		if verdict.ShouldAlert == wantStanding || (verdict.Aggregate == "healthy") != wantStanding {
			t.Fatalf("observation %d: standing failure still affects seat health: %+v", observation, verdict)
		}
		if wantStanding && verdict.FindingDigest != healthFindingDigest(nil) {
			t.Fatalf("standing failure still affects the finding digest: %s", verdict.FindingDigest)
		}
		if !strings.Contains(verdict.Line(), string(role.Role)+"=dead") ||
			(wantStanding && !strings.Contains(verdict.Line(), "standing")) {
			t.Fatalf("the standing diagnostic disappeared from the line: %s", verdict.Line())
		}
		state = verdict.State
	}
	role = standingFailure()
	for observation := 1; observation <= 5; observation++ {
		standingTick(t, bed, []RoleVerdict{role}, bed.base.Add(time.Duration(observation)*time.Second), bed.deliver)
	}
	defects := 0
	for _, episode := range bed.episodes() {
		if episode.Owner == "" && (!episode.Resolved || !episode.Cleared) {
			t.Fatalf("the lone standing failure left its health notice open: %+v", episode)
		}
		if episode.Owner == PatternOwner("health-standing-red") && !episode.Cleared {
			defects++
		}
	}
	if defects != 1 {
		t.Fatalf("the lone standing failure opened %d defects", defects)
	}
	// An automatic remedy keeps its breaker even when diagnostics change.
	role.Role = RoleStewardRunner
	state = HealthObservationState{}
	for observation := 1; observation <= 5; observation++ {
		role.Reason = fmt.Sprintf("runner cause %d", observation)
		verdict := applyHealthObservation(bed.root, state, []RoleVerdict{role}, bed.base.Add(time.Duration(observation)*time.Second))
		if verdict.Roles[0].Standing || verdict.Roles[0].ConsecutiveFailures != observation {
			t.Fatalf("automatic repair's breaker changed: %+v", verdict)
		}
		state = verdict.State
	}
}

func TestStandingCountSurvivesAnAgingReason(t *testing.T) {
	t.Parallel()
	bed := newHealthBed(t, EnrollmentFixture, "")
	var state HealthObservationState
	for observation := 1; observation <= 5; observation++ {
		role := standingFailure()
		role.Reason = fmt.Sprintf("the ledger moved %d minutes ago at 2026-10-05T%02d:00:00Z (attempt=%d tip=abc%d)", 42+observation, observation, observation, observation)
		verdict := applyHealthObservation(bed.root, state, []RoleVerdict{role}, bed.base.Add(time.Duration(observation)*time.Second))
		if verdict.Roles[0].ConsecutiveFailures != observation || verdict.Roles[0].Standing != (observation == 5) {
			t.Fatalf("aging diagnostics restarted the same failure: %+v", verdict)
		}
		state = verdict.State
	}
}

func TestStandingChangedCauseRestartsTheCount(t *testing.T) {
	t.Parallel()
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			t.Parallel()
			bed := newHealthBed(t, EnrollmentFixture, "")
			role := standingFailure()
			if explicit {
				role.Cause = "ledger-unexamined"
			}
			var state HealthObservationState
			for observation := 1; observation <= 5; observation++ {
				if explicit {
					role.Reason = fmt.Sprintf("diagnostic wording %s", strings.Repeat("different ", observation))
				}
				verdict := applyHealthObservation(bed.root, state, []RoleVerdict{role}, bed.base.Add(time.Duration(observation)*time.Second))
				if verdict.Roles[0].ConsecutiveFailures != observation || verdict.Roles[0].Standing != (observation == 5) {
					t.Fatalf("the same cause restarted or stood early: %+v", verdict)
				}
				state = verdict.State
			}
			if explicit {
				role.Cause = "ledger-unreadable"
			} else {
				role.Reason = "the ledger is unreadable"
			}
			changed := applyHealthObservation(bed.root, state, []RoleVerdict{role}, bed.base.Add(6*time.Second))
			if changed.Roles[0].Standing || changed.Roles[0].ConsecutiveFailures != 1 || !changed.ShouldAlert {
				t.Fatalf("a changed cause inherited the standing failure: %+v", changed)
			}
			alive := roleAlive(role.Role, "the ledger recovered")
			reset := applyHealthObservation(bed.root, changed.State, []RoleVerdict{alive}, bed.base.Add(7*time.Second))
			if reset.State.FailureCounts[role.Role] != 0 || reset.State.FailureCauses[role.Role] != "" {
				t.Fatalf("recovery kept the cause or count: %+v", reset.State)
			}
		})
	}
}

func standingTick(t *testing.T, bed *healthBed, roles []RoleVerdict, now time.Time, deliver func(string, string) error) HealthVerdict {
	t.Helper()
	evaluate := func(string, string, time.Time, identity.Prober, bool) ([]RoleVerdict, SpendObservation) {
		return append([]RoleVerdict(nil), roles...), SpendObservation{Valid: true}
	}
	var result TickResult
	if err := completeTickHealthWithDependencies(bed.root, &result, bed.generation, bed.runner, now,
		tickHealthDependencies{evaluate: evaluate, now: func() time.Time { return now }, deliver: deliver}); err != nil {
		t.Fatal(err)
	}
	return result.Health
}

func TestStandingFlipsOnTheSameObservationForEveryReader(t *testing.T) {
	t.Parallel()
	bed := newHealthBed(t, EnrollmentFixture, "")
	roles := []RoleVerdict{standingFailure()}
	preview := func(at time.Time, installed bool) HealthVerdict {
		installation := bed.root
		if installed {
			installation = "/fixture/installation"
		}
		return previewHealthAtWithEvaluation(bed.root, installation, at, bed.probe,
			func(repo, root string, now time.Time, _ identity.Prober, currentHook bool) ([]RoleVerdict, SpendObservation) {
				if repo != bed.root || root != installation || !now.Equal(at) || !currentHook {
					t.Fatalf("preview evaluated at the wrong coordinates: %s %s %s %t", repo, root, now, currentHook)
				}
				return roles, SpendObservation{Valid: true}
			})
	}
	for observation := 1; observation <= 5; observation++ {
		at := bed.base.Add(time.Duration(observation) * time.Second)
		tick := standingTick(t, bed, roles, at, bed.deliver)
		before, err := os.ReadFile(HealthRecordPath(bed.root))
		if err != nil {
			t.Fatal(err)
		}
		for _, installed := range []bool{false, true} {
			for read := 0; read < 3; read++ {
				got := preview(at, installed)
				if got.Roles[0].Standing != (observation == 5) || got.Aggregate != tick.Aggregate ||
					got.ShouldAlert != tick.ShouldAlert || got.FindingDigest != tick.FindingDigest || got.State.Sequence != tick.State.Sequence {
					t.Fatalf("reader installed=%t disagrees at observation %d: tick=%+v preview=%+v", installed, observation, tick, got)
				}
			}
		}
		after, err := os.ReadFile(HealthRecordPath(bed.root))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("previews changed durable health counters: %v", err)
		}
	}
	// A fresh different cause must not inherit the tick's standing decision.
	roles[0].Reason = "the ledger is unreadable"
	if got := preview(bed.base.Add(6*time.Second), true); got.Roles[0].Standing || got.Aggregate != "unhealthy" {
		t.Fatalf("preview inherited another cause's standing: %+v", got)
	}
}

func TestStandingHandoffLeavesOneOpenNotice(t *testing.T) {
	t.Parallel()
	bed := newHealthBed(t, EnrollmentFixture, "")
	role := standingFailure()
	sibling := RoleVerdict{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner is dead", Remedy: "restart"}
	var notices []string
	deliver := func(_ string, message string) error {
		notices = append(notices, message)
		// The close must be durable before the defect is delivered.
		if strings.Contains(message, "standing") {
			paths, err := filepath.Glob(filepath.Join(bed.root, "artifacts/agents/steward/alerts/*.json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var episode AlertEpisode
				if err := json.Unmarshal(data, &episode); err != nil {
					t.Fatal(err)
				}
				if episode.Owner == "" && !episode.Resolved && !episode.Cleared && strings.Contains(episode.Message, role.Reason) {
					t.Fatalf("standing defect opened before its health notice closed: %+v", episode)
				}
			}
		}
		return nil
	}
	var fifth HealthVerdict
	for observation := 1; observation <= 6; observation++ {
		roles := []RoleVerdict{role, sibling}
		fifth = standingTick(t, bed, roles, bed.base.Add(time.Duration(observation)*time.Second), deliver)
	}
	standingNotices := 0
	for _, notice := range notices {
		if strings.Contains(notice, "standing") {
			standingNotices++
		}
	}
	if standingNotices != 1 || fifth.Aggregate != "unhealthy" {
		t.Fatalf("standing failure submitted %d defect notices: %v", standingNotices, notices)
	}
	defects := 0
	for _, episode := range bed.episodes() {
		if episode.Owner != PatternOwner("health-standing-red") || episode.Cleared {
			continue
		}
		defects++
		if episode.ScopeID != string(role.Role) || episode.TransportResult != TransportSubmitted || len(episode.Attempts) != 1 ||
			len(episode.Evidence) == 0 || episode.Evidence[0].Record != HealthRecordPath(bed.root) ||
			!strings.Contains(episode.Evidence[0].Fact, role.Reason) || !strings.Contains(episode.Evidence[0].Fact, role.Remedy) {
			t.Fatalf("standing defect lost its work, diagnostic, remedy or record: %+v", episode)
		}
	}
	if defects != 1 {
		t.Fatalf("standing role has %d open defects", defects)
	}
	if _, err := os.Stat(PatternStatePath(bed.root)); !os.IsNotExist(err) {
		t.Fatalf("standing defect kept pattern state: %v", err)
	}
	standingTick(t, bed, []RoleVerdict{roleAlive(role.Role, "recovered")}, bed.base.Add(7*time.Second), deliver)
	for _, episode := range bed.episodes() {
		if !episode.Cleared && !episode.Resolved {
			t.Fatalf("recovery or absent sibling left a notice open: %+v", episode)
		}
	}
}

func TestStandingSiblingKeepsHealthNoticeForSiblingOnly(t *testing.T) {
	t.Parallel()
	bed := newHealthBed(t, EnrollmentFixture, "")
	role := standingFailure()
	sibling := RoleVerdict{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner is dead", Remedy: "restart"}
	var fifth HealthVerdict
	for observation := 1; observation <= 5; observation++ {
		fifth = standingTick(t, bed, []RoleVerdict{role, sibling}, bed.base.Add(time.Duration(observation)*time.Second), bed.deliver)
	}
	if fifth.Aggregate != "unhealthy" || !fifth.ShouldAlert || fifth.FindingDigest != healthFindingDigest([]RoleVerdict{sibling}) {
		t.Fatalf("standing role hid its active sibling or stayed in its key set: %+v", fifth)
	}
	healthNotices := 0
	for _, episode := range bed.episodes() {
		if episode.Owner != "" || episode.Resolved || episode.Cleared {
			continue
		}
		healthNotices++
		if episode.Digest != fifth.FindingDigest || strings.Contains(episode.Message, string(role.Role)+"=dead") ||
			!strings.Contains(episode.Message, string(sibling.Role)+"=dead") {
			t.Fatalf("the remaining health notice includes the standing failure: %+v", episode)
		}
	}
	if healthNotices != 1 {
		t.Fatalf("active sibling has %d health notices, want one", healthNotices)
	}
}
