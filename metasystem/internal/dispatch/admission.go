package dispatch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

var AdmissionRefusalCodes = []string{"BUDGET_UNKNOWN", "BUDGET_REFUSED", "HAZARD_REFUSED", "RISK_UNANSWERED"}

func ValidateMisclassificationEvidence(repoRoot, goalID, evidence string) error {
	checkRoot := func(jobID string) (map[string]any, error) {
		if !validJobID.MatchString(jobID) {
			return nil, fmt.Errorf("evidence %s has an invalid job identifier", evidence)
		}
		record, err := readObject(filepath.Join(repoRoot, "artifacts", "agents", "jobs", jobID+".json"))
		if err != nil || asString(record["jobId"]) != jobID || asString(record["goalId"]) != goalID {
			return nil, fmt.Errorf("evidence %s does not name a job bound to goal %s", evidence, goalID)
		}
		return record, nil
	}
	if jobID, ok := strings.CutPrefix(evidence, "root:"); ok {
		_, err := checkRoot(jobID)
		return err
	}
	if ref, ok := strings.CutPrefix(evidence, "finding:"); ok {
		jobID, findingID, found := strings.Cut(ref, "/")
		record, err := checkRoot(jobID)
		if err != nil {
			return err
		}
		if !found || findingID == "" {
			return fmt.Errorf("evidence %s does not name a finding", evidence)
		}
		for _, value := range anySlice(record[findingRegisterField]) {
			if row, ok := value.(map[string]any); ok && asString(row["findingId"]) == findingID {
				return nil
			}
		}
		return fmt.Errorf("evidence %s does not name a finding in job %s", evidence, jobID)
	}
	if code, ok := strings.CutPrefix(evidence, "refusal:"); ok {
		for _, admitted := range AdmissionRefusalCodes {
			if code == admitted {
				return nil
			}
		}
		return fmt.Errorf("evidence refusal:%s is not an admission refusal code; one of: %s", code, strings.Join(AdmissionRefusalCodes, ", "))
	}
	return fmt.Errorf("evidence %s must be root:<jobId>, finding:<jobId>/<id>, or refusal:<code>", evidence)
}

func anySlice(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	return nil
}

// GoalAdmissionRefusal is one claim whose structured budget closes
// the dispatch admission seam. Unknown evidence closes admission without
// inventing counters; known evidence names every exhausted limit.
type GoalAdmissionRefusal struct {
	GoalID         string
	GoalRevision   uint64
	Breaches       []BudgetBreach
	Reserved       *ReservedMinutesEvidence
	Unknown        *BudgetUnknownEvidence
	LiveStopReason string
}

// GoalAdmissionVerdict contains every refusal owned by the dispatching
// machine and lineage.
type GoalAdmissionVerdict struct {
	Refusals []GoalAdmissionRefusal
}

// Refused reports whether the structured law closes this dispatch round.
func (v GoalAdmissionVerdict) Refused() bool {
	return len(v.Refusals) > 0
}

// EvaluateGoalAdmission applies the four structured limits at the
// pre-reservation seam. It has no side effects: a refusal prevents publication
// but never winds down an existing job.
func EvaluateGoalAdmission(repoRoot, stopLineage string, now time.Time) (GoalAdmissionVerdict, error) {
	return evaluateGoalAdmissionWithReads(repoRoot, stopLineage, now, concreteGoalAdmissionReads())
}

func evaluateGoalAdmissionWithReads(repoRoot, stopLineage string, now time.Time, reads goalAdmissionReads) (GoalAdmissionVerdict, error) {
	var verdict GoalAdmissionVerdict
	if !reads.NewWorld(repoRoot) {
		return verdict, nil
	}
	endpoint, err := reads.ResolveEndpoint(repoRoot)
	if err != nil {
		return verdict, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		if unknown, ok := GoalRecordBudgetUnknown(err); ok {
			return verdict, fmt.Errorf("BUDGET_UNKNOWN record=%s reason=%s", unknown.Record, unknown.Reason)
		}
		return verdict, err
	}
	if stopLineage == "" {
		return verdict, nil
	}

	needsMachine := false
	for _, file := range projection.Tree.Live {
		if file.State == goal.StateClaimed && file.Claimed != nil {
			needsMachine = true
			break
		}
	}
	if !needsMachine {
		return verdict, nil
	}
	machine, err := reads.ResolveMachine(repoRoot)
	if err != nil {
		return verdict, err
	}
	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil ||
			file.Claimed.Machine != machine || file.Claimed.Lineage != stopLineage {
			continue
		}
		// A breach-stopped goal is waiting on a human and must not keep the
		// machine from working the next item; a claim waiting to land keeps
		// its box for its own receipts and never closes the working claim's
		// dispatch.
		if file.IsFencedClaim() || file.IsLandingClaim() {
			continue
		}
		budget := ProjectBudget(repoRoot, file, now)
		if budget.Status == BudgetUnknown {
			verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
				GoalID: id, GoalRevision: file.Claimed.Revision, Unknown: budget.Unknown,
			})
			continue
		}
		breaches := budgetAdmissionBreaches(budget)
		if len(breaches) > 0 {
			verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
				GoalID: id, GoalRevision: budget.GoalRevision, Breaches: breaches,
				Reserved:       reservedMinutesEvidence(budget),
				LiveStopReason: liveStopReason(budget),
			})
		}
	}
	return verdict, nil
}

