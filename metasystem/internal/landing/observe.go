// Package landing owns the two-bars landing classification. Observe mode
// makes the same decision enforcement will consume, but turns every policy
// failure into a durable would-refuse verdict instead of an exit failure.
package landing

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

const (
	BarChain     = "a"
	BarDirectFix = "b"
	BarRefusal   = "c"
	BarCarried   = "d"
)

var (
	landingID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	rulingID  = regexp.MustCompile(`^R-[0-9]+[a-z]?(?:-[a-z0-9]+)?$`)
	treeOID   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

// ObserveParams names the final project tree and its declaration. Chain may
// combine only with register-carriage; every other declaration is singular.
// RepoRoot may itself be nested in a Git worktree.
type ObserveParams struct {
	RepoRoot        string
	CandidateTree   string
	Chain           string
	DirectFix       string
	RevertOf        string
	Goal            string
	Actor           string
	RootJob         string
	TestReceipt     string
	Recertification string
	Carried         string
	ProjectTree     string
	LedgerTip       string
	Judge           string
	LiveFailure     string
	CarriedBy       string
	Now             time.Time
	VerifyTesting   func() (proofrun.TestResult, error)
}

// Observation is safe to put directly in a commit trailer. The values never
// echo malformed caller text, so bad input cannot mint another trailer line.
type Observation struct {
	SchemaVersion  int      `json:"schemaVersion"`
	Mode           string   `json:"mode"`
	Bar            string   `json:"bar"`
	Verdict        string   `json:"verdict"`
	Code           string   `json:"code"`
	Provenance     string   `json:"provenance"`
	VerdictTrailer string   `json:"verdictTrailer"`
	Unclassified   []string `json:"unclassified,omitempty"`
	Refusal        string   `json:"refusal,omitempty"`
	Detail         string   `json:"detail,omitempty"`
	promotionTree  string
	frozenTarget   string
}

// Observe evaluates one prospective landing, then applies the promotion
// policy recorded in the landing base. The caller remains responsible for
// enforcing Mode only for agent commits; human commits stay sovereign.
func Observe(params ObserveParams) Observation {
	if params.Carried != "" {
		return observeCarried(params)
	}
	observation := observe(params)
	if params.Recertification == "" {
		return applyPromotion(params, observation)
	}
	// Explicit proof failures are hard refusals independent of ordinary
	// observation-promotion policy. Once the proof is valid, load that policy
	// from frozen T rather than whatever commit HEAD may name after a race.
	if observation.Mode != "refuse" {
		observation = applyPromotionAtTree(params, observation, observation.promotionTree)
	}
	if observation.frozenTarget != "" && recertifiedTargetMoved(gittree.Workspace{Dir: params.RepoRoot}, observation.frozenTarget) {
		moved := refuse("chain-recertification-target-moved", observation.Provenance)
		moved.Detail = "target-commit"
		return moved
	}
	return observation
}

func observe(params ObserveParams) Observation {
	if !treeOID.MatchString(params.CandidateTree) {
		return wouldRefuse("malformed-candidate-tree", "none change=unknown")
	}
	change, err := changeDigest(params.RepoRoot, params.CandidateTree)
	if err != nil {
		return wouldRefuse("candidate-tree-unreadable", "none change=unknown")
	}
	if params.RevertOf != "" && params.DirectFix != "exact-revert" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.DirectFix == "exact-revert" && params.RevertOf == "" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.Chain == "" && params.DirectFix == "" {
		return wouldRefuse("missing-declaration", "none change="+change)
	}
	if params.Recertification != "" && (params.Chain == "" || (params.DirectFix != "" && params.DirectFix != "register-carriage")) {
		return refuse("conflicting-declarations", "invalid change="+change)
	}
	if params.Recertification == "" && params.Chain != "" && params.TestReceipt != "" {
		// Ordinary full-width chains retain their existing receipt path.
	} else if params.Recertification != "" && params.TestReceipt == "" {
		return refuse("chain-recertification-test-command-refused", "invalid change="+change)
	}
	if params.Chain != "" && params.DirectFix != "" && params.DirectFix != "register-carriage" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.DirectFix == "tier-1" && (params.RootJob == "" || params.Goal == "" || params.TestReceipt == "") {
		return refuse("tier1-declaration-refused", "invalid change="+change)
	}
	if params.DirectFix != "tier-1" && (params.RootJob != "" || (params.TestReceipt != "" && params.Chain == "")) {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.Chain != "" {
		return observeChain(params, change)
	}
	return observeDirectFix(params, change)
}

// candidatePathPolicy enforces the never-carried path and record laws before
// declaration matching. Missing-declaration remains the ordinary answer only
// when the candidate has no stronger path-policy defect.
func candidatePathPolicy(params ObserveParams, ledgerTree *goal.TreeGoals) error {
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	changedPaths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	classes, err := loadPathClasses(workspace, baseTree)
	if err != nil {
		return err
	}
	resolved, err := resolvePathClasses(workspace, classes, changedPaths)
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	if err := nonBehaviorClassError(resolved, changedPaths, true); err != nil {
		return err
	}
	for _, changedPath := range changedPaths {
		if resolved[changedPath] != pathclass.Record {
			continue
		}
		if params.Carried != "" && strings.HasPrefix(changedPath, "records/counselor/") {
			if !containsString(appendOnlyRegisters, changedPath) {
				return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s is not owned by the carried landing", changedPath)}
			}
			if err := carriedCounselorCarriageError(workspace, baseTree, params.CandidateTree, changedPath, ledgerTree); err != nil {
				return err
			}
			continue
		}
		if err := recordCarriageError(workspace, baseTree, params.CandidateTree, classes, changedPath, params.Goal, params.Actor); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCarriedCandidatePaths applies the carried landing's path classes and
// counselor-row ownership rules against one accepted goal-ledger projection.
func ValidateCarriedCandidatePaths(params ObserveParams, ledgerTree *goal.TreeGoals) error {
	return candidatePathPolicy(params, ledgerTree)
}

func carriedCounselorCarriageError(workspace gittree.Workspace, baseTree, candidateTree, path string, ledgerTree *goal.TreeGoals) error {
	lines, err := carriedCounselorAppendLines(workspace, baseTree, candidateTree, path)
	if err != nil {
		return err
	}
	switch path {
	case "records/counselor/carried-landings.jsonl":
		return carriedLandingAppendError(lines, ledgerTree)
	case "records/counselor/accepted-risk-register.jsonl":
		return carriedAcceptedRiskAppendError(workspace.Dir, lines, ledgerTree)
	default:
		return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s is not owned by the carried landing", path)}
	}
}

func carriedCounselorAppendLines(workspace gittree.Workspace, baseTree, candidateTree, path string) ([]string, error) {
	before, _, err := workspace.FileAt(baseTree, path)
	if err != nil {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	after, present, err := workspace.FileAt(candidateTree, path)
	if err != nil {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	if !present || len(after) <= len(before) || !bytes.Equal(after[:len(before)], before) || len(before) > 0 && before[len(before)-1] != '\n' || after[len(after)-1] != '\n' {
		return nil, &carriageError{code: "register-carriage-not-append-only", err: fmt.Errorf("carried counselor record %s is not an append-only set of complete lines", path)}
	}
	return strings.Split(strings.TrimSuffix(string(after[len(before):]), "\n"), "\n"), nil
}

func carriedLandingAppendError(lines []string, ledgerTree *goal.TreeGoals) error {
	rows := map[string]goal.HistoryLine{}
	if ledgerTree != nil {
		for _, files := range []map[string]*goal.GoalFile{ledgerTree.Live, ledgerTree.Done} {
			for _, file := range files {
				for _, row := range file.History {
					if row.Verb == "carried" {
						rows[row.Opid] = row
					}
				}
			}
		}
	}
	for _, line := range lines {
		var got counselor.CarriedLanding
		if json.Unmarshal([]byte(line), &got) != nil || got.ID != "cl-"+got.OpID {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("carried counselor line is not owned by a carried row")}
		}
		row, ok := rows[got.OpID]
		expected, expectedErr := counselor.CarriedLandingLine(row)
		if !ok || expectedErr != nil || !reflect.DeepEqual(got, expected) {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("carried counselor line %s has no equal carried row", got.ID)}
		}
	}
	return nil
}

func carriedAcceptedRiskAppendError(root string, lines []string, ledgerTree *goal.TreeGoals) error {
	var rows []counselor.CarriedAcceptedRiskAppend
	if ledgerTree != nil {
		for _, files := range []map[string]*goal.GoalFile{ledgerTree.Live, ledgerTree.Done} {
			for _, file := range files {
				for _, risk := range file.AcceptedRisks {
					if risk.Chain != goal.HumanCarriedChain {
						continue
					}
					commit, ok := humanCarriedRiskCommit(risk.Finding)
					if !ok {
						continue
					}
					for _, history := range file.History {
						if history.Verb != "accept-risk" || history.Opid != risk.Opid {
							continue
						}
						recordedAt, parseErr := time.Parse(time.RFC3339, history.At)
						if parseErr == nil {
							rows = append(rows, counselor.CarriedAcceptedRiskAppend{
								Goal: file.Id, Finding: risk.Finding, By: risk.By, Why: history.Reason,
								OpID: risk.Opid, Commit: commit, RecordedAt: recordedAt,
							})
						}
						break
					}
				}
			}
		}
	}
	for _, line := range lines {
		owned := false
		for _, row := range rows {
			if counselor.ValidateCarriedAcceptedRiskLine(root, row, []byte(line)) == nil {
				owned = true
				break
			}
		}
		if !owned {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("accepted-risk counselor line has no equal human-carried accepted-risk row")}
		}
	}
	return nil
}

func humanCarriedRiskCommit(finding string) (string, bool) {
	commit := strings.TrimPrefix(finding, "carried:")
	commit = strings.TrimSuffix(commit, ":battery-red")
	validShape := finding == "carried:"+commit || finding == "carried:"+commit+":battery-red"
	return commit, validShape && len(commit) == 40 && treeOID.MatchString(commit)
}

func changeDigest(root, candidateTree string) (string, error) {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return "", err
	}
	patch, err := workspace.Diff(baseTree, candidateTree)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(patch)
	return fmt.Sprintf("%x", sum), nil
}

func observeChain(params ObserveParams, change string) (observation Observation) {
	frozenTargetCommit, frozenTargetTree := "", ""
	defer func() {
		observation.promotionTree = frozenTargetTree
		observation.frozenTarget = frozenTargetCommit
	}()
	provenance := "invalid change=" + change
	if !landingID.MatchString(params.Chain) {
		return wouldRefuse("malformed-chain-id", provenance)
	}
	provenance = fmt.Sprintf("chain=%s change=%s", params.Chain, change)
	recordPath := filepath.Join(params.RepoRoot, "artifacts", "agents", "jobs", params.Chain+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		return wouldRefuse("chain-record-unreadable", provenance)
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil || record["jobId"] != params.Chain || record["parentJob"] != nil {
		return wouldRefuse("chain-record-malformed", provenance)
	}
	if record["role"] != "implementer" {
		return wouldRefuse("chain-not-implementation", provenance)
	}
	width := "area"
	if recorded, present := record["gateWidth"]; present && recorded != nil {
		var ok bool
		width, ok = recorded.(string)
		if !ok || (width != "area" && width != "full") {
			return wouldRefuse("chain-record-malformed", provenance)
		}
	}
	if testingContractEnabled(params.RepoRoot) && params.Recertification == "" {
		receipt, receiptErr := readTestReceipt(params)
		if receiptErr != nil || receipt.SchemaVersion != 2 {
			observation := wouldRefuse("chain-test-receipt-refused", provenance)
			var verificationFailure *testingReceiptVerificationFailure
			if errors.As(receiptErr, &verificationFailure) {
				observation.Detail = verificationFailure.Error()
			}
			return observation
		}
	} else if width == "full" && params.Recertification == "" {
		receipt, receiptErr := readTestReceipt(params)
		if receiptErr != nil || !fullReceiptCommandAccepted(receipt) {
			return wouldRefuse("chain-full-gate-refused", provenance)
		}
	}
	hazard, _ := record["destructiveReach"].(string)
	if hazard != "DESIGN-BEARING" && hazard != "DESTRUCTIVE-REACH" {
		return wouldRefuse("chain-not-design-bearing", provenance)
	}
	if record["chainClosed"] != true {
		return wouldRefuse("chain-open", provenance)
	}
	var certifiedDigest string
	var extraPaths []string
	if params.Recertification != "" {
		verified, verifyErr := validate.VerifyRecertification(params.RepoRoot, params.Chain, params.Recertification)
		if verifyErr != nil {
			observation := refuse("chain-recertification-unproven", provenance)
			var failure *validate.RecertificationFailure
			if errors.As(verifyErr, &failure) {
				observation.Code = failure.Reason
				observation.Detail = failure.Detail
				observation.VerdictTrailer = "would-refuse code=" + observation.Code
			}
			return observation
		}
		workspace := gittree.Workspace{Dir: params.RepoRoot}
		head, unborn, headErr := workspace.HeadCommit()
		if headErr != nil || unborn || head != verified.Record.TargetCommit {
			observation := refuse("chain-recertification-target-moved", provenance)
			observation.Detail = "target-commit"
			return observation
		}
		frozenTargetCommit, frozenTargetTree = verified.Record.TargetCommit, verified.Record.TargetTree
		if verified.Record.GateWidth != width {
			observation := refuse("chain-recertification-test-command-refused", provenance)
			observation.Detail = "gate-width"
			return observation
		}
		receipt, receiptErr := readTestReceipt(params)
		if verified.Record.TestingReceiptSchema == 2 {
			if receiptErr != nil || receipt.SchemaVersion != 2 || receipt.Testing == nil ||
				!receipt.Testing.Delivery.Sufficient || receipt.Tree != verified.Record.MergedWholeTree {
				observation := refuse("chain-recertification-test-command-refused", provenance)
				observation.Detail = "testing-receipt"
				if receiptErr != nil {
					observation.Detail += ": " + receiptErr.Error()
				}
				return observation
			}
		} else if receiptErr != nil || receipt.Command != verified.Record.TestCommand {
			code := "chain-recertification-test-command-refused"
			if verified.Record.GateWidth == "full" {
				code = "chain-full-gate-refused"
			}
			observation := refuse(code, provenance)
			observation.Detail = "receipt-command"
			if receiptErr != nil {
				observation.Detail += ": " + receiptErr.Error()
			}
			return observation
		}
		certifiedDigest, extraPaths, err = bindRecertifiedChange(params.RepoRoot, params.CandidateTree, verified.Record)
		if err != nil {
			observation := refuse("chain-output-mismatch", provenance)
			observation.Detail = "merged-candidate"
			return observation
		}
		provenance = fmt.Sprintf("chain=%s change=%s certified-change=%s recertification=%s target=%s",
			params.Chain, change, certifiedDigest, verified.Record.RecordDigest, verified.Record.TargetCommit)
	} else {
		output, outputErr := chainCertifiedOutput(params.RepoRoot, params.Chain, record)
		if outputErr != nil {
			return wouldRefuse("chain-output-unreadable", provenance)
		}
		certifiedDigest, extraPaths, err = bindCertifiedChange(params.RepoRoot, params.CandidateTree, output)
		if err != nil {
			return wouldRefuse("chain-output-mismatch", provenance)
		}
		provenance = fmt.Sprintf("chain=%s change=%s certified-change=%s", params.Chain, change, certifiedDigest)
	}
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	baseTree := frozenTargetTree
	if baseTree == "" {
		baseTree, err = workspace.HeadTree()
		if err != nil {
			return wouldRefuse("register-carriage-policy-unreadable", provenance)
		}
	}
	classes, err := loadPathClasses(workspace, baseTree)
	if err != nil {
		return wouldRefuse(carriageRefusalCode(err), provenance)
	}
	changedPaths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
	if err != nil {
		return wouldRefuse("candidate-tree-unreadable", provenance)
	}
	resolved, err := resolvePathClasses(workspace, classes, changedPaths)
	if err != nil {
		return wouldRefuse("register-carriage-policy-unreadable", provenance)
	}
	if params.DirectFix == "register-carriage" {
		if classErr := behaviorResolvedError(resolved, extraPaths); classErr != nil {
			return wouldRefuseFromCarriage(classErr, provenance)
		}
	}
	if params.Goal != "" {
		if err := heldGoalError(workspace, baseTree, params.Goal, params.Actor); err != nil {
			return wouldRefuseFromCarriage(err, provenance)
		}
	}
	if classErr := chainClassError(resolved, changedPaths); classErr != nil {
		return wouldRefuseFromCarriage(classErr, provenance)
	}
	// The receipt line a code landing must append (landing receipt-line) is
	// never part of a delegate's certified diff; an append-only change to the
	// receipt ledger rides with the chain instead of counting as uncarried.
	extraPaths = carriedReceiptLedger(workspace, baseTree, params.CandidateTree, extraPaths)
	if len(extraPaths) > 0 && params.DirectFix != "register-carriage" {
		return wouldRefuse("chain-has-uncarried-paths", provenance)
	}
	if params.DirectFix == "register-carriage" {
		provenance = fmt.Sprintf("chain=%s direct-fix class=register-carriage change=%s certified-change=%s", params.Chain, change, certifiedDigest)
		if err := registerCarriage(params.RepoRoot, params.CandidateTree, extraPaths, params.Goal, params.Actor); err != nil {
			return wouldRefuseFromCarriage(err, provenance)
		}
		if frozenTargetCommit != "" && recertifiedTargetMoved(workspace, frozenTargetCommit) {
			observation := refuse("chain-recertification-target-moved", provenance)
			observation.Detail = "target-commit"
			return observation
		}
		observation := pass(BarChain, "closed-chain", provenance)
		observation.VerdictTrailer = "pass bar=a carriage=register-carriage"
		return observation
	}
	if frozenTargetCommit != "" && recertifiedTargetMoved(workspace, frozenTargetCommit) {
		observation := refuse("chain-recertification-target-moved", provenance)
		observation.Detail = "target-commit"
		return observation
	}
	return pass(BarChain, "closed-chain", provenance)
}

func recertifiedTargetMoved(workspace gittree.Workspace, expected string) bool {
	head, unborn, err := workspace.HeadCommit()
	return err != nil || unborn || head != expected
}

func bindRecertifiedChange(root, candidateTree string, record validate.RecertificationRecord) (string, []string, error) {
	workspace := gittree.Workspace{Dir: root}
	want, err := pathChangeDigest(workspace, record.TargetTree, record.MergedTree, record.CertifiedPaths)
	if err != nil {
		return "", nil, err
	}
	got, err := pathChangeDigest(workspace, record.TargetTree, candidateTree, record.CertifiedPaths)
	if err != nil || got != want {
		return "", nil, fmt.Errorf("candidate changed a certified path")
	}
	landingPaths, err := workspace.ChangedPaths(record.TargetTree, candidateTree)
	if err != nil {
		return "", nil, err
	}
	certified := map[string]bool{}
	for _, path := range record.CertifiedPaths {
		certified[path] = true
	}
	var extras []string
	for _, path := range landingPaths {
		if !certified[path] {
			extras = append(extras, path)
		}
	}
	return want, extras, nil
}

type certifiedOutput struct {
	round          int
	implementerJob string
	reviewedTree   string
	patch          []byte
}

// chainCertifiedOutput follows conformance's real storage seam. A root-id
// invocation writes the current review under rounds/1 even after follow-up
// rounds, while a follow-up-id invocation writes under rounds/N. Every
// parseable review is considered; the closed critic's reviewedTree is the
// strongest selector, the terminal implementer job is next, and the numeric
// round is only the deterministic fallback. A single review needs no guess.
func chainCertifiedOutput(root, chain string, rootRecord map[string]any) (certifiedOutput, error) {
	roundsRoot := filepath.Join(root, "artifacts", "agents", chain, "rounds")
	entries, err := os.ReadDir(roundsRoot)
	if err != nil {
		return certifiedOutput{}, err
	}
	var outputs []certifiedOutput
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		round, err := strconv.Atoi(entry.Name())
		if err != nil || round < 1 {
			continue
		}
		roundDir := filepath.Join(roundsRoot, entry.Name())
		data, err := os.ReadFile(filepath.Join(roundDir, "review.json"))
		if err != nil {
			continue
		}
		var review struct {
			DiffArtifact   string `json:"diffArtifact"`
			ImplementerJob string `json:"implementerJob"`
			ReviewedTree   string `json:"reviewedTree"`
		}
		if json.Unmarshal(data, &review) != nil || review.DiffArtifact != "diff.patch" ||
			!landingID.MatchString(review.ImplementerJob) || !treeOID.MatchString(review.ReviewedTree) {
			continue
		}
		diffPath := filepath.Join(roundDir, review.DiffArtifact)
		diffInfo, err := os.Stat(diffPath)
		if err != nil || !diffInfo.Mode().IsRegular() {
			continue
		}
		patch, err := os.ReadFile(diffPath)
		if err != nil {
			continue
		}
		outputs = append(outputs, certifiedOutput{
			round: round, implementerJob: review.ImplementerJob,
			reviewedTree: review.ReviewedTree, patch: patch,
		})
	}
	if len(outputs) == 0 {
		return certifiedOutput{}, fmt.Errorf("chain has no parseable conformance output")
	}
	if len(outputs) == 1 {
		return outputs[0], nil
	}
	if reviewedTree := closedCriticReviewedTree(root, rootRecord); reviewedTree != "" {
		for _, output := range outputs {
			if output.reviewedTree == reviewedTree {
				return output, nil
			}
		}
	}
	if terminal := terminalImplementerJob(root, chain); terminal != "" {
		for _, output := range outputs {
			if output.implementerJob == terminal {
				return output, nil
			}
		}
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].round > outputs[j].round })
	return outputs[0], nil
}

