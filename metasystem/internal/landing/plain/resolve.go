package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ResolveSeams keeps Git and command execution local to a resolution.
type ResolveSeams struct {
	Proof ProveSeams
	Git   func(dir string, args ...string) (string, error)
	Run   func(argv []string, dir string, log *os.File, started func(int64) error) error
	Now   func() time.Time

	AsSeat       bool
	regenerating *Fix
	mergeCheck   string
}

func (s ResolveSeams) git(dir string, args ...string) (string, error) {
	if s.Git != nil {
		return s.Git(dir, args...)
	}
	return Git(dir, args...)
}
func (s ResolveSeams) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Regeneration is one resolution run, including commands that failed. Each
// command's argv is ordered; a run with source conflicts has no commands.
type Regeneration struct {
	Goal     string           `json:"goal"`
	SHA      string           `json:"sha"`
	Command  [][]string       `json:"command"`
	Exit     int              `json:"exit"`
	Log      string           `json:"log"`
	Outcome  string           `json:"outcome"`
	At       string           `json:"at"`
	Conflict *conflict.Return `json:"conflict,omitempty"`
	Cause    *Cause           `json:"cause,omitempty"`
	Reason   string           `json:"reason,omitempty"`
}

type ResolveOutcome struct {
	Regeneration
	Held  bool   `json:"held"`
	Entry *Entry `json:"queue,omitempty"`
}

func regeneratePath(install string) string { return filepath.Join(Dir(install), "regenerate.jsonl") }

type begunResolve struct {
	SHA   string   `json:"sha"`
	Paths []string `json:"paths"`
}

func resolveBegunPath(install string) string {
	return filepath.Join(Dir(install), "resolve-begun.json")
}

