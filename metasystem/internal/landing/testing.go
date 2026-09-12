package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// PrepareTestingReceiptPayload builds the exact schema-2 receipt bytes before
// terminal success is committed. The current live attempt may own newly
// passed groups; every reused group must already belong to a successful outer
// attempt. Projection is deliberately separate from this atomic payload.
func PrepareTestingReceiptPayload(installationRoot, tree string, result proofrun.TestResult, completedAt time.Time) (TestReceipt, json.RawMessage, error) {
	publicationStarted := time.Now()
	if err := proofrun.ValidateTestResult(result); err != nil || !result.Delivery.Sufficient {
		return TestReceipt{}, nil, fmt.Errorf("testing result is not sufficient delivery evidence: %v", err)
	}
	if result.CandidateEngineIdentityVersion != proofrun.CandidateEngineIdentitySchemaVersion || result.CandidateEngineDigest == "" || result.CandidateEngineBuildIdentity == "" {
		return TestReceipt{}, nil, fmt.Errorf("testing result has no candidate engine build identity or digest")
	}
	if result.CandidateTree != tree {
		return TestReceipt{}, nil, fmt.Errorf("testing result proves tree %s, not receipt tree %s", result.CandidateTree, tree)
	}
	workspace := gittree.Workspace{Dir: result.ProjectRoot}
	indexBefore, worktreeBefore, err := testingReceiptPosture(installationRoot, result)
	indexBeforeMatches, indexBeforeErr := testingReceiptIndexMatches(installationRoot, tree, indexBefore)
	if err != nil || indexBeforeErr != nil || !indexBeforeMatches || worktreeBefore != tree {
		changed, _ := workspace.ChangedPaths(tree, worktreeBefore)
		return TestReceipt{}, nil, fmt.Errorf("testing receipt candidate moved before preparation: index=%s worktree=%s expected=%s changed=%v cause=%v", indexBefore, worktreeBefore, tree, changed, errors.Join(err, indexBeforeErr))
	}
	ids, err := validateTestingAttemptOwners(installationRoot, result, true)
	if err != nil {
		return TestReceipt{}, nil, err
	}
	indexAfter, worktreeAfter, err := testingReceiptPosture(installationRoot, result)
	indexAfterMatches, indexAfterErr := testingReceiptIndexMatches(installationRoot, tree, indexAfter)
	if err != nil || indexAfterErr != nil || !indexAfterMatches || worktreeAfter != tree {
		return TestReceipt{}, nil, fmt.Errorf("testing receipt candidate moved during preparation")
	}
	if completedAt.IsZero() {
		return TestReceipt{}, nil, fmt.Errorf("testing receipt preparation requires the terminal completion time")
	}
	copyResult := result
	publicationMS := time.Since(publicationStarted).Milliseconds()
	if publicationMS < 1 {
		publicationMS = 1
	}
	copyResult.Cost.PublicationDurationMS += publicationMS
	terminalPreparation := time.Now().UTC()
	if started, parseErr := time.Parse(time.RFC3339Nano, copyResult.StartedAt); parseErr == nil && !started.After(terminalPreparation) {
		copyResult.EndedAt = terminalPreparation.Format(time.RFC3339Nano)
		copyResult.DurationMS = terminalPreparation.Sub(started).Milliseconds()
		copyResult.Cost.ActualDurationMS = copyResult.DurationMS
	} else {
		copyResult.Cost.ActualDurationMS += publicationMS
		copyResult.DurationMS += publicationMS
	}
	workspaceTree, err := ProjectWorkspaceTree(installationRoot, tree)
	if err != nil {
		return TestReceipt{}, nil, fmt.Errorf("testing receipt workspace projection: %w", err)
	}
	receipt := TestReceipt{SchemaVersion: 2, Tree: tree, ProvedTree: result.CandidateTree, ExitStatus: 0,
		Time: completedAt.UTC().Format(time.RFC3339Nano), Binding: TestReceiptBinding{IndexTreeBefore: tree,
			WorktreeTreeBefore: tree, IndexTreeAfter: tree, WorktreeTreeAfter: tree},
		PolicyEngineDigest: result.PolicyEngineDigest, CandidateEngineDigest: result.CandidateEngineDigest,
		CandidateEngineBuildIdentity: result.CandidateEngineBuildIdentity,
		Workspace:                    &TestReceiptProjection{Excludes: WorkspaceExclusions(), Tree: workspaceTree}, Testing: &copyResult, AttemptIDs: ids}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return TestReceipt{}, nil, err
	}
	return receipt, encoded, nil
}

func testingReceiptIndexMatches(installationRoot, receiptTree, indexTree string) (bool, error) {
	if indexTree == receiptTree {
		return true, nil
	}
	receiptWorkspace, err := ProjectWorkspaceTree(installationRoot, receiptTree)
	if err != nil {
		return false, err
	}
	indexWorkspace, err := ProjectWorkspaceTree(installationRoot, indexTree)
	if err != nil {
		return false, err
	}
	return receiptWorkspace == indexWorkspace, nil
}

// CreateTestingReceipt projects sufficient schema-2 testing evidence without
// executing another command. It remains the multi-attempt composition path;
// an executed sufficient outer attempt instead publishes its committed bytes.
func CreateTestingReceipt(installationRoot, tree string, result proofrun.TestResult) (TestReceipt, error) {
	receipt, _, err := PrepareTestingReceiptPayload(installationRoot, tree, result, time.Now().UTC())
	if err != nil {
		return TestReceipt{}, err
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return TestReceipt{}, err
	}
	if err := os.MkdirAll(filepath.Dir(TestReceiptPath(installationRoot, tree)), 0o700); err != nil {
		return TestReceipt{}, err
	}
	if err := atomicfile.WriteVolatile(TestReceiptPath(installationRoot, tree), string(encoded)+"\n"); err != nil {
		return TestReceipt{}, err
	}
	return receipt, nil
}