func closedCriticReviewedTree(root string, rootRecord map[string]any) string {
	critic, _ := rootRecord["independentCritiqueJobRef"].(string)
	if !landingID.MatchString(critic) {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", critic+".json"))
	if err != nil {
		return ""
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil || record["jobId"] != critic || record["status"] != "completed" {
		return ""
	}
	round, ok := jsonInteger(record["round"])
	if !ok || round < 1 {
		return ""
	}
	data, err = os.ReadFile(filepath.Join(root, "artifacts", "agents", critic, "rounds", strconv.Itoa(round), "return.json"))
	if err != nil {
		return ""
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		return ""
	}
	reviewed, _ := result["reviewedTree"].(string)
	if !treeOID.MatchString(reviewed) {
		return ""
	}
	return reviewed
}

func terminalImplementerJob(root, chain string) string {
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	records := map[string]map[string]any{}
	for _, recordPath := range paths {
		data, err := os.ReadFile(recordPath)
		if err != nil {
			continue
		}
		var record map[string]any
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		id, _ := record["jobId"].(string)
		if landingID.MatchString(id) {
			records[id] = record
		}
	}
	bestID, bestRound := "", 0
	for id, record := range records {
		if record["role"] != "implementer" || lineageRoot(records, id) != chain {
			continue
		}
		round, ok := jsonInteger(record["round"])
		if ok && round > bestRound {
			bestID, bestRound = id, round
		}
	}
	return bestID
}

func lineageRoot(records map[string]map[string]any, id string) string {
	seen := map[string]bool{}
	for {
		if seen[id] {
			return ""
		}
		seen[id] = true
		record, ok := records[id]
		if !ok {
			return ""
		}
		parent, present := record["parentJob"]
		if !present || parent == nil {
			return id
		}
		id, ok = parent.(string)
		if !ok {
			return ""
		}
	}
}

func jsonInteger(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		integer := int(number)
		return integer, number == float64(integer)
	case json.Number:
		integer, err := strconv.Atoi(number.String())
		return integer, err == nil
	default:
		return 0, false
	}
}

// bindCertifiedChange applies the certified patch to the landing's current
// base, then compares canonical change digests over exactly the certified
// paths. Unrelated base movement and bundled carriage paths are outside that
// digest; a changed certified blob, mode, addition, or deletion changes it.
func bindCertifiedChange(root, candidateTree string, output certifiedOutput) (string, []string, error) {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return "", nil, err
	}
	expectedTree, err := workspace.Apply(baseTree, output.patch)
	if err != nil {
		return "", nil, err
	}
	certifiedPaths, err := workspace.ChangedPaths(baseTree, expectedTree)
	if err != nil || len(certifiedPaths) == 0 {
		return "", nil, fmt.Errorf("certified diff has no changed paths")
	}
	// The patch itself is not enough: its postimage must still be the one
	// named by conformance's reviewedTree.
	expectedEntries, err := workspace.Entries(expectedTree, certifiedPaths)
	if err != nil {
		return "", nil, err
	}
	reviewedEntries, err := workspace.Entries(output.reviewedTree, certifiedPaths)
	if err != nil || !reflect.DeepEqual(expectedEntries, reviewedEntries) {
		return "", nil, fmt.Errorf("certified diff and reviewed tree disagree")
	}
	want, err := pathChangeDigest(workspace, baseTree, expectedTree, certifiedPaths)
	if err != nil {
		return "", nil, err
	}
	got, err := pathChangeDigest(workspace, baseTree, candidateTree, certifiedPaths)
	if err != nil || got != want {
		return "", nil, fmt.Errorf("landing changed certified output")
	}
	landingPaths, err := workspace.ChangedPaths(baseTree, candidateTree)
	if err != nil {
		return "", nil, err
	}
	certified := map[string]bool{}
	for _, changedPath := range certifiedPaths {
		certified[changedPath] = true
	}
	var extras []string
	for _, changedPath := range landingPaths {
		if !certified[changedPath] {
			extras = append(extras, changedPath)
		}
	}
	return want, extras, nil
}

