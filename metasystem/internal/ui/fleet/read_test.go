package fleet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The host facts, read from files in a temporary checkout, and the one rule
// that decides which namespace a page reads.

func TestTheNamespaceFallsBackToTheTickUntilThisServerHasSucceeded(t *testing.T) {
	t.Parallel()

	cold, coldSource := NamespaceFor(false, false)
	warm, warmSource := NamespaceFor(false, true)
	local, localSource := NamespaceFor(true, false)

	testutil.Expect(t, "before a success the tick's copy stands in", cold, seat.TickNamespace)
	testutil.Expect(t, "and the page says so", coldSource, SourceTick)
	testutil.Expect(t, "after one the interface reads its own namespace", warm, seat.UINamespace)
	testutil.Expect(t, "and says so", warmSource, SourceInterface)
	testutil.Expect(t, "in LocalMode the publishing refs are read in place", localSource, SourceLocal)
	// In LocalMode seat.Git reads the publishing refs and ignores the
	// namespace it is handed; the source word is the whole of the answer.
	testutil.Expect(t, "the namespace it is handed there is the interface's own", local, seat.UINamespace)
}

// writeHealth plants a steward health record the way the steward writes one.
func writeHealth(t *testing.T, checkout string, verdict steward.HealthVerdict) {
	t.Helper()
	path := steward.HealthRecordPath(checkout)
	testutil.Require(t, "the steward's directory is made", os.MkdirAll(filepath.Dir(path), 0o755), nil)
	data, err := json.Marshal(map[string]any{"verdict": verdict})
	testutil.Require(t, "the verdict encodes", err, nil)
	testutil.Require(t, "and is written", os.WriteFile(path, data, 0o644), nil)
}

func TestAnAbsentHealthRecordIsNoVerdictRatherThanAProblem(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "a checkout that was never armed has no verdict", ReadHealth(t.TempDir()), (*Health)(nil))
}

func TestAHealthRecordTravelsWithTheInstantItWasRecordedAt(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	observed := now.Add(-12 * time.Minute)
	writeHealth(t, checkout, steward.HealthVerdict{
		Schema: 1, ObservedAt: observed, Aggregate: "healthy",
		Roles: []steward.RoleVerdict{
			{Role: steward.RoleStewardRunner, Status: steward.HealthAlive, Reason: "runner alive"},
			{Role: steward.RoleSeatPresence, Status: steward.HealthDead, Reason: "presence not published"},
		},
	})

	read := ReadHealth(checkout)
	testutil.Require(t, "the verdict is read", read != nil, true)
	testutil.Expect(t, "with its aggregate", read.State, "healthy")
	testutil.Expect(t, "the instant it was recorded at", read.ObservedAt, observed.UTC().Format(time.RFC3339))
	testutil.Expect(t, "and every role it names", len(read.Roles), 2)
	testutil.Expect(t, "each with its status and reason", read.Roles[1],
		Role{Role: "seat-presence", Status: "dead", Reason: "presence not published"})
	testutil.Expect(t, "nothing is wrong with the file", read.Problem, "")
}

func TestAMalformedHealthRecordIsNamedRatherThanReadAsAbsent(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	path := steward.HealthRecordPath(checkout)
	testutil.Require(t, "the directory is made", os.MkdirAll(filepath.Dir(path), 0o755), nil)
	testutil.Require(t, "a torn record is written", os.WriteFile(path, []byte("{not json"), 0o644), nil)

	read := ReadHealth(checkout)
	testutil.Require(t, "something comes back", read != nil, true)
	testutil.Expect(t, "and it names the trouble rather than saying nothing was recorded",
		read.Problem != "", true)
}

// A record that parses but names no aggregate cannot be read as a verdict
// either: a page that showed an empty state word would be a page saying this
// seat is in no state at all.
func TestAHealthRecordWithNoAggregateIsAProblem(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	writeHealth(t, checkout, steward.HealthVerdict{Schema: 1, ObservedAt: now})

	read := ReadHealth(checkout)
	testutil.Require(t, "something comes back", read != nil, true)
	testutil.Expect(t, "naming what is missing",
		read.Problem, "the steward's health record names no aggregate verdict")
}

func TestAnAbsentPublicationStateIsNotAnUnreadableOne(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()

	state, problem := ReadPublication(checkout)
	testutil.Expect(t, "a seat that never published has no state", state, (*seat.PublicationState)(nil))
	testutil.Expect(t, "and that is not a problem", problem, "")

	testutil.Require(t, "a state is written", seat.SavePublicationState(checkout, seat.PublicationState{
		Machine: "m1u", LastOutcome: seat.OutcomePublished, LastSuccessAt: ago(2 * time.Minute), Rung: 1,
	}), nil)
	written, stillFine := ReadPublication(checkout)
	testutil.Require(t, "and read back", written != nil, true)
	testutil.Expect(t, "with the outcome it recorded", written.LastOutcome, seat.OutcomePublished)
	testutil.Expect(t, "and nothing wrong", stillFine, "")
}

func TestStandingsThatCannotBeReadNameNoSinceAtAll(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	path := seat.StandingsPath(checkout)
	testutil.Require(t, "the directory is made", os.MkdirAll(filepath.Dir(path), 0o755), nil)
	testutil.Require(t, "a torn standings file is written", os.WriteFile(path, []byte("{not json"), 0o644), nil)

	testutil.Expect(t, "no frozen observation is invented", ReadStandings(checkout),
		map[string]seat.Observation{})
}