// GoalRevisionAdmission is the final decision made while the chain and
// goal-revision locks are held. LiveStopReason is non-empty only at the
// elapsed breach boundary or for corrupt over-limit state; ordinary exhaustion
// closes admission without cancelling already-authorized jobs.
type GoalRevisionAdmission struct {
	GoalID         string                `json:"goalId"`
	GoalRevision   uint64                `json:"goalRevision"`
	Refusal        *GoalAdmissionRefusal `json:"refusal,omitempty"`
	PolicyRefusal  string                `json:"policyRefusal,omitempty"`
	PolicyNotice   string                `json:"policyNotice,omitempty"`
	LiveStopReason string                `json:"liveStopReason,omitempty"`
	Extension      *BudgetExtensionOffer `json:"extension,omitempty"`
	ExtendedAt     string                `json:"extendedAt,omitempty"`
}

func (v GoalRevisionAdmission) Refused() bool { return v.Refusal != nil || v.PolicyRefusal != "" }

// ProofAdmissionVerdict keeps claim capacity and candidate consumption as two
// independently attributable decisions. A cross-goal proof can therefore
// spend one goal's attempt box without inheriting that goal's claim clock or
// concurrency box.
type ProofAdmissionVerdict struct {
	Authority GoalRevisionAdmission
	Candidate GoalRevisionAdmission
}

func (v ProofAdmissionVerdict) Refused() bool {
	return v.Authority.Refused() || v.Candidate.Refused()
}

type admissionBudgetLens uint8

const (
	allBudgetMembers admissionBudgetLens = iota
	authorityBudgetMembers
)

// EvaluateGoalRevisionAdmission preserves admission for callers that do not
// dispatch a critic chain.
func EvaluateGoalRevisionAdmission(repoRoot, id string, revision, proposedCap uint64, now time.Time, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionWithReads(repoRoot, id, revision, proposedCap, now, concreteGoalAdmissionReads(), hazards...)
}

func evaluateGoalRevisionAdmissionWithReads(repoRoot, id string, revision, proposedCap uint64, now time.Time, reads goalAdmissionReads, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id, revision, proposedCap, now, "implementer", "fresh", allBudgetMembers, reads, hazards...)
}

// EvaluateGoalRevisionAdmissionForDispatch binds the final fence and projected
// cap decision to the exact accepted revision and dispatch context about to
// publish a reservation.
func EvaluateGoalRevisionAdmissionForDispatch(repoRoot, id string, revision, proposedCap uint64, now time.Time, role, dispatchMode string, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	return evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id, revision, proposedCap, now, role, dispatchMode, allBudgetMembers, concreteGoalAdmissionReads(), hazards...)
}

// EvaluateGoalRevisionAdmissionForDispatchWithReads evaluates the same
// dispatch policy against supplied repository facts.
func EvaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id string, revision, proposedCap uint64, now time.Time, role, dispatchMode string, reads ProofAdmissionReads, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	if err := reads.Validate(); err != nil {
		return GoalRevisionAdmission{}, err
	}
	return evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id, revision, proposedCap, now, role, dispatchMode, allBudgetMembers, reads.private(), hazards...)
}

// EvaluateProofAdmissionForDispatch evaluates the claimed authority's clock
// and concurrency independently from the candidate's attempt and minute
// consumption. An earned extension is actionable only when both lenses name
// the same goal.
func EvaluateProofAdmissionForDispatch(repoRoot, authorityID string, authorityRevision uint64, candidate *goal.GoalFile,
	candidateRevision, proposedCap uint64, now time.Time, role, dispatchMode string, hazards ...HazardClass) (ProofAdmissionVerdict, error) {
	return evaluateProofAdmissionForDispatchWithReads(repoRoot, authorityID, authorityRevision, candidate,
		candidateRevision, proposedCap, now, role, dispatchMode, concreteGoalAdmissionReads(), hazards...)
}