func pathChangeDigest(workspace gittree.Workspace, fromTree, toTree string, paths []string) (string, error) {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	before, err := workspace.Entries(fromTree, ordered)
	if err != nil {
		return "", err
	}
	after, err := workspace.Entries(toTree, ordered)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, changedPath := range ordered {
		fmt.Fprintf(hash, "%d:%s\n", len(changedPath), changedPath)
		writeDigestEntry(hash, before[changedPath])
		writeDigestEntry(hash, after[changedPath])
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type digestWriter interface {
	Write([]byte) (int, error)
}

func writeDigestEntry(writer digestWriter, entry gittree.Entry) {
	if entry.Mode == "" {
		fmt.Fprintln(writer, "absent")
		return
	}
	fmt.Fprintf(writer, "%s %s\n", entry.Mode, entry.OID)
}

type carriageError struct {
	code         string
	err          error
	unclassified []string
}

func (e *carriageError) Error() string { return e.err.Error() }

func carriageRefusalCode(err error) string {
	var carriage *carriageError
	if errors.As(err, &carriage) {
		return carriage.code
	}
	return "register-carriage-policy-unreadable"
}

func wouldRefuseFromCarriage(err error, provenance string) Observation {
	observation := wouldRefuse(carriageRefusalCode(err), provenance)
	var carriage *carriageError
	if errors.As(err, &carriage) && len(carriage.unclassified) > 0 {
		observation.Unclassified = append([]string(nil), carriage.unclassified...)
		lines := make([]string, 0, len(carriage.unclassified))
		for _, changedPath := range carriage.unclassified {
			lines = append(lines, pathclass.RefusalText(changedPath))
		}
		observation.Refusal = strings.Join(lines, "\n")
	}
	return observation
}

type landingClassManifest struct {
	SchemaVersion       int `json:"schemaVersion"`
	EnginePolicyVersion int `json:"enginePolicyVersion"`
	Classes             []struct {
		ID              string   `json:"id"`
		PathRule        string   `json:"pathRule"`
		RequiredFields  []string `json:"requiredFields"`
		AuthorizedBy    string   `json:"authorizedBy"`
		MaxFiles        int      `json:"maxFiles,omitempty"`
		MaxChangedLines int      `json:"maxChangedLines,omitempty"`
	} `json:"classes"`
}

func registerCarriage(root, candidateTree string, changedPaths []string, goalID, actor string) error {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return err
	}
	classes, err := loadPathClasses(workspace, baseTree)
	if err != nil {
		return err
	}
	resolved, err := resolvePathClasses(workspace, classes, changedPaths)
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	if err := behaviorResolvedError(resolved, changedPaths); err != nil {
		return err
	}
	if goalID != "" {
		if err := heldGoalError(workspace, baseTree, goalID, actor); err != nil {
			return err
		}
	}
	if err := nonBehaviorClassError(resolved, changedPaths, false); err != nil {
		return err
	}
	for _, changedPath := range changedPaths {
		if err := recordCarriageError(workspace, baseTree, candidateTree, classes, changedPath, goalID, actor); err != nil {
			return err
		}
	}
	return nil
}

func resolvePathClasses(workspace gittree.Workspace, classes *pathclass.Manifest, changedPaths []string) (map[string]pathclass.Class, error) {
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]pathclass.Class, len(changedPaths))
	for _, changedPath := range changedPaths {
		repositoryPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(prefix), filepath.FromSlash(changedPath)))
		ownership, modeText, err := stateroot.OwnerForInstallation(workspace.Dir, repositoryPath)
		if err != nil {
			return nil, fmt.Errorf("classify landing path %s: %w", changedPath, err)
		}
		resolution := classes.ResolveRepositoryPath(pathclass.Mode(modeText), ownership, prefix, repositoryPath)
		resolved[changedPath] = resolution.Class
	}
	return resolved, nil
}

