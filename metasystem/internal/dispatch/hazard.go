package dispatch

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// HazardClass is the closed configuration class carried by every delegated
// task. The JSON field is named destructiveReach so it composes directly with
// the consequence trigger schema that judges destructive reach.
type HazardClass string

const (
	HazardMechanical       HazardClass = "MECHANICAL"
	HazardDesignBearing    HazardClass = "DESIGN-BEARING"
	HazardDestructiveReach HazardClass = "DESTRUCTIVE-REACH"
)

// ConfigurationObligations is the effective minimum carried by a job. The
// hazard class supplies its floor, and a goal tier can raise its critique duty.
type ConfigurationObligations struct {
	BuilderEffortTier                  string `json:"builderEffortTier"`
	BuilderReasoningEffort             string `json:"builderReasoningEffort"`
	IndependentCritiqueRequired        bool   `json:"independentCritiqueRequired"`
	IndependentCritiqueEffortTier      string `json:"independentCritiqueEffortTier"`
	IndependentCritiqueReasoningEffort string `json:"independentCritiqueReasoningEffort"`
	LiveProofRequired                  bool   `json:"liveProofRequired"`
}

var requiredConfigurationByHazard = map[HazardClass]ConfigurationObligations{
	HazardMechanical: {
		BuilderEffortTier: "ordinary", BuilderReasoningEffort: "medium",
		IndependentCritiqueRequired: false, IndependentCritiqueEffortTier: "none",
		IndependentCritiqueReasoningEffort: "none", LiveProofRequired: false,
	},
	HazardDesignBearing: {
		BuilderEffortTier: "maximal", BuilderReasoningEffort: "xhigh",
		IndependentCritiqueRequired: true, IndependentCritiqueEffortTier: "maximal",
		IndependentCritiqueReasoningEffort: "xhigh", LiveProofRequired: false,
	},
	HazardDestructiveReach: {
		BuilderEffortTier: "maximal", BuilderReasoningEffort: "xhigh",
		IndependentCritiqueRequired: true, IndependentCritiqueEffortTier: "maximal",
		IndependentCritiqueReasoningEffort: "xhigh", LiveProofRequired: true,
	},
}

var independentCritiqueRequiredByTier = map[uint8]bool{1: false, 2: true, 3: true}

// EffectiveObligations applies a goal's critique duty to the class floor.
// Builder effort and live proof remain properties of the hazard class.
func EffectiveObligations(class HazardClass, tier uint8) (ConfigurationObligations, error) {
	configuration, err := MinimumHazardConfiguration(class)
	if err != nil {
		return ConfigurationObligations{}, err
	}
	critiqueRequired, known := independentCritiqueRequiredByTier[tier]
	if tier != 0 && !known {
		return ConfigurationObligations{}, fmt.Errorf("goal tier must be 1, 2, or 3")
	}
	if critiqueRequired && !configuration.IndependentCritiqueRequired {
		configuration.IndependentCritiqueRequired = true
		configuration.IndependentCritiqueEffortTier = "maximal"
		configuration.IndependentCritiqueReasoningEffort = "xhigh"
	}
	return configuration, nil
}

// ResolveHazardConfiguration reads the role-packet table and refuses any
// missing, extra, or weakened class or tier rule. The table remains the
// recorded schema, while this admission boundary fixes the lawful obligations.
func ResolveHazardConfiguration(root string, class HazardClass, tier uint8) (ConfigurationObligations, error) {
	expected, err := EffectiveObligations(class, tier)
	if err != nil {
		return ConfigurationObligations{}, err
	}
	_, table, err := readRolePacketTable(root)
	if err != nil {
		return ConfigurationObligations{}, err
	}
	if len(table.DestructiveReach) != len(requiredConfigurationByHazard) {
		return ConfigurationObligations{}, fmt.Errorf("role packet table must define exactly the three destructiveReach classes")
	}
	for requiredClass, required := range requiredConfigurationByHazard {
		configured, present := table.DestructiveReach[requiredClass]
		if !present || !reflect.DeepEqual(configured, required) {
			return ConfigurationObligations{}, fmt.Errorf("role packet table destructiveReach class %s does not match its required configuration", requiredClass)
		}
	}
	requiredTierRule := make(map[string]bool, len(independentCritiqueRequiredByTier))
	for requiredTier, required := range independentCritiqueRequiredByTier {
		requiredTierRule[strconv.FormatUint(uint64(requiredTier), 10)] = required
	}
	if !reflect.DeepEqual(table.IndependentCritiqueByTier, requiredTierRule) {
		return ConfigurationObligations{}, fmt.Errorf("role packet table independentCritiqueByTier does not match its required rule")
	}
	return expected, nil
}

