package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// Manual submission: work a person or agent wrote is committed as the goal's
// named work on its goal branch through the branch commit owner, published
// through the push owner and examined by the same committed review as built
// work. The goal branch range is the only record of manual work: a work item
// is its Goal-Unit commit, a version is a commit, and a replay is decided by
// comparing trees, never by a registry of submissions.

// manualCapture is the frozen candidate of one submission.
type manualCapture struct {
	source   string   // the caller's checkout top level
	head     string   // the source HEAD the changes are against
	patch    []byte   // the exact binary patch
	paths    []string // the captured paths (implicit changes only)
	omitted  []string // system records and the named brief left in the source
	explicit bool     // a supplied patch
}

func runIntentReviewManual(inv *intentInvocation, id string) int {
	result := inv.submitManualWork(id)
	return inv.render(result)
}

func (inv *intentInvocation) submitManualWork(id string) intentResult {
	targets := inv.targets(id)
	refuse := func(code int, format string, args ...any) intentResult {
		return intentResult{Targets: targets, Outcome: intentRefused, code: code, Summary: fmt.Sprintf(format, args...)}
	}
	if inv.input.has("changes") == inv.input.has("patch") {
		return refuse(2, "manual submission takes exactly one of --changes (this checkout's current changes) and --patch PATCH; nothing was done")
	}
	if !inv.input.has("brief") {
		return refuse(2, "manual submission needs --brief FILE: what the work is meant to do, frozen for its review; nothing was done")
	}
	for _, other := range []string{"retry", "finding", "test", "model", "tool-calls"} {
		if inv.input.has(other) {
			return refuse(2, "manual submission takes --changes or --patch, --brief, --work, --after and --dispositions, not --%s; nothing was done", other)
		}
	}
	if problem := inv.selectRoot(); problem != nil {
		return *problem
	}
	// The goal is resolved on the accepted ledger before anything is
	// written under a path derived from it.
	projection, _, problem := inv.projection()
	if problem != nil {
		return *problem
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return refuse(1, "no goal %s on the accepted ledger; nothing was done", id)
	}
	if where != "live" {
		return refuse(1, "goal %s is %s; nothing was submitted", id, where)
	}
	briefPath := inv.flagPath("brief")
	brief, err := os.ReadFile(briefPath)
	if err != nil || len(bytes.TrimSpace(brief)) == 0 {
		return refuse(2, "the brief %s cannot be read or is empty; nothing was done", briefPath)
	}
	capture, err := inv.captureManual(briefPath)
	if err != nil {
		return refuse(2, "%v; nothing was done", err)
	}
	// Custody of the inputs: the patch and brief are frozen by content
	// before any effect; a repeat of the same inputs reaches the same files.
	identity := sha256.Sum256(append(append(append([]byte{}, capture.patch...), 0), brief...))
	digest := hex.EncodeToString(identity[:])[:16]
	frozen := filepath.Join(inv.layout.InstallationRoot, "artifacts", "agents", "intent-manual", file.Id, digest)
	frozenPatch, frozenBrief := filepath.Join(frozen, "change.patch"), filepath.Join(frozen, "brief.md")
	for path, content := range map[string][]byte{frozenPatch: capture.patch, frozenBrief: brief} {
		if existing, readErr := os.ReadFile(path); readErr == nil {
			if !bytes.Equal(existing, content) {
				return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("the frozen input %s does not match its identity; nothing was done", path)}
			}
			continue
		}
		if err := os.MkdirAll(frozen, 0o755); err != nil {
			return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
		if _, err := atomicfile.WriteText(path, string(content), inv.layout.InstallationRoot); err != nil {
			return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
	}
	data := map[string]any{"goal": id, "source": capture.source, "sourceHead": capture.head, "patch": frozenPatch, "brief": frozenBrief}
	if len(capture.omitted) > 0 {
		data["leftInSource"] = capture.omitted
	}
	if !capture.explicit {
		data["paths"] = capture.paths
	}

	if file.State != goal.StateClaimed {
		if claimed := inv.acquireClaim(id); claimed.Outcome != intentConfirmed {
			claimed.Summary = fmt.Sprintf("submitting work claims goal %s first, and the claim was not granted: %s; nothing was submitted", id, strings.TrimSpace(claimed.Summary))
			return claimed
		}
	}
	conn := inv.connection()
	original := inv.layout.InstallationRoot
	endpoint, err := conn.endpoint(original)
	if err != nil {
		return refuse(1, "the goal branch endpoint is unavailable: %v; nothing was submitted", err)
	}
	check := conn.claimCheck(original, id, endpoint)
	if err := branch.CheckCommitAccess(id, check); err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error() + "; nothing was submitted",
			Decision: "the session holding the goal submits its work, or a person takes the goal over: metasystem claim " + id + " --take-over --reason TEXT"}
	}
	worktree, problem := inv.prepareGoalWorktree(id)
	if problem != nil {
		return *problem
	}
	install := inv.goalWorktreeInstallation(worktree)
	data["worktree"] = worktree
	base, err := conn.endpointTip(original, endpoint)
	if err != nil {
		return refuse(1, "cannot resolve the landing endpoint's tip: %v; nothing was submitted", err)
	}
	git := inv.work().git
	line := func(dir string, args ...string) (string, error) {
		out, err := git(dir, args...)
		return strings.TrimSpace(string(out)), err
	}
	same := sameDirectory(capture.source, worktree)
	data["sameCheckout"] = same

	var commit, work, action string
	var failure *intentResult
	sectionErr := conn.section(install, func(withToken func(func() error) error) error {
		// Inside the checkout mutation section: inspect the range, decide
		// the replay, stage and commit. Nothing here pushes or waits.
		tip, err := line(worktree, "rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			return err
		}
		commits, err := branch.ValidateRange(install, base, tip, id)
		if err != nil {
			return err
		}
		work, failure = inv.manualWorkName(id, commits)
		if failure != nil {
			return nil
		}
		data["work"] = work
		current := manualUnitCommit(commits, work)
		after := ""
		if inv.input.has("after") {
			resolved, err := line(worktree, "rev-parse", "--verify", "--quiet", inv.input.text("after")+"^{commit}")
			if err != nil || resolved == "" {
				failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
					Summary: fmt.Sprintf("--after %s names no commit this checkout has; nothing was submitted", inv.input.text("after")),
					next:    inv.publicArgv("status", "goal", id, "--work", work), nextReason: "the work's current version"}
				return nil
			}
			info, err := branch.KindOf(install, resolved, id)
			if err != nil || info.Kind != branch.Unit || !slices.Equal(info.Units, []string{work}) {
				failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
					Summary: fmt.Sprintf("--after %s is not a version of work %s of goal %s; nothing was submitted", shortSHA(resolved), work, id)}
				return nil
			}
			after = resolved
		}
		switch {
		case len(capture.patch) == 0 && current != "":
			commit, action = current, "rejoined"
			return nil
		case len(capture.patch) == 0:
			failure = &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
				Summary: fmt.Sprintf("there are no changes to submit as work %s of goal %s; nothing was submitted", work, id)}
			return nil
		case after == "" && current != "":
			parent, err := line(install, "rev-parse", current+"^")
			if err != nil {
				return err
			}
			if matches, err := manualTreeMatches(install, parent, current, capture.patch); err != nil {
				return err
			} else if matches {
				commit, action = current, "rejoined"
				return nil
			}
			failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary:  fmt.Sprintf("work %s of goal %s already has version %s, and this change is not it; nothing was submitted", work, id, shortSHA(current)),
				Decision: "a correction of that version names it: " + shellCommand(inv.manualArgv(id, work, current))}
			return nil
		case after != "" && after != current:
			if current != "" {
				if matches, err := manualTreeMatches(install, after, current, capture.patch); err != nil {
					return err
				} else if matches {
					commit, action = current, "rejoined"
					return nil
				}
			}
			failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary: fmt.Sprintf("version %s of work %s is no longer current (current: %s), and the current version is not this correction of it; nothing was submitted",
					shortSHA(after), work, cmpOr(shortSHA(current), "none")),
				next: inv.publicArgv("status", "goal", id, "--work", work), nextReason: "the work's current version"}
			return nil
		}
		staged, stageErr := stageManual(git, worktree, same, capture)
		if stageErr != nil {
			failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: stageErr.Error() + "; nothing was committed"}
			return nil
		}
		operation := "manual-" + digest
		var installed string
		commitErr := withToken(func() error {
			var err error
			installed, err = conn.commit(branch.CommitRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: id,
				Units: []string{work}, OpID: operation + "-unit", Kind: branch.Unit, Amend: after != "", CheckClaim: check, Transport: conn.transport})
			return err
		})
		// A lost commit response is resolved from the actual range: the
		// unit is adopted only when its tree is this change's result.
		tipAfter, _ := line(worktree, "rev-parse", "--verify", "HEAD^{commit}")
		if tipAfter != "" && tipAfter != tip {
			if commits, err := branch.ValidateRange(install, base, tipAfter, id); err == nil {
				if found := manualUnitCommit(commits, work); found != "" && found != current {
					from := after
					if from == "" {
						from, _ = line(install, "rev-parse", found+"^")
					}
					if ok, _ := manualTreeMatches(install, from, found, capture.patch); ok {
						commit, action = found, "committed"
						if installed == "" {
							action = "adopted"
						}
						return nil
					}
				}
			}
		}
		if commitErr == nil {
			commitErr = fmt.Errorf("the commit owner installed %s, which holds no version of work %s made from this change", shortSHA(installed), work)
		}
		if tipAfter != tip {
			// The goal worktree is not where this submission staged it, and
			// what moved it is not proved to be this change: nothing is
			// rolled back on a guess.
			failure = &intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
				Summary: fmt.Sprintf("the branch commit owner did not commit work %s: %v; the goal worktree is now at %s, which does not hold this change, so its staging was left exactly as it is",
					work, commitErr, cmpOr(shortSHA(tipAfter), "an unreadable HEAD")),
				next: inv.publicArgv("status", "goal", id, "--work", work), nextReason: "the work's current version on the goal branch"}
			return nil
		}
		if undoErr := staged.undo(); undoErr != nil {
			failure = &intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
				Summary: fmt.Sprintf("the branch commit owner did not commit work %s: %v; restoring the goal worktree's staging from before this submission failed: %v", work, commitErr, undoErr)}
			return nil
		}
		failure = &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("the branch commit owner did not commit work %s: %v; the goal worktree's staging is as it was before this submission", work, commitErr)}
		return nil
	})
	if sectionErr != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: sectionErr.Error() + "; nothing was submitted"}
	}
	if failure != nil {
		if failure.Data == nil {
			failure.Data = data
		}
		return *failure
	}
	data["commit"], data["action"] = commit, action
	targets = append(targets, intentTarget{Kind: "work", ID: work}, intentTarget{Kind: "commit", ID: commit})
	// Publication happens after the section: the push owner's journal
	// reconciles a lost response under the same operation.
	pushed, err := conn.push(branch.PushRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: id,
		OpID: "manual-" + digest + "-push", CheckClaim: check, Transport: conn.transport})
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
			Summary: fmt.Sprintf("work %s is committed as %s but not published: %v", work, shortSHA(commit), err),
			next:    inv.sameCommand(), nextReason: "the same command publishes this commit; it never makes another"}
	}
	data["published"] = pushed.Tip
	args := []string{"--root", install, "--goal", id, "--unit", commit, "--brief", frozenBrief}
	if install != original {
		args = append(args, "--selected-installation", original)
	}
	result := inv.commitReview(targets, install, id, commit, args)
	if result.Outcome == intentRefused && strings.Contains(result.Summary, "different brief") {
		// The read owner binds a version's read to the brief it was first
		// read with; a new brief is a new version, never the same read.
		result.Decision = fmt.Sprintf("version %s of work %s is read against the brief it was first submitted with: repeat with that brief, or correct the work so the new version is read against this brief: %s",
			shortSHA(commit), work, shellCommand(inv.manualArgv(id, work, commit)))
	}
	if merged, ok := result.Data.(map[string]any); ok {
		for key, value := range data {
			if _, taken := merged[key]; !taken {
				merged[key] = value
			}
		}
	} else if result.Data == nil {
		result.Data = data
	}
	if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
		result.next, result.nextReason = inv.publicArgv("land", id), "the work's read is published on the goal branch; landing admits it by its own rules"
	}
	return result
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// manualArgv is the correction command of one version of the work.
func (inv *intentInvocation) manualArgv(id, work, after string) []string {
	words := reviewGoalWords(id)
	if inv.input.has("patch") {
		words = append(words, "--patch", inv.flagPath("patch"))
	} else {
		words = append(words, "--changes")
	}
	return inv.publicArgv(append(words, "--brief", inv.flagPath("brief"), "--work", work, "--after", after)...)
}

