package landing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const fullBatteryCommand = "scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh"

func observeTierOne(params ObserveParams, change string) Observation {
	provenance := "direct-fix class=tier-1 change=" + change
	if !landingID.MatchString(params.RootJob) || !landingID.MatchString(params.Goal) {
		return refuse("tier1-declaration-refused", provenance)
	}
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return refuse("tier1-policy-unreadable", provenance)
	}
	classes, err := loadPathClasses(workspace, baseTree, true)
	if err != nil {
		return refuse("tier1-policy-unreadable", provenance)
	}
	if err := heldGoalError(workspace, baseTree, params.Goal, params.Actor); err != nil {
		return refuse(carriageRefusalCode(err), provenance)
	}
	gateWidth, err := tierOneRootGateWidth(params)
	if err != nil {
		return refuse("tier1-root-refused", provenance)
	}
	provenance = fmt.Sprintf("direct-fix class=tier-1 root-job=%s goal=%s change=%s", params.RootJob, params.Goal, change)
	paths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
	if err != nil {
		return refuse("candidate-tree-unreadable", provenance)
	}
	for _, changedPath := range paths {
		if classes.TierOneRefused(changedPath) {
			return refuse("tier1-floor-refused", provenance)
		}
	}
	resolved, err := resolvePathClasses(workspace, classes, paths)
	if err != nil {
		return refuse("tier1-policy-unreadable", provenance)
	}
	if err := chainClassError(resolved, paths); err != nil {
		return refuse(carriageRefusalCode(err), provenance)
	}
	metric, err := tierOneDiffMetric(params.RepoRoot, workspace, baseTree, params.CandidateTree, paths)
	if err != nil {
		return refuse("tier1-diff-shape-refused", provenance)
	}
	if len(paths) > 3 {
		return refuse("tier1-file-bound-refused", provenance)
	}
	if metric > 40 {
		return refuse("tier1-line-bound-refused", provenance)
	}
	receipt, err := readTestReceipt(params)
	if err != nil {
		return refuse("tier1-receipt-refused", provenance)
	}
	if gateWidth == "full" && receipt.Command != fullBatteryCommand {
		return refuse("tier1-full-gate-refused", provenance)
	}
	return pass(BarDirectFix, "tier-1", provenance)
}

func tierOneRootGateWidth(params ObserveParams) (string, error) {
	if !landingID.MatchString(params.RootJob) {
		return "", fmt.Errorf("root job id is malformed")
	}
	data, err := os.ReadFile(filepath.Join(params.RepoRoot, "artifacts", "agents", "jobs", params.RootJob+".json"))
	if err != nil {
		return "", err
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil || record["jobId"] != params.RootJob || record["parentJob"] != nil ||
		record["role"] != "implementer" || record["goalId"] != params.Goal {
		return "", fmt.Errorf("root job does not identify the goal's root implementer")
	}
	tier, ok := jsonInteger(record["goalTier"])
	if !ok || tier != 1 {
		return "", fmt.Errorf("root job is not tier 1")
	}
	width, present := record["gateWidth"]
	if !present {
		return "area", nil
	}
	text, ok := width.(string)
	if !ok || (text != "area" && text != "full") {
		return "", fmt.Errorf("root job gateWidth must be area or full")
	}
	return text, nil
}

func tierOneDiffMetric(root string, workspace gittree.Workspace, baseTree, candidateTree string, paths []string) (int, error) {
	shape, err := landingGit(root, "diff-tree", "-r", "--no-commit-id", "-M", "-C", "--name-status", baseTree, candidateTree)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(shape), "\n"), "\n") {
		status, _, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			return 0, fmt.Errorf("rename or copy shape")
		}
	}

	numstat, err := landingGit(root, "diff", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", baseTree, candidateTree, "--")
	if err != nil {
		return 0, err
	}
	type counts struct{ added, deleted int }
	byPath := make(map[string]counts, len(paths))
	total := 0
	for _, record := range bytes.Split(numstat, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.SplitN(record, []byte{'\t'}, 3)
		if len(fields) != 3 || len(fields[2]) == 0 || bytes.Equal(fields[0], []byte{'-'}) || bytes.Equal(fields[1], []byte{'-'}) {
			return 0, fmt.Errorf("binary or malformed numstat shape")
		}
		added, addErr := strconv.Atoi(string(fields[0]))
		deleted, deleteErr := strconv.Atoi(string(fields[1]))
		if addErr != nil || deleteErr != nil || added < 0 || deleted < 0 {
			return 0, fmt.Errorf("malformed numstat count")
		}
		byPath[string(fields[2])] = counts{added: added, deleted: deleted}
		total += added + deleted
	}

	before, err := workspace.Entries(baseTree, paths)
	if err != nil {
		return 0, err
	}
	after, err := workspace.Entries(candidateTree, paths)
	if err != nil {
		return 0, err
	}
	for _, changedPath := range paths {
		oldEntry, existed := before[changedPath]
		newEntry, present := after[changedPath]
		stat := byPath[changedPath]
		if existed && present && oldEntry.Mode != newEntry.Mode && stat.added+stat.deleted == 0 {
			return 0, fmt.Errorf("mode-only shape")
		}
	}
	return total, nil
}

func landingGit(root string, args ...string) ([]byte, error) {
	full := append([]string{"-C", root, "-c", "core.fileMode=true", "-c", "core.useReplaceRefs=false"}, args...)
	command := exec.Command("git", full...)
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := boundedexec.Run(command, boundedexec.Timeout(filepath.Join(root, "metasystem.conf"), boundedexec.Local), "landing git diff"); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
