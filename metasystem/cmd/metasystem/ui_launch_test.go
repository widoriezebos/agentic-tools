package main

// The interface's half of a launch under g1-s72 D1: the starter is the one
// process holding an accepted proof, so it checks that proof itself, stamps
// the record it creates with the proof's verdict, and leaves the proof as
// audit evidence before anything is spawned.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

var starterNow = time.Date(2026, 9, 29, 8, 30, 0, 0, time.UTC)

type starterBed struct {
	roots   lifecycle.Roots
	spawned []launch.Record
	args    [][]string
	// atSpawn is what the spawn seam saw on disk when it was called.
	proofAtSpawn []bool
	spawnErr     error
}

func newStarterBed(t *testing.T) *starterBed {
	t.Helper()
	checkout := t.TempDir()
	return &starterBed{roots: lifecycle.Roots{
		Checkout: checkout, Installation: stateroottest.Installation(t, filepath.Join(checkout, "metasystem")), StateRoot: stateroottest.State(t, t.TempDir()),
	}}
}

func (b *starterBed) starter() func(*session.Session, launch.Request) (launch.Record, error) {
	return launchStarterWith(b.roots, launchSeams{
		// The fleet as seatLaunchFacts reads it, with nothing taken: a
		// resume is exempted for what its record says it created.
		facts: func(asked launch.Request, record launch.Record) (launch.Facts, error) {
			facts := launch.Facts{EvidenceRoot: "/evidence/agentic-tools-ui"}
			if asked.Resuming() {
				facts.Created = record.Created
			}
			return facts, nil
		},
		spawn: func(roots lifecycle.Roots, asked launch.Request, record launch.Record, path string) error {
			b.spawned = append(b.spawned, record)
			b.args = append(b.args, launchArgs(roots, asked, record, path))
			_, err := os.Stat(b.proofPath(record))
			b.proofAtSpawn = append(b.proofAtSpawn, err == nil)
			return b.spawnErr
		},
		now: func() time.Time { return starterNow },
	})
}

func (b *starterBed) proofPath(record launch.Record) string {
	return b.roots.StateRoot.Path("artifacts", "agents", "authority", "proofs",
		launchProofOperation(record.Launch, starterNow)+".json")
}

func signedSession(t *testing.T, root, reference string) *session.Session {
	t.Helper()
	proof, err := humanauthority.SignedInSessionProof(root, "wido", reference, "browser", starterNow)
	if err != nil {
		t.Fatal(err)
	}
	// The session's own Human is not what is stamped: the proof is.
	return &session.Session{Reference: reference, Human: "someone-else", Proof: proof}
}

func freshAsk(t *testing.T) launch.Request {
	return launch.Request{Machine: "m1f", Destination: filepath.Join(t.TempDir(), "agentic-tools-m1f")}
}

func TestTheStarterRefusesANilSessionAndAProofNotValidForTheStateRoot(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	elsewhere := signedSession(t, t.TempDir(), "01M3SESSIONREFERENCE000000")
	for what, signed := range map[string]*session.Session{
		"no session":               nil,
		"a session with no proof":  {Reference: "01M3SESSIONREFERENCE000000", Human: "wido"},
		"another root's proof":     elsewhere,
		"a proof for the checkout": signedSession(t, bed.roots.Checkout, "01M3SESSIONREFERENCE000000"),
	} {
		if _, err := bed.starter()(signed, freshAsk(t)); err == nil {
			t.Fatalf("%s: the starter launched", what)
		}
	}
	if len(bed.spawned) != 0 {
		t.Fatalf("spawned %d launches", len(bed.spawned))
	}
	if entries, _ := os.ReadDir(launch.Dir(bed.roots.Checkout)); len(entries) != 0 {
		t.Fatalf("records were written: %v", entries)
	}
}

