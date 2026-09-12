package proofrun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	TestResultSchemaVersion              = 1
	candidateEngineDigestIdentityVersion = 1
	CandidateEngineIdentitySchemaVersion = 2
)

type NativeTestIdentity struct {
	Report    string `json:"report,omitempty"`
	Classname string `json:"classname,omitempty"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

type GroupResult struct {
	ID                    string               `json:"id"`
	Kind                  string               `json:"kind"`
	Obligations           []string             `json:"obligations"`
	InputDigest           string               `json:"inputDigest"`
	InputManifest         []string             `json:"inputManifest"`
	ExecutionIdentity     string               `json:"executionIdentity"`
	Argv                  []string             `json:"argv"`
	CWD                   string               `json:"cwd"`
	EnvironmentDigest     string               `json:"environmentDigest"`
	ToolIdentities        map[string]string    `json:"toolIdentities"`
	ExecutableDigests     map[string]string    `json:"executableDigests,omitempty"`
	Status                string               `json:"status"`
	ProgressRule          string               `json:"progressRule"`
	CPUSeconds            float64              `json:"cpuSeconds"`
	LongestSilentSeconds  int64                `json:"longestSilentSeconds"`
	LongestZeroCPUSeconds int64                `json:"longestZeroCpuSeconds"`
	ProgressReadings      []string             `json:"progressReadings,omitempty"`
	NativeLaunched        bool                 `json:"nativeLaunched"`
	OtherLaunches         int                  `json:"otherLaunches"`
	NativeExitStatus      *int                 `json:"nativeExitStatus"`
	Signal                *string              `json:"signal"`
	Expected              []NativeTestIdentity `json:"expected"`
	Observed              []NativeTestIdentity `json:"observed"`
	Missing               []NativeTestIdentity `json:"missing"`
	Blocked               []NativeTestIdentity `json:"blocked"`
	Unexpected            []NativeTestIdentity `json:"unexpected"`
	CollectionComplete    bool                 `json:"collectionComplete"`
	LogPath               string               `json:"logPath"`
	LogDigest             string               `json:"logDigest"`
	ReportDigests         map[string]string    `json:"reportDigests"`
	StartedAt             string               `json:"startedAt,omitempty"`
	EndedAt               string               `json:"endedAt,omitempty"`
	DurationMS            int64                `json:"durationMs"`
	ReuseAttempt          string               `json:"reuseAttempt,omitempty"`
	NotRunReason          string               `json:"notRunReason,omitempty"`
	BlockingGroups        []string             `json:"blockingGroups,omitempty"`
}

type LaunchCounts struct {
	Test           int  `json:"test"`
	Build          int  `json:"build"`
	Other          int  `json:"other"`
	CountsComplete bool `json:"countsComplete"`
	ReusedTest     int  `json:"reusedTest"`
	ReusedBuild    int  `json:"reusedBuild"`
	ReusedOther    int  `json:"reusedOther"`
}

func ValidateTestResult(result TestResult) error {
	candidateEngineMissing := result.CandidateEngineDigest == ""
	candidateEngineInvalid := !candidateEngineMissing && !validResultDigest(result.CandidateEngineDigest)
	candidateEngineIdentityInvalid := result.CandidateEngineIdentityVersion < 0 ||
		result.CandidateEngineIdentityVersion > CandidateEngineIdentitySchemaVersion ||
		result.CandidateEngineIdentityVersion == candidateEngineDigestIdentityVersion && candidateEngineMissing ||
		result.CandidateEngineIdentityVersion == CandidateEngineIdentitySchemaVersion &&
			(candidateEngineMissing || !validTreeDigest(result.CandidateEngineBuildIdentity))
	if result.SchemaVersion != TestResultSchemaVersion || result.Purpose == "" || result.RequestedMode == "" ||
		result.RequiredMode == "" || result.ExecutedMode == "" || result.ProjectRoot == "" || result.BaseCommit == "" ||
		!validTreeDigest(result.CandidateTree) || !validResultDigest(result.ContractDigest) ||
		!validResultDigest(result.BaseContractDigest) || !validResultDigest(result.PolicyEngineDigest) ||
		candidateEngineInvalid || candidateEngineIdentityInvalid ||
		!validResultDigest(result.BehaviorPolicyDigest) || !validResultDigest(result.PlanDigest) ||
		!result.LaunchCounts.CountsComplete || result.Cost.DeclaredTargetMS <= 0 {
		return fmt.Errorf("test result has incomplete identity or launch accounting")
	}
	selected := map[string]bool{}
	for _, id := range result.SelectedGroups {
		if id == "" || selected[id] {
			return fmt.Errorf("test result selected groups are invalid or duplicated")
		}
		selected[id] = true
	}
	seen := map[string]bool{}
	for _, group := range result.Groups {
		if group.ID == "" || group.Kind == "" || len(group.InputManifest) == 0 || seen[group.ID] || !selected[group.ID] {
			return fmt.Errorf("test result group inventory is invalid or duplicated")
		}
		seen[group.ID] = true
		switch group.Status {
		case "passed":
			if !group.NativeLaunched || !group.CollectionComplete || group.NativeExitStatus == nil || *group.NativeExitStatus != 0 || group.ReuseAttempt != "" {
				return fmt.Errorf("passed group %s has incomplete native evidence", group.ID)
			}
		case "reused":
			if !group.CollectionComplete || group.ReuseAttempt == "" {
				return fmt.Errorf("reused group %s has no successful source", group.ID)
			}
		case "failed", "invalid", "unavailable", "cancelled", "runaway", "dead":
		case "not-run":
			if group.NativeLaunched || group.StartedAt != "" || group.EndedAt != "" || group.NativeExitStatus != nil || group.NotRunReason == "" {
				return fmt.Errorf("not-run group %s invents execution facts or lacks a reason", group.ID)
			}
		default:
			return fmt.Errorf("test result group %s has invalid status %q", group.ID, group.Status)
		}
	}
	if len(seen) != len(selected) {
		return fmt.Errorf("test result omits selected groups")
	}
	recomputed := result
	recomputed.RecomputeDelivery()
	if !reflect.DeepEqual(result.Delivery, recomputed.Delivery) {
		return fmt.Errorf("test result delivery judgment does not match its group evidence")
	}
	return nil
}

func validTreeDigest(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func validResultDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type DeliveryJudgment struct {
	Sufficient           bool     `json:"sufficient"`
	MissingGroups        []string `json:"missingGroups"`
	FailingGroups        []string `json:"failingGroups"`
	UncoveredObligations []string `json:"uncoveredObligations"`
	Discrepancies        []string `json:"discrepancies"`
}

type TestCost struct {
	DeclaredTargetMS      int64 `json:"declaredTargetMs"`
	ActualDurationMS      int64 `json:"actualDurationMs"`
	PreparationDurationMS int64 `json:"preparationDurationMs"`
	ExecutionDurationMS   int64 `json:"executionDurationMs"`
	PublicationDurationMS int64 `json:"publicationDurationMs"`
	ChildDurationMS       int64 `json:"childDurationMs"`
	ReusedLaunches        int   `json:"reusedLaunches"`
}

type TestResult struct {
	SchemaVersion                  int                       `json:"schemaVersion"`
	AttemptID                      string                    `json:"attemptId"`
	Purpose                        testpolicy.Purpose        `json:"purpose"`
	RequestedMode                  testpolicy.Mode           `json:"requestedMode"`
	RequiredMode                   testpolicy.Mode           `json:"requiredMode"`
	ExecutedMode                   testpolicy.Mode           `json:"executedMode"`
	ProjectRoot                    string                    `json:"projectRoot"`
	InstallationPrefix             string                    `json:"installationPrefix"`
	BaseCommit                     string                    `json:"baseCommit"`
	CandidateTree                  string                    `json:"candidateTree"`
	PolicyBaseCommit               string                    `json:"policyBaseCommit"`
	ContractDigest                 string                    `json:"contractDigest"`
	BaseContractDigest             string                    `json:"baseContractDigest"`
	PolicyEngineDigest             string                    `json:"policyEngineDigest"`
	JudgeKey                       string                    `json:"judgeKey,omitempty"`
	CandidateEngineIdentityVersion int                       `json:"candidateEngineIdentityVersion,omitempty"`
	CandidateEngineDigest          string                    `json:"candidateEngineDigest"`
	CandidateEngineBuildIdentity   string                    `json:"candidateEngineBuildIdentity,omitempty"`
	BehaviorPolicyDigest           string                    `json:"behaviorPolicyDigest"`
	PlanDigest                     string                    `json:"planDigest"`
	Risk                           testpolicy.RiskAssessment `json:"risk"`
	RequiredGroups                 []string                  `json:"requiredGroups"`
	SelectedGroups                 []string                  `json:"selectedGroups"`
	Omissions                      []testpolicy.Omission     `json:"omissions"`
	Uncertainty                    []string                  `json:"uncertainty,omitempty"`
	Groups                         []GroupResult             `json:"groups"`
	LaunchCounts                   LaunchCounts              `json:"launchCounts"`
	StartedAt                      string                    `json:"startedAt,omitempty"`
	EndedAt                        string                    `json:"endedAt"`
	DurationMS                     int64                     `json:"durationMs"`
	ChildDurationMS                int64                     `json:"childDurationMs"`
	Cost                           TestCost                  `json:"cost"`
	Delivery                       DeliveryJudgment          `json:"delivery"`
}

func (result *TestResult) RecomputeDelivery() {
	byID := map[string]GroupResult{}
	for _, group := range result.Groups {
		byID[group.ID] = group
	}
	result.Delivery = DeliveryJudgment{Sufficient: len(result.Uncertainty) == 0,
		Discrepancies: append([]string(nil), result.Uncertainty...)}
	requiredObligations := map[string]bool{}
	coveredObligations := map[string]bool{}
	for _, id := range result.RequiredGroups {
		group, ok := byID[id]
		if ok {
			for _, obligation := range group.Obligations {
				requiredObligations[obligation] = true
				if group.Status == "passed" || group.Status == "reused" {
					coveredObligations[obligation] = true
				}
			}
		}
		if !ok || group.Status == "not-run" {
			result.Delivery.MissingGroups = append(result.Delivery.MissingGroups, id)
			result.Delivery.Sufficient = false
			continue
		}
		if group.Status != "passed" && group.Status != "reused" {
			result.Delivery.FailingGroups = append(result.Delivery.FailingGroups, id)
			result.Delivery.Sufficient = false
		}
	}
	for obligation := range requiredObligations {
		if !coveredObligations[obligation] {
			result.Delivery.UncoveredObligations = append(result.Delivery.UncoveredObligations, obligation)
			result.Delivery.Sufficient = false
		}
	}
	if len(result.Delivery.Discrepancies) > 0 {
		result.Delivery.Sufficient = false
	}
	sort.Strings(result.Delivery.MissingGroups)
	sort.Strings(result.Delivery.FailingGroups)
	sort.Strings(result.Delivery.UncoveredObligations)
}

func TestPlanDigest(contract testpolicy.Contract, plan testpolicy.Plan, candidateTree string) string {
	encoded, _ := json.Marshal(struct {
		Contract testpolicy.Contract `json:"contract"`
		Plan     testpolicy.Plan     `json:"plan"`
		Tree     string              `json:"tree"`
	}{contract, plan, candidateTree})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// ReusedTestResult composes the selected groups from the newest retained
// observation of each group's current execution identity on this seat,
// whatever goal or attempt produced it (retained-proof-reuse-crosses-
// claims-and-attempts). It starts no test or build and creates no attempt.
func ReusedTestResult(template TestResult, attempts []Attempt, identities map[string]string, contract testpolicy.Contract) TestResult {
	return reusedTestResult(template, attempts, identities, contract, "")
}

// ReusedTestResultExcluding composes evidence retained before the current
// reservation. The current live attempt is the mutation-locked exclusion
// that prevents another caller from reserving the same missing components.
func ReusedTestResultExcluding(template TestResult, attempts []Attempt, identities map[string]string, contract testpolicy.Contract, excludedAttempt string) TestResult {
	return reusedTestResult(template, attempts, identities, contract, excludedAttempt)
}

// reuseObservation is one retained sighting of a group at an execution
// identity: a launched or reused record of an attempt that has a terminal,
// or a live attempt's plan for the group. Live plans rank newest of all;
// records rank by the group's own end, then by the attempt's start.
type reuseObservation struct {
	attemptID string
	at        time.Time
	started   time.Time
	live      bool
	passed    bool
	group     GroupResult
}

func (observation reuseObservation) newerThan(other reuseObservation) bool {
	if observation.live != other.live {
		return observation.live
	}
	if !observation.at.Equal(other.at) {
		return observation.at.After(other.at)
	}
	return observation.started.After(other.started)
}

// newestReuseObservation scans every attempt on the seat for the newest
// observation of one group at one identity. A not-run or invalid record is
// not an observation: the group was not judged there. Goal, accounting
// revision and the attempt's terminal result do not scope the scan; the
// result-level digests (contract, base contract, judge key, behavior
// policy) must match the template's.
func newestReuseObservation(template TestResult, attempts []Attempt, id, identity, excludedAttempt string) (reuseObservation, bool) {
	var newest reuseObservation
	found := false
	consider := func(observation reuseObservation) {
		if !found || observation.newerThan(newest) {
			newest, found = observation, true
		}
	}
	if identity == "" {
		return newest, false
	}
	for index := range attempts {
		attempt := attempts[index]
		if attempt.AttemptID == excludedAttempt {
			continue
		}
		started, _ := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if attempt.Terminal == nil {
			planned := componentInputs(attempt.ProofIdentity.IdentityInputs)
			for plannedID, plannedIdentity := range attempt.PendingTestGroups {
				planned[plannedID] = plannedIdentity
			}
			if planned[id] == identity {
				consider(reuseObservation{attemptID: attempt.AttemptID, started: started, live: true})
			}
			continue
		}
		source := attempt.TestResult
		if source == nil || source.ContractDigest != template.ContractDigest || source.BaseContractDigest != template.BaseContractDigest ||
			source.JudgeKey != template.JudgeKey || source.BehaviorPolicyDigest != template.BehaviorPolicyDigest {
			continue
		}
		for _, group := range source.Groups {
			if group.ID != id || group.ExecutionIdentity != identity || !(group.NativeLaunched || group.Status == "reused") {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, group.EndedAt)
			if err != nil {
				at = started
			}
			consider(reuseObservation{attemptID: attempt.AttemptID, at: at, started: started, group: group,
				passed: (group.Status == "passed" || group.Status == "reused") && group.CollectionComplete})
		}
	}
	return newest, found
}

// ExactReusableTestResult returns the original successful outer result when
// one attempt owns the complete current selection. Keeping its AttemptID is
// what lets the public receipt command recover the byte-exact payload already
// committed with that attempt. Composed component reuse remains a separate
// path and intentionally has no synthetic outer owner.
func ExactReusableTestResult(template TestResult, attempts []Attempt, identities map[string]string, goalID string, accountingRevision uint64) (TestResult, bool) {
	var newest *Attempt
	for index := range attempts {
		attempt := attempts[index]
		if goalID == "" || accountingRevision == 0 || attempt.GoalID != goalID || attempt.AccountingRevision != accountingRevision ||
			attempt.Terminal == nil || attempt.Terminal.Result != TerminalSuccess || attempt.TestResult == nil || len(CommittedDeliveryReceipt(attempt)) == 0 {
			continue
		}
		result := attempt.TestResult
		if result.ContractDigest != template.ContractDigest ||
			result.BaseContractDigest != template.BaseContractDigest || result.JudgeKey != template.JudgeKey ||
			result.CandidateEngineDigest != template.CandidateEngineDigest ||
			result.CandidateEngineIdentityVersion != template.CandidateEngineIdentityVersion ||
			result.CandidateEngineBuildIdentity != template.CandidateEngineBuildIdentity ||
			result.BehaviorPolicyDigest != template.BehaviorPolicyDigest ||
			result.Purpose != template.Purpose || result.RequiredMode != template.RequiredMode || result.ExecutedMode != template.ExecutedMode ||
			!reflect.DeepEqual(result.SelectedGroups, template.SelectedGroups) || !reflect.DeepEqual(result.RequiredGroups, template.RequiredGroups) ||
			!result.Delivery.Sufficient || ValidateTestResult(*result) != nil {
			continue
		}
		observed := make(map[string]string, len(result.Groups))
		for _, group := range result.Groups {
			if group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") {
				observed[group.ID] = group.ExecutionIdentity
			}
		}
		matches := len(observed) == len(identities)
		for id, identity := range identities {
			matches = matches && observed[id] == identity
		}
		// The newest observation on the seat still decides: a newer failure or
		// a live plan for any group, under any goal, means the committed
		// result no longer describes what the seat knows.
		for id, identity := range identities {
			observation, found := newestReuseObservation(template, attempts, id, identity, "")
			if !found || observation.live || !observation.passed {
				matches = false
			}
		}
		if matches {
			newest = newerAttempt(newest, attempt)
		}
	}
	if newest == nil {
		return TestResult{}, false
	}
	return *newest.TestResult, true
}

func reusedTestResult(template TestResult, attempts []Attempt, identities map[string]string, contract testpolicy.Contract, excludedAttempt string) TestResult {
	result := template
	result.AttemptID = ""
	result.Groups = nil
	preparationLaunches := result.LaunchCounts.Other
	result.LaunchCounts = LaunchCounts{Other: preparationLaunches, CountsComplete: true}
	result.DurationMS, result.ChildDurationMS = result.Cost.PreparationDurationMS, 0
	result.Cost.ActualDurationMS, result.Cost.ChildDurationMS = result.Cost.PreparationDurationMS, 0
	result.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
	definitions := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		definitions[group.ID] = group
	}
	for _, id := range result.SelectedGroups {
		definition := definitions[id]
		unrun := func(reason string) GroupResult {
			return GroupResult{ID: id, Kind: definition.Kind, Obligations: append([]string(nil), definition.Obligations...),
				InputDigest: result.CandidateTree, InputManifest: append([]string(nil), definition.Inputs...), ExecutionIdentity: identities[id], CWD: definition.CWD, Status: "not-run", NotRunReason: reason,
				ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
		}
		// A cadence attempt is the fresh sweep that establishes trust in a
		// judge; it inherits nothing.
		if result.Purpose == testpolicy.PurposeCadence {
			result.Groups = append(result.Groups, unrun("cadence-executes-afresh"))
			continue
		}
		observation, found := newestReuseObservation(result, attempts, id, identities[id], excludedAttempt)
		switch {
		case !found:
			result.Groups = append(result.Groups, unrun("missing-proof"))
			continue
		case observation.live:
			result.Groups = append(result.Groups, unrun("live-observation-blocks-reuse"))
			continue
		case !observation.passed:
			result.Groups = append(result.Groups, unrun("newest-observation-failed"))
			continue
		}
		reused := observation.group
		reused.Status = "reused"
		reused.ReuseAttempt = observation.attemptID
		result.Groups = append(result.Groups, reused)
		switch reused.Kind {
		case "build":
			result.LaunchCounts.ReusedBuild++
		default:
			result.LaunchCounts.ReusedTest++
		}
	}
	result.Cost.ReusedLaunches = result.LaunchCounts.ReusedTest + result.LaunchCounts.ReusedBuild + result.LaunchCounts.ReusedOther
	result.RecomputeDelivery()
	return result
}

func resultDuration(start time.Time) (string, int64) {
	end := time.Now().UTC()
	return end.Format(time.RFC3339Nano), end.Sub(start).Milliseconds()
}

// GovernedTestResult returns only the newest testing attempt owned by a
// governed outer run. An older success cannot bypass a newer failed or
// unfinished attempt, and outer terminal success remains the sole authority.
func GovernedTestResult(root, runID string) (Attempt, TestResult, error) {
	attempt, result, err := LatestGovernedTestResult(root, runID)
	if err != nil {
		return attempt, result, err
	}
	if attempt.Terminal.Result != TerminalSuccess {
		return attempt, result, fmt.Errorf("newest governed testing attempt %s is not terminal success", attempt.AttemptID)
	}
	if !result.Delivery.Sufficient {
		return attempt, result, fmt.Errorf("newest governed testing attempt %s is not sufficient", attempt.AttemptID)
	}
	return attempt, result, nil
}

// LatestGovernedTestResult returns the newest terminal structured result for
// one governed outer run, including a valid red result for observation. It
// never falls back to an older success when newer evidence failed or remains
// unfinished.
func LatestGovernedTestResult(root, runID string) (Attempt, TestResult, error) {
	attempts, err := ReadAttempts(root)
	if err != nil {
		return Attempt{}, TestResult{}, err
	}
	var newest *Attempt
	for _, attempt := range attempts {
		if attempt.ReservationOwner != nil && attempt.ReservationOwner.RunID == runID {
			newest = newerAttempt(newest, attempt)
		}
	}
	if newest == nil {
		return Attempt{}, TestResult{}, fmt.Errorf("governed run %s has no testing attempt", runID)
	}
	if newest.Terminal == nil || newest.TestResult == nil {
		return *newest, TestResult{}, fmt.Errorf("newest governed testing attempt %s has no terminal structured result", newest.AttemptID)
	}
	if err := ValidateTestResult(*newest.TestResult); err != nil {
		return *newest, *newest.TestResult, fmt.Errorf("newest governed testing attempt %s is invalid: %w", newest.AttemptID, err)
	}
	return *newest, *newest.TestResult, nil
}

func RequireResultGroups(result TestResult, required []string) error {
	selected := map[string]bool{}
	for _, id := range result.RequiredGroups {
		selected[id] = true
	}
	passed := map[string]bool{}
	for _, group := range result.Groups {
		passed[group.ID] = group.CollectionComplete && (group.Status == "passed" || group.Status == "reused")
	}
	var missing []string
	for _, id := range required {
		if !selected[id] || !passed[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("testing result lacks required successful groups: %s", strings.Join(missing, ","))
	}
	return nil
}