func chainClassError(resolved map[string]pathclass.Class, changedPaths []string) error {
	return nonBehaviorClassError(resolved, changedPaths, true)
}

func behaviorResolvedError(resolved map[string]pathclass.Class, changedPaths []string) error {
	for _, changedPath := range changedPaths {
		if resolved[changedPath] == pathclass.Behavior {
			return &carriageError{code: "direct-fix-floor-refused", err: fmt.Errorf("path %s is on the never-direct-fix floor", changedPath)}
		}
	}
	return nil
}

func nonBehaviorClassError(resolved map[string]pathclass.Class, changedPaths []string, outsideAllowed bool) error {
	for _, changedPath := range changedPaths {
		if resolved[changedPath] == pathclass.Ledger {
			return &carriageError{code: "ledger-path-not-goal-verb", err: fmt.Errorf("ledger path %s changes only through a goal verb", changedPath)}
		}
	}
	for _, changedPath := range changedPaths {
		if resolved[changedPath] == pathclass.Runtime {
			return &carriageError{code: "runtime-path-refused", err: fmt.Errorf("runtime path %s cannot be landed", changedPath)}
		}
	}
	var unclassified []string
	for _, changedPath := range changedPaths {
		if resolved[changedPath] == pathclass.Unclassified {
			unclassified = append(unclassified, changedPath)
		}
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		return &carriageError{code: "path-unclassified", err: fmt.Errorf("path has no class"), unclassified: unclassified}
	}
	if !outsideAllowed {
		for _, changedPath := range changedPaths {
			if resolved[changedPath] == pathclass.Outside {
				return &carriageError{code: "register-carriage-path-refused", err: fmt.Errorf("path %s is outside register carriage", changedPath)}
			}
		}
	}
	return nil
}