// MinimumHazardConfiguration is the engine-owned admission mapping used when
// a reservation is published before its composed packet replaces the husk.
func MinimumHazardConfiguration(class HazardClass) (ConfigurationObligations, error) {
	expected, ok := requiredConfigurationByHazard[class]
	if !ok {
		return ConfigurationObligations{}, fmt.Errorf("destructiveReach must be MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	}
	return expected, nil
}

// ValidateRuntimeHazardConfiguration refuses a class when the selected
// adapter has no executable channel for the class's minimum effort. Recording
// a duty that the launcher cannot enforce would not satisfy admission.
func ValidateRuntimeHazardConfiguration(root, runtime, model string, class HazardClass) error {
	configuration, err := MinimumHazardConfiguration(class)
	if err != nil {
		return err
	}
	if configuration.BuilderReasoningEffort != "xhigh" {
		return nil
	}
	proven, err := runtimeProvesMaximalExecution(root, runtime, model)
	if err != nil {
		return err
	}
	if !proven {
		return fmt.Errorf("runtime %s has no executable maximal-effort mapping for destructiveReach %s", runtime, class)
	}
	return nil
}

func runtimeProvesMaximalExecution(root, runtime, model string) (bool, error) {
	if runtime == "codex" || runtime == "fake" {
		return true, nil
	}
	key := "runtime." + runtime + ".maximal-models"
	value, _, err := config.Get(config.GetParams{
		Key: key, ConfPath: filepath.Join(root, "metasystem.conf"), Default: "", DefaultSet: true,
	})
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", key, err)
	}
	if value == "" {
		return false, nil
	}
	seen := map[string]bool{}
	matched := false
	for _, configuredModel := range strings.Split(value, ",") {
		configuredModel = strings.TrimSpace(configuredModel)
		if configuredModel == "" {
			return false, fmt.Errorf("%s must contain only non-empty comma-separated model names", key)
		}
		if seen[configuredModel] {
			return false, fmt.Errorf("%s contains duplicate model %s", key, configuredModel)
		}
		seen[configuredModel] = true
		if configuredModel == model {
			matched = true
		}
	}
	return matched, nil
}

func configurationObligationsMatchObject(expected ConfigurationObligations, object map[string]any) bool {
	return len(object) == 6 && asString(object["builderEffortTier"]) == expected.BuilderEffortTier &&
		asString(object["builderReasoningEffort"]) == expected.BuilderReasoningEffort &&
		object["independentCritiqueRequired"] == expected.IndependentCritiqueRequired &&
		asString(object["independentCritiqueEffortTier"]) == expected.IndependentCritiqueEffortTier &&
		asString(object["independentCritiqueReasoningEffort"]) == expected.IndependentCritiqueReasoningEffort &&
		object["liveProofRequired"] == expected.LiveProofRequired
}

const (
	hazardCritiqueClosureRefusal    = "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE"
	hazardLiveProofClosureRefusal   = "REFUSED-R22-M1-RULING-O-LIVE-PROOF"
	hazardRecordClosureRefusal      = "REFUSED-R22-M1-RULING-O-HAZARD-RECORD"
	hazardEvidenceProvenanceRefusal = "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE"
	hazardCritiqueStaleRefusal      = "REFUSED-R22-M1-RULING-O-CRITIQUE-STALE"
	hazardLiveProofStaleRefusal     = "REFUSED-R22-M1-RULING-O-LIVE-PROOF-STALE"
)

type hazardFinalWorkState struct {
	job     string
	round   int64
	endedAt time.Time
}

func validateHazardCompletion(repoRoot, jobsDir, root string, members []chainMember) error {
	var governed *ConfigurationObligations
	var rootRecord map[string]any
	for _, member := range members {
		if asString(member.record["jobId"]) == root {
			rootRecord = member.record
			break
		}
	}
	if rootRecord == nil {
		return nil
	}
	rootTier := uint8(0)
	if rawTier, present := rootRecord["goalTier"]; present && rawTier != nil {
		tier, ok := numInt(rawTier)
		if !ok || tier < 0 || tier > 255 {
			return &OpError{Code: 9, Reason: hazardRecordClosureRefusal, Message: fmt.Sprintf("chain %s has an invalid goalTier record", root)}
		}
		rootTier = uint8(tier)
	}
	criticOnly := len(members) > 0
	memberIDs := make(map[string]bool, len(members))
	memberSessions := make(map[string]bool, len(members))
	for _, member := range members {
		job := asString(member.record["jobId"])
		memberIDs[job] = true
		if session := asString(member.record["sessionId"]); session != "" {
			memberSessions[session] = true
		}
		if !exactCriticRole(member.record["role"]) {
			criticOnly = false
		}
		class := HazardClass(asString(member.record["destructiveReach"]))
		if class == "" {
			continue
		}
		configuration, err := EffectiveObligations(class, rootTier)
		if err != nil {
			return &OpError{Code: 9, Reason: hazardRecordClosureRefusal, Message: fmt.Sprintf("chain %s has an invalid destructiveReach or goalTier record", root)}
		}
		if governed == nil || (!governed.IndependentCritiqueRequired && configuration.IndependentCritiqueRequired) ||
			(!governed.LiveProofRequired && configuration.LiveProofRequired) {
			copy := configuration
			governed = &copy
		}
	}
	if governed == nil {
		return nil
	}
	// Review-only chains close through their own register, terminal, and
	// durability checks. Applying the builder's independent-review or live
	// proof duties to a chain made exclusively of exact critic roles would
	// require an infinite chain of critics. Any malformed role or any work
	// member keeps the normal builder duties below.
	if criticOnly {
		return nil
	}
	if !governed.IndependentCritiqueRequired && !governed.LiveProofRequired {
		return nil
	}
	finalState, detail := finalHazardWorkState(members)
	if detail != "" {
		return &OpError{Code: 9, Reason: hazardRecordClosureRefusal, Message: detail}
	}
	if governed.IndependentCritiqueRequired {
		if err := validateIndependentCritiqueReference(repoRoot, jobsDir, rootRecord, memberIDs, memberSessions, *governed, finalState); err != nil {
			return err
		}
	}
	if governed.LiveProofRequired {
		if err := validateLiveProofReference(jobsDir, rootRecord, finalState); err != nil {
			return err
		}
	}
	return nil
}

func exactCriticRole(value any) bool {
	role, ok := value.(string)
	if !ok {
		return false
	}
	switch role {
	case "code-critic", "design-critic", "warden":
		return true
	default:
		return false
	}
}

func finalHazardWorkState(members []chainMember) (hazardFinalWorkState, string) {
	candidates := make([]chainMember, 0, len(members))
	for _, member := range members {
		switch asString(member.record["role"]) {
		case "code-critic", "design-critic", "warden":
			continue
		default:
			candidates = append(candidates, member)
		}
	}
	if len(candidates) == 0 {
		candidates = members
	}
	var final hazardFinalWorkState
	found := false
	tied := false
	for _, member := range candidates {
		round, ok := numInt(member.record["round"])
		if !ok || round < 1 {
			return hazardFinalWorkState{}, "hazard-governed chain has a work record without a positive round"
		}
		if found && round < final.round {
			continue
		}
		if found && round == final.round {
			tied = true
			continue
		}
		endedAt, err := parseRecordTime(asString(member.record["endedAt"]))
		if err != nil {
			return hazardFinalWorkState{}, fmt.Sprintf("hazard-governed work record %q has no valid terminal end time", asString(member.record["jobId"]))
		}
		final = hazardFinalWorkState{job: asString(member.record["jobId"]), round: round, endedAt: endedAt}
		found = true
		tied = false
	}
	if tied {
		return hazardFinalWorkState{}, fmt.Sprintf("hazard-governed chain has more than one terminal work record at round %d", final.round)
	}
	if !found || !validJobID.MatchString(final.job) {
		return hazardFinalWorkState{}, "hazard-governed chain has no identifiable terminal work state"
	}
	return final, ""
}

func validateIndependentCritiqueReference(repoRoot, jobsDir string, rootRecord map[string]any, memberIDs, memberSessions map[string]bool, required ConfigurationObligations, finalState hazardFinalWorkState) error {
	ref := asString(rootRecord["independentCritiqueJobRef"])
	if !validJobID.MatchString(ref) || memberIDs[ref] {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, "chain completion requires a distinct independent-critique job reference")
	}
	critic, err := readObject(filepath.Join(jobsDir, ref+".json"))
	if err != nil || asString(critic["jobId"]) != ref || asString(critic["status"]) != "completed" {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique reference %q does not point at a completed job record", ref))
	}
	if detail := validateHazardEvidenceAdmissionProvenance(critic, ref); detail != "" {
		return hazardClosureRefusal(hazardEvidenceProvenanceRefusal, detail)
	}
	role := asString(critic["role"])
	if role != "code-critic" && role != "design-critic" && role != "warden" {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique reference %q is not a critic job", ref))
	}
	if parent, present := critic["parentJob"]; (present && parent != nil) || asString(critic["dispatchMode"]) != string(DispatchModeFresh) || asString(critic["resumedSessionId"]) != "" {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique job %q is not a fresh-context chain", ref))
	}
	session := asString(critic["sessionId"])
	if session == "" || memberSessions[session] {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique job %q does not carry a distinct fresh session", ref))
	}
	configuration, ok := critic["configurationObligations"].(map[string]any)
	if !ok || asString(configuration["builderEffortTier"]) != required.IndependentCritiqueEffortTier ||
		asString(configuration["builderReasoningEffort"]) != required.IndependentCritiqueReasoningEffort ||
		asString(critic["reasoningEffort"]) != required.IndependentCritiqueReasoningEffort {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf(
			"independent-critique job %q does not prove the required maximum critic effort: its builder rows are %s/%s and its reasoning effort %s, the critique requires %s/%s; dispatch the critic at a class whose builder rows are the critique's (DESIGN-BEARING)",
			ref, asString(configuration["builderEffortTier"]), asString(configuration["builderReasoningEffort"]), asString(critic["reasoningEffort"]),
			required.IndependentCritiqueEffortTier, required.IndependentCritiqueReasoningEffort))
	}
	proven, proofErr := runtimeProvesMaximalExecution(repoRoot, asString(critic["runtime"]), asString(critic["requestedModel"]))
	if proofErr != nil || !proven {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("independent-critique job %q does not prove the required maximum critic effort", ref))
	}
	criticChain, err := chainMembers(jobsDir, ref)
	if err != nil {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure for stamped root %q is unreadable: %v", ref, err))
	}
	criticRecords := make([]map[string]any, 0, len(criticChain))
	for _, member := range criticChain {
		criticRecords = append(criticRecords, member.record)
	}
	closure, closurePresent, closureErr := readsubject.ReadClosedClosure(filepath.Dir(jobsDir), critic, criticRecords)
	if closureErr != nil {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure for stamped root %q is invalid: %v", ref, closureErr))
	}
	if closurePresent {
		if closure.CriticRoot != ref {
			return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique stamp %q does not equal closure root %q at round %d", ref, closure.CriticRoot, closure.Round))
		}
		if err := validateClosureProvingCritic(repoRoot, jobsDir, ref, role, closure.Round, criticChain, memberIDs, memberSessions, required); err != nil {
			return err
		}
		terminalSubject, _, subjectPresent, subjectErr := liveReadSubject(loadCritiqueState(repoRoot), finalState.job)
		if subjectErr != nil || !subjectPresent {
			detail := "no review.json and diff.patch"
			if subjectErr != nil {
				detail = subjectErr.Error()
			}
			return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure root %q round %d has no readable live subject for terminal work round %q: %s", closure.CriticRoot, closure.Round, finalState.job, detail))
		}
		if closure.Subject.Kind != readsubject.SubjectLive || !closure.Subject.Equal(terminalSubject) {
			return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure root %q round %d subject %s is not the live subject %s of terminal work round %q", closure.CriticRoot, closure.Round, closure.Subject.Digest(), terminalSubject.Digest(), finalState.job))
		}
		return nil
	}
	if asString(critic["reviews"]) != finalState.job {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique job %q reviews %q instead of final work round %q", ref, asString(critic["reviews"]), finalState.job))
	}
	endedAt, err := parseRecordTime(asString(critic["endedAt"]))
	if err != nil || endedAt.Before(finalState.endedAt) {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique job %q did not end at or after final work round %q", ref, finalState.job))
	}
	return nil
}

