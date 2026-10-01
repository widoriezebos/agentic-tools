package main

// landing validate: the lane's scheduled deep validation of landed main
// (lane runtime design r10 §4), a kernel verb. It is gated by the lane's
// pause and custodied (K9): the key and run id are recorded before the run
// launches, a live run is attached, a settled one finalized or run again,
// and a key already published is returned without running.

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// laneValidationLineage is the lineage the lane claims the standing
// validation authority under: the lane's stable claim identity {lane
// machine, lineage landing-lane, custody epoch} (design K7), which unit K-d
// owns.
const laneValidationLineage = "landing-lane"

// laneValidateRequest is one landing validate.
type laneValidateRequest struct {
	Home, Checkout, Installation string
	Epoch                        uint64
	// Force and By: a person's word past custody that can't be read.
	Force bool
	By    string
}

// productionLaneValidate runs landing validate on this host.
func productionLaneValidate(request laneValidateRequest) (gaterun.ValidateOutcome, error) {
	authority := lane.AuthorityAgent
	if request.Force {
		authority = lane.AuthorityPerson
	}
	gate := func(start func() error) error {
		return lane.Gate(request.Home, lane.OpValidate, authority, func(lane.Record) error { return start() })
	}
	// The pause is read before anything is read or waited on; the launch
	// and the publication read it again, each immediately before it acts.
	if err := gate(func() error { return nil }); err != nil {
		return gaterun.ValidateOutcome{}, err
	}
	probes := laneprobe.Production(request.Home, request.Installation, true)
	if request.Force {
		if _, err := custody.Override(request.Home, request.By, probes); err != nil {
			return gaterun.ValidateOutcome{}, err
		}
	}
	validate := cadence.ValidateLane{Home: request.Home, Root: request.Installation,
		Owner: cadence.Owner{Epoch: int64(request.Epoch), Lineage: laneValidationLineage, Require: func() error { return nil },
			FetchOrigin: batchowner.FetchBatchOrigin, WeightThreshold: weightThreshold, Prepare: prepareTestingForCommand, WorkerPolicy: testingWorkerPolicy},
		Clock: time.Now, Probes: probes, Gate: gate, Force: request.Force}
	seams, err := validate.Seams()
	if err != nil {
		return gaterun.ValidateOutcome{}, err
	}
	return gaterun.Validate(request.Force, seams)
}

func landingValidateCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "validate", audience: "both", summary: "run the landing lane's deep validation of landed main when it is due",
		usage: []string{"metasystem landing validate [--force] [--by NAME]"},
		details: []string{"Runs the deep validation of landed main when it is due, under the standing-validation goal, and publishes its result once.",
			"A validation already running is waited for; one that ended is published; a key already published is shown, never run again.",
			"Nothing starts while other landing work still runs; landing work whose state can't be read holds it until a person runs it with --force.",
			"When standing-validation is not approved with an obligation, nothing runs and nothing is written; the result names the command that fixes it."},
		flags:    []intentFlag{{name: "force", usage: "a person goes past landing work whose state can't be read"}, {name: "by", value: "NAME", usage: "the person who forces it"}},
		maxArgs:  0,
		examples: []string{"metasystem landing validate", "metasystem landing validate --force"},
	}, runIntentLandingValidate)
}

func runIntentLandingValidate(inv *intentInvocation, kernel laneKernel) int {
	targets := laneTargets(kernel.record.Root)
	request := laneValidateRequest{Home: kernel.home, Checkout: kernel.record.Root, Installation: kernel.installation, Epoch: kernel.record.CustodyEpoch}
	if inv.input.switched("force") {
		by, refused := inv.forcingPerson(kernel, "the landing validation")
		if refused != nil {
			return inv.render(*refused)
		}
		request.Force, request.By = true, by
	}
	outcome, err := kernel.owners.validateRun(request)
	if err != nil {
		var refusal *lane.Refusal
		if errors.As(err, &refusal) {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: refusal.Message,
				next: refusal.Argv, nextReason: refusal.Fix, Details: []string{"refused because: " + refusal.Code}})
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing validation couldn't go on: " + oneLine(err.Error()), retry: "tries again once the cause is fixed",
			Details: []string{"landing validate: " + err.Error()}})
	}
	return inv.render(landingValidateResult(inv, outcome, targets, kernel.record.Root))
}