func heldGoal(workspace gittree.Workspace, baseTree, goalID, actor string) (*goal.GoalFile, error) {
	data, present, err := workspace.FileAt(baseTree, "plans/goals/"+goalID+".md")
	if err != nil || !present {
		return nil, &carriageError{code: "goal-item-not-held", err: fmt.Errorf("goal item %s is not held by %s", goalID, actor)}
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Id != goalID || file.State != goal.StateClaimed || file.Claimed == nil ||
		actor != file.Claimed.Machine+"+"+file.Claimed.Lineage {
		return nil, &carriageError{code: "goal-item-not-held", err: fmt.Errorf("goal item %s is not held by %s", goalID, actor)}
	}
	return file, nil
}

func heldGoalError(workspace gittree.Workspace, baseTree, goalID, actor string) error {
	_, err := heldGoal(workspace, baseTree, goalID, actor)
	return err
}

func recordCarriageError(workspace gittree.Workspace, baseTree, candidateTree string, classes *pathclass.Manifest, changedPath, goalID, actor string) error {
	baseEntries, err := workspace.Entries(baseTree, []string{changedPath})
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	candidateEntries, err := workspace.Entries(candidateTree, []string{changedPath})
	if err != nil {
		return &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	_, existed := baseEntries[changedPath]
	_, present := candidateEntries[changedPath]
	held := goalID != ""

	switch changedPath {
	case "memory/rulings.md":
		if err := addRulingRowsOnly(workspace, baseTree, candidateTree); err != nil {
			return &carriageError{code: "register-carriage-not-append-only", err: err}
		}
		return nil
	case "memory/receipts.log", "records/narrator-digest.log":
		if err := appendOnly(workspace, baseTree, candidateTree, changedPath); err != nil {
			return &carriageError{code: "register-carriage-not-append-only", err: err}
		}
		return nil
	}

	if seat, ok := handoffSeat(changedPath); ok {
		if !present {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("handoff %s was deleted", changedPath)}
		}
		if !existed {
			return nil
		}
		actorSeat, _, _ := strings.Cut(actor, "+")
		if actorSeat != seat {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("handoff %s is owned by %s", changedPath, seat)}
		}
		return nil
	}

	if strings.HasPrefix(changedPath, "plans/") {
		owner, owned := classes.GoalOwner(changedPath)
		if !owned {
			owner, err = longestGoalOwner(workspace, baseTree, changedPath)
			if err != nil {
				return &carriageError{code: "register-carriage-policy-unreadable", err: err}
			}
			owned = owner != ""
		}
		if owned {
			if !present || !held || goalID != owner {
				return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s is owned by goal %s", changedPath, owner)}
			}
			return nil
		}
		if existed {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("existing plan record %s is frozen", changedPath)}
		}
		if !present {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("plan record %s was deleted", changedPath)}
		}
		return nil
	}

	if strings.HasPrefix(changedPath, "memory/") || strings.HasPrefix(changedPath, "development/") {
		if !present || (existed && !held) {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s requires a held goal", changedPath)}
		}
		return nil
	}

	if strings.HasPrefix(changedPath, "records/") {
		if !existed {
			if !present {
				return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s was deleted", changedPath)}
			}
			return nil
		}
		if !held {
			return &carriageError{code: "record-not-owned", err: fmt.Errorf("existing record %s requires a held goal", changedPath)}
		}
		if err := appendOnly(workspace, baseTree, candidateTree, changedPath); err != nil {
			return &carriageError{code: "register-carriage-not-append-only", err: err}
		}
		return nil
	}

	if existed || !present {
		return &carriageError{code: "record-not-owned", err: fmt.Errorf("record %s is not owned", changedPath)}
	}
	return nil
}

