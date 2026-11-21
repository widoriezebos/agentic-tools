package landing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// engineRequiredCode leads a refusal the engine re-arm resolves; a person's
// command shows it only with --verbose.
const engineRequiredCode = "TEST_POLICY_ENGINE_REQUIRED"

type registerSnapshot struct {
	path, fullPath           string
	working, suffix, pending []byte
}

const fastForwardRecoveryName = "metasystem/fast-forward-recovery.txt"

func FastForwardPreservingRegisters(ctx context.Context, root, tip string) error {
	workspace := gittree.Workspace{Dir: root}
	return fastForwardPreservingRegistersWith(ctx, root, tip, workspace, fastForwardGit, workspace.GitPath, appendOnlyRegisters, "")
}

// ForwardEngineCheckout also carries the accepted-risk register when the follow
// observed it dirty. An empty tip checks a checkout already at the deployment.
func ForwardEngineCheckout(ctx context.Context, root, tip, riskEntry string, gitPath func(string) (string, error)) error {
	registers := AppendOnlyRegisters()
	if tip == "" {
		registers = nil
	}
	if riskEntry != "" {
		registers = append(registers, counselor.AcceptedRiskRegisterPath)
	}
	return fastForwardPreservingRegistersWith(ctx, root, tip, gittree.Workspace{Dir: root}, fastForwardGit, gitPath, registers, riskEntry)
}

