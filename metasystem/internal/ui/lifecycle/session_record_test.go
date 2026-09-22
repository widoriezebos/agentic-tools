package lifecycle

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// What the server records about its sessions, and what another process makes
// of it. `ui status` runs somewhere else entirely and has no way to ask the
// server, so the record is the only place the live sessions can be read from —
// and the one-time-code floor is in a file of its own, because the record does
// not outlive the run that wrote it.

// Update rewrites the running server's own record in place, and leaves
// everything it did not touch exactly as it stood.
func TestUpdateRewritesTheRunningRecord(t *testing.T) {
	t.Parallel()
	stateRoot := t.TempDir()
	before, _ := resultTestRecord(t, stateRoot)

	err := Update(stateRoot, func(rec *Record) {
		rec.Sessions = []string{"signed in: Wido until 2026-09-22T21:00:00Z"}
	})
	testutil.Require(t, "update the record", err, nil)

	after, readErr := readRecord(stateRoot)
	testutil.Require(t, "read it back", readErr, nil)
	testutil.Expect(t, "the signed-in sessions", after.Sessions, []string{"signed in: Wido until 2026-09-22T21:00:00Z"})
	testutil.Expect(t, "and nothing else moved", after.Address, before.Address)
	testutil.Expect(t, "the schema is unchanged", after.SchemaVersion, 1)
}

// A record nobody wrote is a server that has not started or has stopped.
// Neither is an error, and neither is recreated by an update.
func TestUpdateWritesNoRecordWhereThereIsNone(t *testing.T) {
	t.Parallel()
	stateRoot := t.TempDir()
	testutil.Require(t, "create the state directory", os.MkdirAll(Dir(stateRoot), 0o755), nil)

	touched := false
	err := Update(stateRoot, func(*Record) { touched = true })

	testutil.Require(t, "update a missing record", err, nil)
	testutil.Expect(t, "nothing was applied", touched, false)
	if _, statErr := os.Stat(recordPath(stateRoot)); !os.IsNotExist(statErr) {
		t.Fatalf("an update recreated the record: %v", statErr)
	}
}

// `ui status` says who else is acting through this server, under the line
// that says whether the server can act at all.
func TestStatusPrintsTheSignedInSessions(t *testing.T) {
	t.Parallel()
	stateRoot := t.TempDir()
	_, exact := resultTestRecord(t, stateRoot)
	testutil.Require(t, "record the sessions", Update(stateRoot, func(rec *Record) {
		rec.Authority = "acting as human:Wido — proven at the enrolled terminal"
		rec.Sessions = []string{
			"signed in: Wido until 2026-09-22T21:00:00Z",
			"signed in: Sol until 2026-09-22T22:00:00Z",
		}
	}), nil)

	result := StatusResult(stateRoot, resultTestProber{exact: exact, state: identity.Alive},
		func() (string, error) { return "sha256:serving", nil })

	testutil.Expect(t, "status result", result, Result{Lines: []string{
		"interface running at http://127.0.0.1:49152 (pid 4201, started 2026-09-21T12:34:56Z, build dev-test)",
		"acting as human:Wido — proven at the enrolled terminal",
		"signed in: Wido until 2026-09-22T21:00:00Z",
		"signed in: Sol until 2026-09-22T22:00:00Z",
	}, Code: 0})
}

// A code accepted before a restart is still spent after one. The server's own
// record is removed when it stops, so the floor lives in a file beside it and
// the next run reads it from there.
func TestTheOneTimeCodeFloorOutlivesTheRunThatWroteIt(t *testing.T) {
	t.Parallel()
	stateRoot := t.TempDir()

	testutil.Expect(t, "a checkout that signed nobody in", ReadSessions(stateRoot),
		SessionFloor{SchemaVersion: 1})

	testutil.Require(t, "record a spent code", WriteSessions(stateRoot,
		SessionFloor{LastStep: 58_000_000, Human: "Wido"}), nil)
	// The run that wrote it ends: its record goes, its lock goes, its sessions
	// go with the process that observed them.
	testutil.Require(t, "the run ends", removeRecord(stateRoot), nil)

	floor := ReadSessions(stateRoot)
	testutil.Expect(t, "the floor the next run may not go below", floor.LastStep, int64(58_000_000))
	testutil.Expect(t, "the handle it still knows", floor.Human, "Wido")
	testutil.Expect(t, "the schema it was written under", floor.SchemaVersion, 1)
}

// The rule the floor exists for is that a spent step stays spent, so a write
// that would lower it is refused rather than applied.
func TestTheFloorNeverMovesBackwards(t *testing.T) {
	t.Parallel()
	stateRoot := t.TempDir()
	testutil.Require(t, "set the floor", WriteSessions(stateRoot, SessionFloor{LastStep: 58_000_000}), nil)

	err := WriteSessions(stateRoot, SessionFloor{LastStep: 57_999_999})

	if err == nil {
		t.Fatal("the floor was lowered")
	}
	testutil.Expect(t, "the floor stands", ReadSessions(stateRoot).LastStep, int64(58_000_000))
}

// A floor file that is not this schema, or not readable at all, is a checkout
// that has signed nobody in rather than a server that refuses to start.
func TestAnUnreadableFloorReadsAsNobodySignedIn(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"not json":       "{",
		"another schema": `{"schemaVersion":2,"lastStep":58000000}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stateRoot := t.TempDir()
			testutil.Require(t, "create the state directory", os.MkdirAll(Dir(stateRoot), 0o755), nil)
			testutil.Require(t, "write the floor",
				os.WriteFile(sessionsPath(stateRoot), []byte(content), 0o644), nil)

			testutil.Expect(t, "what it reads as", ReadSessions(stateRoot), SessionFloor{SchemaVersion: 1})
		})
	}
}