func validateClosureProvingCritic(repoRoot, jobsDir, criticRoot, criticRole string, closureRound int64, criticChain []chainMember, implementationMemberIDs, implementationSessions map[string]bool, required ConfigurationObligations) error {
	var proving *chainMember
	for index := range criticChain {
		round, ok := numInt(criticChain[index].record["round"])
		if !ok || round != closureRound {
			continue
		}
		if proving != nil {
			return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure root %q has ambiguous proving-job selection at round %d", criticRoot, closureRound))
		}
		proving = &criticChain[index]
	}
	if proving == nil {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("independent-critique closure root %q has no proving job at round %d", criticRoot, closureRound))
	}
	provingJob := asString(proving.record["jobId"])
	prefix := fmt.Sprintf("independent-critique closure root %q proving job %q round %d", criticRoot, provingJob, closureRound)
	if proving.path != filepath.Join(jobsDir, provingJob+".json") {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("%s has record path %q instead of jobs/%s.json", prefix, proving.path, provingJob))
	}
	if !validJobID.MatchString(provingJob) || implementationMemberIDs[provingJob] {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("%s is not distinct from the implementation chain", prefix))
	}
	if asString(proving.record["status"]) != "completed" {
		return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("%s has status %q instead of completed", prefix, asString(proving.record["status"])))
	}
	if asString(proving.record["role"]) != criticRole {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has role %q instead of root role %q", prefix, asString(proving.record["role"]), criticRole))
	}
	if provingJob == criticRoot {
		return nil
	}

	byJob := make(map[string]*chainMember, len(criticChain))
	for index := range criticChain {
		job := asString(criticChain[index].record["jobId"])
		if _, present := byJob[job]; present {
			return hazardClosureRefusal(hazardCritiqueStaleRefusal, fmt.Sprintf("%s has ambiguous ancestry at member %q", prefix, job))
		}
		byJob[job] = &criticChain[index]
	}
	ancestry := []*chainMember{proving}
	seen := map[string]bool{provingJob: true}
	for child := proving; asString(child.record["jobId"]) != criticRoot; {
		childJob := asString(child.record["jobId"])
		parentJob, ok := child.record["parentJob"].(string)
		if !ok || !validJobID.MatchString(parentJob) {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q with malformed parentJob", prefix, childJob))
		}
		parent, present := byJob[parentJob]
		if !present {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose parentJob %q is missing", prefix, childJob, parentJob))
		}
		if seen[parentJob] {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has an ancestry cycle through %q", prefix, parentJob))
		}
		seen[parentJob] = true
		ancestry = append(ancestry, parent)
		child = parent
	}

	for index := len(ancestry) - 2; index >= 0; index-- {
		child := ancestry[index]
		parent := ancestry[index+1]
		childJob := asString(child.record["jobId"])
		parentJob := asString(parent.record["jobId"])
		childRound, childRoundOK := numInt(child.record["round"])
		parentRound, parentRoundOK := numInt(parent.record["round"])
		if asString(child.record["role"]) != criticRole {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose role %q differs from root role %q", prefix, childJob, asString(child.record["role"]), criticRole))
		}
		childSession := asString(child.record["sessionId"])
		if childSession == "" || implementationSessions[childSession] {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q with sessionId %q that is empty or belongs to the implementation chain", prefix, childJob, childSession))
		}
		if asString(child.record["parentJob"]) != parentJob || !childRoundOK || !parentRoundOK || childRound != parentRound+1 {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose parentJob/round does not immediately follow %q round %d", prefix, childJob, parentJob, parentRound))
		}
		if asString(child.record["dispatchMode"]) != string(DispatchModeFollowUp) {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q with dispatchMode %q instead of follow-up", prefix, childJob, asString(child.record["dispatchMode"])))
		}
		if asString(child.record["runtime"]) != asString(parent.record["runtime"]) {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose runtime %q differs from parent %q runtime %q", prefix, childJob, asString(child.record["runtime"]), parentJob, asString(parent.record["runtime"])))
		}
		parentSession := asString(parent.record["sessionId"])
		resumedSession := asString(child.record["resumedSessionId"])
		if resumedSession == "" || resumedSession != parentSession {
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose resumedSessionId %q does not equal parent %q sessionId %q", prefix, childJob, resumedSession, parentJob, parentSession))
		}
		switch asString(child.record["resumeMode"]) {
		case "resumed":
			if childSession != resumedSession {
				return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose sessionId %q does not equal resumedSessionId %q for resumeMode resumed", prefix, childJob, childSession, resumedSession))
			}
		case "fresh-context":
			if childSession == resumedSession {
				return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q whose sessionId equals resumedSessionId for resumeMode fresh-context", prefix, childJob))
			}
		default:
			return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has ancestor %q with unsupported resumeMode %q", prefix, childJob, asString(child.record["resumeMode"])))
		}
	}

	if detail := validateHazardEvidenceAdmissionProvenance(proving.record, provingJob); detail != "" {
		return hazardClosureRefusal(hazardEvidenceProvenanceRefusal, fmt.Sprintf("%s: %s", prefix, detail))
	}
	configuration, ok := proving.record["configurationObligations"].(map[string]any)
	if !ok || asString(configuration["builderEffortTier"]) != required.IndependentCritiqueEffortTier {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has configurationObligations.builderEffortTier %q instead of %q", prefix, asString(configuration["builderEffortTier"]), required.IndependentCritiqueEffortTier))
	}
	if asString(configuration["builderReasoningEffort"]) != required.IndependentCritiqueReasoningEffort {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has configurationObligations.builderReasoningEffort %q instead of %q", prefix, asString(configuration["builderReasoningEffort"]), required.IndependentCritiqueReasoningEffort))
	}
	if asString(proving.record["reasoningEffort"]) != required.IndependentCritiqueReasoningEffort {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has reasoningEffort %q instead of %q", prefix, asString(proving.record["reasoningEffort"]), required.IndependentCritiqueReasoningEffort))
	}
	runtimeName := asString(proving.record["runtime"])
	requestedModel := asString(proving.record["requestedModel"])
	if runtimeName == "" || requestedModel == "" {
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has empty runtime/requestedModel %q/%q", prefix, runtimeName, requestedModel))
	}
	proven, proofErr := runtimeProvesMaximalExecution(repoRoot, runtimeName, requestedModel)
	if proofErr != nil || !proven {
		detail := "mapping does not admit this pair"
		if proofErr != nil {
			detail = proofErr.Error()
		}
		return hazardClosureRefusal(hazardCritiqueClosureRefusal, fmt.Sprintf("%s has runtime/requestedModel %q/%q that does not prove maximal execution: %s", prefix, runtimeName, requestedModel, detail))
	}
	return nil
}