// landingValidateResult says what landing validate did in two lines.
func landingValidateResult(inv *intentInvocation, outcome gaterun.ValidateOutcome, targets []intentTarget, root string) intentResult {
	result := intentResult{Targets: targets, Data: outcome}
	if outcome.Code != "" {
		result.Details = append(result.Details, "code: "+outcome.Code)
	}
	result.Details = append(append(result.Details, outcome.Live...), outcome.Unknown...)
	tree := shortLandingID(outcome.Key.TrunkTree)
	switch outcome.Result {
	case gaterun.ValidateFinalized:
		verdict := "red"
		if outcome.Status != nil && validationGreen(*outcome.Status) {
			verdict = "green"
		}
		result.Outcome = intentConfirmed
		result.Summary = "published the landing validation of main " + tree + ": " + verdict + " (run " + outcome.RunID + ")"
		if outcome.Discharged {
			result.Summary += "; the validation weight was reset"
		}
		result.view = landingDone(result.Summary, root)
	case gaterun.ValidateCompleted:
		result.Outcome = intentUnchanged
		result.Summary = "main " + tree + " is already validated (run " + outcome.RunID + "); nothing ran"
		result.view = landingDone(result.Summary, root)
	case gaterun.ValidateNotDue:
		result.Outcome = intentUnchanged
		result.Summary = "no landing validation is due for main " + tree + "; nothing ran"
		result.view = landingDone(result.Summary, root)
	case gaterun.ValidateWaiting:
		result.Outcome, result.code = intentInProgress, 1
		result.Summary = "the landing validation waits: " + firstNonEmpty(outcome.Reason, "other landing work still runs")
		result.next, result.nextReason = inv.publicArgv("landing", "validate"), "runs it once that work has ended"
	case gaterun.ValidatePending:
		result.Outcome, result.code = intentFailed, 1
		result.Summary = "the landing validation run " + outcome.RunID + " ended but isn't recorded yet: " + outcome.Reason
		result.next, result.nextReason = inv.publicArgv("landing", "validate"), "records it once the cause has passed"
	default:
		result.Outcome, result.code = intentRefused, 1
		result.Summary = firstNonEmpty(outcome.Reason, "the landing validation couldn't run") + "; main is untouched"
		switch {
		case strings.HasSuffix(outcome.Fix, " --help"):
			result.next = strings.Fields(outcome.Fix)
			result.nextReason = "lists every field goal edit standing-validation --obligation ENFORCED needs; then run landing validate again"
		case outcome.Fix != "":
			result.next, result.nextReason = strings.Fields(outcome.Fix), "then run metasystem landing validate again"
		case outcome.Code == gaterun.CodeValidateCustodyUnknown:
			result.next, result.nextReason = inv.publicArgv("landing", "validate", "--force"), "a person goes past it once they have checked it"
		default:
			result.next, result.nextReason = inv.publicArgv("landing", "validate"), "tries again once the cause is fixed"
		}
	}
	return result
}

func validationGreen(status goal.CadenceStatus) bool {
	if len(status.Groups) == 0 {
		return false
	}
	for _, group := range status.Groups {
		if group.Status != "passed" && group.Status != "reused" {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// forcingPerson is the person a kernel verb's --force acts for: the one
// proven at the enrolled terminal. --by may only name that same person; it
// never replaces who was proven.
func (inv *intentInvocation) forcingPerson(kernel laneKernel, what string) (string, *intentResult) {
	by, err := kernel.owners.person(kernel.installation)
	if err != nil {
		refused := inv.personRefusal("", err, inv.input.text("by"))
		refused.code = 3
		refused.Summary = "only a person may force " + what + ", and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was changed"
		return "", refused
	}
	if named := strings.TrimSpace(inv.input.text("by")); named != "" && !strings.EqualFold(named, by) {
		argv := slices.DeleteFunc(inv.typedArgv(), func(word string) bool { return word == "--by" || word == named || strings.HasPrefix(word, "--by=") })
		return "", &intentResult{Outcome: intentRefused, code: 3, Targets: laneTargets(kernel.record.Root),
			Summary: "--by names " + named + ", but the person at this terminal is " + by + "; nothing was changed",
			next:    argv, nextReason: "forces it as " + by}
	}
	return by, nil
}