// Resolve returns an untouched conflicted merge with its paths. AUTO_MERGE
// is Git's original snapshot; checkout edits must never be discarded.
func Resolve(home, install, checkout string, contract testpolicy.Contract, seams ResolveSeams) (out ResolveOutcome, err error) {
	var execute func() error
	err = withLock(install, func() (runErr error) {
		git := func(args ...string) (string, error) { return seams.git(checkout, args...) }
		listed, err := git("diff", "--name-only", "--diff-filter=U", "-z")
		if err != nil {
			return err
		}
		paths := nulPaths(listed)
		fix, err := ReadFix(install)
		if err != nil {
			return err
		}
		if err := refreshMergeLocked(install, checkout, fix, ProveSeams{Git: seams.Git}); err != nil {
			return err
		}
		if fix != nil && fix.State == "abandoned" {
			out.Outcome, out.Reason = "abandoned", fix.Reason
			if seams.regenerating != nil {
				*seams.regenerating = *fix
				return errors.New(fix.Reason)
			}
			if strings.Contains(fix.Reason, "pending merge is gone") {
				return nil
			}
			fix = nil
		}
		if seams.regenerating != nil {
			for _, path := range paths {
				if !slices.ContainsFunc(seams.regenerating.Paths, func(item conflict.Path) bool { return item.Path == path && item.Class == conflict.Generated }) {
					return errors.New("the resolution left source conflicts; nothing was committed")
				}
			}
		}
		if fix != nil && fix.State == "resolving" && !seams.AsSeat {
			paths = nil
			for _, path := range fix.Paths {
				if seams.regenerating == nil || path.Class == conflict.Generated {
					paths = append(paths, path.Path)
				}
			}
		}
		if len(paths) == 0 && seams.regenerating == nil {
			out.Held = true
			return nil
		}
		main, err := git("rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			return err
		}
		sha, err := git("rev-parse", "--verify", "MERGE_HEAD^{commit}")
		if err != nil {
			if len(paths) == 0 {
				out.Held = true
				return nil
			}
			return fmt.Errorf("there is no conflicted lane merge: %w", err)
		}
		if seams.regenerating != nil {
			if main != seams.regenerating.Commit || sha != seams.regenerating.Tip || seams.regenerating.State != "resolving" {
				return errors.New("the merge changed during its resolution; nothing was committed")
			}
			if len(paths) == 0 {
				execute = func() error {
					return commitResolvedMerge(home, install, checkout, seams.regenerating, seams.mergeCheck, seams, git)
				}
				return nil
			}
		}
		entries, err := Entries(install)
		if err != nil {
			return err
		}
		var entry Entry
		for _, candidate := range entries {
			if candidate.SHA == sha && candidate.State == StateWaiting {
				entry = candidate
				break
			}
		}
		if entry.Goal == "" {
			return errors.New("the conflicted merge does not name a waiting hand-in; nothing was changed")
		}
		out.Regeneration = Regeneration{Goal: entry.Goal, SHA: sha, Command: [][]string{}, At: seams.now().Format(time.RFC3339), Outcome: "failed", Exit: -1}
		if entry.Held {
			out.Cause, out.Reason, out.Outcome, out.Held, out.Entry = entry.Cause, entry.Reason, "held", true, &entry
			return nil
		}
		prefix, err := installationPrefix(install, checkout)
		if err != nil {
			return err
		}
		dirty, err := git("diff", "--name-only", "-z", "AUTO_MERGE", "--")
		if err != nil {
			return fmt.Errorf("the merge's original tree can't be read to exclude checkout edits; nothing was changed: %w", err)
		}
		untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
		if err != nil {
			return err
		}
		if (fix == nil || fix.State != "resolving" || fix.Tip != sha || fix.Commit != main) && (len(nulPaths(dirty)) > 0 || len(nulPaths(untracked)) > 0) {
			out.Outcome = "refused"
			return errors.New("the lane checkout has changes outside the recorded merge; nothing was changed")
		}
		generated := func(name string) bool { return conflict.GeneratedBy(contract.Generated, prefix, name) }
		classified, err := conflict.Classify(git, paths, generated)
		if err != nil {
			return err
		}
		if fix != nil && fix.State == "resolving" && fix.Tip == sha && seams.regenerating == nil {
			classified = fix.Paths
		}
		detail := &conflict.Return{Main: main, Paths: classified}
		if seams.AsSeat {
			if fix != nil && (fix.State != "resolving" || fix.Tip != sha || fix.Commit != main) {
				return errors.New("another lane repair is active; nothing was changed")
			}
			if fix == nil {
				fix = &Fix{Goal: entry.Goal, Units: []string{"lane-merge-1"}, Commit: main, Tip: sha, State: "resolving", Attempt: seams.Proof.newID(), Paths: classified, Generated: contract.Generated}
				if err := WriteFix(install, fix); err != nil {
					return err
				}
			}
			out.Conflict, out.Outcome, out.Exit = detail, "resolving", 0
			return nil
		}
		if len(classified) > 0 && seams.regenerating == nil {
			trunk, err := git("rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
			if err != nil {
				return err
			}
			detail.Main = trunk
			var after []GoalSHA
			if main != trunk {
				_, mergeErr := git("merge-tree", "--write-tree", trunk, sha)
				if mergeErr == nil {
					contains := checkoutGit(checkout, ProveSeams{Git: seams.Git}).contains
					for _, candidate := range entries {
						inHead, err := contains(main, candidate.SHA)
						if err != nil {
							return err
						}
						inMain, err := contains(trunk, candidate.SHA)
						if err != nil {
							return err
						}
						if one := (GoalSHA{Goal: candidate.Goal, SHA: candidate.SHA}); inHead && !inMain && !slices.Contains(after, one) {
							after = append(after, one)
						}
					}
					if len(after) == 0 {
						return errors.New("the batch conflict has no queued merge to wait for; nothing was changed")
					}
				} else {
					var exit *exec.ExitError
					if !errors.As(mergeErr, &exit) || exit.ExitCode() != 1 {
						return mergeErr
					}
				}
			}
			out.Conflict = detail
			if _, err := git("merge", "--abort"); err != nil {
				return fmt.Errorf("abort the merge conflict: %w", err)
			}
			if fix != nil && fix.State == "resolving" && fix.Tip == sha {
				fix.State = "returned"
				if err := WriteFix(install, fix); err != nil {
					return err
				}
			}
			out.Exit = 0
			if len(after) > 0 {
				names := []string{}
				for _, before := range after {
					names = append(names, before.Goal)
				}
				out.Reason = "conflicts with goal " + strings.Join(names, ", ") + ", which is in the same batch; it is merged again when " + strings.Join(names, ", ") + " has landed"
				if fix != nil && fix.Tip == sha {
					out.Reason += "; resolution job " + fix.Job + " cannot resolve"
				}
				out.Held, out.Outcome = true, "held"
				return resolveWaitingLocked(install, entry, after, false, &out)
			}
			out.Cause = &Cause{Kind: "own", Goal: entry.Goal, SHA: entry.SHA}
			names := make([]string, 0, len(classified))
			for _, path := range classified {
				names = append(names, path.Path+" ("+path.Class+")")
			}
			out.Reason = "the merge conflicts with main in " + strings.Join(names, ", ") + "; run metasystem work rebase " + entry.Goal + ", which regenerates what the contract declares, then hand in again"
			if fix != nil && fix.Tip == sha {
				out.Reason += "; resolution job " + fix.Job + " cannot resolve"
			}
			returned, _, err := returnLocked(install, entry.Goal, out.Reason, out.Cause, detail, seams.now(), seams.Proof)
			out.Entry, out.Outcome = &returned, "returned"
			if returned.State == StateWaiting {
				out.Held, out.Outcome = true, "held"
				return errors.Join(err, resolveWaitingLocked(install, entry, nil, true, &out))
			}
			return err
		}
		execute = func() (runErr error) {
			recordWaiting := func(after []GoalSHA, held bool) error {
				return withLock(install, func() error { return resolveWaitingLocked(install, entry, after, held, &out) })
			}
			returnEntry := func() (returned Entry, added bool, err error) {
				err = withLock(install, func() error {
					returned, added, err = returnLocked(install, entry.Goal, out.Reason, out.Cause, detail, seams.now(), seams.Proof)
					return err
				})
				return
			}
			var begun *begunResolve
			data, err := os.ReadFile(resolveBegunPath(install))
			if err == nil {
				if err := json.Unmarshal(data, &begun); err != nil {
					return err
				}
				if begun == nil || begun.SHA == "" || len(begun.Paths) == 0 {
					return errors.New("the begun resolve record has no merge or paths; nothing was changed")
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if len(paths) == 0 && begun == nil {
				out.Held = true
				return nil
			}
			resuming := begun != nil && begun.SHA == sha
			if len(paths) == 0 && !resuming {
				out.Held = true
				return nil
			}
			if resuming {
				for _, name := range begun.Paths {
					if !slices.Contains(paths, name) {
						paths = append(paths, name)
					}
				}
			}
			ownsBegun := resuming
			defer func() {
				recordErr := withLock(install, func() error {
					recordErr := appendLine(regeneratePath(install), out.Regeneration)
					if recordErr == nil && out.Held && out.Cause != nil {
						cause := *out.Cause
						if cause.Evidence == "" {
							cause.Evidence = regeneratePath(install)
						}
						recordErr = recordProofStop(install, Result{Result: Red, Scope: "regeneration", Goals: []GoalSHA{{Goal: out.Goal, SHA: out.SHA}}, Cause: &cause, Log: out.Log, Reason: out.Reason, At: out.At})
					}
					if recordErr == nil && ownsBegun && (out.Outcome == "resolved" || out.Outcome == "returned" || out.Outcome == "held" || out.Outcome == "failed") {
						recordErr = os.Remove(resolveBegunPath(install))
					}
					return recordErr
				})
				runErr = errors.Join(runErr, recordErr)
			}()
			var sets []testpolicy.Generated
			for _, set := range contract.Generated {
				if slices.ContainsFunc(paths, func(name string) bool { return conflict.GeneratedBy([]testpolicy.Generated{set}, prefix, name) }) {
					sets = append(sets, set)
				}
			}
			outputs := func(name string) bool { return conflict.GeneratedBy(sets, prefix, name) }
			// A failed regeneration restores the merge snapshot before aborting, and
			// removes only new outputs in the sets this run actually regenerated.
			abort := func(cause error) error {
				untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
				var created []string
				for _, name := range nulPaths(untracked) {
					if outputs(name) {
						created = append(created, name)
					}
				}
				if err == nil && len(created) > 0 {
					_, err = git(append([]string{"clean", "-f", "--"}, created...)...)
				}
				names, listErr := conflict.GeneratedFiles(git, outputs, paths)
				err = errors.Join(err, listErr)
				if err == nil && len(names) > 0 {
					if seams.regenerating != nil {
						err = conflict.TakeMain(git, checkout, names)
					} else {
						_, err = git(append([]string{"restore", "--source=AUTO_MERGE", "--worktree", "--"}, names...)...)
					}
				}
				if seams.regenerating != nil {
					return errors.Join(cause, err)
				}
				_, abortErr := git("merge", "--abort")
				if err != nil || abortErr != nil {
					return errors.Join(cause, err, abortErr)
				}
				out.Cause = &Cause{Kind: "environment", Name: "lost-process", Evidence: out.Log}
				out.Reason = "regeneration did not complete; retry at the next turn"
				out.Outcome = "held"
				var exit *exec.ExitError
				lost := out.Exit < 0
				if errors.As(cause, &exit) {
					if status, ok := exit.Sys().(syscall.WaitStatus); ok {
						lost = lost || status.Signaled()
					}
				}
				if lost {
					out.Held = entry.Cause != nil && entry.Cause.Kind == "environment" && entry.Cause.Name == "lost-process"
					if out.Held {
						out.Outcome, out.Reason = "held", "regeneration did not complete twice; hold and ask about the lane"
					}
					return errors.Join(cause, recordWaiting(nil, out.Held))
				}
				// The merge is gone before replay; only the selected generators' own
				// outputs are restored, so unrelated checkout files survive.
				out.Cause.Kind, out.Cause.Name = "unclassified", ""
				if err := withLock(install, func() error { return appendLine(regeneratePath(install), out.Regeneration) }); err != nil {
					return errors.Join(cause, err)
				}
				value := PolicyValue{Value: "auto"}
				var policyErr error
				if seams.Proof.Policy != nil {
					value, policyErr = seams.Proof.Policy("landing.on-red")
				}
				if policyErr != nil || value.Value == "person" {
					out.Cause.Kind = "unclassified"
					out.Held, out.Outcome = true, "held"
					_, _, holdErr := returnEntry()
					return errors.Join(cause, holdErr, recordWaiting(nil, true))
				}
				baselineErr := replayRegeneration(home, install, git, sets, prefix, seams, out.Goal, out.SHA, out.Log)
				if baselineErr != nil {
					out.Cause.Kind, out.Cause.Name = "unclassified", ""
					out.Held, out.Outcome, out.Reason = true, "held", "regeneration fails on the tree before the merge too; hold and ask about the lane; log: "+out.Log
					return errors.Join(cause, baselineErr, recordWaiting(nil, true))
				}
				out.Cause = &Cause{Kind: "own", Goal: entry.Goal, SHA: entry.SHA, Evidence: out.Log}
				out.Conflict = detail
				out.Reason = fmt.Sprintf("regeneration exited %d; log: %s; run metasystem work rebase %s", out.Exit, out.Log, entry.Goal)
				returned, _, returnErr := returnEntry()
				out.Entry, out.Outcome = &returned, "returned"
				if returned.State == StateWaiting {
					out.Held, out.Outcome = true, "held"
					return errors.Join(cause, returnErr, recordWaiting(nil, true))
				}
				return errors.Join(cause, returnErr)
			}
			if !resuming {
				data, err := json.Marshal(begunResolve{SHA: sha, Paths: paths})
				if err != nil {
					return err
				}
				if err := withLock(install, func() error { return os.WriteFile(resolveBegunPath(install), data, 0o600) }); err != nil {
					return err
				}
				ownsBegun = true
			}
			if err := conflict.TakeMain(git, checkout, paths); err != nil {
				return abort(err)
			}
			logs := filepath.Join(Dir(install), "regenerations")
			if err := os.MkdirAll(logs, 0o755); err != nil {
				return abort(err)
			}
			log, err := os.CreateTemp(logs, "run-*.log")
			if err != nil {
				return abort(err)
			}
			defer log.Close()
			out.Log = log.Name()
			for _, set := range sets {
				dir := filepath.Join(install, set.Cwd)
				for _, argv := range [][]string{set.Command, set.Then} {
					if len(argv) == 0 {
						continue
					}
					out.Command = append(out.Command, argv)
					running := RunningRegeneration{Goal: entry.Goal, SHA: sha, Command: argv, Log: out.Log, Since: seams.now().Format(time.RFC3339)}
					err := runRegeneration(home, install, running, argv, dir, log, seams.Run)
					if err != nil {
						out.Exit = commandExit(err)
						return abort(err)
					}
				}
			}
			// A conflict absent from both HEAD and the regenerated index is already
			// staged as a deletion; Git cannot add that now-unknown path again.
			if err := conflict.StageGenerated(git, outputs); err != nil {
				return abort(err)
			}
			out.Outcome, out.Exit = "resolved", 0
			if seams.regenerating != nil {
				return commitResolvedMerge(home, install, checkout, seams.regenerating, seams.mergeCheck, seams, git)
			}
			if entry.Cause != nil || len(entry.After) > 0 {
				return recordWaiting(nil, false)
			}
			return nil
		}
		return nil
	})
	if err == nil && execute != nil {
		err = execute()
	}
	return out, err
}

func installationPrefix(install, checkout string) (string, error) {
	rel, err := filepath.Rel(checkout, install)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("the lane installation is outside its checkout")
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel) + "/", nil
}

func nulPaths(list string) []string {
	return strings.FieldsFunc(list, func(r rune) bool { return r == 0 })
}

func writeRegeneration(install string, running RunningRegeneration) error {
	data, err := json.Marshal(running)
	if err != nil {
		return err
	}
	return os.WriteFile(regenerationRunningPath(install), data, 0o600)
}

// resolveWaitingLocked records a resolution that leaves the hand-in waiting.
func resolveWaitingLocked(install string, entry Entry, after []GoalSHA, held bool, out *ResolveOutcome) error {
	line := Line{Goal: entry.Goal, SHA: entry.SHA, At: out.At, Outcome: StateWaiting, After: after, Held: held, Reason: out.Reason, Cause: out.Cause, Conflict: out.Conflict}
	if err := appendLine(queuePath(install), line); err != nil {
		return err
	}
	entry.After, entry.Held, entry.Reason, entry.Cause = after, out.Held, out.Reason, out.Cause
	if out.Outcome != "resolved" {
		out.Entry = &entry
	}
	return nil
}

func replayRegeneration(home, install string, git conflict.Git, sets []testpolicy.Generated, prefix string, seams ResolveSeams, goal, sha, evidence string) (err error) {
	log, err := os.OpenFile(evidence, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	_, _ = log.WriteString("\nRegeneration on the tree before the merge:\n")
	var ran []testpolicy.Generated
	defer func() {
		outputs := func(name string) bool { return conflict.GeneratedBy(ran, prefix, name) }
		listed, cleanErr := git("ls-files", "--others", "--exclude-standard", "-z")
		var created []string
		for _, name := range nulPaths(listed) {
			if outputs(name) {
				created = append(created, name)
			}
		}
		if cleanErr == nil && len(created) > 0 {
			_, cleanErr = git(append([]string{"clean", "-f", "--"}, created...)...)
		}
		names, listErr := conflict.GeneratedFiles(git, outputs, nil)
		if listErr == nil && len(names) > 0 {
			_, listErr = git(append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, names...)...)
		}
		err = errors.Join(err, cleanErr, listErr)
	}()
	for _, set := range sets {
		ran = append(ran, set)
		for _, argv := range [][]string{set.Command, set.Then} {
			if len(argv) == 0 {
				continue
			}
			running := RunningRegeneration{Goal: goal, SHA: sha, Command: argv, Log: evidence, Since: seams.now().Format(time.RFC3339)}
			if err := runRegeneration(home, install, running, argv, filepath.Join(install, set.Cwd), log, seams.Run); err != nil {
				return err
			}
		}
	}
	return nil
}

// CompleteMerge regenerates declared outputs and commits the builder's staged resolution.
func CompleteMerge(home, install, checkout string, fix *Fix, check string, seams ResolveSeams) error {
	if err := RefreshMerge(install, checkout, fix, ProveSeams{Git: seams.Git}); err != nil {
		return err
	}
	if fix.State == "resolved" || fix.State == "reviewing" || fix.State == "done" {
		return nil
	}
	if fix.State != "resolving" {
		return mergeStateError(fix)
	}
	seams.AsSeat, seams.regenerating, seams.mergeCheck = false, fix, check
	_, err := Resolve(home, install, checkout, testpolicy.Contract{Generated: fix.Generated}, seams)
	return err
}

func commitResolvedMerge(home, install, checkout string, fix *Fix, check string, seams ResolveSeams, git conflict.Git) error {
	log, err := os.CreateTemp(Dir(install), "merge-check-*.log")
	if err != nil {
		return err
	}
	defer log.Close()
	if err := runRegeneration(home, install, RunningRegeneration{Goal: fix.Goal, SHA: fix.Tip, Log: log.Name()}, []string{"env", "LANDING_PROOF_BASE=" + fix.Commit, "/bin/sh", "-c", check}, install, log, seams.Run); err != nil {
		return err
	}
	return withLock(install, func() error {
		if err := refreshMergeLocked(install, checkout, fix, ProveSeams{Git: seams.Git}); err != nil {
			return err
		}
		if fix.State != "resolving" {
			return mergeStateError(fix)
		}
		head, err := git("rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			return err
		}
		if head != fix.Commit {
			return errors.New("the merge changed during its check; nothing was committed")
		}
		args := []string{"grep", "--cached", "-n", "-e", "^<<<<<<< ", "-e", "^=======", "-e", "^>>>>>>> ", "-e", "^|||||||", "--"}
		for _, path := range fix.Paths {
			args = append(args, ":(literal)"+path.Path)
		}
		markers, err := git(args...)
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				return fmt.Errorf("the staged resolution could not be checked for conflict markers; nothing was committed: %w", err)
			}
		} else if markers != "" {
			return fmt.Errorf("the staged resolution has conflict markers; nothing was committed: %s", markers)
		}
		patch, err := git("diff", "--binary", "AUTO_MERGE", "--")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(Dir(install), "fixes", fix.Attempt+".patch"), []byte(patch), 0o600); err != nil {
			return err
		}
		if _, err := git("diff", "--quiet", "--"); err != nil {
			return errors.New("the check left unstaged changes; nothing was committed")
		}
		messagePath, err := git("rev-parse", "--path-format=absolute", "--git-path", "MERGE_MSG")
		if err != nil {
			return err
		}
		message, err := os.ReadFile(messagePath)
		if err != nil {
			return err
		}
		if _, err := git("commit", "-m", strings.TrimSpace(string(message))+"\n\nGoal-Unit: "+fix.Goal+"/"+fix.Units[0]); err != nil {
			return err
		}
		fix.Commit, err = git("rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			return err
		}
		fix.State = "reviewing"
		return WriteFix(install, fix)
	})
}

func mergeStateError(fix *Fix) error {
	if fix.Reason != "" {
		return errors.New(fix.Reason)
	}
	return fmt.Errorf("the merge resolution is %s; it cannot be committed", fix.State)
}