func validateLiveProofReference(jobsDir string, rootRecord map[string]any, finalState hazardFinalWorkState) error {
	ref := asString(rootRecord["liveProofEvidenceRef"])
	if !validJobID.MatchString(ref) {
		return hazardClosureRefusal(hazardLiveProofClosureRefusal, "chain completion requires a live-proof evidence reference")
	}
	proof, err := readObject(filepath.Join(jobsDir, ref+".json"))
	if err != nil || asString(proof["jobId"]) != ref || asString(proof["status"]) != "completed" {
		return hazardClosureRefusal(hazardLiveProofClosureRefusal, fmt.Sprintf("live-proof evidence reference %q does not point at a completed job record", ref))
	}
	if detail := validateHazardEvidenceAdmissionProvenance(proof, ref); detail != "" {
		return hazardClosureRefusal(hazardEvidenceProvenanceRefusal, detail)
	}
	if asString(proof["role"]) != "verifier" {
		return hazardClosureRefusal(hazardLiveProofClosureRefusal, fmt.Sprintf("live-proof evidence job %q is not a verifier linked to this chain", ref))
	}
	if asString(proof["reviews"]) != finalState.job {
		return hazardClosureRefusal(hazardLiveProofStaleRefusal, fmt.Sprintf("live-proof evidence job %q reviews %q instead of final work round %q", ref, asString(proof["reviews"]), finalState.job))
	}
	endedAt, err := parseRecordTime(asString(proof["endedAt"]))
	if err != nil || endedAt.Before(finalState.endedAt) {
		return hazardClosureRefusal(hazardLiveProofStaleRefusal, fmt.Sprintf("live-proof evidence job %q did not end at or after final work round %q", ref, finalState.job))
	}
	return nil
}

