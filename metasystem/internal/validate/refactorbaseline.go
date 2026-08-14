package validate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The refactor gate (script-misc-2/D29): the completion-gate decision engine
// behind skills/refactor's Risk-Sized Batches, ported from the last policy
// gate still deciding in shell. The on-disk baseline format (sha= /
// recorded_epoch= / gate= lines), every message, and the 0/1/2 exit-code
// contract are preserved byte for byte — the suite's positive and negative
// fixtures assert them through the shim.

// RefactorBaselineParams is one gate invocation. Cwd anchors the git
// context exactly as the script's working directory did; MaxAgeMinutes and
// MaxCommits arrive already resolved (flag > environment > conf > default)
// or resolve here when negative.
type RefactorBaselineParams struct {
	Command       string // record or check
	Cwd           string // directory the git commands run from
	File          string // baseline path, normalized against Cwd when relative
	Gate          string // record: the acceptance gate command that passed
	MaxAgeMinutes int
	MaxCommits    int
}

// RefactorBaseline runs the gate and returns its exit code: 0 safe, 1
// blocked, 2 usage or environment error. Messages go to out (verdicts) and
// errOut (refusals), matching the script's stdout/stderr split.
func RefactorBaseline(p RefactorBaselineParams, out, errOut io.Writer) int {
	if gitIn(p.Cwd, "rev-parse", "--is-inside-work-tree") != nil {
		fmt.Fprintln(errOut, "not inside a git repository")
		return 2
	}
	toplevel, err := gitOutputIn(p.Cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(errOut, "not inside a git repository")
		return 2
	}
	file := p.File
	if !filepath.IsAbs(file) {
		file = filepath.Join(p.Cwd, file)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(file))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	absFile := filepath.Join(resolvedDir, filepath.Base(file))
	if !strings.HasPrefix(absFile, toplevel+string(filepath.Separator)) {
		fmt.Fprintf(errOut, "baseline file must live inside the repository so git can see its dirt: %s\n", p.File)
		return 2
	}
	relFile := strings.TrimPrefix(absFile, toplevel+string(filepath.Separator))

	switch p.Command {
	case "record":
		return refactorBaselineRecord(p, absFile, out, errOut)
	case "check":
		return refactorBaselineCheck(p, toplevel, absFile, relFile, out, errOut)
	}
	fmt.Fprintln(errOut, "refactor-baseline command must be record or check")
	return 2
}

func refactorBaselineRecord(p RefactorBaselineParams, absFile string, out, errOut io.Writer) int {
	if p.Gate == "" {
		fmt.Fprintln(errOut, "record requires --gate with the acceptance gate command that passed")
		return 2
	}
	dirt, err := gitOutputIn(p.Cwd, "status", "--porcelain")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if dirt != "" {
		fmt.Fprintln(errOut, "worktree is dirty; a baseline must be an exact committed state")
		return 1
	}
	sha, err := gitOutputIn(p.Cwd, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	record := fmt.Sprintf("sha=%s\nrecorded_epoch=%d\ngate=%s\n", sha, time.Now().UTC().Unix(), p.Gate)
	if err := os.WriteFile(absFile, []byte(record), 0o644); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "trusted baseline recorded: %s\n", sha)
	fmt.Fprintf(out, "commit %s now or with the next checkpoint; check ignores this file's own dirt\n", p.File)
	return 0
}

func refactorBaselineCheck(p RefactorBaselineParams, toplevel, absFile, relFile string, out, errOut io.Writer) int {
	data, err := os.ReadFile(absFile)
	if err != nil {
		fmt.Fprintf(errOut, "no trusted baseline at %s; run the acceptance gate, then record it\n", p.File)
		return 1
	}
	sha := baselineField(string(data), "sha")
	epochText := baselineField(string(data), "recorded_epoch")
	if sha == "" || !allDigits(epochText) {
		fmt.Fprintf(errOut, "baseline file is malformed: %s\n", p.File)
		return 2
	}
	epoch, _ := strconv.ParseInt(epochText, 10, 64)
	if gitIn(p.Cwd, "rev-parse", "--verify", "--quiet", sha+"^{commit}") != nil {
		fmt.Fprintf(errOut, "baseline commit %s is unknown to this repository\n", sha)
		return 1
	}
	foreign, err := dirtBeyondBaseline(toplevel, relFile)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if foreign {
		fmt.Fprintln(errOut, "worktree is dirty beyond the baseline file; commit, stash, or revert before a new refactor batch")
		return 1
	}
	if gitIn(p.Cwd, "merge-base", "--is-ancestor", sha, "HEAD") != nil {
		fmt.Fprintf(errOut, "baseline %s is not an ancestor of HEAD; history diverged from the trusted baseline\n", sha)
		return 1
	}
	countText, err := gitOutputIn(p.Cwd, "rev-list", "--count", sha+"..HEAD")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	commitsSince, err := strconv.Atoi(countText)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	ageMinutes := (time.Now().UTC().Unix() - epoch) / 60
	if commitsSince > p.MaxCommits {
		fmt.Fprintf(errOut, "cadence backstop due: %d commits since the trusted baseline (max %d); run the acceptance gate\n", commitsSince, p.MaxCommits)
		return 1
	}
	if ageMinutes > int64(p.MaxAgeMinutes) {
		fmt.Fprintf(errOut, "cadence backstop due: baseline is %d minutes old (max %d); run the acceptance gate\n", ageMinutes, p.MaxAgeMinutes)
		return 1
	}
	fmt.Fprintf(out, "refactor baseline safe: %s (%d commits, %d minutes since acceptance)\n", sha, commitsSince, ageMinutes)
	return 0
}

// baselineField reads the first key=value line for one key — the sed -n
// 's/^key=//p' | head -1 this replaces.
func baselineField(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if value, found := strings.CutPrefix(line, key+"="); found {
			return value
		}
	}
	return ""
}

// dirtBeyondBaseline reports whether the worktree carries any dirt other
// than the baseline file itself. The porcelain is NUL-delimited and never
// C-quoted, so paths with spaces or non-ASCII bytes compare literally; a
// rename's second record carries no status prefix and therefore reads as
// foreign dirt, which is the safe direction: it blocks.
func dirtBeyondBaseline(toplevel, relFile string) (bool, error) {
	output, err := gitOutputRawIn(toplevel, "status", "--porcelain", "--untracked-files=all", "-z")
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.ReplaceAll(output, "\x00", "\n"), "\n")
	if length := len(lines); length > 0 && lines[length-1] == "" {
		lines = lines[:length-1]
	}
	for _, line := range lines {
		if len(line) < 4 || line[3:] != relFile {
			return true, nil
		}
	}
	return false, nil
}

func gitIn(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitOutputIn(dir string, args ...string) (string, error) {
	output, err := gitOutputRawIn(dir, args...)
	return strings.TrimSpace(output), err
}

func gitOutputRawIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	return string(output), err
}
