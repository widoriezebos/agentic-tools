package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type HeldRefusal struct {
	Code   string `json:"code"`
	Commit string `json:"commit"`
	Detail string `json:"detail"`
}

type HeldVerdict struct {
	Outcome  string       `json:"outcome"`
	Base     string       `json:"base"`
	Commit   string       `json:"commit"`
	Commits  int          `json:"commits"`
	Refusal  *HeldRefusal `json:"refusal,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
	ExitCode int          `json:"exitCode"`
}

type heldCommit struct {
	commit string
	parent string
}

type heldTrailers struct {
	machine    []string
	goalItem   []string
	revision   []string
	provenance []string
}

// Held rechecks every commit in the first-parent range against the goal
// ledger in that commit's parent. Policy outcomes are verdicts; only Git
// plumbing failures needed to open or resolve the repository are errors.
func Held(root, base, commit, remote, ref string) (HeldVerdict, error) {
	resolvedBase, err := resolveHeldCommit(root, base)
	if err != nil {
		return HeldVerdict{}, err
	}
	resolvedCommit, err := resolveHeldCommit(root, commit)
	if err != nil {
		return HeldVerdict{}, err
	}
	verdict := HeldVerdict{Outcome: "ok", Base: resolvedBase, Commit: resolvedCommit}
	if resolvedBase == resolvedCommit {
		verdict.Outcome = "nothing-to-push"
		return verdict, nil
	}

	rangeCommits, refusal, err := heldRange(root, resolvedBase, resolvedCommit)
	if err != nil {
		return HeldVerdict{}, err
	}
	verdict.Commits = len(rangeCommits)
	if refusal != nil {
		verdict.Outcome, verdict.Refusal, verdict.ExitCode = "refused", refusal, 2
		return verdict, nil
	}

	workspace := gittree.Workspace{Dir: root}
commitLoop:
	for _, entry := range rangeCommits {
		verdict.Outcome = "ok"
		object, messageErr := landingGit(root, "cat-file", "commit", entry.commit)
		if messageErr != nil {
			verdict.Outcome = "unreadable"
			verdict.ExitCode = 2
			verdict.Warnings = append(verdict.Warnings, "held: unreadable: "+entry.commit)
			return verdict, nil
		}
		_, message, foundMessage := strings.Cut(string(object), "\n\n")
		if !foundMessage {
			verdict.Outcome = "unreadable"
			verdict.ExitCode = 2
			verdict.Warnings = append(verdict.Warnings, "held: unreadable: "+entry.commit)
			return verdict, nil
		}
		trailers := parseHeldTrailers(message)
		if len(trailers.machine) != 1 {
			return heldHardRefusal(verdict, entry.commit, "machine-trailer-malformed", fmt.Sprintf("the commit carries %d Machine trailers", len(trailers.machine))), nil
		}
		if len(trailers.goalItem) > 1 {
			return heldHardRefusal(verdict, entry.commit, "machine-trailer-malformed", fmt.Sprintf("the commit carries %d Goal-Item trailers", len(trailers.goalItem))), nil
		}
		if len(trailers.revision) > 1 {
			return heldHardRefusal(verdict, entry.commit, "machine-trailer-malformed", fmt.Sprintf("the commit carries %d Goal-Revision trailers", len(trailers.revision))), nil
		}

		actor := trailers.machine[0]
		human := heldHumanActor(actor)
		softened := false
		refuse := func(code, detail string) bool {
			if human {
				verdict.Warnings = append(verdict.Warnings, fmt.Sprintf("held: warning %s: %s: %s", code, entry.commit, detail))
				softened = true
				return false
			}
			verdict.Outcome = "refused"
			verdict.Refusal = &HeldRefusal{Code: code, Commit: entry.commit, Detail: detail}
			verdict.ExitCode = 1
			return true
		}

		endpoint, endpointErr := goal.ResolveEndpoint(root)
		if endpointErr != nil {
			return HeldVerdict{}, endpointErr
		}
		if endpoint.LocalMode() || remote != endpoint.Remote || ref != endpoint.Branch {
			ledgerBranch := endpoint.Branch
			if endpoint.LocalMode() {
				ledgerBranch = goal.LocalLedgerBranch
			}
			detail := fmt.Sprintf("this landing pushes %s %s; the goal ledger is %s %s", remote, ref, endpoint.Remote, ledgerBranch)
			if refuse("endpoint-mismatch", detail) {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
		}

		goalID := ""
		if len(trailers.goalItem) == 1 {
			goalID = trailers.goalItem[0]
		}
		if chainID := heldChainID(trailers.provenance); chainID != "" {
			if boundGoal, readable := heldChainGoal(root, chainID); readable {
				if goalID == "" {
					if refuse("goal-binding-missing", fmt.Sprintf("chain %s was dispatched under goal %s; the commit names no Goal-Item", chainID, boundGoal)) {
						return verdict, nil
					}
					if softened {
						continue commitLoop
					}
				} else if goalID != boundGoal {
					if refuse("goal-binding-mismatch", fmt.Sprintf("chain %s was dispatched under goal %s, not %s", chainID, boundGoal, goalID)) {
						return verdict, nil
					}
					if softened {
						continue commitLoop
					}
				}
			}
		}

		parentTree, treeErr := workspace.TreeOf(entry.parent)
		if treeErr != nil {
			verdict.Outcome = "unreadable"
			verdict.ExitCode = 2
			verdict.Warnings = append(verdict.Warnings, "held: unreadable: "+entry.commit)
			return verdict, nil
		}
		if goalID == "" {
			if goalFreeAt(workspace, parentTree) {
				verdict.Outcome = "goal-free"
				continue commitLoop
			}
			if refuse("goal-binding-missing", "agent landings are goal work: name the held goal with --goal, or the ledger must be declared Goal-free") {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
			continue
		}
		if len(trailers.revision) == 0 {
			if refuse("goal-revision-unbound", fmt.Sprintf("goal item %s has no Goal-Revision", goalID)) {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
			continue
		}
		want, parseErr := strconv.ParseUint(trailers.revision[0], 10, 64)
		if parseErr != nil || want == 0 {
			if refuse("goal-revision-unbound", fmt.Sprintf("goal item %s has no positive Goal-Revision", goalID)) {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
			continue
		}

		file, state := heldGoalAtParent(workspace, parentTree, goalID)
		parentShort := shortHeldCommit(entry.parent)
		if file == nil || file.State != goal.StateClaimed || file.Claimed == nil || (!human && actor != file.Claimed.Machine+"+"+file.Claimed.Lineage) {
			if refuse("goal-item-not-held", fmt.Sprintf("goal %s is %s at %s", goalID, state, parentShort)) {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
			continue
		}
		if file.Claimed.Revision != want {
			if refuse("goal-revision-moved", fmt.Sprintf("goal %s is claimed at revision %d; commit binds revision %d", goalID, file.Claimed.Revision, want)) {
				return verdict, nil
			}
			if softened {
				continue commitLoop
			}
		}
	}
	return verdict, nil
}

func resolveHeldCommit(root, value string) (string, error) {
	out, err := landingGit(root, "rev-parse", "--verify", value+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func heldRange(root, base, tip string) ([]heldCommit, *HeldRefusal, error) {
	// The ancestry path keeps a moved base from turning the proposed push
	// range into an inspection of the tip's entire first-parent history.
	out, err := landingGit(root, "rev-list", "--first-parent", "--ancestry-path", "--parents", "--reverse", base+".."+tip)
	if err != nil {
		return nil, nil, err
	}
	var ordered []heldCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) > 2 {
			return ordered, &HeldRefusal{Code: "range-not-linear", Commit: fields[0],
				Detail: fmt.Sprintf("commit %s is a merge; a landing pushes a linear range above %s", fields[0], base)}, nil
		}
		if len(fields) < 2 {
			return ordered, &HeldRefusal{Code: "range-not-linear", Commit: tip,
				Detail: fmt.Sprintf("%s is not a first-parent ancestor of %s", base, tip)}, nil
		}
		ordered = append(ordered, heldCommit{commit: fields[0], parent: fields[1]})
	}
	previous := base
	for _, entry := range ordered {
		if entry.parent != previous {
			return ordered, &HeldRefusal{Code: "range-not-linear", Commit: tip,
				Detail: fmt.Sprintf("%s is not a first-parent ancestor of %s", base, tip)}, nil
		}
		previous = entry.commit
	}
	if previous != tip {
		return ordered, &HeldRefusal{Code: "range-not-linear", Commit: tip,
			Detail: fmt.Sprintf("%s is not a first-parent ancestor of %s", base, tip)}, nil
	}
	return ordered, nil, nil
}

func parseHeldTrailers(message string) heldTrailers {
	var result heldTrailers
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "Machine:"):
			result.machine = append(result.machine, strings.TrimSpace(strings.TrimPrefix(line, "Machine:")))
		case strings.HasPrefix(line, "Goal-Item:"):
			result.goalItem = append(result.goalItem, strings.TrimSpace(strings.TrimPrefix(line, "Goal-Item:")))
		case strings.HasPrefix(line, "Goal-Revision:"):
			result.revision = append(result.revision, strings.TrimSpace(strings.TrimPrefix(line, "Goal-Revision:")))
		case strings.HasPrefix(line, "Landing-Provenance:"):
			result.provenance = append(result.provenance, strings.TrimSpace(strings.TrimPrefix(line, "Landing-Provenance:")))
		}
	}
	return result
}

func heldHumanActor(actor string) bool {
	_, lineage, found := strings.Cut(actor, "+")
	return found && lineage == "human"
}

func heldChainID(provenance []string) string {
	for _, value := range provenance {
		for _, field := range strings.Fields(value) {
			if id, found := strings.CutPrefix(field, "chain="); found && landingID.MatchString(id) {
				return id
			}
		}
	}
	return ""
}

func heldChainGoal(root, chain string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", chain+".json"))
	if err != nil {
		return "", false
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil {
		return "", false
	}
	goalID, ok := record["goalId"].(string)
	return goalID, ok && goalID != ""
}

func heldGoalAtParent(workspace gittree.Workspace, tree, id string) (*goal.GoalFile, string) {
	for _, path := range []string{"plans/goals/" + id + ".md", "records/goals/" + id + ".md"} {
		data, present, err := workspace.FileAt(tree, path)
		if err != nil || !present {
			continue
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 || file.Id != id {
			return nil, "absent"
		}
		return file, file.State
	}
	return nil, "absent"
}

func heldHardRefusal(verdict HeldVerdict, commit, code, detail string) HeldVerdict {
	verdict.Outcome = "refused"
	verdict.Refusal = &HeldRefusal{Code: code, Commit: commit, Detail: detail}
	verdict.ExitCode = 1
	return verdict
}

func shortHeldCommit(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
