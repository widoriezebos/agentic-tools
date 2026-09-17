package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func init() {
	compiledBatchCapabilities[prefixReceipts] = struct{}{}
	compiledBatchCapabilities[landingTransportHelpers] = struct{}{}
	compiledBatchCapabilities[atomicSeriesAndRecovery] = struct{}{}
}

func executeBatchLanding(root, id, actor string, at time.Time) error {
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.Landing == nil || !record.Landing.PushComplete {
		if err := batch.ComposePrefixReceipts(store, id, actor, at, batch.PrefixReceiptSeams{Execute: func(goalID, tree string, groups []string) (batch.PrefixRunResult, error) {
			return executeBatchPrefixReceipt(root, id, record, goalID, tree, groups)
		}}); err != nil {
			return err
		}
		baseCommit, err := commitForTree(root, "origin/main", record.BaseTree)
		if err != nil {
			return err
		}
		seams := batch.LandSeams{
			Prepare: func(_ string) error { return batch.PrepareLandingBranch(root, id, baseCommit) },
			Apply:   func(unit batch.Unit) error { return batch.ApplyCertifiedPatch(root, root, unit.Chain) },
			AppendReceipt: func(unit batch.Unit, receipt batch.PrefixReceipt) error {
				return batch.RunCommand(batch.CommandSpec{Dir: root, Name: filepath.Join(root, "scripts", "receipt.sh"), Args: []string{"add", "--type", "implement", "--outcome", "shipped", "--goal", unit.GoalID, "--built-by", "coordinator", "--note", "batch " + id + " prefix " + receipt.Tree}})
			},
			Commit: func(unit batch.Unit, receipt batch.PrefixReceipt) (string, error) {
				path := receipt.ResultPath
				if path == "" {
					path = filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", id+".json")
				}
				message := fmt.Sprintf("land %s in batch %s\n\nOriginal join order; prefix tree %s.\n", unit.GoalID, id, receipt.Tree)
				if err := batch.CommitWithWrapper(root, unit.Chain, unit.GoalID, path, message, unit.AuthorName, unit.AuthorEmail, actor); err != nil {
					return "", err
				}
				return gitOutput(root, "rev-parse", "HEAD")
			},
			Held: func(_, tip string) error {
				return batchChildRunner(root, landingOwnerLineage, "landing", "held", "--root", root, "--base", baseCommit, "--commit", tip, "--remote", "origin", "--ref", "refs/heads/main")
			},
			PublishBranch: func(expected, tip string) error {
				return batch.PublishLandingBranch(root, id, expected, tip)
			},
			Push: func(_, tip string) error {
				return batch.LandLandingBranch(root, id, baseCommit, tip)
			},
			Reset: func(_ string) error {
				command := exec.Command("git", "-C", root, "reset", "--hard", baseCommit)
				command.Env = gittree.ScrubbedEnviron()
				return command.Run()
			},
			Cleanup: func() error {
				return batch.CleanupLandingBranch(root, id, gitHead(root))
			},
		}
		if err := batch.LandSeries(store, id, actor, at, seams); err != nil {
			return err
		}
	}
	return recoverBatchLanding(root, store, id, actor, at)
}

func gitHead(root string) string {
	tip, _ := gitOutput(root, "rev-parse", "HEAD")
	return tip
}

func executeBatchPrefixReceipt(root, id string, record batch.Record, goalID, tree string, groups []string) (batch.PrefixRunResult, error) {
	var unit batch.Unit
	for _, candidate := range record.Units {
		if candidate.GoalID == goalID {
			unit = candidate
			break
		}
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", id+"-prefix-"+goalID+".json")
	binary, err := os.Executable()
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	args := batchPrefixReceiptArgs(root, goalID, tree, resultPath, groups, unit.Claim)
	command := exec.Command(binary, args...)
	command.Dir, command.Env = root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, runErr := command.CombinedOutput()
	if runErr != nil && command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitAdmissionRefused {
		return batch.PrefixRunResult{}, &batch.PrefixBudgetRefusal{Reason: strings.TrimSpace(string(output))}
	}
	var result proofrun.TestResult
	if err := readStrictJSON(resultPath, &result); err != nil {
		return batch.PrefixRunResult{}, err
	}
	out := batch.PrefixRunResult{AttemptID: result.AttemptID, ResultPath: resultPath, Reused: map[string]string{}}
	for _, group := range result.Groups {
		if group.NativeLaunched {
			out.Executed = append(out.Executed, group.ID)
		}
		if group.ReuseAttempt != "" {
			out.Reused[group.ID] = group.ReuseAttempt
		}
		if group.Status != "passed" && group.Status != "reused" {
			out.Red = append(out.Red, batch.RedGroup{ID: group.ID, Status: group.Status, LogPath: group.LogPath, LogDigest: group.LogDigest, InputManifest: slices.Clone(group.InputManifest)})
		}
	}
	return out, runErr
}

func batchPrefixReceiptArgs(root, goalID, tree, resultPath string, groups []string, claim batch.Claim) []string {
	return []string{"test", "run", "--root", root, "--goal", goalID, "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--groups", strings.Join(groups, ","), "--result", resultPath,
		"--expected-goal-revision", fmt.Sprint(claim.Revision), "--expected-accounting-revision", fmt.Sprint(claim.AccountingRevision)}
}

func recoverBatchLanding(root string, store batch.Store, id, actor string, at time.Time) error {
	return batch.RecoverPushedSeries(store, id, actor, at, batch.RecoverySeams{
		OriginCommit: func(unit batch.Unit) (string, bool, error) {
			format := "%H%x00%B%x00"
			output, err := gitOutput(root, "log", "origin/main", "--format="+format)
			if err != nil {
				return "", false, err
			}
			parts := strings.Split(output, "\x00")
			for index := 0; index+1 < len(parts); index += 2 {
				if strings.Contains(parts[index+1], "Landing-Provenance: chain="+unit.Chain) {
					return strings.TrimSpace(parts[index]), true, nil
				}
			}
			return "", false, nil
		},
		Finalize: func(unit batch.Unit, commit string) error {
			next := "landed commit:" + commit + ":chain=" + unit.Chain
			return batchChildRunner(root, landingOwnerLineage, "goal", "edit", "--root", root, "--id", unit.GoalID, "--next", next, "--lineage", landingOwnerLineage)
		},
		Cleanup: func() error {
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			return batch.CleanupLandingBranch(root, id, record.Landing.PushedTip)
		},
	})
}

func commitForTree(root, ref, tree string) (string, error) {
	output, err := gitOutput(root, "log", "--first-parent", "--format=%H %T", ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == tree {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("BATCH_LAND_TRUNK_MOVED: no %s commit has base tree %s", ref, tree)
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
