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
	err = withLock(install, func() (runErr error) {
		git := func(args ...string) (string, error) { return seams.git(checkout, args...) }
		listed, err := git("diff", "--name-only", "--diff-filter=U", "-z")
		if err != nil {
			return err
		}
		paths := nulPaths(listed)
		if len(paths) == 0 {
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
		if len(nulPaths(dirty)) > 0 || len(nulPaths(untracked)) > 0 {
			out.Outcome = "refused"
			return errors.New("the lane checkout has changes outside the recorded merge; nothing was changed")
		}
		generated := func(name string) bool { return conflict.GeneratedBy(contract.Generated, prefix, name) }
		classified, err := conflict.Classify(git, paths, generated)
		if err != nil {
			return err
		}
		detail := &conflict.Return{Main: main, Paths: classified}
		for range classified {
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
			out.Exit = 0
			if len(after) > 0 {
				names := []string{}
				for _, before := range after {
					names = append(names, before.Goal)
				}
				out.Reason = "conflicts with goal " + strings.Join(names, ", ") + ", which is in the same batch; it is merged again when " + strings.Join(names, ", ") + " has landed"
				out.Held, out.Outcome = true, "held"
				return resolveWaitingLocked(install, entry, after, false, &out)
			}
			out.Cause = &Cause{Kind: "own", Goal: entry.Goal, SHA: entry.SHA}
			names := make([]string, 0, len(classified))
			for _, path := range classified {
				names = append(names, path.Path+" ("+path.Class+")")
			}
			out.Reason = "the merge conflicts with main in " + strings.Join(names, ", ") + "; run metasystem work rebase " + entry.Goal + ", which regenerates what the contract declares, then hand in again"
			returned, _, err := returnLocked(install, entry.Goal, out.Reason, out.Cause, detail, seams.now(), seams.Proof)
			out.Entry, out.Outcome = &returned, "returned"
			if returned.State == StateWaiting {
				out.Held, out.Outcome = true, "held"
				return errors.Join(err, resolveWaitingLocked(install, entry, nil, true, &out))
			}
			return err
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
			recordErr := appendLine(regeneratePath(install), out.Regeneration)
			if recordErr == nil && out.Held && out.Cause != nil {
				cause := *out.Cause
				if cause.Evidence == "" {
					cause.Evidence = regeneratePath(install)
				}
				recordErr = recordProofStop(install, Result{Result: Red, Scope: "regeneration", Goals: []GoalSHA{{Goal: out.Goal, SHA: out.SHA}}, Cause: &cause, Log: out.Log, Reason: out.Reason, At: out.At})
			}
			if recordErr == nil && ownsBegun && (out.Outcome == "resolved" || out.Outcome == "returned" || out.Outcome == "held") {
				recordErr = os.Remove(resolveBegunPath(install))
			}
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
				_, err = git(append([]string{"restore", "--source=AUTO_MERGE", "--worktree", "--"}, names...)...)
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
				return errors.Join(cause, resolveWaitingLocked(install, entry, nil, out.Held, &out))
			}
			// The merge is gone before replay; only the selected generators' own
			// outputs are restored, so unrelated checkout files survive.
			out.Cause.Kind, out.Cause.Name = "unclassified", ""
			if err := appendLine(regeneratePath(install), out.Regeneration); err != nil {
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
				_, _, holdErr := returnLocked(install, entry.Goal, out.Reason, out.Cause, detail, seams.now(), seams.Proof)
				return errors.Join(cause, holdErr, resolveWaitingLocked(install, entry, nil, true, &out))
			}
			baselineErr := replayRegeneration(home, install, git, sets, prefix, seams, out.Goal, out.SHA, out.Log)
			if baselineErr != nil {
				out.Cause.Kind, out.Cause.Name = "unclassified", ""
				out.Held, out.Outcome, out.Reason = true, "held", "regeneration fails on the tree before the merge too; hold and ask about the lane; log: "+out.Log
				return errors.Join(cause, baselineErr, resolveWaitingLocked(install, entry, nil, true, &out))
			}
			out.Cause = &Cause{Kind: "own", Goal: entry.Goal, SHA: entry.SHA, Evidence: out.Log}
			out.Conflict = detail
			out.Reason = fmt.Sprintf("regeneration exited %d; log: %s; run metasystem work rebase %s", out.Exit, out.Log, entry.Goal)
			returned, _, returnErr := returnLocked(install, entry.Goal, out.Reason, out.Cause, detail, seams.now(), seams.Proof)
			out.Entry, out.Outcome = &returned, "returned"
			if returned.State == StateWaiting {
				out.Held, out.Outcome = true, "held"
				return errors.Join(cause, returnErr, resolveWaitingLocked(install, entry, nil, true, &out))
			}
			return errors.Join(cause, returnErr)
		}
		if !resuming {
			data, err := json.Marshal(begunResolve{SHA: sha, Paths: paths})
			if err != nil {
				return err
			}
			if err := os.WriteFile(resolveBegunPath(install), data, 0o600); err != nil {
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
		if entry.Cause != nil || len(entry.After) > 0 {
			return resolveWaitingLocked(install, entry, nil, false, &out)
		}
		return nil
	})
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

// Regeneration records are deliberately independent of results.jsonl: only
// landing prove can authorize pushing the regenerated tree.
func LastRegeneration(install string) (*Regeneration, error) {
	records, err := readLines[Regeneration](regeneratePath(install))
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[len(records)-1], nil
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
	line := Line{Goal: entry.Goal, SHA: entry.SHA, At: out.At, Outcome: StateWaiting, After: after, Held: held, Reason: out.Reason, Cause: out.Cause}
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