// manualWorkName is the named work: --work, else the one work item the
// goal branch holds, else main for a goal with no work.
func (inv *intentInvocation) manualWorkName(id string, commits []branch.Commit) (string, *intentResult) {
	if inv.input.has("work") {
		name := inv.input.text("work")
		if !validManualWorkName(name) {
			return "", &intentResult{Targets: inv.targets(id), Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%q is not a work name; nothing was submitted", name)}
		}
		return name, nil
	}
	var names []string
	for _, commit := range commits {
		if commit.Kind == branch.Unit && len(commit.Units) == 1 && !slices.Contains(names, commit.Units[0]) {
			names = append(names, commit.Units[0])
		}
	}
	switch len(names) {
	case 0:
		return "main", nil
	case 1:
		return names[0], nil
	}
	lines := []string{}
	for _, name := range names {
		lines = append(lines, "  "+shellCommand(append(inv.sameCommand(), "--work", name)))
	}
	return "", &intentResult{Targets: inv.targets(id), Outcome: intentRefused, code: 2, text: lines, Data: map[string]any{"candidates": names},
		Summary:  fmt.Sprintf("goal %s has %d work items (%s); nothing was submitted", id, len(names), strings.Join(names, ", ")),
		Decision: "name one with --work NAME, or a new name for new work"}
}

func validManualWorkName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/+ \t\n") && !strings.HasPrefix(name, "-")
}

