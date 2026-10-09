package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

// The launch contract's verbs under R-129-ui (acts are idempotent) and U-idem:
// an act whose effect already holds is success with no second record.
//
// start and stop set a state and are witnessed twice against a real fixture
// application. status and log read. restart, reset and check are creations
// by the design's own words: a restart always replaces the process, a reset
// always remakes the run's data, and a check always runs the group freshly
// (plans/designs/app-launch-contract.md, D2 and D3).
func init() {
	registerIdempotency("app start", idemStateful,
		"the application already runs, or is already starting, at the run's address: success, the run rejoined, nothing launched and no second record", witnessAppStartRepeat)
	registerIdempotency("app stop", idemStateful,
		"no run is recorded, or the run has already ended: success, nothing signalled and nothing removed twice", witnessAppStopRepeat)
	registerIdempotency("app restart", idemCreation,
		"an explicit request for a fresh application process from what is on disk; a second restart replaces the process again, so it is not a repeat; a start of what already runs is app start", nil)
	registerIdempotency("app status", idemRead,
		"reads the run record and probes the application; it writes nothing", nil)
	registerIdempotency("app log", idemRead,
		"reads the run's log, and --follow reads until interrupted; it writes nothing", nil)
	registerIdempotency("app reset", idemCreation,
		"stop, prepare and start again: every reset remakes the run's data by design, so a second reset is another remaking and not a repeat", nil)
	registerIdempotency("app check", idemCreation,
		"every check runs the contract's testing group freshly against the live run and records that run's verdict and time; a second check is a new observation by design", nil)
}

// witnessAppStartRepeat: a second start of a running application rejoins it,
// exits 0, launches nothing, and leaves the run record byte for byte as it was.
//
// What is witnessed is the repeat, so neither start carries a readiness
// clock: the contract declares no probe (the supervisor answers once its
// application is spawned, and the rejoin accepts a running run with no
// probe at its first read). readyMs is 1 so that any readiness clock left on
// this path fails the witness every time instead of on a loaded host.
func witnessAppStartRepeat(t *testing.T) {
	address := appHeldPort(t)
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["ready"] = map[string]any{"kind": applaunch.ReadyNone}
	contract["readyMs"] = 1
	bed := newAppBed(t, contract)
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("first start: %d\n%s", code, out)
	}
	recordPath := filepath.Join(bed.installation, "artifacts", "agents", "app", "standing.json")
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("the first start records the run: %v", err)
	}
	code, result := bed.runJSON("app", "start")
	if code != 0 {
		t.Fatalf("a repeated start is success: %v", result)
	}
	if summary, _ := result["summary"].(string); !strings.Contains(summary, "already running at "+address.address) {
		t.Fatalf("a repeated start says the application already runs: %q", summary)
	}
	after, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("a repeated start writes no second record:\n%s\n---\n%s", before, after)
	}
}

// witnessAppStopRepeat: a stop of a stopped application, and a stop of a run
// that a stop already ended, both exit 0 with nothing signalled and nothing
// removed twice; the state directory is the same before and after.
//
// What is witnessed is the stop, so its start carries no clock: the contract
// declares no readiness probe (the supervisor answers once its application
// is spawned) and the start waits for that answer or the supervisor's exit.
// Two wall-clock waits across two processes (a readiness wait of readyMs in
// the supervisor, readyMs+10s in the start) otherwise decided this witness
// under load. readyMs is 1 so that any readiness clock left on this path
// fails the witness every time instead of on a loaded host.
func witnessAppStopRepeat(t *testing.T) {
	address := appHeldPort(t)
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["ready"] = map[string]any{"kind": applaunch.ReadyNone}
	contract["readyMs"] = 1
	bed := newAppBed(t, contract)
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("start: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "stop"); code != 0 {
		t.Fatalf("first stop: %d\n%s", code, out)
	}
	appDir := filepath.Join(bed.installation, "artifacts", "agents", "app")
	before := idemTreeDigest(t, appDir)
	code, result := bed.runJSON("app", "stop")
	if code != 0 {
		t.Fatalf("a repeated stop is success: %v", result)
	}
	if summary, _ := result["summary"].(string); !strings.Contains(summary, "no application run is recorded") {
		t.Fatalf("a repeated stop says nothing is running: %q", summary)
	}
	idemSameTree(t, "a repeated application stop", before, idemTreeDigest(t, appDir))
}
