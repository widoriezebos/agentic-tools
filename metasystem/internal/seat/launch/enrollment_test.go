package launch

// Step 7 as g1-s72 D4 leaves it: a record the interface stamped with a
// signed-in session's verdict is armed from that record, a pair request is
// armed with the pair as before, a fresh launch with neither is armed at the
// terminal it runs from, and a record with no enrollment of its own is never
// armed by the new path.

import (
	"strings"
	"testing"
)

const (
	sessionHuman  = "wido"
	sessionRef    = "01M3SESSIONREFERENCE000000"
	sessionRecord = "/w/agentic-tools/artifacts/agents/ui/launches/" + launchID + ".json"
)

// sessionRequest is a browser launch: no pair ever travels with it.
func sessionRequest() Request {
	return Request{Machine: machineName, From: fromRoot, Destination: destRoot}
}

// enrolled is a record the interface stamped before it spawned the verb.
func enrolled() Record {
	record := fresh()
	record.Machine, record.Destination = machineName, destRoot
	record.Enrollment = &Enrollment{
		Kind: EnrollmentHumanSession, Provider: "browser", Human: sessionHuman,
		Session: sessionRef, At: "2026-09-29T08:00:00Z",
	}
	return record
}

func armCommands(runner *fakeRunner) []string {
	armed := []string{}
	for _, key := range runner.keys() {
		if strings.HasPrefix(key, "metasystem steward arm") {
			armed = append(armed, key)
		}
	}
	return armed
}

func TestASessionEnrolledRecordArmsFromTheRecordAndCarriesNoWord(t *testing.T) {
	t.Parallel()
	built := newWorld(sessionRequest())
	built.sequencer.RecordPath = sessionRecord
	record, err := built.sequencer.Run(enrolled())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "metasystem steward arm --repo " + destInstall() + " --launch-record " + sessionRecord
	if armed := armCommands(built.runner); len(armed) != 1 || armed[0] != want {
		t.Fatalf("arm = %v, want [%s]", armed, want)
	}
	step, _ := record.StepOf(StepEnrollment)
	if step.Outcome != StepDone || step.Words != "enrolled as wido from a signed-in browser session" {
		t.Fatalf("enrollment step = %+v", step)
	}
	if record.ReviewBy != "" {
		t.Fatalf("a session launch recorded a review date %q", record.ReviewBy)
	}
	if record.Enrollment == nil || record.Enrollment.Human != sessionHuman {
		t.Fatalf("the record lost its enrollment: %+v", record.Enrollment)
	}
	// The record the arm reads was written with the verdict in it before the
	// arm ran: every write the sequencer made carries it.
	for _, written := range built.written {
		if written.Enrollment == nil {
			t.Fatal("a write of the record dropped its enrollment")
		}
	}
}

func TestASessionEnrolledRecordWithNoPathToHandTheArmIsRefused(t *testing.T) {
	t.Parallel()
	built := newWorld(sessionRequest())
	_, err := built.sequencer.Run(enrolled())
	if err == nil {
		t.Fatal("a session-enrolled record was armed with no record path")
	}
	if armed := armCommands(built.runner); len(armed) != 0 {
		t.Fatalf("armed %v", armed)
	}
}

func TestThePairIsForwardedAsTodayForAPairRequest(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.sequencer.RecordPath = sessionRecord
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "metasystem steward arm --repo " + destInstall() + " --temporary-human-word " + humanWord + " --review-by " + reviewBy
	if armed := armCommands(built.runner); len(armed) != 1 || armed[0] != want {
		t.Fatalf("arm = %v, want [%s]", armed, want)
	}
	if step, _ := record.StepOf(StepEnrollment); step.Words != "temporary enrollment, review due "+reviewBy {
		t.Fatalf("enrollment step = %+v", step)
	}
}

func TestAFreshLaunchWithNeitherArmsAtTheTerminalItRunsFrom(t *testing.T) {
	t.Parallel()
	built := newWorld(sessionRequest())
	built.sequencer.RecordPath = sessionRecord
	record, err := built.sequencer.Run(fresh())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "metasystem steward arm --repo " + destInstall()
	if armed := armCommands(built.runner); len(armed) != 1 || armed[0] != want {
		t.Fatalf("arm = %v, want [%s]", armed, want)
	}
	if step, _ := record.StepOf(StepEnrollment); step.Words != "enrolled at this terminal" {
		t.Fatalf("enrollment step = %+v", step)
	}
}

// The pair beside a session-enrolled record is refused before anything runs
// or is written: the record already says whose machine this is.
func TestThePairBesideASessionEnrolledRecordIsRefused(t *testing.T) {
	t.Parallel()
	built := newWorld(request())
	built.sequencer.RecordPath = sessionRecord
	_, err := built.sequencer.Run(enrolled())
	refusal, named := err.(*Refusal)
	if !named || refusal.Code != CodeWordInvalid {
		t.Fatalf("error = %v, want %s", err, CodeWordInvalid)
	}
	if len(built.runner.ran) != 0 || len(built.written) != 0 {
		t.Fatalf("ran %v and wrote %d records before refusing", built.runner.keys(), len(built.written))
	}
}

// S72-01 (round 2): a session-enrolled record is authoritative for its
// machine and destination on a fresh invocation as on a resume.
func TestASessionRecordRefusesAnotherMachineOrDestination(t *testing.T) {
	t.Parallel()
	for what, asked := range map[string]Request{
		"another machine":     {Machine: "m1g", From: fromRoot, Destination: destRoot},
		"another destination": {Machine: machineName, From: fromRoot, Destination: "/w/agentic-tools-m1g"},
		"a resume elsewhere":  {Machine: "m1g", From: fromRoot, Destination: "/w/agentic-tools-m1g", Resume: launchID},
	} {
		built := newWorld(asked)
		built.sequencer.RecordPath = sessionRecord
		_, err := built.sequencer.Run(enrolled())
		refusal, named := err.(*Refusal)
		if !named || refusal.Code != CodeRecordConflict {
			t.Fatalf("%s: error = %v, want %s", what, err, CodeRecordConflict)
		}
		if len(built.runner.ran) != 0 || len(built.written) != 0 {
			t.Fatalf("%s: ran %v and wrote %d records before refusing", what, built.runner.keys(), len(built.written))
		}
	}
}

func TestAdmitTakesASessionRecordsOwnMachineAndDestination(t *testing.T) {
	t.Parallel()
	if err := Admit(Request{From: fromRoot}, enrolled()); err != nil {
		t.Fatalf("flags left empty: %v", err)
	}
	if err := Admit(Request{Machine: machineName, Destination: destRoot + "/"}, enrolled()); err != nil {
		t.Fatalf("the record's own values: %v", err)
	}
	// A record with no enrollment keeps today's rules: the verb decides.
	legacy := fresh()
	legacy.Machine, legacy.Destination = machineName, destRoot
	if err := Admit(Request{Machine: "m1g", Destination: "/elsewhere", Word: humanWord, ReviewBy: reviewBy}, legacy); err != nil {
		t.Fatalf("a record with no enrollment: %v", err)
	}
}
