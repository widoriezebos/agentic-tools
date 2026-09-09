package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const (
	receiptBoundKey            = "landing.receipt-bound-min"
	receiptBoundDefaultMinutes = 40
)

// TestReceipt records the command result together with the four tree
// observations that bind its execution to one candidate.
type TestReceipt struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Tree          string                     `json:"tree"`
	Command       string                     `json:"command"`
	ExitStatus    int                        `json:"exitStatus"`
	Time          string                     `json:"time"`
	Binding       TestReceiptBinding         `json:"binding"`
	Proof         *TestReceiptProof          `json:"proof,omitempty"`
	Coverage      *proofrun.CoverageEvidence `json:"coverage,omitempty"`
	ProvedTree    string                     `json:"provedTree,omitempty"`
	AttemptIDs    []string                   `json:"attemptIds,omitempty"`
	Testing       *proofrun.TestResult       `json:"testing,omitempty"`
}

type TestReceiptProof struct {
	SchemaVersion      int    `json:"schemaVersion"`
	ControlRoot        string `json:"controlRoot"`
	GoalID             string `json:"goalId"`
	AccountingRevision uint64 `json:"accountingRevision"`
	AttemptID          string `json:"attemptId"`
	Deadline           string `json:"deadline"`
}

type TestReceiptBinding struct {
	IndexTreeBefore    string `json:"indexTreeBefore"`
	WorktreeTreeBefore string `json:"worktreeTreeBefore"`
	IndexTreeAfter     string `json:"indexTreeAfter"`
	WorktreeTreeAfter  string `json:"worktreeTreeAfter"`
}

// TestReceiptPath is the only accepted location for a candidate's receipt.
func TestReceiptPath(root, tree string) string {
	return filepath.Join(root, "artifacts", "agents", "landing", "receipts", tree+".json")
}

// ReceiptPreparation owns the detached candidate until a successful proof
// callback has checked both projections and removed the scratch worktree.
type ReceiptPreparation struct {
	root, tree, command string
	detached            *gittree.DetachedWorktree
	candidate           gittree.Workspace
	indexBefore         string
	worktreeBefore      string
	closed              bool
	closeErr            error
	evidenceTimeout     time.Duration
	evidenceMax         int64
}

func PrepareTestReceipt(root, tree, command string) (*ReceiptPreparation, error) {
	if root == "" || !treeOID.MatchString(tree) || command == "" {
		return nil, fmt.Errorf("landing test receipt requires --root, a full tree object id, and a non-empty --command")
	}
	workspace := gittree.Workspace{Dir: root}
	if _, err := workspace.Diff(tree, tree); err != nil {
		return nil, fmt.Errorf("candidate tree is unreadable: %w", err)
	}
	indexBefore, worktreeBefore, err := receiptPosture(workspace)
	if err != nil {
		return nil, err
	}
	if indexBefore != tree || worktreeBefore != tree {
		return nil, fmt.Errorf("test receipt refused: supplied tree %s differs from the real index tree %s or working-tree projection %s", tree, indexBefore, worktreeBefore)
	}
	detached, err := workspace.NewDetachedWorktree(tree)
	if err != nil {
		return nil, fmt.Errorf("prepare isolated candidate: %w", err)
	}
	candidate := detached.Workspace()
	candidateIndex, candidateWorktree, err := receiptPosture(candidate)
	if err != nil || candidateIndex != tree || candidateWorktree != tree {
		_ = detached.Close()
		return nil, fmt.Errorf("test receipt refused: isolated candidate differs from supplied tree %s", tree)
	}
	evidenceTimeout, evidenceMax := receiptEvidenceLimits(root)
	return &ReceiptPreparation{root: root, tree: tree, command: command, detached: detached,
		candidate: candidate, indexBefore: candidateIndex, worktreeBefore: candidateWorktree,
		evidenceTimeout: evidenceTimeout, evidenceMax: evidenceMax}, nil
}

func (preparation *ReceiptPreparation) ExecutionRoot() string { return preparation.candidate.Dir }