func fastForwardPreservingRegistersWith(ctx context.Context, root, tip string, workspace gittree.Workspace, rawGit func(context.Context, string, ...string) ([]byte, error), gitPath func(string) (string, error), registers []string, riskEntry string) error {
	recoveryPath, err := gitPath(fastForwardRecoveryName)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(recoveryPath); err == nil {
		return fmt.Errorf("fast-forward recovery file already exists at %s", recoveryPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect fast-forward recovery file %s: %w", recoveryPath, err)
	}
	if len(registers) == 0 {
		return nil
	}
	top, err := workspace.TopLevel()
	if err != nil {
		return err
	}
	prefix, err := workspace.Prefix()
	if err != nil {
		return err
	}
	head, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return fmt.Errorf("fast-forward requires a committed HEAD")
	}
	topWorkspace := gittree.Workspace{Dir: top, RawSource: workspace.RawSource}
	indexPath, err := workspace.GitPath("index")
	if err != nil {
		return err
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var dirty []registerSnapshot
	var restorePaths []string
	destination := tip
	if destination == "" && riskEntry != "" {
		destination = head
	}
	for _, register := range registers {
		path := prefix + register
		before, beforeOK, err := topWorkspace.FileAt(head, path)
		if err != nil {
			return err
		}
		landed, landedOK, err := topWorkspace.FileAt(destination, path)
		if err != nil {
			return err
		}
		staged, stagedOK, err := fastForwardIndexFileWithGit(ctx, top, path, rawGit)
		if err != nil {
			return err
		}
		fullPath := filepath.Join(root, filepath.FromSlash(register))
		working, err := os.ReadFile(fullPath)
		workingOK := err == nil
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		_, landedShape := appendDelta(before, beforeOK, landed, landedOK)
		stagedSuffix, stagedShape := appendDelta(before, beforeOK, staged, stagedOK)
		workingSuffix, workingShape := appendDelta(staged, stagedOK, working, workingOK)
		if register == counselor.AcceptedRiskRegisterPath && (!landedShape || !stagedShape || !workingShape || len(stagedSuffix)+len(workingSuffix) == 0) {
			return fmt.Errorf("the checkout has tracked changes: %s", riskEntry)
		}
		if !landedShape || !stagedShape || !workingShape {
			return fmt.Errorf("%s: the append-only record %s was rewritten, not only appended to", engineRequiredCode, register)
		}
		suffix := append(append([]byte(nil), stagedSuffix...), workingSuffix...)
		if len(suffix) != 0 {
			pending := suffix
			if register == counselor.AcceptedRiskRegisterPath {
				pending, err = pendingRiskRows(landed, suffix)
				if err != nil {
					return err
				}
			}
			dirty = append(dirty, registerSnapshot{path, fullPath, working, suffix, pending})
			restorePaths = append(restorePaths, path)
		}
	}
	if tip == "" && riskEntry != "" {
		return nil
	}
	if len(dirty) != 0 {
		if _, err := writeFastForwardRecovery(recoveryPath, recoveryText(dirty), ""); err != nil {
			return fmt.Errorf("write fast-forward recovery file %s: %w", recoveryPath, err)
		}
	}
	recoverRegisters := func() (recoverErr error) {
		for _, snapshot := range dirty {
			recoverErr = errors.Join(recoverErr, os.WriteFile(snapshot.fullPath, snapshot.working, 0o644))
		}
		return errors.Join(recoverErr, os.WriteFile(indexPath, indexBytes, 0o644))
	}
	if len(dirty) != 0 {
		if _, err := rawGit(ctx, top, append([]string{"restore", "--staged", "--worktree", "--"}, restorePaths...)...); err != nil {
			return fmt.Errorf("fast-forward failed; recovery file remains at %s: %w", recoveryPath, errors.Join(err, recoverRegisters()))
		}
	}
	if _, err := rawGit(ctx, top, "merge", "--ff-only", tip); err != nil {
		if len(dirty) == 0 {
			return err
		}
		return fmt.Errorf("fast-forward failed; recovery file remains at %s: %w", recoveryPath, errors.Join(err, recoverRegisters()))
	}
	for i, snapshot := range dirty {
		if len(snapshot.pending) == 0 {
			continue
		}
		if err := appendRegister(snapshot.fullPath, snapshot.pending); err != nil {
			return fmt.Errorf("re-append registers: %w; recovery file %s; registers not yet re-appended: %s", err, recoveryPath, strings.Join(restorePaths[i:], ", "))
		}
	}
	if len(dirty) != 0 {
		if err := os.Remove(recoveryPath); err != nil {
			return fmt.Errorf("remove completed fast-forward recovery file %s: %w", recoveryPath, err)
		}
	}
	return nil
}

// pendingRiskRows compares exact line bytes. Lines without a usable ID can
// still arrive verbatim, but cannot establish an identity conflict.
func pendingRiskRows(landed, local []byte) ([]byte, error) {
	rowID := func(line []byte) string {
		var row struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(line, &row) != nil {
			return ""
		}
		return strings.TrimSpace(row.ID)
	}
	rows := make(map[string]map[string]bool)
	for line := range bytes.Lines(landed) {
		id := rowID(line)
		if rows[id] == nil {
			rows[id] = make(map[string]bool)
		}
		rows[id][string(line)] = true
	}
	var pending []byte
	for line := range bytes.Lines(local) {
		id := rowID(line)
		versions := rows[id]
		identical := versions[string(line)]
		if id != "" && len(versions) > 0 && (!identical || len(versions) > 1) {
			return nil, fmt.Errorf("accepted risk %q differs between the checkout and the deployment", id)
		}
		if !identical {
			pending = append(pending, line...)
		}
	}
	return pending, nil
}

func recoveryText(dirty []registerSnapshot) string {
	output := "fast-forward recovery v1\n"
	for _, snapshot := range dirty {
		output += fmt.Sprintf("\nregister %q\nworking-length %d\nworking-sha256 %x\nsuffix-length %d\nsuffix-bytes %q\n", snapshot.path, len(snapshot.working), sha256.Sum256(snapshot.working), len(snapshot.suffix), snapshot.suffix)
	}
	return output
}

var writeFastForwardRecovery = atomicfile.WriteText
var appendRegister = func(path string, suffix []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(suffix)
	return errors.Join(writeErr, file.Close())
}

func appendDelta(before []byte, beforeOK bool, after []byte, afterOK bool) ([]byte, bool) {
	if !beforeOK || !afterOK {
		return nil, beforeOK == afterOK
	}
	if !bytes.HasPrefix(after, before) ||
		(len(before) != 0 && before[len(before)-1] != '\n') || (len(after) != 0 && after[len(after)-1] != '\n') {
		return nil, false
	}
	return after[len(before):], true
}
func fastForwardIndexFileWithGit(ctx context.Context, root, path string, rawGit func(context.Context, string, ...string) ([]byte, error)) ([]byte, bool, error) {
	content, err := rawGit(ctx, root, "show", ":"+path)
	if err != nil && ctx.Err() != nil {
		return nil, false, err
	}
	return content, err == nil, nil
}
func fastForwardGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
