package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
)

// validateBed is a kernel bed whose engine is enrolled (this test binary),
// with landing validate's run recorded or left to production.
type validateBed struct {
	*kernelBed
	outcome  gaterun.ValidateOutcome
	err      error
	requests []laneValidateRequest
	// production leaves landing validate to its production steps.
	production bool
}

func newValidateVerbBed(t *testing.T) *validateBed {
	t.Helper()
	bed := &validateBed{kernelBed: newKernelBed(t)}
	bed.enroll(t, runningTestBinary(t))
	// A registered lane has a machine nickname (landing set requires it):
	// the lane's claim identity names it.
	if out, err := exec.Command("git", "-C", bed.checkout, "config", "metasystem.goal.machine", "landing").CombinedOutput(); err != nil {
		t.Fatalf("name the lane's machine: %v %s", err, out)
	}
	return bed
}

func (bed *validateBed) run(t *testing.T, words ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	owners := bed.kernelBed.owners()
	if !bed.production {
		owners.landing.validateRun = func(request laneValidateRequest) (gaterun.ValidateOutcome, error) {
			bed.requests = append(bed.requests, request)
			return bed.outcome, bed.err
		}
	}
	var stdout, stderr strings.Builder
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, owners)
	return code, stdout.String(), stderr.String()
}

// An unapproved standing-validation goal: nothing runs, nothing is
// written, and the refusal is two plain lines, the second the command that
// fixes it.
func TestLandingValidateNamesTheAuthorityGapInTwoLines(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	bed.outcome = gaterun.ValidateOutcome{Result: gaterun.ValidateUnavailable, Code: gaterun.CodeValidateAuthority,
		Reason: "goal standing-validation is not approved, so no landing validation runs", Fix: "metasystem goal approve standing-validation"}
	code, stdout, stderr := bed.run(t, "landing", "validate")
	if code == 0 || len(bed.requests) != 1 || bed.requests[0].Force {
		t.Fatalf("exit %d, requests %+v\n%s%s", code, bed.requests, stdout, stderr)
	}
	// The run claims the standing authority as the lane's stable claim
	// identity at its custody epoch (K7).
	if claim, err := lane.Claim(bed.home); err != nil || bed.requests[0].Claim != claim {
		t.Fatalf("validate acts as %+v; want the lane's claim identity %+v (%v)", bed.requests[0].Claim, claim, err)
	}
	text := stdout + stderr
	for _, want := range []string{"goal standing-validation is not approved, so no landing validation runs; main is untouched",
		"metasystem goal approve standing-validation"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, gaterun.CodeValidateAuthority) {
		t.Errorf("default text shows the code:\n%s", text)
	}
	_, jsonOut, jsonErr := bed.run(t, "landing", "validate", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(jsonOut+jsonErr), &result); err != nil || result.Outcome != intentRefused || result.Next == nil ||
		strings.Join(result.Next.Argv, " ") != "metasystem goal approve standing-validation" {
		t.Fatalf("--json = %+v %v\n%s%s", result, err, jsonOut, jsonErr)
	}
}

// Going past custody that can't be read is a person's act; the person's
// name reaches the run, and without one nothing runs.
func TestLandingValidateForceIsAPersonsAct(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	code, stdout, stderr := bed.run(t, "landing", "validate", "--force")
	if code != 3 || len(bed.requests) != 0 || !strings.Contains(stderr, "only a person may force the landing validation") {
		t.Fatalf("force by no person: exit %d, requests %+v\n%s%s", code, bed.requests, stdout, stderr)
	}
	bed.person = nil
	bed.outcome = gaterun.ValidateOutcome{Result: gaterun.ValidateFinalized, RunID: "cadence-run-1",
		Key:    goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)},
		Status: &goal.CadenceStatus{Groups: []goal.CadenceGroupStatus{{Group: "deep", Status: "passed"}}}, Discharged: true}
	code, stdout, stderr = bed.run(t, "landing", "validate", "--force")
	if code != 0 || len(bed.requests) != 1 || !bed.requests[0].Force || bed.requests[0].By != "Wido" {
		t.Fatalf("force by the person: exit %d, requests %+v\n%s%s", code, bed.requests, stdout, stderr)
	}
	if flat := strings.Join(strings.Fields(stdout), " "); !strings.Contains(flat, "published the landing validation of main 111111111111: green (run cadence-run-1); the validation weight was reset") {
		t.Fatalf("finalized text:\n%s--\n%s", stdout, stderr)
	}
}