func handoffSeat(changedPath string) (string, bool) {
	if !strings.HasPrefix(changedPath, "plans/handoff-") || !strings.HasSuffix(changedPath, ".md") ||
		strings.Contains(strings.TrimPrefix(changedPath, "plans/"), "/") {
		return "", false
	}
	remainder := strings.TrimSuffix(strings.TrimPrefix(changedPath, "plans/handoff-"), ".md")
	seat, suffix, ok := strings.Cut(remainder, "-")
	return seat, ok && seat != "" && suffix != ""
}

func longestGoalOwner(workspace gittree.Workspace, baseTree, changedPath string) (string, error) {
	if filepath.Dir(changedPath) != "plans" || filepath.Ext(changedPath) != ".md" {
		return "", nil
	}
	entries, err := workspace.Entries(baseTree, []string{"plans/goals/"})
	if err != nil {
		return "", err
	}
	filename := strings.TrimSuffix(filepath.Base(changedPath), ".md")
	longest := ""
	for goalPath := range entries {
		if filepath.Dir(goalPath) != "plans/goals" || filepath.Ext(goalPath) != ".md" {
			continue
		}
		goalID := strings.TrimSuffix(filepath.Base(goalPath), ".md")
		if len(goalID) > len(longest) && strings.HasPrefix(filename, goalID+"-") {
			longest = goalID
		}
	}
	return longest, nil
}

func loadPathClasses(workspace gittree.Workspace, baseTree string, requireTierOne ...bool) (*pathclass.Manifest, error) {
	tierOneRequired := len(requireTierOne) > 0 && requireTierOne[0]
	if err := loadLandingClasses(workspace, baseTree, tierOneRequired); err != nil {
		return nil, err
	}
	manifestBytes, present, err := workspace.FileAt(baseTree, pathclass.ManifestPath)
	if err != nil || !present {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("path class manifest is unreadable")}
	}
	manifest, err := pathclass.Parse(manifestBytes)
	if err != nil {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: err}
	}
	return manifest, nil
}

func loadLandingClasses(workspace gittree.Workspace, baseTree string, requireTierOne bool) error {
	manifestBytes, present, err := workspace.FileAt(baseTree, "scripts/agents/landing-classes.json")
	if err != nil || !present {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is unreadable")}
	}
	var manifest landingClassManifest
	if json.Unmarshal(manifestBytes, &manifest) != nil || manifest.SchemaVersion != 1 ||
		manifest.EnginePolicyVersion != 1 || (len(manifest.Classes) != 2 && len(manifest.Classes) != 3) {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is malformed")}
	}
	rulings, present, err := workspace.FileAt(baseTree, "memory/rulings.md")
	if err != nil || !present {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("rulings register is unreadable")}
	}
	rulingRows := parseRulingRows(rulings)
	found := map[string]bool{}
	for _, class := range manifest.Classes {
		if found[class.ID] || !rulingID.MatchString(class.AuthorizedBy) || !rulingRows[class.AuthorizedBy] {
			return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is malformed")}
		}
		switch class.ID {
		case "register-carriage":
			if class.PathRule != "path-class-record" || len(class.RequiredFields) != 0 ||
				class.MaxFiles != 0 || class.MaxChangedLines != 0 {
				return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest has the wrong carriage rule")}
			}
		case "exact-revert":
			if class.PathRule != "tree-shaped-exact-inverse" || !reflect.DeepEqual(class.RequiredFields, []string{"revert-of"}) ||
				class.MaxFiles != 0 || class.MaxChangedLines != 0 {
				return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest has the wrong exact-revert rule")}
			}
		case "tier-1":
			if class.PathRule != "tier-1-bounded" ||
				!reflect.DeepEqual(class.RequiredFields, []string{"goal", "root-job", "test-receipt"}) ||
				class.MaxFiles != 3 || class.MaxChangedLines != 40 {
				return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest has the wrong tier-1 rule")}
			}
		default:
			return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest contains an unknown class")}
		}
		found[class.ID] = true
	}
	if !found["register-carriage"] || !found["exact-revert"] || (requireTierOne && !found["tier-1"]) {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest omits a compiled class")}
	}
	return nil
}