func (preparation *ReceiptPreparation) Close() error {
	if preparation == nil {
		return nil
	}
	if preparation.closed {
		return preparation.closeErr
	}
	preparation.closed = true
	_, _, err := proofrun.PreserveDetachedSuiteFailures(preparation.root, preparation.candidate.Dir,
		"landing-receipt", preparation.evidenceTimeout, preparation.evidenceMax)
	if err == nil {
		err = preparation.detached.Close()
	}
	preparation.closeErr = err
	return err
}

func receiptEvidenceLimits(root string) (time.Duration, int64) {
	timeoutSeconds, maxMegabytes := 60, 512
	if raw := config.ConfValue(filepath.Join(root, "metasystem.conf"), "suite.evidence-copy-timeout-sec", ""); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 600 {
			timeoutSeconds = parsed
		}
	}
	if raw := config.ConfValue(filepath.Join(root, "metasystem.conf"), "suite.evidence-copy-max-mb", ""); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 10240 {
			maxMegabytes = parsed
		}
	}
	return time.Duration(timeoutSeconds) * time.Second, int64(maxMegabytes) * 1024 * 1024
}

func (preparation *ReceiptPreparation) Complete(attempt proofrun.Attempt, completedAt time.Time) (TestReceipt, error) {
	if preparation == nil || preparation.closed {
		return TestReceipt{}, fmt.Errorf("receipt candidate is no longer open")
	}
	indexAfter, worktreeAfter, err := receiptPosture(preparation.candidate)
	if err != nil || indexAfter != preparation.tree || worktreeAfter != preparation.tree {
		changed := []string(nil)
		if err == nil {
			changed, _ = preparation.candidate.ChangedPaths(preparation.tree, worktreeAfter)
		}
		return TestReceipt{}, fmt.Errorf("test receipt refused: the candidate changed while the command ran (index=%s worktree=%s expected=%s paths=%v cause=%v)",
			indexAfter, worktreeAfter, preparation.tree, changed, err)
	}
	if err := preparation.Close(); err != nil {
		return TestReceipt{}, fmt.Errorf("cleanup isolated candidate: %w", err)
	}
	receipt := TestReceipt{SchemaVersion: 1, Tree: preparation.tree, Command: preparation.command, ExitStatus: 0,
		Time: completedAt.UTC().Format(time.RFC3339Nano), Binding: TestReceiptBinding{IndexTreeBefore: preparation.indexBefore,
			WorktreeTreeBefore: preparation.worktreeBefore, IndexTreeAfter: indexAfter, WorktreeTreeAfter: worktreeAfter},
		Proof: &TestReceiptProof{SchemaVersion: 1, ControlRoot: attempt.ControlRoot, GoalID: attempt.GoalID,
			AccountingRevision: attempt.AccountingRevision, AttemptID: attempt.AttemptID, Deadline: attempt.Deadline}}
	if attempt.PendingCoverage != nil && attempt.PendingCoverage.Evidence != nil {
		coverage := *attempt.PendingCoverage.Evidence
		receipt.Coverage = &coverage
	}
	return receipt, nil
}