func EvaluateProofAdmissionForDispatchWithReads(repoRoot, authorityID string, authorityRevision uint64, candidate *goal.GoalFile,
	candidateRevision, proposedCap uint64, now time.Time, role, dispatchMode string, reads ProofAdmissionReads, hazards ...HazardClass) (ProofAdmissionVerdict, error) {
	if err := reads.Validate(); err != nil {
		return ProofAdmissionVerdict{}, err
	}
	return evaluateProofAdmissionForDispatchWithReads(repoRoot, authorityID, authorityRevision, candidate,
		candidateRevision, proposedCap, now, role, dispatchMode, reads.private(), hazards...)
}

func evaluateProofAdmissionForDispatchWithReads(repoRoot, authorityID string, authorityRevision uint64, candidate *goal.GoalFile,
	candidateRevision, proposedCap uint64, now time.Time, role, dispatchMode string, reads goalAdmissionReads, hazards ...HazardClass) (ProofAdmissionVerdict, error) {
	var result ProofAdmissionVerdict
	if candidate == nil || candidate.Id == "" {
		return result, fmt.Errorf("proof admission requires a candidate goal")
	}
	if candidate.Id == authorityID {
		verdict, err := evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, authorityID, authorityRevision, proposedCap,
			now, role, dispatchMode, allBudgetMembers, reads, hazards...)
		result.Authority = verdict
		return result, err
	}
	var err error
	result.Authority, err = evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, authorityID, authorityRevision, proposedCap,
		now, role, dispatchMode, authorityBudgetMembers, reads, hazards...)
	if err != nil {
		return result, err
	}
	result.Candidate, err = evaluateCandidateConsumptionAdmissionWithReads(repoRoot, candidate, candidateRevision, proposedCap, now, reads)
	return result, err
}

func evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id string, revision, proposedCap uint64, now time.Time,
	role, dispatchMode string, lens admissionBudgetLens, reads goalAdmissionReads, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	verdict := GoalRevisionAdmission{GoalID: id, GoalRevision: revision}
	if proposedCap == 0 {
		return verdict, fmt.Errorf("goal revision admission requires a positive proposed cap")
	}
	if strings.TrimSpace(role) == "" {
		return verdict, fmt.Errorf("goal revision admission requires a role")
	}
	if dispatchMode != "fresh" && dispatchMode != "follow-up" {
		return verdict, fmt.Errorf("goal revision admission requires dispatch mode fresh or follow-up")
	}
	if len(hazards) > 1 {
		return verdict, fmt.Errorf("exactly one destructiveReach class may govern goal revision admission")
	}
	hazard := HazardMechanical
	if len(hazards) == 1 {
		hazard = hazards[0]
	}
	if _, err := MinimumHazardConfiguration(hazard); err != nil {
		return verdict, err
	}
	binding, err := resolveGoalBindingWithReads(repoRoot, id, now, reads)
	if err != nil {
		return verdict, err
	}
	if binding.Revision != revision {
		return verdict, fmt.Errorf("goal %s accepted revision moved from %d to %d", id, revision, binding.Revision)
	}
	if binding.File.BudgetExtension != nil {
		verdict.ExtendedAt = binding.File.BudgetExtension.At
	}
	if binding.File.Risk == nil {
		verdict.PolicyRefusal = fmt.Sprintf("RISK_UNANSWERED goal=%s tier=%d next: goal edit --risk", id, binding.Tier)
		return verdict, nil
	}
	if binding.Tier == 1 && hazard != HazardMechanical {
		verdict.PolicyRefusal = "HAZARD_REFUSED: the hazard needs review the tier does not have; goal edit --tier 2"
		return verdict, nil
	}
	if binding.Fence != nil {
		verdict.LiveStopReason = binding.Fence.Reason
		verdict.Refusal = &GoalAdmissionRefusal{
			GoalID: id, GoalRevision: revision,
			Unknown: &BudgetUnknownEvidence{Code: BudgetUnknown, Record: goalRecordPath(id),
				Reason: fmt.Sprintf("launch fence closed by stop batch %s", binding.Fence.StopID)},
			LiveStopReason: binding.Fence.Reason,
		}
		return verdict, nil
	}
	projection := ProjectBudget(repoRoot, binding.File, now)
	if projection.Status != BudgetKnown {
		verdict.Refusal = &GoalAdmissionRefusal{GoalID: id, GoalRevision: revision, Unknown: projection.Unknown}
		return verdict, nil
	}
	stopProjection := projection
	if lens == authorityBudgetMembers {
		stopProjection.Breaches = budgetBreachesForLens(projection.Breaches, lens)
	}
	if reason := stopReasonFor(binding.File, stopProjection); reason != "" {
		verdict.LiveStopReason = reason
		verdict.Refusal = &GoalAdmissionRefusal{
			GoalID: id, GoalRevision: revision, Breaches: admissionBreachesFor(binding.File, stopProjection.Breaches),
			Reserved: reservedMinutesEvidence(projection), LiveStopReason: reason,
		}
		return verdict, nil
	}
	breaches := budgetBreachesForLens(admissionBreachesFor(binding.File, budgetAdmissionBreaches(projection)), lens)
	if role == "design-critic" || role == "code-critic" {
		if dispatchMode == "fresh" {
			used := projection.CodeCritiques
			field := "codeCritiques"
			if role == "design-critic" {
				used = projection.DesignCritiques
				field = "designCritiques"
			}
			limit := uint64(projection.Limits.ReviewRoundLimit)
			if used >= limit {
				breaches = append(breaches, budgetIntegerBreach(field, used, limit))
			}
		}
	}
	if lens == allBudgetMembers && proposedCap > 0 && projection.ReservedJobMinutes < projection.Limits.ReservedJobMinutesLimit &&
		proposedCap > projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		breaches = append(breaches, BudgetBreach{
			Field: "reservedJobMinutesLimit",
			Used:  fmt.Sprintf("%d+%d proposed", projection.ReservedJobMinutes, proposedCap),
			Limit: fmt.Sprintf("%d", projection.Limits.ReservedJobMinutesLimit),
		})
	}
	if len(breaches) > 0 {
		verdict.Refusal = &GoalAdmissionRefusal{GoalID: id, GoalRevision: revision, Breaches: breaches,
			Reserved: reservedMinutesEvidence(projection)}
		if lens == allBudgetMembers && binding.File.BudgetExtension == nil && consumptionBreachesOnly(breaches) {
			verdict.Extension, err = budgetExtensionOfferWithReads(repoRoot, binding.File, binding.Tier, now, reads.Receipt)
			if err != nil {
				return verdict, err
			}
			// The raise must admit this very proposal, or the once marker
			// would be spent on a refusal that stands.
			if offer := verdict.Extension; offer != nil &&
				(projection.Attempts >= offer.To.AttemptLimit || proposedCap > offer.To.ReservedJobMinutesLimit-projection.ReservedJobMinutes) {
				verdict.Extension = nil
			}
		}
	}
	return verdict, nil
}

func evaluateCandidateConsumptionAdmissionWithReads(repoRoot string, candidate *goal.GoalFile, revision, proposedCap uint64, now time.Time, reads goalAdmissionReads) (GoalRevisionAdmission, error) {
	verdict := GoalRevisionAdmission{GoalID: candidate.Id, GoalRevision: revision}
	if proposedCap == 0 {
		return verdict, fmt.Errorf("candidate consumption admission requires a positive proposed cap")
	}
	if current := goal.BudgetEpisodeRevision(candidate); current == 0 || current != revision {
		return verdict, fmt.Errorf("candidate goal %s budget episode moved from %d to %d", candidate.Id, revision, current)
	}
	if candidate.BudgetExtension != nil {
		verdict.ExtendedAt = candidate.BudgetExtension.At
	}
	projection := BudgetProjection(ProjectConsumption(repoRoot, candidate, now))
	if projection.Status != BudgetKnown {
		verdict.Refusal = &GoalAdmissionRefusal{GoalID: candidate.Id, GoalRevision: revision, Unknown: projection.Unknown}
		return verdict, nil
	}
	breaches := budgetBreachesForLens(budgetAdmissionBreaches(projection), allBudgetMembers)
	breaches = keepBudgetBreaches(breaches, "attemptLimit", "reservedJobMinutesLimit")
	if projection.ReservedJobMinutes < projection.Limits.ReservedJobMinutesLimit &&
		proposedCap > projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		breaches = append(breaches, BudgetBreach{Field: "reservedJobMinutesLimit",
			Used:  fmt.Sprintf("%d+%d proposed", projection.ReservedJobMinutes, proposedCap),
			Limit: fmt.Sprintf("%d", projection.Limits.ReservedJobMinutesLimit)})
	}
	if len(breaches) == 0 {
		return verdict, nil
	}
	verdict.Refusal = &GoalAdmissionRefusal{GoalID: candidate.Id, GoalRevision: revision,
		Breaches: breaches, Reserved: reservedMinutesEvidence(projection)}
	if candidate.BudgetExtension != nil || !consumptionBreachesOnly(breaches) {
		return verdict, nil
	}
	tier := candidate.Tier
	if tier == 0 {
		tier = 3
	}
	offer, err := budgetExtensionOfferWithReads(repoRoot, candidate, tier, now, reads.Receipt)
	if err != nil {
		return verdict, err
	}
	if offer == nil || projection.Attempts >= offer.To.AttemptLimit ||
		projection.ReservedJobMinutes > offer.To.ReservedJobMinutesLimit ||
		proposedCap > offer.To.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		return verdict, nil
	}
	limits := make([]string, 0, len(breaches))
	for _, breach := range breaches {
		limits = append(limits, fmt.Sprintf("%s used=%s limit=%s", breach.Field, breach.Used, breach.Limit))
	}
	verdict.PolicyRefusal = fmt.Sprintf("CANDIDATE_EXTENSION_REFUSED: candidate goal %s reached %s; remedies: claim %s as authority, or have Wido run goal set-budget for %s",
		candidate.Id, strings.Join(limits, ", "), candidate.Id, candidate.Id)
	return verdict, nil
}