// Unknown custody holds the validation, and its second line is the
// person's forced run.
func TestLandingValidateUnknownCustodyNamesTheForcedRun(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	bed.outcome = gaterun.ValidateOutcome{Result: gaterun.ValidateUnavailable, Code: gaterun.CodeValidateCustodyUnknown,
		Reason: "whether other landing work still runs can't be read, so no validation started", Unknown: []string{"prove b1: its launcher died"}}
	code, stdout, stderr := bed.run(t, "landing", "validate", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil || code == 0 || result.Next == nil ||
		!strings.HasSuffix(strings.Join(result.Next.Argv, " "), "landing validate --force") {
		t.Fatalf("exit %d, result %+v %v\n%s%s", code, result, err, stdout, stderr)
	}
}

// Production: a stopped lane holds landing validate before it reads or
// writes anything (K2); no reservation is made.
func TestLandingValidateHoldsWhileTheLaneIsStopped(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	bed.production = true
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(t, "landing", "validate")
	if code == 0 || !strings.Contains(stdout+stderr, "stopped") {
		t.Fatalf("validate on a stopped lane: exit %d\n%s%s", code, stdout, stderr)
	}
	if _, err := os.Stat(cadence.ReservationPath(bed.home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a stopped lane's validate reserved a run: %v", err)
	}
}

// landing engine advance --force is a person's act too: the person's name
// reaches the advance, which records the custody it went past in it.
func TestLandingEngineAdvanceForceIsAPersonsAct(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.enroll(t, runningTestBinary(t))
	var requests []laneengine.AdvanceRequest
	run := func(words ...string) (int, string) {
		command, _ := findIntentAction("landing", "engine")
		owners := bed.owners()
		owners.landing.advance = func(request laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error) {
			requests = append(requests, request)
			return laneengine.AdvanceOutcome{Changed: true, Commit: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"}, nil
		}
		var stdout, stderr strings.Builder
		code := runIntentIn(command, words, &stdout, &stderr, bed.cwd, owners)
		return code, stdout.String() + stderr.String()
	}
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	if code, text := run("advance", "--force"); code != 3 || len(requests) != 0 || !strings.Contains(text, "only a person may force the landing engine's advance") {
		t.Fatalf("force by no person: exit %d, requests %d\n%s", code, len(requests), text)
	}
	bed.person = nil
	if code, text := run("advance", "--force"); code != 0 || len(requests) != 1 || !requests[0].Force || requests[0].By != "Wido" {
		t.Fatalf("force by the person: exit %d, requests %+v\n%s", code, requests, text)
	}
	if code, _ := run("advance"); code != 0 || len(requests) != 2 || requests[1].Force {
		t.Fatalf("plain advance: exit %d, requests %+v", code, requests)
	}
}

func init() {
	registerIdempotency("landing validate", idemStateful,
		"the key of landed main is already validated: success with that result, nothing run, reserved or written", witnessLandingValidateRepeat)
}

// completedValidationSeams are landing validate's seams over the real
// reservation and custody stores in home, whose ledger already holds the
// status of the key that is due: nothing may run, launch or publish.
func completedValidationSeams(t *testing.T, home string) gaterun.ValidateSeams {
	tree := strings.Repeat("1", 40)
	key := goal.CadenceClaimKey{TrunkTree: tree, WeightGeneration: 3, ForcedWindowStart: "2026-09-30T18:00:00Z"}
	refuse := func(what string) { t.Errorf("a completed key reached %s", what) }
	return gaterun.ValidateSeams{
		Clock:       func() time.Time { return laneTestNow },
		Reservation: func() (*gaterun.Validation, error) { return cadence.ReadReservation(home) },
		Start: func(v gaterun.Validation) (gaterun.Validation, error) {
			refuse("start")
			return v, nil
		},
		Clear: func(v gaterun.Validation) error { return cadence.ClearReservation(home, v) },
		Gap:   func() error { return nil },
		Plan: func() (gaterun.ValidationPlan, error) {
			return gaterun.ValidationPlan{Trunk: gaterun.CadenceTrunk{Commit: strings.Repeat("2", 40), Tree: tree}, Key: key, Due: true}, nil
		},
		Latest: func() (*goal.CadenceStatus, error) {
			return &goal.CadenceStatus{TrunkTree: tree, WeightGeneration: 3, ForcedWindowStart: key.ForcedWindowStart, RunID: "cadence-run-0",
				Groups: []goal.CadenceGroupStatus{{Group: "deep", Status: "passed"}}}, nil
		},
		Publish: func(goal.CadenceClaimKey, goal.CadenceStatus, []goal.TrunkRedRecordGroup) error {
			refuse("publish")
			return nil
		},
	}
}

// witnessLandingValidateRepeat runs landing validate twice on an enrolled
// lane whose key is already validated: both runs are success with the same
// result, and the second leaves the host home as it was.
func witnessLandingValidateRepeat(t *testing.T) {
	bed := newValidateVerbBed(t)
	run := func() (int, string, string) {
		command, _ := findIntentAction("landing", "validate")
		owners := bed.kernelBed.owners()
		owners.landing.validateRun = func(request laneValidateRequest) (gaterun.ValidateOutcome, error) {
			return gaterun.Validate(request.Force, completedValidationSeams(t, request.Home))
		}
		var stdout, stderr strings.Builder
		code := runIntentIn(command, nil, &stdout, &stderr, bed.cwd, owners)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := run(); code != 0 || !strings.Contains(stdout, "already validated (run cadence-run-0)") {
		t.Fatalf("first validate = %d %q %q", code, stdout, stderr)
	}
	before := idemTreeDigest(t, bed.home)
	if code, stdout, stderr := run(); code != 0 || !strings.Contains(stdout, "already validated (run cadence-run-0)") {
		t.Fatalf("repeated validate = %d %q %q", code, stdout, stderr)
	}
	idemSameTree(t, "a repeated landing validate (home)", before, idemTreeDigest(t, bed.home))
}

// landing validate's layout goldens join G1b through the group hook.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, landingValidateLayoutCases)
	return true
}()

func landingValidateLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "landing-validate-completed", args: []string{"landing", "validate"}, bed: landingValidateLayoutBed(gaterun.ValidateOutcome{
			Result: gaterun.ValidateCompleted, RunID: "cadence-run-0", Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}})},
		{name: "landing-validate-authority", args: []string{"landing", "validate"}, bed: landingValidateLayoutBed(gaterun.ValidateOutcome{
			Result: gaterun.ValidateUnavailable, Code: gaterun.CodeValidateAuthority,
			Reason: "goal standing-validation is not approved, so no landing validation runs", Fix: "metasystem goal approve standing-validation"})},
	}
}

