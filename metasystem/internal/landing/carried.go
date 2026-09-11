package landing

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type CarryStatus struct {
	Word        string `json:"word"`
	Consumption string `json:"consumption"`
	Reservation string `json:"reservation"`
	Intent      string `json:"intent"`
	Counselor   string `json:"counselor,omitempty"`
	Past        string `json:"past,omitempty"`
	By          string `json:"by,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	Source      string `json:"source,omitempty"`
}

// ReadCarryStatus reports every durable interval the wrapper can resume from.
func ReadCarryStatus(root, carried, goalID, ledgerTip string, now time.Time) (CarryStatus, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return CarryStatus{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return CarryStatus{}, err
	}
	if projection.Tip != ledgerTip {
		return CarryStatus{}, fmt.Errorf("carry-ledger-moved: accepted ledger is %s, not %s", projection.Tip, ledgerTip)
	}
	status := CarryStatus{Word: "missing", Consumption: "none", Reservation: "reservation: none", Intent: "none"}
	word, wordErr := goal.CarryWordAt(projection.Tree, goalID, carried)
	if wordErr == nil {
		status.Past = word.Past
		status.By = word.History.Actor
		status.Workspace = word.Workspace
		status.Source = word.History.Verb
		status.Word = "ok"
		if !goal.CarryWordProven(root, word) {
			status.Word = "unproven"
		} else if !now.Before(word.Expires) {
			status.Word = "expired"
		}
		codeTip := "refs/remotes/origin/main"
		if endpoint.LocalMode() {
			codeTip = "refs/heads/main"
		}
		consumption, consumeErr := goal.CarryConsumptionAt(root, projection.Tree, codeTip, word)
		if consumeErr != nil {
			return CarryStatus{}, consumeErr
		}
		status.Consumption = consumption.Kind
		if consumption.Kind != "none" {
			status.Consumption += ":" + consumption.ID
		}
		if consumption.Kind == "none" {
			if commit := localCarryCommit(root, carried); commit != "" {
				status.Consumption = "local:" + commit
			}
		}
		reservation := goal.CarryReservationAt(projection.Tree, goalID, carried, now)
		status.Reservation = "reservation: " + reservation.State
		if reservation.History.Opid != "" {
			status.Reservation += ":" + reservation.History.Opid
		}
	}
	entries, err := goal.Entries(root)
	if err != nil {
		return CarryStatus{}, err
	}
	for _, entry := range entries {
		if entry.Phase == goal.PhaseCreated && entry.Intent.Verb == "carried" && entry.Intent.Args["approvedRef"] == carried {
			status.Intent = "carrying:" + entry.Opid
		}
	}
	if strings.HasPrefix(status.Consumption, "ledger:") {
		rowOpid := strings.TrimPrefix(status.Consumption, "ledger:")
		status.Counselor = "counselor: missing"
		if carriedCounselorLinePresent(root, "cl-"+rowOpid) {
			status.Counselor = "counselor: written"
		}
	}
	return status, nil
}

func localCarryCommit(root, carried string) string {
	command := exec.Command("git", "-C", root, "log", "-1", "--format=%H%x00%B", "HEAD")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	commit, message, ok := strings.Cut(string(output), "\x00")
	if !ok || !trailerLinePresent(message, "Carry", carried) {
		return ""
	}
	return strings.TrimSpace(commit)
}

func trailerLinePresent(message, key, value string) bool {
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		if line == key+": "+value {
			count++
		}
	}
	return count == 1
}

func carriedCounselorLinePresent(root, id string) bool {
	file, err := os.Open(filepath.Join(root, "records", "counselor", "carried-landings.jsonl"))
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row.ID == id {
			return true
		}
	}
	return false
}

func carriedRefusal(code, text, provenance string) Observation {
	observation := refuse(code, provenance)
	observation.Refusal = text
	return observation
}

func observeCarried(params ObserveParams) Observation {
	provenance := fmt.Sprintf("carried opid=%s", params.Carried)
	now := params.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	endpoint, err := goal.ResolveEndpoint(params.RepoRoot)
	if err != nil {
		return carriedRefusal("carry-ledger-moved", "the goal ledger endpoint is unreadable; repair it and rerun", provenance)
	}
	if endpoint.LocalMode() {
		return carriedRefusal("carry-remote-required", "a carried landing needs a code remote: set goal.sync-remote", provenance)
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil || projection.Tip != params.LedgerTip {
		return carriedRefusal("carry-ledger-moved", fmt.Sprintf("the accepted goal ledger moved from %s; fetch it and rerun", params.LedgerTip), provenance)
	}
	file := projection.Tree.Live[params.Goal]
	if file == nil {
		return carriedRefusal("carry-goal-not-live", fmt.Sprintf("goal %s is not live", params.Goal), provenance)
	}
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return carriedRefusal("goal-item-not-held", fmt.Sprintf("goal %s cannot be proven held; use goal steal after repairing the base tree", params.Goal), provenance)
	}
	if err := heldGoalError(workspace, baseTree, params.Goal, params.Actor); err != nil {
		return carriedRefusal("goal-item-not-held", fmt.Sprintf("%s; use goal steal", err), provenance)
	}
	word, err := goal.CarryWordAt(projection.Tree, params.Goal, params.Carried)
	if err != nil || word.History.Verb == "carry" && word.History.AuthorityOutcome != goal.AuthorityOutcomeHumanAuthorityProven || word.History.Verb == "answer" && word.History.AuthorityOutcome != goal.AuthorityOutcomeAuthenticatedChannelWord {
		return carriedRefusal("carry-word-missing", fmt.Sprintf("carry word %s is missing on goal %s; fetch the ledger", params.Carried, params.Goal), provenance)
	}
	if params.CarriedBy != "" && params.CarriedBy != word.History.Actor {
		return carriedRefusal("carry-refusal-mismatch", fmt.Sprintf("Carried-By names %s, not the word actor %s", params.CarriedBy, word.History.Actor), provenance)
	}
	if refusal := carryWordAuthorityRefusal(params.RepoRoot, word, provenance); refusal != nil {
		return *refusal
	}
	actorMachine, _, _ := strings.Cut(params.Actor, "+")
	wordMachine, _ := goal.OpidMachine(word.History.Opid)
	if wordMachine != actorMachine {
		return carriedRefusal("carry-seat-mismatch", fmt.Sprintf("word %s belongs to seat %s; run goal carry --supersede %s --transfer on %s", params.Carried, wordMachine, params.Carried, actorMachine), provenance)
	}
	projectWorkspace, err := ProjectWorkspaceTree(params.RepoRoot, params.ProjectTree)
	if err != nil || word.Workspace != projectWorkspace {
		return carriedRefusal("carry-tree-mismatch", fmt.Sprintf("word workspace=%s candidate workspace=%s; issue goal carry --supersede %s", word.Workspace, projectWorkspace, params.Carried), provenance)
	}
	if !goal.CarryableName(params.RepoRoot, word.Past) {
		return carriedRefusal("carry-not-carryable", fmt.Sprintf("%s is not a refusal or testing group that can be carried", word.Past), provenance)
	}
	if !now.Before(word.Expires) {
		return carriedRefusal("carry-word-expired", fmt.Sprintf("word %s expired at %s; issue a fresh goal carry", params.Carried, word.Expires.UTC().Format(time.RFC3339)), provenance)
	}
	codeTip := "refs/remotes/origin/main"
	if endpoint.LocalMode() {
		codeTip = "refs/heads/main"
	}
	consumption, err := goal.CarryConsumptionAt(params.RepoRoot, projection.Tree, codeTip, word)
	if err != nil {
		return carriedRefusal("carry-word-missing", err.Error()+"; fetch the ledger", provenance)
	}
	if consumption.Kind != "none" {
		return carriedRefusal("carry-word-consumed", fmt.Sprintf("word %s is consumed at %s:%s", params.Carried, consumption.Kind, consumption.ID), provenance)
	}
	if debt, found, debtErr := goal.CarryDebtAt(params.RepoRoot, projection.Tree, "HEAD", params.Carried, now); debtErr != nil {
		return carriedRefusal("carry-debt-unpaid", debtErr.Error(), provenance)
	} else if found {
		return carriedRefusal("carry-debt-unpaid", carriedDebtText(debt), provenance)
	}
	open, err := goal.OpenCarryWords(params.RepoRoot, projection.Tree, codeTip, wordMachine, now)
	if err != nil {
		return carriedRefusal("carry-cap-reached", err.Error(), provenance)
	}
	other := make([]goal.CarryWord, 0, len(open))
	for _, candidate := range open {
		if candidate.History.Opid != params.Carried {
			other = append(other, candidate)
		}
	}
	maximum, err := config.CarryOpenMax(filepath.Join(params.RepoRoot, "metasystem.conf"))
	if err != nil || uint64(len(other)) >= maximum {
		return carriedRefusal("carry-cap-reached", fmt.Sprintf("other open words reach the cap: %s", carryWordIDs(other)), provenance)
	}
	if refusal := baseJudgeFenceRefusal(params, provenance); refusal != nil {
		return *refusal
	}
	if params.Judge != "base" && params.Judge != "live" {
		return carriedRefusal("carry-battery-unverified", "no live or base judge decided; rebuild and arm an engine at a good commit", provenance)
	}
	// Path classes and record ownership are never carryable. Evaluate them
	// before the declared refusal so a carry word cannot mask a stronger
	// candidate-policy defect such as a ledger or runtime path.
	if err := ValidateCarriedCandidatePaths(params, projection.Tree); err != nil {
		return carriedRefusal(carriageRefusalCode(err), err.Error(), provenance)
	}

	ordinary := params
	ordinary.Carried, ordinary.ProjectTree, ordinary.LedgerTip, ordinary.Judge, ordinary.LiveFailure = "", "", "", "", ""
	ordinaryObservation := Observe(ordinary)
	result, verifyErr := proofrun.TestResult{}, fmt.Errorf("test verify was not configured")
	if params.VerifyTesting != nil {
		result, verifyErr = params.VerifyTesting()
	}
	decision, missingFailing := decideCarriedMatch(word.Past, params.Judge, params.LiveFailure, ordinaryObservation, result.Delivery, verifyErr)
	if decision == carriedBatteryUnverified {
		return carriedRefusal("carry-battery-unverified", fmt.Sprintf("test verify failed: %v; no word carries an unverified battery; repair the testing tool or its evidence and rerun", verifyErr), provenance)
	}
	if decision == carriedUnneeded {
		return carriedRefusal("carry-unneeded", "the refusal you named did not occur; land without --carried, or name what you see", provenance)
	}
	if decision == carriedMismatch {
		return carriedRefusal("carry-refusal-mismatch", fmt.Sprintf("the landing saw ordinary=%s missing-or-failing=%v uncovered=%v discrepancies=%v, not %s", ordinaryObservation.Code, missingFailing, result.Delivery.UncoveredObligations, result.Delivery.Discrepancies, word.Past), provenance)
	}
	provenance = fmt.Sprintf("carried opid=%s past=%s seat=%s ledger=%s judge=%s", params.Carried, word.Past, wordMachine, projection.Tip, params.Judge)
	if params.Judge == "base" {
		provenance += " live-failure=" + params.LiveFailure
	}
	provenance += " " + ordinaryObservation.Provenance
	return Observation{SchemaVersion: 1, Mode: "observe", Bar: BarCarried, Verdict: "pass", Code: "human-carried", Provenance: provenance,
		VerdictTrailer: "pass bar=d carried=" + word.Past + " base=" + ordinaryObservation.VerdictTrailer,
		Detail:         fmt.Sprintf("sufficient=%t missing=%v failing=%v uncovered=%v discrepancies=%v", result.Delivery.Sufficient, result.Delivery.MissingGroups, result.Delivery.FailingGroups, result.Delivery.UncoveredObligations, result.Delivery.Discrepancies)}
}

func carryWordAuthorityRefusal(root string, word goal.CarryWord, provenance string) *Observation {
	if word.History.Verb != "carry" || word.History.AuthorityGeneration > 0 || fixtureauth.FixtureModeRoot(root) {
		return nil
	}
	observation := carriedRefusal("carry-word-unproven", fmt.Sprintf("carry word %s has no proven terminal generation", word.History.Opid), provenance)
	return &observation
}

func baseJudgeFenceRefusal(params ObserveParams, provenance string) *Observation {
	if params.Judge != "base" {
		return nil
	}
	blind, err := baseJudgeBlindPaths(params.RepoRoot, params.ProjectTree)
	if err == nil && len(blind) == 0 {
		return nil
	}
	observation := carriedRefusal("carry-base-judge-blind", fmt.Sprintf("the base judge cannot judge a candidate it cannot read: %s; rebuild the live engine (`steward arm` at a good commit) and retry", strings.Join(blind, ",")), provenance)
	return &observation
}

type carriedMatchDecision string

const (
	carriedMatched           carriedMatchDecision = "human-carried"
	carriedUnneeded          carriedMatchDecision = "carry-unneeded"
	carriedMismatch          carriedMatchDecision = "carry-refusal-mismatch"
	carriedBatteryUnverified carriedMatchDecision = "carry-battery-unverified"
)

// decideCarriedMatch keeps evaluator failure ahead of the ordinary-pass
// shortcut: once the base judge is standing in for a failed live evaluator,
// the word must name that exact defect.
func decideCarriedMatch(past, judge, liveFailure string, ordinary Observation, delivery proofrun.DeliveryJudgment, verifyErr error) (carriedMatchDecision, []string) {
	missingFailing := setUnion(delivery.MissingGroups, delivery.FailingGroups)
	if verifyErr != nil {
		return carriedBatteryUnverified, missingFailing
	}
	evaluatorUnavailable := judge == "base" && liveFailure != "" && ordinary.Verdict == "pass" && delivery.Sufficient
	if evaluatorUnavailable {
		if past == "evaluator-unavailable" {
			return carriedMatched, missingFailing
		}
		return carriedMismatch, missingFailing
	}
	if ordinary.Verdict == "pass" && delivery.Sufficient {
		return carriedUnneeded, missingFailing
	}
	matched := false
	if strings.HasPrefix(past, "group:") {
		group := strings.TrimPrefix(past, "group:")
		allowedOrdinary := ordinary.Verdict == "pass" || containsString([]string{"chain-test-receipt-refused", "chain-full-gate-refused", "tier1-receipt-refused", "tier1-full-gate-refused"}, ordinary.Code)
		matched = allowedOrdinary && len(missingFailing) == 1 && missingFailing[0] == group && len(delivery.UncoveredObligations) == 0 && len(delivery.Discrepancies) == 0
	} else if past != "evaluator-unavailable" {
		matched = ordinary.Verdict == "would-refuse" && ordinary.Code == past && delivery.Sufficient
	}
	if matched {
		return carriedMatched, missingFailing
	}
	return carriedMismatch, missingFailing
}

func carriedDebtText(debt goal.CarryDebt) string {
	switch debt.Kind {
	case "obligation":
		return fmt.Sprintf("carry debt is unpaid: goal %s has open obligation %s at %s; discharge it with a code critic of the commit or goal accept-risk", debt.Goal, debt.ID, debt.Detail)
	case "inflight":
		return fmt.Sprintf("carry debt is unpaid: goal %s has carrying row %s %s", debt.Goal, debt.ID, debt.Detail)
	default:
		return fmt.Sprintf("carry debt is unpaid: word %s landed as %s without a carried row; run land.sh --carried %s", debt.ID, debt.Detail, debt.ID)
	}
}

func carryWordIDs(words []goal.CarryWord) string {
	ids := make([]string, 0, len(words))
	for _, word := range words {
		ids = append(ids, word.History.Opid)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func setUnion(groups ...[]string) []string {
	seen := map[string]bool{}
	for _, list := range groups {
		for _, value := range list {
			seen[value] = true
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func baseJudgeBlindPaths(root, projectTree string) ([]string, error) {
	command := exec.Command("git", "-C", root, "diff-tree", "-r", "--name-only", "HEAD^{tree}", projectTree, "--")
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	prefix, err := (gittree.Workspace{Dir: root}).Prefix()
	if err != nil {
		return nil, err
	}
	prefix = strings.Trim(filepath.ToSlash(prefix), "/")
	var blind []string
	for _, raw := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		path := filepath.ToSlash(strings.TrimSpace(raw))
		if prefix != "" {
			if !strings.HasPrefix(path, prefix+"/") {
				continue
			}
			path = strings.TrimPrefix(path, prefix+"/")
		}
		if baseJudgeOwns(path) {
			blind = append(blind, path)
		}
	}
	sort.Strings(blind)
	return blind, nil
}

func baseJudgeOwns(path string) bool {
	owners := []string{"internal/landing/", "internal/goal/", "internal/proofrun/", "internal/testpolicy/", "internal/behaviorsurface/", "internal/config/", "internal/refusal/", "internal/governance/", "internal/humanauthority/", "internal/fixtureauth/"}
	exact := map[string]bool{"metasystem.conf": true, "testing.json": true, "scripts/agents/landing-classes.json": true, "scripts/agents/path-classes.txt": true, "scripts/agents/landing-promotion.json": true}
	if exact[path] {
		return true
	}
	for _, owner := range owners {
		if strings.HasPrefix(path, owner) {
			return true
		}
	}
	return false
}