func budgetBreachesForLens(breaches []BudgetBreach, lens admissionBudgetLens) []BudgetBreach {
	if lens == allBudgetMembers {
		return breaches
	}
	return keepBudgetBreaches(breaches, "elapsedLimit", "activeJobLimit")
}

func keepBudgetBreaches(breaches []BudgetBreach, fields ...string) []BudgetBreach {
	kept := make([]BudgetBreach, 0, len(breaches))
	for _, breach := range breaches {
		for _, field := range fields {
			if breach.Field == field {
				kept = append(kept, breach)
				break
			}
		}
	}
	return kept
}

// FormatProofAdmission keeps each refusal attached to the goal and lens that
// produced it.
func FormatProofAdmission(verdict ProofAdmissionVerdict) []string {
	var lines []string
	for _, lens := range []GoalRevisionAdmission{verdict.Authority, verdict.Candidate} {
		if lens.PolicyRefusal != "" {
			lines = append(lines, lens.PolicyRefusal)
			continue
		}
		lines = append(lines, FormatGoalRevisionAdmission(lens)...)
	}
	return lines
}

// consumptionBreachesOnly reports whether every breach is on attempts or
// reserved minutes, the two members consumption spends and the extension
// raises (R-94-m1e): one of them, or both at once, and no other member.
func consumptionBreachesOnly(breaches []BudgetBreach) bool {
	if len(breaches) == 0 {
		return false
	}
	for _, breach := range breaches {
		if breach.Field != "attemptLimit" && breach.Field != "reservedJobMinutesLimit" {
			return false
		}
	}
	return true
}