func TestTheStarterStampsTheEnrollmentFromTheProofAndTheServersClock(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	signed := signedSession(t, bed.roots.StateRoot.Path(), "01M3SESSIONREFERENCE000000")
	record, err := bed.starter()(signed, freshAsk(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	want := launch.Enrollment{
		Kind: launch.EnrollmentHumanSession, Provider: "browser", Human: "wido",
		Session: "01M3SESSIONREFERENCE000000", At: "2026-09-29T08:30:00Z",
	}
	onDisk, err := launch.Load(bed.roots.Checkout, record.Launch)
	if err != nil {
		t.Fatal(err)
	}
	for what, held := range map[string]launch.Record{"answered": record, "on disk": onDisk, "spawned": bed.spawned[0]} {
		if held.Enrollment == nil || *held.Enrollment != want {
			t.Fatalf("%s enrollment = %+v, want %+v", what, held.Enrollment, want)
		}
		if held.ReviewBy != "" {
			t.Fatalf("%s record names a review date %q", what, held.ReviewBy)
		}
	}
	// Nothing about the human travels on the verb's argv: the record is
	// what the clone's arm reads.
	for _, arg := range bed.args[0] {
		if arg == "wido" || arg == "01M3SESSIONREFERENCE000000" || arg == "--temporary-human-word" || arg == "--review-by" {
			t.Fatalf("argv carries %q: %v", arg, bed.args[0])
		}
	}
	if !slices.Contains(bed.args[0], "--record") {
		t.Fatalf("argv names no record: %v", bed.args[0])
	}
}

func TestTheProofIsRecordedBeforeTheSpawn(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	signed := signedSession(t, bed.roots.StateRoot.Path(), "01M3SESSIONREFERENCE000000")
	record, err := bed.starter()(signed, freshAsk(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(bed.proofAtSpawn) != 1 || !bed.proofAtSpawn[0] {
		t.Fatal("the proof artifact was not on disk when the verb was spawned")
	}
	data, err := os.ReadFile(bed.proofPath(record))
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		Action string               `json:"action"`
		Proof  humanauthority.Proof `json:"proof"`
	}
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	if written.Action != "seat launch" || written.Proof.ChannelUser != "wido" || written.Proof.ChannelRef != "01M3SESSIONREFERENCE000000" {
		t.Fatalf("proof artifact = %+v", written)
	}
}

func TestAnUnrecordableProofStartsNothing(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	// A file where the proofs directory's parent should be: nothing can be
	// written beneath it.
	blocked := bed.roots.StateRoot.Path("artifacts", "agents", "authority")
	if err := os.MkdirAll(filepath.Dir(blocked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	signed := signedSession(t, bed.roots.StateRoot.Path(), "01M3SESSIONREFERENCE000000")
	if _, err := bed.starter()(signed, freshAsk(t)); err == nil {
		t.Fatal("a launch started with a proof that could not be recorded")
	}
	if len(bed.spawned) != 0 {
		t.Fatal("the verb was spawned")
	}
	if entries, _ := os.ReadDir(launch.Dir(bed.roots.Checkout)); len(entries) != 0 {
		t.Fatalf("a record was written: %v", entries)
	}
}

// A retry re-stamps a record that carries an enrollment under the session
// retrying it, and never stamps one created without (S72-02).
func TestARetryReStampsOnlyARecordThatCarriesAnEnrollment(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	destination := t.TempDir()
	ended := "2026-09-29T07:00:00Z"
	stopped := func(id string, enrollment *launch.Enrollment) launch.Record {
		record := launch.Record{
			SchemaVersion: launch.SchemaVersion, Launch: id, Machine: "m1f", Destination: destination,
			Outcome: launch.OutcomeFailed, EndedAt: &ended, ReviewBy: "",
			Created:    launch.Created{Destination: true, Nickname: true, EvidenceRoot: true},
			Enrollment: enrollment,
		}
		if err := launch.Save(bed.roots.Checkout, record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	const withOne, without = "01M3RETRYWITHENROLLMENT000", "01M3RETRYWITHOUTENROLLMENT"
	stopped(withOne, &launch.Enrollment{
		Kind: launch.EnrollmentHumanSession, Provider: "browser", Human: "wido",
		Session: "01M3OLDSESSION000000000000", At: "2026-09-29T06:00:00Z",
	})
	legacy := stopped(without, nil)
	legacy.ReviewBy = "2026-10-02"
	if err := launch.Save(bed.roots.Checkout, legacy); err != nil {
		t.Fatal(err)
	}

	signed := signedSession(t, bed.roots.StateRoot.Path(), "01M3NEWSESSION000000000000")
	retried, err := bed.starter()(signed, launch.Request{Resume: withOne})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Enrollment == nil || retried.Enrollment.Session != "01M3NEWSESSION000000000000" || retried.Enrollment.At != "2026-09-29T08:30:00Z" {
		t.Fatalf("re-stamped enrollment = %+v", retried.Enrollment)
	}

	kept, err := bed.starter()(signed, launch.Request{Resume: without})
	if err != nil {
		t.Fatalf("legacy retry: %v", err)
	}
	onDisk, err := launch.Load(bed.roots.Checkout, without)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Enrollment != nil || onDisk.Enrollment != nil {
		t.Fatalf("a record created without an enrollment was stamped: %+v", onDisk.Enrollment)
	}
	if onDisk.ReviewBy != "2026-10-02" {
		t.Fatalf("the pair launch's review date = %q, want it kept", onDisk.ReviewBy)
	}
}

// A spawn that fails is a failed launch, written as one.
func TestASpawnThatFailsIsRecordedAsFailed(t *testing.T) {
	t.Parallel()
	bed := newStarterBed(t)
	bed.spawnErr = errors.New("the launch could not be started: no engine")
	signed := signedSession(t, bed.roots.StateRoot.Path(), "01M3SESSIONREFERENCE000000")
	if _, err := bed.starter()(signed, freshAsk(t)); err == nil {
		t.Fatal("a failed spawn answered a launch")
	}
	records, err := launch.List(bed.roots.Checkout)
	if err != nil || len(records) != 1 || records[0].Outcome != launch.OutcomeFailed {
		t.Fatalf("records = %+v, %v", records, err)
	}
}

// The verb is spawned with the record it was handed and never with a pair.
func TestTheSpawnedVerbCarriesNoPair(t *testing.T) {
	t.Parallel()
	roots := lifecycle.Roots{Checkout: "/w/agentic-tools", Installation: "/w/agentic-tools/metasystem"}
	record := launch.Record{Launch: "01M3BQAVYXE2AT6F0JG9YB64PG"}
	fresh := launchArgs(roots, launch.Request{Machine: "m1f", Destination: "/w/agentic-tools-m1f", Word: "w", ReviewBy: "2026-10-02"}, record, "/w/r.json")
	want := []string{"seat", "launch", "--from", "/w/agentic-tools", "--record", "/w/r.json", "--machine", "m1f", "--destination", "/w/agentic-tools-m1f"}
	if !slices.Equal(fresh, want) {
		t.Fatalf("fresh argv = %v, want %v", fresh, want)
	}
	resumed := launchArgs(roots, launch.Request{Resume: record.Launch}, record, "/w/r.json")
	want = []string{"seat", "launch", "--from", "/w/agentic-tools", "--record", "/w/r.json", "--resume", record.Launch}
	if !slices.Equal(resumed, want) {
		t.Fatalf("resume argv = %v, want %v", resumed, want)
	}
}