func hazardClosureRefusal(reason, detail string) error {
	return &OpError{Code: 9, Reason: reason, Message: detail}
}

func validateHazardEvidenceAdmissionProvenance(record map[string]any, ref string) string {
	operationID := asString(record["operationId"])
	round, roundOK := numInt(record["round"])
	version, versionOK := numInt(record["fingerprintVersion"])
	fingerprint, fingerprintOK := record["fingerprint"].(string)
	if asString(record["jobId"]) != ref || !validJobID.MatchString(operationID) ||
		!roundOK || round < 1 || asString(record["proofLevel"]) != "proven" || !versionOK || version != LaunchFingerprintVersion ||
		!fingerprintOK || !incarnationRe.MatchString(fingerprint) {
		return fmt.Sprintf("linked evidence job %q has no well-formed admission identity and fingerprint", ref)
	}
	request, ok := hazardEvidenceLaunchRequest(record)
	if !ok {
		return fmt.Sprintf("linked evidence job %q has incomplete or malformed admission identity fields", ref)
	}
	expected, err := LaunchFingerprintV2(request)
	if err != nil || expected.Digest != fingerprint {
		return fmt.Sprintf("linked evidence job %q has a fingerprint inconsistent with its admission identity", ref)
	}
	parent, parentPresent := record["parentJob"]
	if !parentPresent || (parent != nil && !validJobID.MatchString(asString(parent))) {
		return fmt.Sprintf("linked evidence job %q has malformed parent identity", ref)
	}
	instanceTag := asString(record["instanceTag"])
	prefix := "metasystem-job-" + ref + "-"
	if !strings.HasPrefix(instanceTag, prefix) || !launchSuffixRE.MatchString(strings.TrimPrefix(instanceTag, prefix)) {
		return fmt.Sprintf("linked evidence job %q has a malformed admitted instance identity", ref)
	}
	capability, ok := record["launchCapability"].(map[string]any)
	expectedVerb := "dispatch"
	if request.DispatchMode == DispatchModeFollowUp {
		if asString(record["resumeMode"]) == "fresh-context" {
			expectedVerb = "dispatch"
		} else {
			expectedVerb = "follow-up"
		}
	}
	mintedAt, mintedErr := parseRecordTime(asString(capability["mintedAt"]))
	consumedAt, consumedErr := parseRecordTime(asString(capability["consumedAt"]))
	supervisor, supervisorOK := capability["supervisor"].(map[string]any)
	supervisorRef, supervisorIdentityOK := identityRefFromObject(supervisor)
	if !ok || len(capability) != 9 || !incarnationRe.MatchString(asString(capability["digest"])) ||
		asString(capability["jobId"]) != ref || asString(capability["operationId"]) != operationID ||
		asString(capability["instanceTag"]) != instanceTag || asString(capability["adapterVerb"]) != expectedVerb ||
		asString(capability["status"]) != "consumed" || mintedErr != nil || consumedErr != nil || consumedAt.Before(mintedAt) ||
		!supervisorOK || !supervisorIdentityOK || !supervisorRef.NativeExact() {
		return fmt.Sprintf("linked evidence job %q has no internally consistent consumed launch admission", ref)
	}
	return ""
}