func testingContractEnabled(root string) bool {
	value, present, err := config.ConfLookup(filepath.Join(root, "metasystem.conf"), "testing.contract")
	return err == nil && present && value != ""
}

func validateTestingAttemptOwners(installationRoot string, result proofrun.TestResult, allowCurrentLive bool) ([]string, error) {
	attemptIDs := map[string]bool{}
	for _, group := range result.Groups {
		attemptID := result.AttemptID
		if group.Status == "reused" {
			attemptID = group.ReuseAttempt
		}
		if attemptID == "" {
			return nil, fmt.Errorf("testing group %s has no terminal attempt owner", group.ID)
		}
		attempt, readErr := proofrun.ReadAttempt(installationRoot, attemptID)
		if readErr != nil {
			return nil, fmt.Errorf("testing group %s has no successful terminal outer attempt", group.ID)
		}
		currentLive := allowCurrentLive && attemptID == result.AttemptID && attempt.Terminal == nil && attempt.CancellationIntent == ""
		// A group the result reused may come from a failed delivery attempt that
		// stopped at another group (R-96-m1e); the owner's own record of the
		// group, checked below, still has to be a complete pass. The result's
		// own attempt must be a success or the live attempt being finalized.
		ownerAccepted := attempt.Terminal != nil && attempt.TestResult != nil &&
			(attempt.Terminal.Result == proofrun.TerminalSuccess ||
				(group.Status == "reused" && proofrun.ReusableTerminal(result.Purpose, attempt.Terminal.Result)))
		if !currentLive && !ownerAccepted {
			return nil, fmt.Errorf("testing group %s has no terminal outer attempt its purpose may reuse", group.ID)
		}
		owned := false
		ownerResult := attempt.TestResult
		if currentLive {
			ownerResult = &result
			if attempt.TestResult != nil && !reflect.DeepEqual(*attempt.TestResult, result) {
				return nil, fmt.Errorf("live outer attempt %s contradicts its joined testing result", attemptID)
			}
		}
		for _, recorded := range ownerResult.Groups {
			if recorded.ID == group.ID && recorded.ExecutionIdentity == group.ExecutionIdentity &&
				(recorded.Status == "passed" || recorded.Status == "reused") && recorded.CollectionComplete {
				owned = true
			}
		}
		if !owned {
			return nil, fmt.Errorf("testing group %s contradicts retained attempt %s", group.ID, attemptID)
		}
		attemptIDs[attemptID] = true
	}
	ids := make([]string, 0, len(attemptIDs))
	for id := range attemptIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func decodeCommittedTestingReceipt(payload json.RawMessage) (TestReceipt, error) {
	var receipt TestReceipt
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return TestReceipt{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || receipt.SchemaVersion != 2 || receipt.Testing == nil {
		return TestReceipt{}, fmt.Errorf("schema-2 testing receipt is incomplete or has trailing data")
	}
	if err := validateTestingReceiptEngineIdentity(receipt); err != nil {
		return TestReceipt{}, err
	}
	return receipt, nil
}

func validateTestingReceiptEngineIdentity(receipt TestReceipt) error {
	if receipt.Testing == nil {
		return fmt.Errorf("schema-2 testing receipt has no testing evidence")
	}
	switch receipt.Testing.CandidateEngineIdentityVersion {
	case 0:
		return nil
	case 1:
		if receipt.CandidateEngineDigest != "" && receipt.PolicyEngineDigest == receipt.Testing.PolicyEngineDigest &&
			receipt.CandidateEngineDigest == receipt.Testing.CandidateEngineDigest {
			return nil
		}
	case proofrun.CandidateEngineIdentitySchemaVersion:
		if receipt.CandidateEngineDigest != "" && receipt.CandidateEngineBuildIdentity != "" &&
			receipt.PolicyEngineDigest == receipt.Testing.PolicyEngineDigest &&
			receipt.CandidateEngineDigest == receipt.Testing.CandidateEngineDigest &&
			receipt.CandidateEngineBuildIdentity == receipt.Testing.CandidateEngineBuildIdentity {
			return nil
		}
	}
	return fmt.Errorf("schema-2 testing receipt contradicts its policy or candidate engine identity")
}

// Schema two binds the whole staged candidate and projects only actual test inputs.
func testingReceiptPosture(installationRoot string, result proofrun.TestResult) (string, string, error) {
	workspace := gittree.Workspace{Dir: result.ProjectRoot}
	index, err := workspace.StagedTree()
	if err != nil {
		return "", "", err
	}
	installationRoot, err = filepath.EvalSymlinks(installationRoot)
	if err != nil {
		return "", "", err
	}
	prefix, err := filepath.Rel(result.ProjectRoot, installationRoot)
	if err != nil {
		return "", "", err
	}
	confPath := filepath.Join(installationRoot, "metasystem.conf")
	contract, _, err := config.ConfLookup(confPath, "testing.contract")
	if err != nil {
		return "", "", err
	}
	paths := []string{filepath.ToSlash(filepath.Join(prefix, "metasystem.conf"))}
	if contract != "" {
		paths = append(paths, filepath.ToSlash(filepath.Join(prefix, contract)))
	}
	for _, group := range result.Groups {
		paths = append(paths, group.InputManifest...)
	}
	working, err := workspace.SnapshotRelevant(result.CandidateTree, paths)
	return index, working, err
}