func manualUnitCommit(commits []branch.Commit, work string) string {
	for _, commit := range commits {
		if commit.Kind == branch.Unit && slices.Equal(commit.Units, []string{work}) {
			return commit.ID
		}
	}
	return ""
}

// manualTreeMatches reports whether the commit owner's own application of
// patch onto from yields exactly unit's tree.
func manualTreeMatches(repo, from, unit string, patch []byte) (bool, error) {
	tree, err := branch.ScratchApplyTree(repo, from, patch)
	if err != nil {
		var refusal *branch.OpError
		if errors.As(err, &refusal) {
			return false, nil
		}
		return false, err
	}
	out, err := exec.Command("git", "-C", repo, "rev-parse", unit+"^{tree}").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == tree, nil
}

func sameDirectory(a, b string) bool {
	left, errA := filepath.EvalSymlinks(a)
	right, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && filepath.Clean(left) == filepath.Clean(right)
}

// captureManual freezes the candidate. A supplied patch is taken exactly.
// Current changes are the checkout's tracked and untracked, unignored
// changes against its HEAD, selected in a private index so the caller's own
// index is never touched; paths the goal branch keeps out of any unit
// (system records) and the named brief itself stay in the source and out of
// the candidate.
func (inv *intentInvocation) captureManual(briefPath string) (manualCapture, error) {
	patchPath := ""
	if inv.input.has("patch") {
		patchPath = inv.flagPath("patch")
	}
	return captureManualAt(inv.cwd, patchPath, briefPath)
}

