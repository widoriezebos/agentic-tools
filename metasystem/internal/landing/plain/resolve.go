package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ResolveSeams keeps Git and command execution local to a resolution.
type ResolveSeams struct {
	Git func(dir string, args ...string) (string, error)
	Run func(argv []string, dir string, log *os.File, started func(int64) error) error
	Now func() time.Time
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

// Resolve starts on an untouched conflicted merge or resumes its own generated
// changes. AUTO_MERGE is Git's original snapshot, including conflict markers;
// changes outside the resumed output sets must never be discarded.
func Resolve(home, install, checkout string, contract testpolicy.Contract, seams ResolveSeams) (out ResolveOutcome, err error) {
	err = withLock(install, func() (runErr error) {
		git := func(args ...string) (string, error) { return seams.git(checkout, args...) }
		listed, err := git("diff", "--name-only", "--diff-filter=U", "-z")
		if err != nil {
			return err
		}
		paths := nulPaths(listed)
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
		entries, err := Waiting(install)
		if err != nil {
			return err
		}
		var entry Entry
		for _, candidate := range entries {
			if candidate.SHA == sha {
				entry = candidate
				break
			}
		}
		if entry.Goal == "" {
			return errors.New("the conflicted merge does not name a waiting hand-in; nothing was changed")
		}
		out.Regeneration = Regeneration{Goal: entry.Goal, SHA: sha, Command: [][]string{}, At: seams.now().Format(time.RFC3339), Outcome: "failed", Exit: -1}
		ownsBegun := resuming
		defer func() {
			recordErr := appendLine(regeneratePath(install), out.Regeneration)
			if recordErr == nil && ownsBegun && (out.Outcome == "resolved" || out.Outcome == "returned") {
				recordErr = os.Remove(resolveBegunPath(install))
			}
			runErr = errors.Join(runErr, recordErr)
		}()
		prefix, err := installationPrefix(install, checkout)
		if err != nil {
			return err
		}
		var sets []testpolicy.Generated
		for _, set := range contract.Generated {
			if slices.ContainsFunc(paths, func(name string) bool { return conflict.GeneratedBy([]testpolicy.Generated{set}, prefix, name) }) {
				sets = append(sets, set)
			}
		}
		outputs := func(name string) bool { return conflict.GeneratedBy(sets, prefix, name) }
		dirty, err := git("diff", "--name-only", "-z", "AUTO_MERGE", "--")
		if err != nil {
			return fmt.Errorf("the merge's original tree can't be read to exclude checkout edits; nothing was changed: %w", err)
		}
		untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
		if err != nil {
			return err
		}
		foreign := func(name string) bool { return !resuming || !outputs(name) }
		if slices.ContainsFunc(nulPaths(dirty), foreign) || slices.ContainsFunc(nulPaths(untracked), foreign) {
			out.Outcome = "refused"
			return errors.New("the lane checkout has changes outside the recorded merge; nothing was changed")
		}
		generated := func(name string) bool { return conflict.GeneratedBy(contract.Generated, prefix, name) }
		classified, err := conflict.Classify(git, paths, generated)
		if err != nil {
			return err
		}
		detail := &conflict.Return{Main: main, Paths: classified}
		for _, item := range classified {
			if item.Class != conflict.Generated {
				out.Conflict = detail
				if _, err := git("merge", "--abort"); err != nil {
					return fmt.Errorf("abort the source conflict: %w", err)
				}
				reason := "source conflicts need resolution on the goal branch"
				returned, _, err := returnLocked(install, entry.Goal, reason, &Cause{Kind: "own", Goal: entry.Goal, SHA: entry.SHA}, detail, seams.now())
				out.Entry, out.Outcome, out.Exit = &returned, "returned", 0
				return err
			}
		}
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
			reason := fmt.Sprintf("regeneration exited %d; log: %s", out.Exit, out.Log)
			returned, _, returnErr := returnLocked(install, entry.Goal, reason, &Cause{Kind: "unclassified", Evidence: out.Log}, nil, seams.now())
			out.Entry, out.Outcome = &returned, "returned"
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