// PublishCommittedReceipt restores the canonical projection only from the
// exact payload atomically committed with a successful retained attempt.
func PublishCommittedReceipt(root, attemptID string) (TestReceipt, error) {
	attempt, err := proofrun.ReadAttempt(root, attemptID)
	payload := proofrun.CommittedDeliveryReceipt(attempt)
	if err != nil || attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || len(payload) == 0 {
		return TestReceipt{}, fmt.Errorf("successful proof attempt has no committed delivery receipt")
	}
	var version struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(payload, &version); err != nil {
		return TestReceipt{}, fmt.Errorf("committed delivery receipt is malformed: %w", err)
	}
	if version.SchemaVersion == 2 {
		receipt, decodeErr := decodeCommittedTestingReceipt(payload)
		if decodeErr != nil || receipt.Testing.AttemptID != attempt.AttemptID || attempt.TestResult == nil ||
			!reflect.DeepEqual(*attempt.TestResult, *receipt.Testing) {
			return TestReceipt{}, fmt.Errorf("committed schema-2 delivery receipt contradicts its proof attempt: %v", decodeErr)
		}
		ownerIDs, ownerErr := validateTestingAttemptOwners(root, *receipt.Testing, false)
		if ownerErr != nil || !reflect.DeepEqual(ownerIDs, receipt.AttemptIDs) {
			return TestReceipt{}, fmt.Errorf("committed schema-2 delivery receipt has invalid outer owners: %v", ownerErr)
		}
		indexTree, worktreeTree, postureErr := testingReceiptPosture(root, *receipt.Testing)
		if postureErr != nil || indexTree != receipt.Tree || worktreeTree != receipt.Tree {
			return TestReceipt{}, fmt.Errorf("candidate moved after the committed schema-2 delivery receipt")
		}
		if err := atomicfile.WriteVolatile(TestReceiptPath(root, receipt.Tree), string(payload)+"\n"); err != nil {
			return TestReceipt{}, fmt.Errorf("project committed schema-2 delivery receipt: %w", err)
		}
		return receipt, nil
	}
	var receipt TestReceipt
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return TestReceipt{}, fmt.Errorf("committed delivery receipt is malformed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || receipt.Proof == nil || receipt.Proof.AttemptID != attempt.AttemptID ||
		receipt.Proof.ControlRoot != root || receipt.Proof.GoalID != attempt.GoalID || receipt.Proof.AccountingRevision != attempt.AccountingRevision ||
		receipt.Proof.Deadline != attempt.Deadline {
		return TestReceipt{}, fmt.Errorf("committed delivery receipt contradicts its proof attempt")
	}
	indexTree, worktreeTree, postureErr := receiptPosture(gittree.Workspace{Dir: root})
	if postureErr != nil || indexTree != receipt.Tree || worktreeTree != receipt.Tree {
		return TestReceipt{}, fmt.Errorf("candidate moved after the committed delivery receipt")
	}
	if err := atomicfile.WriteVolatile(TestReceiptPath(root, receipt.Tree), string(payload)+"\n"); err != nil {
		return TestReceipt{}, fmt.Errorf("project committed delivery receipt: %w", err)
	}
	return receipt, nil
}

// CreateTestReceipt accepts tree only when the live index and working tree
// both represent it, then runs command in a temporary detached worktree that
// contains that exact candidate. The stale target is removed first so a
// refused retry cannot leave an older receipt available for observation.
func CreateTestReceipt(root, tree, command string, stdout, stderr io.Writer) (receipt TestReceipt, err error) {
	if root == "" || !treeOID.MatchString(tree) || command == "" {
		return receipt, fmt.Errorf("landing test receipt requires --root, a full tree object id, and a non-empty --command")
	}
	target := TestReceiptPath(root, tree)
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return receipt, fmt.Errorf("invalidate prior test receipt: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(target)
		}
	}()
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(interrupts)

	workspace := gittree.Workspace{Dir: root}
	if _, err := workspace.Diff(tree, tree); err != nil {
		return receipt, fmt.Errorf("candidate tree is unreadable: %w", err)
	}
	indexBefore, worktreeBefore, err := receiptPosture(workspace)
	if err != nil {
		return receipt, err
	}
	if indexBefore != tree || worktreeBefore != tree {
		return receipt, fmt.Errorf("test receipt refused: supplied tree %s differs from the real index tree %s or working-tree projection %s", tree, indexBefore, worktreeBefore)
	}
	if err := receiptInterrupted(interrupts); err != nil {
		return receipt, err
	}

	detached, err := workspace.NewDetachedWorktree(tree)
	if err != nil {
		return receipt, fmt.Errorf("prepare isolated candidate: %w", err)
	}
	defer func() {
		if cleanupErr := detached.Close(); cleanupErr != nil {
			published = false
			receipt = TestReceipt{}
			err = errors.Join(err, fmt.Errorf("cleanup isolated candidate: %w", cleanupErr))
		}
	}()
	if err := receiptInterrupted(interrupts); err != nil {
		return receipt, err
	}
	candidateWorkspace := detached.Workspace()
	candidateIndexBefore, candidateWorktreeBefore, err := receiptPosture(candidateWorkspace)
	if err != nil {
		return receipt, fmt.Errorf("verify isolated candidate: %w", err)
	}
	if candidateIndexBefore != tree || candidateWorktreeBefore != tree {
		return receipt, fmt.Errorf("test receipt refused: isolated candidate differs from supplied tree %s", tree)
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = candidateWorkspace.Dir
	cmd.Env = gittree.ScrubbedEnviron()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := runReceiptCommand(cmd, receiptCommandBound(filepath.Join(root, "metasystem.conf")), interrupts)
	exitStatus := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() < 0 {
			return receipt, runErr
		}
		exitStatus = exit.ExitCode()
	}
	if err := receiptInterrupted(interrupts); err != nil {
		return receipt, err
	}

	candidateIndexAfter, candidateWorktreeAfter, err := receiptPosture(candidateWorkspace)
	if err != nil {
		return receipt, err
	}
	if candidateIndexAfter != tree || candidateWorktreeAfter != tree {
		return receipt, fmt.Errorf("test receipt refused: the candidate changed while the command ran")
	}
	if err := detached.Close(); err != nil {
		return receipt, fmt.Errorf("cleanup isolated candidate: %w", err)
	}
	if err := receiptInterrupted(interrupts); err != nil {
		return receipt, err
	}

	receipt = TestReceipt{
		SchemaVersion: 1,
		Tree:          tree,
		Command:       command,
		ExitStatus:    exitStatus,
		Time:          time.Now().UTC().Format(time.RFC3339Nano),
		Binding: TestReceiptBinding{
			IndexTreeBefore: candidateIndexBefore, WorktreeTreeBefore: candidateWorktreeBefore,
			IndexTreeAfter: candidateIndexAfter, WorktreeTreeAfter: candidateWorktreeAfter,
		},
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return TestReceipt{}, err
	}
	if err := atomicfile.WriteVolatile(target, string(append(encoded, '\n'))); err != nil {
		return TestReceipt{}, fmt.Errorf("write test receipt: %w", err)
	}
	if err := receiptInterrupted(interrupts); err != nil {
		return TestReceipt{}, err
	}
	signal.Stop(interrupts)
	if err := receiptInterrupted(interrupts); err != nil {
		return TestReceipt{}, err
	}
	published = true
	return receipt, nil
}

