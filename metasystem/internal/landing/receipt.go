package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	receiptBoundKey            = "landing.receipt-bound-min"
	receiptBoundDefaultMinutes = 40
)

// TestReceipt records the command result together with the four tree
// observations that bind its execution to one candidate.
type TestReceipt struct {
	SchemaVersion int                `json:"schemaVersion"`
	Tree          string             `json:"tree"`
	Command       string             `json:"command"`
	ExitStatus    int                `json:"exitStatus"`
	Time          string             `json:"time"`
	Binding       TestReceiptBinding `json:"binding"`
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

// CreateTestReceipt runs command only when the real index and working tree
// both represent tree, and publishes a receipt only if they still do after
// the command exits. The stale target is removed first so a refused retry
// cannot leave an older receipt available for observation.
func CreateTestReceipt(root, tree, command string, stdout, stderr io.Writer) (TestReceipt, error) {
	receipt := TestReceipt{}
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

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = root
	cmd.Env = gittree.ScrubbedEnviron()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := boundedexec.Run(cmd, receiptCommandBound(filepath.Join(root, "metasystem.conf")), "landing test command")
	exitStatus := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() < 0 {
			return receipt, runErr
		}
		exitStatus = exit.ExitCode()
	}

	indexAfter, worktreeAfter, err := receiptPosture(workspace)
	if err != nil {
		return receipt, err
	}
	if indexAfter != tree || worktreeAfter != tree {
		return receipt, fmt.Errorf("test receipt refused: the real index or working tree moved while the command ran")
	}

	receipt = TestReceipt{
		SchemaVersion: 1,
		Tree:          tree,
		Command:       command,
		ExitStatus:    exitStatus,
		Time:          time.Now().UTC().Format(time.RFC3339Nano),
		Binding: TestReceiptBinding{
			IndexTreeBefore: indexBefore, WorktreeTreeBefore: worktreeBefore,
			IndexTreeAfter: indexAfter, WorktreeTreeAfter: worktreeAfter,
		},
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return TestReceipt{}, err
	}
	if err := atomicfile.WriteVolatile(target, string(append(encoded, '\n'))); err != nil {
		return TestReceipt{}, fmt.Errorf("write test receipt: %w", err)
	}
	published = true
	return receipt, nil
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
	expected, err := filepath.Abs(TestReceiptPath(params.RepoRoot, params.CandidateTree))
	if err != nil {
		return TestReceipt{}, err
	}
	supplied := params.TestReceipt
	if !filepath.IsAbs(supplied) {
		supplied = filepath.Join(params.RepoRoot, supplied)
	}
	supplied, err = filepath.Abs(supplied)
	if err != nil || filepath.Clean(supplied) != filepath.Clean(expected) {
		return TestReceipt{}, fmt.Errorf("test receipt must be %s", expected)
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
	if receipt.SchemaVersion != 1 || receipt.Tree != params.CandidateTree || receipt.Command == "" || receipt.ExitStatus != 0 {
		return TestReceipt{}, fmt.Errorf("test receipt does not record a successful command for the candidate tree")
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