// captureManualAt captures from the caller's location cwd: its checkout's
// top level, whatever directory inside it the caller stands in.
func captureManualAt(cwd, patchPath, briefPath string) (manualCapture, error) {
	topOut, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return manualCapture{}, fmt.Errorf("%s is not inside a Git checkout", cwd)
	}
	top := strings.TrimSpace(string(topOut))
	headOut, err := exec.Command("git", "-C", top, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil {
		return manualCapture{}, fmt.Errorf("checkout %s has no HEAD commit", top)
	}
	capture := manualCapture{source: top, head: strings.TrimSpace(string(headOut))}
	if patchPath != "" {
		patch, err := os.ReadFile(patchPath)
		if err != nil {
			return manualCapture{}, fmt.Errorf("the patch cannot be read: %v", err)
		}
		capture.patch, capture.explicit = patch, true
		return capture, nil
	}
	return captureCheckoutChanges(top, capture.head, briefPath)
}

func captureCheckoutChanges(top, head, briefPath string) (manualCapture, error) {
	capture := manualCapture{source: top, head: head}
	scratch, err := os.MkdirTemp("", "metasystem-manual-capture-")
	if err != nil {
		return manualCapture{}, err
	}
	defer os.RemoveAll(scratch)
	index := filepath.Join(scratch, "index")
	private := func(args ...string) ([]byte, error) {
		command := exec.Command("git", append([]string{"-C", top, "--literal-pathspecs"}, args...)...)
		command.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
	if _, err := private("read-tree", head); err != nil {
		return manualCapture{}, err
	}
	if _, err := private("add", "-A", "--", "."); err != nil {
		return manualCapture{}, err
	}
	changed, err := private("diff", "--cached", "--name-only", "-z", "--no-renames", head)
	if err != nil {
		return manualCapture{}, err
	}
	briefRel := ""
	if resolvedTop, err := filepath.EvalSymlinks(top); err == nil {
		if resolvedBrief, err := filepath.EvalSymlinks(briefPath); err == nil {
			if rel, err := filepath.Rel(resolvedTop, resolvedBrief); err == nil && !strings.HasPrefix(rel, "..") {
				briefRel = filepath.ToSlash(rel)
			}
		}
	}
	for _, path := range splitNUL(changed) {
		if path == briefRel || branch.PathClass(path) == branch.ClassExcluded {
			capture.omitted = append(capture.omitted, path)
			continue
		}
		capture.paths = append(capture.paths, path)
	}
	if len(capture.omitted) > 0 {
		if _, err := private(append([]string{"reset", "-q", head, "--"}, capture.omitted...)...); err != nil {
			return manualCapture{}, err
		}
	}
	if capture.patch, err = private("diff", "--cached", "--binary", "--full-index", "--no-renames", head); err != nil {
		return manualCapture{}, err
	}
	return capture, nil
}

// manualStaging restores exactly the staging this submission made; its
// error is the restoration's own, never ignored.
type manualStaging struct{ undo func() error }

// stageManual stages the frozen candidate in the goal worktree for the
// commit owner. Captured changes of the goal worktree itself are already its
// files: only the captured paths are staged, after checking they are still
// exactly the capture, and any staged path outside them is refused rather
// than committed or discarded. Every other candidate, including a patch
// supplied from inside the goal worktree, is applied: the worktree must be
// clean, or already hold exactly this candidate staged; git apply --index
// --binary with no three-way merge changes index and files together, so a
// refusal leaves both untouched.
func stageManual(git func(string, ...string) ([]byte, error), worktree string, same bool, capture manualCapture) (manualStaging, error) {
	cached := func() ([]byte, error) {
		return git(worktree, "diff", "--cached", "--binary", "--full-index", "--no-renames", "HEAD")
	}
	if same && !capture.explicit {
		head, err := git(worktree, "rev-parse", "--verify", "HEAD^{commit}")
		if err != nil || strings.TrimSpace(string(head)) != capture.head {
			return manualStaging{}, fmt.Errorf("the goal worktree moved since the changes were captured")
		}
		stagedNames, err := git(worktree, "diff", "--cached", "--name-only", "-z", "--no-renames", "HEAD")
		if err != nil {
			return manualStaging{}, err
		}
		partial, err := git(worktree, "diff", "--name-only", "-z", "--no-renames")
		if err != nil {
			return manualStaging{}, err
		}
		for _, path := range splitNUL(stagedNames) {
			if !slices.Contains(capture.paths, path) {
				return manualStaging{}, fmt.Errorf("%s is staged in the goal worktree but is not part of these changes; commit or unstage it yourself first", path)
			}
			if slices.Contains(splitNUL(partial), path) {
				return manualStaging{}, fmt.Errorf("%s is staged with a version other than the file's; staging the file would discard it, so stage or unstage it yourself first", path)
			}
		}
		// The index as the caller left it, pre-staged paths included; a
		// restoration puts back exactly these entries of the captured paths.
		saved, err := git(worktree, "write-tree")
		if err != nil {
			return manualStaging{}, fmt.Errorf("cannot record the goal worktree's staging before submitting: %v", err)
		}
		savedTree := strings.TrimSpace(string(saved))
		undo := func() error {
			_, err := git(worktree, append([]string{"--literal-pathspecs", "restore", "--staged", "--source=" + savedTree, "--"}, capture.paths...)...)
			return err
		}
		if _, err := git(worktree, append([]string{"--literal-pathspecs", "add", "-A", "--"}, capture.paths...)...); err != nil {
			if undoErr := undo(); undoErr != nil {
				return manualStaging{}, fmt.Errorf("cannot stage the changes (%v), and restoring the earlier staging failed: %v", err, undoErr)
			}
			return manualStaging{}, fmt.Errorf("cannot stage the changes: %v", err)
		}
		if staged, err := cached(); err != nil || !bytes.Equal(staged, capture.patch) {
			if undoErr := undo(); undoErr != nil {
				return manualStaging{}, fmt.Errorf("the checkout's changes moved while they were being submitted, and restoring the earlier staging failed: %v", undoErr)
			}
			return manualStaging{}, fmt.Errorf("the checkout's changes moved while they were being submitted; the earlier staging is restored")
		}
		return manualStaging{undo: undo}, nil
	}
	staged, err := cached()
	if err != nil {
		return manualStaging{}, err
	}
	if len(staged) != 0 && bytes.Equal(staged, capture.patch) {
		// Exactly this candidate is already staged, by an interrupted
		// earlier submission or by the caller; it is not this call's to undo.
		return manualStaging{undo: func() error { return nil }}, nil
	}
	if len(staged) != 0 {
		return manualStaging{}, fmt.Errorf("the goal worktree %s has other staged changes; it must be clean to receive this work", worktree)
	}
	// Uncommitted work of any kind, an untracked build result included,
	// keeps the destination from receiving other work.
	unstaged, err := git(worktree, "ls-files", "-z", "--modified", "--others", "--exclude-standard")
	if err != nil {
		return manualStaging{}, err
	}
	for _, path := range splitNUL(unstaged) {
		if branch.PathClass(path) != branch.ClassExcluded {
			return manualStaging{}, fmt.Errorf("the goal worktree %s has uncommitted changes to %s; it must be clean to receive this work", worktree, path)
		}
	}
	if err := gitApplyIndex(worktree, capture.patch, false); err != nil {
		return manualStaging{}, fmt.Errorf("the change does not apply to the goal branch at %s: %v", worktree, err)
	}
	return manualStaging{undo: func() error { return gitApplyIndex(worktree, capture.patch, true) }}, nil
}

func gitApplyIndex(dir string, patch []byte, reverse bool) error {
	args := []string{"-C", dir, "apply", "--index", "--binary"}
	if reverse {
		args = append(args, "-R")
	}
	command := exec.Command("git", append(args, "-")...)
	command.Stdin = bytes.NewReader(patch)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