func appendOnly(workspace gittree.Workspace, baseTree, candidateTree, changedPath string) error {
	baseEntries, err := workspace.Entries(baseTree, []string{changedPath})
	if err != nil {
		return err
	}
	candidateEntries, err := workspace.Entries(candidateTree, []string{changedPath})
	if err != nil {
		return err
	}
	if baseEntry, existed := baseEntries[changedPath]; existed {
		if candidateEntry, present := candidateEntries[changedPath]; !present || candidateEntry.Mode != baseEntry.Mode {
			return fmt.Errorf("append-only register %s was deleted or changed mode", changedPath)
		}
	}
	before, existed, err := workspace.FileAt(baseTree, changedPath)
	if err != nil {
		return err
	}
	after, present, err := workspace.FileAt(candidateTree, changedPath)
	if err != nil {
		return err
	}
	if !present || len(after) == 0 || after[len(after)-1] != '\n' {
		return fmt.Errorf("append-only register %s was deleted or has an incomplete appended line", changedPath)
	}
	if !existed {
		return nil
	}
	if len(before) > 0 && before[len(before)-1] != '\n' {
		return fmt.Errorf("append-only register %s has an unterminated existing line", changedPath)
	}
	if len(after) <= len(before) || !bytes.Equal(after[:len(before)], before) {
		return fmt.Errorf("append-only register %s deletes or rewrites an existing line", changedPath)
	}
	return nil
}

func addRulingRowsOnly(workspace gittree.Workspace, baseTree, candidateTree string) error {
	const rulingsPath = "memory/rulings.md"
	before, existed, err := workspace.FileAt(baseTree, rulingsPath)
	if err != nil {
		return err
	}
	if !existed {
		return fmt.Errorf("rulings register does not exist in the landing base")
	}
	if err := appendOnly(workspace, baseTree, candidateTree, rulingsPath); err != nil {
		return err
	}
	after, _, err := workspace.FileAt(candidateTree, rulingsPath)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(after[len(before):]), "\n"), "\n") {
		if _, ok := rulingRowID(line); !ok {
			return fmt.Errorf("rulings carriage appended a malformed ruling row")
		}
	}
	return nil
}

func parseRulingRows(data []byte) map[string]bool {
	rows := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if id, ok := rulingRowID(line); ok {
			rows[id] = true
		}
	}
	return rows
}

func rulingRowID(line string) (string, bool) {
	if !strings.HasPrefix(line, "| ") {
		return "", false
	}
	remainder := strings.TrimPrefix(line, "| ")
	id, remainder, ok := strings.Cut(remainder, " |")
	if !ok || !rulingID.MatchString(id) || (remainder != "" && !strings.HasPrefix(remainder, " ")) {
		return "", false
	}
	return id, true
}

func observeDirectFix(params ObserveParams, change string) Observation {
	switch params.DirectFix {
	case "register-carriage":
		provenance := "direct-fix class=register-carriage change=" + change
		workspace := gittree.Workspace{Dir: params.RepoRoot}
		baseTree, err := workspace.HeadTree()
		if err != nil {
			return wouldRefuse("register-carriage-policy-unreadable", provenance)
		}
		paths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
		if err != nil {
			return wouldRefuse("register-carriage-policy-unreadable", provenance)
		}
		if err := registerCarriage(params.RepoRoot, params.CandidateTree, paths, params.Goal, params.Actor); err != nil {
			return wouldRefuseFromCarriage(err, provenance)
		}
		return pass(BarDirectFix, "register-carriage", provenance)
	case "exact-revert":
		provenance := "direct-fix class=exact-revert change=" + change
		if !treeOID.MatchString(params.RevertOf) {
			return wouldRefuse("malformed-revert-commit", provenance)
		}
		workspace := gittree.Workspace{Dir: params.RepoRoot}
		baseTree, err := workspace.HeadTree()
		classes, classErr := loadPathClasses(workspace, baseTree)
		if err != nil || classErr != nil {
			return wouldRefuse("direct-fix-policy-unreadable", provenance)
		}
		provenance = fmt.Sprintf("direct-fix class=exact-revert revert-of=%s change=%s", params.RevertOf, change)
		if err := exactRevert(params.RepoRoot, params.CandidateTree, params.RevertOf, classes, params.Goal, params.Actor); err != nil {
			return wouldRefuseFromExactRevert(err, provenance)
		}
		return pass(BarDirectFix, "exact-revert", provenance)
	case "tier-1":
		return observeTierOne(params, change)
	default:
		return wouldRefuse("unknown-direct-fix-class", "invalid change="+change)
	}
}

type exactRevertError struct {
	code         string
	err          error
	unclassified []string
}

func wouldRefuseFromExactRevert(err error, provenance string) Observation {
	observation := wouldRefuse(exactRevertRefusalCode(err), provenance)
	var revert *exactRevertError
	if errors.As(err, &revert) && len(revert.unclassified) > 0 {
		observation.Unclassified = append([]string(nil), revert.unclassified...)
		lines := make([]string, 0, len(revert.unclassified))
		for _, changedPath := range revert.unclassified {
			lines = append(lines, pathclass.RefusalText(changedPath))
		}
		observation.Refusal = strings.Join(lines, "\n")
	}
	return observation
}

func (e *exactRevertError) Error() string { return e.err.Error() }

func exactRevertRefusalCode(err error) string {
	var revert *exactRevertError
	if errors.As(err, &revert) {
		return revert.code
	}
	return "not-exact-revert"
}

func exactRevert(root, candidateTree, revertOf string, classes *pathclass.Manifest, goalID, actor string) error {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return err
	}
	candidatePaths, candidateErr := workspace.ChangedPaths(baseTree, candidateTree)
	parent, err := workspace.SingleParent(revertOf)
	if err != nil {
		return exactRevertPolicyError(workspace, baseTree, classes, candidatePaths, nil, goalID, actor, err)
	}
	preimageTree, err := workspace.TreeOf(parent)
	if err != nil {
		return exactRevertPolicyError(workspace, baseTree, classes, candidatePaths, nil, goalID, actor, err)
	}
	postimageTree, err := workspace.TreeOf(revertOf)
	if err != nil {
		return exactRevertPolicyError(workspace, baseTree, classes, candidatePaths, nil, goalID, actor, err)
	}
	targetPaths, err := workspace.ChangedPaths(preimageTree, postimageTree)
	if err != nil || len(targetPaths) == 0 {
		return exactRevertPolicyError(workspace, baseTree, classes, candidatePaths, targetPaths, goalID, actor, fmt.Errorf("reverted commit has no decidable changed paths"))
	}
	if err := exactRevertPolicyError(workspace, baseTree, classes, candidatePaths, targetPaths, goalID, actor, nil); err != nil {
		return err
	}
	if candidateErr != nil {
		return candidateErr
	}
	sort.Strings(targetPaths)
	sort.Strings(candidatePaths)
	if !reflect.DeepEqual(candidatePaths, targetPaths) {
		return fmt.Errorf("candidate changes paths outside the exact inverse")
	}
	baseEntries, err := workspace.Entries(baseTree, targetPaths)
	if err != nil {
		return err
	}
	postimageEntries, err := workspace.Entries(postimageTree, targetPaths)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(baseEntries, postimageEntries) {
		return fmt.Errorf("current base no longer carries the reverted commit postimage")
	}
	candidateEntries, err := workspace.Entries(candidateTree, targetPaths)
	if err != nil {
		return err
	}
	preimageEntries, err := workspace.Entries(preimageTree, targetPaths)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(candidateEntries, preimageEntries) {
		return fmt.Errorf("candidate is not the exact tree-shaped inverse")
	}
	return nil
}

