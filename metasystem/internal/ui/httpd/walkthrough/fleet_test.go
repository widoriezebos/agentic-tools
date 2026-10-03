package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// Signed in is enough (g1-s72): the fixture's launches name no review date,
// and a launch made in a browser is stamped with the enrollment the signed
// session names, as the engine's starter stamps it.
func TestTheFixturesLaunchesNameNoReviewDate(t *testing.T) {
	t.Parallel()
	for _, record := range fixtureLaunches(time.Now().UTC()) {
		if record.ReviewBy != "" {
			t.Fatalf("launch %s names a review date %q", record.Launch, record.ReviewBy)
		}
	}
}

func TestTheFixturesLaunchHookStampsTheSignedSessionsEnrollment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	proof, err := humanauthority.SignedInSessionProof(t.TempDir(), "wido", "01M3SESSIONREFERENCE000000", "browser", now)
	if err != nil {
		t.Fatal(err)
	}
	signed := &session.Session{Reference: "01M3SESSIONREFERENCE000000", Human: "wido", Proof: proof}
	record := fixtureLaunchOf(signed, launch.Request{Machine: "m1k", Destination: "/w/agentic-tools-m1k"}, now)
	want := launch.Enrollment{
		Kind: launch.EnrollmentHumanSession, Provider: "browser", Human: "wido",
		Session: "01M3SESSIONREFERENCE000000", At: "2026-09-29T08:00:00Z",
	}
	if record.Enrollment == nil || *record.Enrollment != want {
		t.Fatalf("enrollment = %+v, want %+v", record.Enrollment, want)
	}
	if record.Machine != "m1k" || record.ReviewBy != "" {
		t.Fatalf("record = %+v", record)
	}
}

// The fixture's lane holds one red proof whose log the page's Open log reads
// through the engine's own lookup (fleet-panel-ux step 2, 2a.3), and no
// board bridge to dial.
func TestTheFixtureServesItsRedProofsLog(t *testing.T) {
	t.Parallel()
	source, err := fixtureProofLogs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := source.ProofLog(fixtureProofAttempt)
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(log), "landing prove: the proving command exited 1") {
		t.Fatalf("the red proof's log = %q, %v", log, err)
	}
	if _, err := source.Dial(); err == nil {
		t.Fatal("the fixture dialled a board bridge")
	}
}

// The fixture's Pause, Resume and Stop answer as the verbs do, in their
// words, and touch nothing (fleet-panel-ux step 2, slice 2b): Pause names the
// person who paused, and Stop's admission refuses this fixture's own seat, as
// the engine refuses the checkout serving the page, and admits any other.
func TestTheFixturesFleetActsAnswerAsTheVerbsDo(t *testing.T) {
	t.Parallel()
	paused, err := fixturePauseLane("Wido")
	if err != nil || paused.Outcome != "confirmed" || !strings.HasPrefix(paused.Summary, "stopped the landing lane for Wido") {
		t.Fatalf("pause = %+v %v", paused, err)
	}
	resumed, err := fixtureResumeLane("Wido")
	if err != nil || resumed.Outcome != "confirmed" || !strings.HasPrefix(resumed.Summary, "resumed the landing lane") {
		t.Fatalf("resume = %+v %v", resumed, err)
	}
	refused, err := fixtureAdmitStop(fixtureThis)
	if err != nil || refused == nil || refused.Outcome != "refused" || refused.Next == nil ||
		strings.Join(refused.Next.Argv, " ") != "metasystem machine stop "+fixtureThis {
		t.Fatalf("admission of this fixture's own seat = %+v %v; want it refused with the terminal's command", refused, err)
	}
	if admitted, err := fixtureAdmitStop("m1f"); err != nil || admitted != nil {
		t.Fatalf("admission of another seat = %+v %v; want it admitted", admitted, err)
	}
	stopped, err := fixtureStopMachine("Wido", "m1f")
	if err != nil || stopped.Outcome != "confirmed" || !strings.HasPrefix(stopped.Summary, "stopped MetaSystem on m1f") {
		t.Fatalf("stop = %+v %v", stopped, err)
	}
}