func hazardEvidenceLaunchRequest(record map[string]any) (CanonicalLaunchRequest, bool) {
	stringField := func(key string, allowEmpty bool) (string, bool) {
		value, ok := record[key].(string)
		return value, ok && (allowEmpty || value != "")
	}
	sessionKey, sessionOK := stringField("sessionKey", false)
	dispatchMode, dispatchOK := stringField("dispatchMode", false)
	resumedSessionID, resumedOK := stringField("resumedSessionId", true)
	runtimeName, runtimeOK := stringField("runtime", false)
	model, modelOK := stringField("canonicalModelKey", false)
	role, roleOK := stringField("role", false)
	launchMode, launchOK := stringField("launchMode", false)
	permissionDigest, permissionOK := stringField("permissionEnvelopeDigest", false)
	inputHash, inputOK := stringField("inputHash", false)
	capMinutes, capOK := numInt(record["capMin"])
	capRequest, capRequestOK := record["capRequest"].(map[string]any)
	requestedCap, requestedCapOK := numInt(capRequest["minutes"])
	if !sessionOK || !dispatchOK || !resumedOK || !runtimeOK || !modelOK || !roleOK || !launchOK ||
		!permissionOK || !inputOK || !capOK || capMinutes < 1 || !capRequestOK || len(capRequest) != 1 ||
		!requestedCapOK || requestedCap != capMinutes {
		return CanonicalLaunchRequest{}, false
	}
	rootsValue, rootsOK := record["productRoots"].([]any)
	if !rootsOK {
		return CanonicalLaunchRequest{}, false
	}
	roots := make([]string, len(rootsValue))
	for index, value := range rootsValue {
		root, ok := value.(string)
		if !ok {
			return CanonicalLaunchRequest{}, false
		}
		roots[index] = root
	}
	goalID := ""
	goalValue, goalPresent := record["goalId"]
	if !goalPresent {
		return CanonicalLaunchRequest{}, false
	}
	if goalValue != nil {
		var ok bool
		goalID, ok = goalValue.(string)
		if !ok || goalID == "" {
			return CanonicalLaunchRequest{}, false
		}
	}
	goalRevision := uint64(0)
	revisionValue, revisionPresent := record["goalRevision"]
	if !revisionPresent {
		return CanonicalLaunchRequest{}, false
	}
	if revisionValue != nil {
		parsed, ok := JobRecordOf(record).GoalRevision()
		if !ok || parsed == 0 {
			return CanonicalLaunchRequest{}, false
		}
		goalRevision = parsed
	}
	return CanonicalLaunchRequest{
		SessionKey: sessionKey, DispatchMode: DispatchMode(dispatchMode), ResumedSessionID: resumedSessionID,
		Runtime: runtimeName, CanonicalModelKey: model, Role: role, LaunchMode: LaunchMode(launchMode),
		PermissionEnvelopeDigest: permissionDigest, ProductRoots: roots, CapMinutes: capMinutes,
		InputHash: inputHash, GoalID: goalID, GoalRevision: goalRevision,
		DestructiveReach: HazardClass(asString(record["destructiveReach"])),
	}, true
}