// landingValidateLayoutBed is the running lane's bed whose engine is
// admitted, with landing validate's outcome given.
func landingValidateLayoutBed(outcome gaterun.ValidateOutcome) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := landingEngineLayoutBed(false)(t)
		bed.owners.landing.validateRun = func(laneValidateRequest) (gaterun.ValidateOutcome, error) { return outcome, nil }
		return bed
	}
}

// --by may name only the person proven at the terminal: another name is
// refused on both kernel verbs that take --force, and nothing runs.
func TestKernelForceByNamesOnlyTheProvenPerson(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	code, stdout, stderr := bed.run(t, "landing", "validate", "--force", "--by", "Mallory")
	if code != 3 || len(bed.requests) != 0 || !strings.Contains(stdout+stderr, "--by names Mallory, but the person at this terminal is Wido") {
		t.Fatalf("validate --by another: exit %d, requests %+v\n%s%s", code, bed.requests, stdout, stderr)
	}
	bed.outcome = gaterun.ValidateOutcome{Result: gaterun.ValidateNotDue, Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}}
	if code, _, _ := bed.run(t, "landing", "validate", "--force", "--by", "wido"); code != 0 || len(bed.requests) != 1 || bed.requests[0].By != "Wido" {
		t.Fatalf("validate --by the same person: exit %d, requests %+v", code, bed.requests)
	}
	var advances []laneengine.AdvanceRequest
	command, _ := findIntentAction("landing", "engine")
	owners := bed.kernelBed.owners()
	owners.landing.advance = func(request laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error) {
		advances = append(advances, request)
		return laneengine.AdvanceOutcome{Commit: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"}, nil
	}
	var out, errOut strings.Builder
	if code := runIntentIn(command, []string{"advance", "--force", "--by", "Mallory"}, &out, &errOut, bed.cwd, owners); code != 3 || len(advances) != 0 {
		t.Fatalf("advance --by another: exit %d, advances %d\n%s%s", code, len(advances), out.String(), errOut.String())
	}
}

// An unbound obligation names the command that lists every field an
// ENFORCED obligation needs, not a goal edit that would be refused.
func TestLandingValidateObligationGapNamesTheFields(t *testing.T) {
	t.Parallel()
	bed := newValidateVerbBed(t)
	bed.outcome = gaterun.ValidateOutcome{Result: gaterun.ValidateUnavailable, Code: gaterun.CodeValidateAuthority,
		Reason: "goal standing-validation binds no governed obligation, so no landing validation runs", Fix: "metasystem goal edit --help"}
	_, stdout, stderr := bed.run(t, "landing", "validate", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil || result.Next == nil ||
		strings.Join(result.Next.Argv, " ") != "metasystem goal edit --help" || !strings.Contains(result.Next.Reason, "--obligation ENFORCED") {
		t.Fatalf("result = %+v %v", result, err)
	}
}