const receiptCommandStopWait = 5 * time.Second

func receiptInterrupted(interrupts <-chan os.Signal) error {
	select {
	case received := <-interrupts:
		return fmt.Errorf("landing test receipt interrupted by %s", received)
	default:
		return nil
	}
}

func runReceiptCommand(cmd *exec.Cmd, bound boundedexec.Bound, interrupts <-chan os.Signal) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(bound.Limit)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case received := <-interrupts:
		stopReceiptCommand(cmd, done, received)
		return fmt.Errorf("landing test receipt interrupted by %s", received)
	case <-timer.C:
		stopReceiptCommand(cmd, done, syscall.SIGKILL)
		if bound.Key == "" {
			return fmt.Errorf("landing test command %w after %s", boundedexec.ErrTimedOut, bound.Limit)
		}
		return fmt.Errorf("landing test command %w after %s (raise it with %s)", boundedexec.ErrTimedOut, bound.Limit, bound.Key)
	}
}

func stopReceiptCommand(cmd *exec.Cmd, done <-chan error, received os.Signal) {
	if cmd.Process == nil {
		return
	}
	forwarded, ok := received.(syscall.Signal)
	if !ok {
		forwarded = syscall.SIGTERM
	}
	_ = syscall.Kill(-cmd.Process.Pid, forwarded)
	grace := time.NewTimer(receiptCommandStopWait)
	defer grace.Stop()
	select {
	case <-done:
		return
	case <-grace.C:
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	finalWait := time.NewTimer(receiptCommandStopWait)
	defer finalWait.Stop()
	select {
	case <-done:
	case <-finalWait.C:
	}
}

// receiptCommandBound is deliberately separate from the generic local
// execution bound. A full landing battery may lawfully run longer than a
// routine local subprocess, while still needing a hard process-group ceiling.
func receiptCommandBound(confPath string) boundedexec.Bound {
	minutes := receiptBoundDefaultMinutes
	if raw := config.ConfValue(confPath, receiptBoundKey, ""); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			minutes = parsed
		}
	}
	return boundedexec.FixedBound(time.Duration(minutes)*time.Minute, receiptBoundKey)
}