// FormatGoalRevisionAdmission adds the revision seam's actionable offer or
// durable once marker to the ordinary budget refusal line.
func FormatGoalRevisionAdmission(verdict GoalRevisionAdmission) []string {
	if verdict.Refusal == nil {
		return nil
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	if len(lines) != 1 {
		return lines
	}
	if offer := verdict.Extension; offer != nil {
		lines[0] += fmt.Sprintf("; extension available: %s %s at %s", offer.EvidenceKind, offer.EvidenceID, offer.EvidenceAt)
	} else if verdict.ExtendedAt != "" {
		lines[0] += fmt.Sprintf("; extended once at %s; a further raise is a person's set-budget", verdict.ExtendedAt)
	}
	return lines
}

// stopReasonFor is the live-stop reason for one claim. A claim waiting to
// land (goal land-ready) has its elapsed fence suspended: the wait is
// printed, never stopped; attempts, minutes and active jobs still bind, so
// a corrupt-over-limit stop still fences it.
func stopReasonFor(file *goal.GoalFile, projection BudgetProjection) string {
	if !file.IsLandingClaim() {
		return liveStopReason(projection)
	}
	for _, breach := range projection.Breaches {
		if breach.Field != "elapsedLimit" {
			return goal.StopReasonCorruptOverLimit
		}
	}
	return ""
}

// admissionBreachesFor drops the elapsed dimension for a claim waiting to
// land; every other breach stands.
func admissionBreachesFor(file *goal.GoalFile, breaches []BudgetBreach) []BudgetBreach {
	if !file.IsLandingClaim() {
		return breaches
	}
	kept := make([]BudgetBreach, 0, len(breaches))
	for _, breach := range breaches {
		if breach.Field != "elapsedLimit" {
			kept = append(kept, breach)
		}
	}
	return kept
}

// budgetAdmissionBreaches uses admission boundaries rather than health's
// corrupt-over-limit boundaries. A proposal may fill attempt or minute
// capacity, and an active set may equal its concurrency limit; once current
// spending is at that boundary, another reservation is refused.
func budgetAdmissionBreaches(projection BudgetProjection) []BudgetBreach {
	var breaches []BudgetBreach
	if projection.Elapsed >= projection.Limits.ElapsedDuration() {
		state := projection.ElapsedState
		if state == "" {
			state = AdmissionClosedElapsed
		}
		limit := projection.Limits.ElapsedLimit
		if state == ElapsedBreach && projection.ElapsedBreachLimit > 0 {
			limit = projection.ElapsedBreachLimit.String()
		}
		breaches = append(breaches, BudgetBreach{
			Field: "elapsedLimit", Used: projection.Elapsed.Round(time.Second).String(), Limit: limit, State: state,
		})
	}
	if projection.Attempts >= projection.Limits.AttemptLimit {
		breaches = append(breaches, budgetIntegerBreach("attemptLimit", projection.Attempts, projection.Limits.AttemptLimit))
	}
	if projection.ReservedJobMinutes >= projection.Limits.ReservedJobMinutesLimit {
		breaches = append(breaches, budgetIntegerBreach("reservedJobMinutesLimit", projection.ReservedJobMinutes, projection.Limits.ReservedJobMinutesLimit))
	}
	if projection.ActiveJobs >= projection.Limits.ActiveJobLimit {
		breaches = append(breaches, budgetIntegerBreach("activeJobLimit", projection.ActiveJobs, projection.Limits.ActiveJobLimit))
	}
	return breaches
}

// FormatGoalAdmission renders stable, human-readable evidence without making
// callers parse prose to decide the exit status.
func FormatGoalAdmission(verdict GoalAdmissionVerdict) []string {
	lines := make([]string, 0, len(verdict.Refusals))
	for _, refusal := range verdict.Refusals {
		if refusal.Unknown != nil {
			lines = append(lines, fmt.Sprintf("BUDGET_UNKNOWN record=%s goal=%s revision=%d reason=%s",
				refusal.Unknown.Record, refusal.GoalID, refusal.GoalRevision, refusal.Unknown.Reason))
			continue
		}
		detail := formatRefusalDetail(refusal.Breaches, refusal.Reserved)
		for _, breach := range refusal.Breaches {
			if breach.Field == "attemptLimit" || breach.Field == "reservedJobMinutesLimit" {
				detail += fmt.Sprintf("; rule=%s: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes",
					goalbudget.SetupRefusalReleaseRule)
				break
			}
		}
		line := fmt.Sprintf("BUDGET_REFUSED: goal %s revision=%d admission closed: %s",
			refusal.GoalID, refusal.GoalRevision, detail)
		lines = append(lines, line)
	}
	return lines
}

func formatRefusalDetail(breaches []BudgetBreach, reserved *ReservedMinutesEvidence) string {
	fields := make([]string, 0, len(breaches))
	for _, breach := range breaches {
		state := ""
		if breach.State != "" {
			state = " state=" + string(breach.State)
		}
		if breach.Field == "designCritiques" || breach.Field == "codeCritiques" {
			fields = append(fields, fmt.Sprintf("%s=%s/%s", breach.Field, breach.Used, breach.Limit))
		} else {
			fields = append(fields, fmt.Sprintf("%s%s used=%s limit=%s", breach.Field, state, breach.Used, breach.Limit))
		}
	}
	detail := strings.Join(fields, ", ")
	if reserved != nil {
		if detail != "" {
			detail += "; "
		}
		detail += fmt.Sprintf("reserved observed=%d open-caps=%d", reserved.Observed, reserved.OpenCaps)
		if reserved.Proof != 0 {
			detail += fmt.Sprintf(" proof=%d", reserved.Proof)
		}
		detail += fmt.Sprintf(" limit=%d", reserved.Limit)
	}
	return detail
}