func exactRevertPolicyError(workspace gittree.Workspace, baseTree string, classes *pathclass.Manifest, candidatePaths, targetPaths []string, goalID, actor string, fallback error) error {
	paths := exactRevertPaths(candidatePaths, targetPaths)
	resolved, err := resolvePathClasses(workspace, classes, paths)
	if err != nil {
		return &exactRevertError{code: "direct-fix-policy-unreadable", err: err}
	}
	if err := behaviorResolvedError(resolved, paths); err != nil {
		var carriage *carriageError
		if errors.As(err, &carriage) {
			return &exactRevertError{code: carriage.code, err: carriage.err}
		}
		return err
	}
	if goalID != "" {
		if err := heldGoalError(workspace, baseTree, goalID, actor); err != nil {
			var carriage *carriageError
			if errors.As(err, &carriage) {
				return &exactRevertError{code: carriage.code, err: carriage.err}
			}
			return err
		}
	}
	return exactRevertClassError(resolved, paths, fallback)
}

func exactRevertPaths(candidatePaths, targetPaths []string) []string {
	set := map[string]bool{}
	for _, changedPath := range append(append([]string(nil), targetPaths...), candidatePaths...) {
		set[changedPath] = true
	}
	paths := make([]string, 0, len(set))
	for changedPath := range set {
		paths = append(paths, changedPath)
	}
	sort.Strings(paths)
	return paths
}

func exactRevertClassError(resolved map[string]pathclass.Class, paths []string, fallback error) error {
	for _, changedPath := range paths {
		if resolved[changedPath] == pathclass.Behavior {
			return &exactRevertError{
				code: "direct-fix-floor-refused",
				err:  fmt.Errorf("path %s is on the never-direct-fix floor", changedPath),
			}
		}
	}
	for _, changedPath := range paths {
		if resolved[changedPath] == pathclass.Record {
			return &exactRevertError{code: "exact-revert-record-refused", err: fmt.Errorf("record path %s cannot be reverted", changedPath)}
		}
	}
	for _, changedPath := range paths {
		if resolved[changedPath] == pathclass.Ledger {
			return &exactRevertError{code: "ledger-path-not-goal-verb", err: fmt.Errorf("ledger path %s changes only through a goal verb", changedPath)}
		}
	}
	for _, changedPath := range paths {
		if resolved[changedPath] == pathclass.Runtime {
			return &exactRevertError{code: "runtime-path-refused", err: fmt.Errorf("runtime path %s cannot be landed", changedPath)}
		}
	}
	var unclassified []string
	for _, changedPath := range paths {
		if resolved[changedPath] == pathclass.Unclassified {
			unclassified = append(unclassified, changedPath)
		}
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		return &exactRevertError{code: "path-unclassified", err: fmt.Errorf("path has no class"), unclassified: unclassified}
	}
	return fallback
}

func pass(bar, code, provenance string) Observation {
	return Observation{
		SchemaVersion: 1, Mode: "observe", Bar: bar, Verdict: "pass", Code: code,
		Provenance: provenance, VerdictTrailer: "pass bar=" + bar,
	}
}

func wouldRefuse(code, provenance string) Observation {
	return Observation{
		SchemaVersion: 1, Mode: "observe", Bar: BarRefusal, Verdict: "would-refuse", Code: code,
		Provenance: provenance, VerdictTrailer: "would-refuse code=" + code,
	}
}

func refuse(code, provenance string) Observation {
	observation := wouldRefuse(code, provenance)
	observation.Mode = "refuse"
	return observation
}

// AdoptionRulings prepares the landing authority register while preserving the
// application's existing rulings. Only rows referenced by the shipped landing
// classes enter a fresh register.
func AdoptionRulings(sourceRoot, targetRoot string) ([]byte, error) {
	readRegular := func(path string) ([]byte, error) {
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("adoption requires a regular policy file: %s", path)
		}
		return os.ReadFile(path)
	}
	manifestBytes, err := readRegular(filepath.Join(sourceRoot, "scripts", "agents", "landing-classes.json"))
	if err != nil {
		return nil, err
	}
	var manifest landingClassManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != 1 || manifest.EnginePolicyVersion != 1 || len(manifest.Classes) == 0 {
		return nil, fmt.Errorf("adoption landing classes are malformed")
	}
	source, err := readRegular(filepath.Join(sourceRoot, "memory", "rulings.md"))
	if err != nil {
		return nil, err
	}
	rows := func(data []byte) map[string][]string {
		result := map[string][]string{}
		for _, row := range strings.Split(string(data), "\n") {
			if id, ok := rulingRowID(row); ok {
				result[id] = append(result[id], row)
			}
		}
		return result
	}
	canonical := rows(source)
	data, err := readRegular(filepath.Join(targetRoot, "memory", "rulings.md"))
	if os.IsNotExist(err) {
		data = []byte("# Standing rulings register\n\nCanonical human rulings required by the shipped landing policy. Application rulings append here.\n\n| id | date | ruling | context | owner | review condition |\n|---|---|---|---|---|---|\n")
	} else if err != nil {
		return nil, err
	}
	existing := rows(data)
	seen := map[string]bool{}
	for _, class := range manifest.Classes {
		id := class.AuthorizedBy
		if seen[id] {
			continue
		}
		seen[id] = true
		if !rulingID.MatchString(id) || len(canonical[id]) != 1 {
			return nil, fmt.Errorf("adoption landing authority %s is absent or ambiguous", id)
		}
		row := canonical[id][0]
		if len(existing[id]) > 1 || (len(existing[id]) == 1 && existing[id][0] != row) {
			return nil, fmt.Errorf("adoption conflicts with application ruling %s", id)
		}
		if len(existing[id]) == 0 {
			if len(data) > 0 && data[len(data)-1] != '\n' {
				data = append(data, '\n')
			}
			data = append(data, row...)
			data = append(data, '\n')
		}
	}
	return data, nil
}

// carriedReceiptLedger drops the receipt ledger from a chain landing's
// uncarried extras when its change is a pure append; any other change to it
// stays an extra and is refused as before.
func carriedReceiptLedger(workspace gittree.Workspace, baseTree, candidateTree string, extras []string) []string {
	var kept []string
	for _, extra := range extras {
		if extra == receiptLedgerPath && appendOnly(workspace, baseTree, candidateTree, extra) == nil {
			continue
		}
		kept = append(kept, extra)
	}
	return kept
}