func receiptPosture(workspace gittree.Workspace) (string, string, error) {
	indexTree, err := workspace.StagedTree()
	if err != nil {
		return "", "", fmt.Errorf("read real index tree: %w", err)
	}
	worktreeTree, err := workspace.Snapshot("HEAD")
	if err != nil {
		return "", "", fmt.Errorf("read working-tree projection: %w", err)
	}
	return indexTree, worktreeTree, nil
}

func readTestReceipt(params ObserveParams) (TestReceipt, error) {
	if params.TestReceipt == "" {
		return TestReceipt{}, fmt.Errorf("tier-1 requires --test-receipt")
	}
	receiptDirectory, err := filepath.Abs(filepath.Dir(TestReceiptPath(params.RepoRoot, params.CandidateTree)))
	if err != nil {
		return TestReceipt{}, err
	}
	supplied := params.TestReceipt
	if !filepath.IsAbs(supplied) {
		supplied = filepath.Join(params.RepoRoot, supplied)
	}
	supplied, err = filepath.Abs(supplied)
	if err != nil || filepath.Clean(filepath.Dir(supplied)) != filepath.Clean(receiptDirectory) {
		return TestReceipt{}, fmt.Errorf("test receipt must be inside %s", receiptDirectory)
	}
	data, err := os.ReadFile(supplied)
	if err != nil {
		return TestReceipt{}, fmt.Errorf("read test receipt: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt TestReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return TestReceipt{}, fmt.Errorf("test receipt is malformed: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return TestReceipt{}, fmt.Errorf("test receipt contains trailing JSON")
	}
	expectedTree := params.CandidateTree
	if receipt.SchemaVersion == 2 {
		expectedTree = receipt.Tree
	}
	expected := TestReceiptPath(params.RepoRoot, expectedTree)
	if filepath.Clean(supplied) != filepath.Clean(expected) {
		return TestReceipt{}, fmt.Errorf("test receipt must be %s", expected)
	}
	if receipt.SchemaVersion == 2 {
		if receipt.ProvedTree == "" || receipt.Testing == nil || receipt.ExitStatus != 0 || receipt.Command != "" || receipt.Proof != nil || receipt.Coverage != nil {
			return TestReceipt{}, fmt.Errorf("schema-2 test receipt has incomplete or legacy evidence")
		}
		projected, projectErr := (gittree.Workspace{Dir: params.RepoRoot}).TreeOf(receipt.Tree)
		if projectErr != nil || projected != params.CandidateTree {
			return TestReceipt{}, fmt.Errorf("schema-2 whole-project receipt does not project to landing candidate")
		}
		if err := proofrun.ValidateTestResult(*receipt.Testing); err != nil || !receipt.Testing.Delivery.Sufficient || receipt.Testing.CandidateTree != receipt.ProvedTree {
			return TestReceipt{}, fmt.Errorf("schema-2 test receipt is not sufficient: %v", err)
		}
		attemptIDs, err := validateTestingAttemptOwners(params.RepoRoot, *receipt.Testing, false)
		if err != nil || !reflect.DeepEqual(attemptIDs, receipt.AttemptIDs) {
			return TestReceipt{}, fmt.Errorf("schema-2 test receipt attempt ownership is invalid: %v", err)
		}
		if _, err := time.Parse(time.RFC3339Nano, receipt.Time); err != nil {
			return TestReceipt{}, fmt.Errorf("test receipt time is malformed")
		}
		for _, observed := range []string{receipt.Binding.IndexTreeBefore, receipt.Binding.WorktreeTreeBefore,
			receipt.Binding.IndexTreeAfter, receipt.Binding.WorktreeTreeAfter} {
			if observed != receipt.Tree {
				return TestReceipt{}, fmt.Errorf("schema-2 test receipt binding does not equal the current candidate tree")
			}
		}
		indexTree, worktreeTree, postureErr := testingReceiptPosture(params.RepoRoot, *receipt.Testing)
		if postureErr != nil || indexTree != receipt.Tree || worktreeTree != receipt.Tree {
			return TestReceipt{}, fmt.Errorf("the whole-project index or working tree moved after the schema-2 test receipt")
		}
		return receipt, nil
	}
	if receipt.SchemaVersion != 1 || receipt.Tree != params.CandidateTree || receipt.Command == "" || receipt.ExitStatus != 0 {
		return TestReceipt{}, fmt.Errorf("test receipt does not record a successful command for the candidate tree")
	}
	if receipt.Proof != nil {
		attempt, proofErr := proofrun.ReadAttempt(params.RepoRoot, receipt.Proof.AttemptID)
		if proofErr != nil || attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess ||
			receipt.Proof.SchemaVersion != 1 || receipt.Proof.ControlRoot != params.RepoRoot ||
			receipt.Proof.GoalID != attempt.GoalID || receipt.Proof.AccountingRevision != attempt.AccountingRevision ||
			receipt.Proof.Deadline != attempt.Deadline || !bytes.Equal(bytes.TrimSpace(data), bytes.TrimSpace(proofrun.CommittedDeliveryReceipt(attempt))) {
			return TestReceipt{}, fmt.Errorf("test receipt proof binding does not equal a successful retained attempt payload")
		}
		if receipt.Command == CanonicalValidatorCommand {
			if receipt.Coverage == nil || attempt.PendingCoverage == nil || attempt.PendingCoverage.Evidence == nil {
				return TestReceipt{}, fmt.Errorf("canonical validator receipt has no complete coverage producer evidence")
			}
			recorded, _ := json.Marshal(attempt.PendingCoverage.Evidence)
			projected, _ := json.Marshal(receipt.Coverage)
			if !bytes.Equal(recorded, projected) {
				return TestReceipt{}, fmt.Errorf("canonical validator coverage differs from its retained producer evidence")
			}
			baseline := filepath.Join(params.RepoRoot, "scripts", "agents", "coverage-ratchet.json")
			if runtime.GOOS == "linux" {
				baseline = filepath.Join(params.RepoRoot, "scripts", "agents", "coverage-ratchet-linux.json")
			}
			_, found, reuseErr := proofrun.ReusableCoverageForAttempt(params.RepoRoot, params.RepoRoot, baseline,
				attempt.AttemptID, receipt.Coverage.PackageInventory)
			if reuseErr != nil || !found {
				return TestReceipt{}, fmt.Errorf("canonical validator coverage no longer matches current engine, toolchain, platform, policy, or floors")
			}
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.Time); err != nil {
		return TestReceipt{}, fmt.Errorf("test receipt time is malformed")
	}
	for _, observed := range []string{
		receipt.Binding.IndexTreeBefore,
		receipt.Binding.WorktreeTreeBefore,
		receipt.Binding.IndexTreeAfter,
		receipt.Binding.WorktreeTreeAfter,
	} {
		if observed != params.CandidateTree {
			return TestReceipt{}, fmt.Errorf("test receipt binding does not equal the candidate tree")
		}
	}
	indexTree, worktreeTree, err := receiptPosture(gittree.Workspace{Dir: params.RepoRoot})
	if err != nil || indexTree != params.CandidateTree || worktreeTree != params.CandidateTree {
		return TestReceipt{}, fmt.Errorf("the index or working tree moved after the test receipt was created")
	}
	return receipt, nil
}
