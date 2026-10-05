package branch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// RebaseConflict preserves the versions a person chooses between. Blob names
// keep the question tied to these versions when main or the goal moves again.
type RebaseConflict struct {
	*OpError
	MainTip string
	Unit    string
	Paths   []rebaseJudgementPath
}

func (e *RebaseConflict) Error() string { return e.OpError.Error() }

func (e *RebaseConflict) Unwrap() error { return e.OpError }

type rebaseJudgementPath struct {
	Path, Original, Main, Goal string
	FirstLine, LastLine        int
	MainCommit, MainGoal       string
}

func rebaseRegenerationLog(req RebaseRequest) string {
	return filepath.Join(req.Repo, "artifacts", "agents", "goals", req.GoalID, "rebase-regenerate.log")
}

func resolveRebaseStop(req RebaseRequest, local, dir string, log *os.File, d rebaseDependencies, stopped error) ([]string, *os.File, error) {
	git := func(args ...string) (string, error) { out, err := d.git(dir, args...); return string(out), err }
	abort := func(cause error) ([]string, *os.File, error) {
		_, err := git("rebase", "--abort")
		return nil, log, errors.Join(cause, err)
	}
	listed, err := git("diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return abort(err)
	}
	paths := strings.FieldsFunc(listed, func(r rune) bool { return r == 0 })
	if len(paths) == 0 {
		return abort(stopped)
	}
	prefix, err := d.git(req.Repo, "rev-parse", "--show-prefix")
	if err != nil {
		return abort(err)
	}
	installation := strings.TrimSuffix(string(prefix), "\n")
	sets, err := rebaseGeneratedSets(git, installation)
	if err != nil {
		return abort(err)
	}
	classified, err := conflict.Classify(git, paths, func(path string) bool { return conflict.GeneratedBy(sets, installation, path) })
	if err != nil {
		return abort(err)
	}
	patch, err := git("-c", "format.pretty=%H", "rebase", "--show-current-patch")
	if err != nil {
		return abort(err)
	}
	fields := strings.Fields(patch)
	if len(fields) == 0 || !hex40(fields[0]) {
		return abort(stopped)
	}
	kind, err := d.repository.facts.Kind(req.Repo, fields[0], req.GoalID)
	if err != nil {
		return abort(err)
	}
	name := commitWord(kind.Kind)
	if kind.Kind == Unit {
		name = "build " + kind.Unit
	}
	refusal := &RebaseConflict{OpError: &OpError{Code: RebaseJudgementCode}, MainTip: req.EndpointTip, Unit: kind.Unit}
	if refusal.Unit == "" {
		refusal.Unit = name
	}
	var descriptions []string
	for _, item := range classified {
		descriptions = append(descriptions, fmt.Sprintf("%s: %s", item.Path, item.Class))
		if item.Class == conflict.Judgement {
			detail, err := rebaseJudgement(req, local, git, item.Path)
			if err != nil {
				return abort(err)
			}
			refusal.Paths = append(refusal.Paths, detail)
		}
	}
	message := fmt.Sprintf("rebase stopped at %s; main is at %.7s; nothing was changed\npaths:\n%s\nrun: metasystem work status %s", name, req.EndpointTip, strings.Join(descriptions, "\n"), req.GoalID)
	if len(refusal.Paths) > 0 {
		refusal.Message = message
		return abort(refusal)
	}
	if slices.ContainsFunc(classified, func(path conflict.Path) bool { return path.Class != conflict.Generated }) {
		return abort(&OpError{Code: RebaseConflictCode, Message: message})
	}
	if log == nil {
		logPath := rebaseRegenerationLog(req)
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return abort(err)
		}
		opened, err := os.Create(logPath)
		if err != nil {
			return abort(err)
		}
		log = opened
	}
	var matching []testpolicy.Generated
	for _, set := range sets {
		if slices.ContainsFunc(paths, func(path string) bool { return conflict.GeneratedBy([]testpolicy.Generated{set}, installation, path) }) {
			matching = append(matching, set)
		}
	}
	outputs := func(path string) bool { return conflict.GeneratedBy(matching, installation, path) }
	// During a rebase HEAD contains main and the commits already replayed.
	// Its generated files are rebuilt from those merged sources.
	if err := conflict.TakeMain(git, dir, paths); err != nil {
		return abort(err)
	}
	sources, err := rebaseSources(git, outputs)
	if err != nil {
		return abort(err)
	}
	var last []string
	for _, set := range matching {
		for _, argv := range [][]string{set.Command, set.Then} {
			if len(argv) == 0 {
				continue
			}
			last = argv
			err := d.run(argv, filepath.Join(dir, installation, set.Cwd), log, func(int64) error { return nil })
			if err != nil {
				exit := -1
				var status *exec.ExitError
				if errors.As(err, &status) {
					exit = status.ExitCode()
					if wait, ok := status.Sys().(syscall.WaitStatus); ok && wait.Signaled() {
						exit = 128 + int(wait.Signal())
					}
				}
				return abort(operationRefusal(RebaseConflictCode, "rebase command %q exited %d; nothing was changed\nlog: %s\nrun: metasystem work status %s", argv, exit, log.Name(), req.GoalID))
			}
			after, err := rebaseSources(git, outputs)
			if err != nil {
				return abort(err)
			}
			if !bytes.Equal(sources, after) {
				return abort(operationRefusal(RebaseConflictCode, "rebase command %q changed sources; nothing was changed\nlog: %s\nrun: metasystem work status %s", last, log.Name(), req.GoalID))
			}
		}
	}
	if err := conflict.StageGenerated(git, outputs); err != nil {
		return abort(err)
	}
	remaining, err := git("diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return abort(err)
	}
	if remaining != "" {
		return abort(operationRefusal(RebaseConflictCode, "rebase command %q exited 0 but left conflicts; nothing was changed\nlog: %s\nrun: metasystem work status %s", last, log.Name(), req.GoalID))
	}
	return paths, log, nil
}

func rebaseSources(git conflict.Git, outputs func(string) bool) ([]byte, error) {
	patch, err := git("diff", "--no-ext-diff", "--no-textconv", "--binary", "HEAD")
	if err != nil {
		return nil, err
	}
	sources, err := normalizeChangePatch([]byte(patch), outputs)
	if err != nil {
		return nil, err
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(untracked, "\x00") {
		if path != "" && !outputs(path) {
			sources = append(sources, []byte("\x00"+path)...)
		}
	}
	return sources, nil
}

func rebaseGeneratedSets(git conflict.Git, prefix string) ([]testpolicy.Generated, error) {
	listed, err := git("ls-tree", "--name-only", "-z", "HEAD", "--", prefix+"metasystem.conf")
	if err != nil || listed == "" {
		return nil, err
	}
	data, err := git("show", "HEAD:"+prefix+"metasystem.conf")
	if err != nil {
		return nil, err
	}
	// Read only HEAD's configuration, even if its checkout copy is unmerged.
	scratch, done, err := diskstore.ScratchDir("goal-rebase-conf-*")
	if err != nil {
		return nil, err
	}
	defer done()
	conf := filepath.Join(scratch, "metasystem.conf")
	if err := os.WriteFile(conf, []byte(data), 0o600); err != nil {
		return nil, err
	}
	relative, _, err := config.CommittedLookup(conf, "testing.contract")
	if err != nil || relative == "" {
		return nil, err
	}
	data, err = git("show", "HEAD:"+prefix+relative)
	if err != nil {
		return nil, err
	}
	contract, err := testpolicy.Decode([]byte(data))
	return contract.Generated, err
}

func rebaseJudgement(req RebaseRequest, local string, git conflict.Git, path string) (rebaseJudgementPath, error) {
	detail := rebaseJudgementPath{Path: path}
	stages, err := git("ls-files", "--unmerged", "-z", "--", path)
	if err != nil {
		return detail, err
	}
	for _, record := range strings.Split(stages, "\x00") {
		if record == "" {
			continue
		}
		fields := strings.Fields(strings.SplitN(record, "\t", 2)[0])
		if len(fields) != 3 {
			return detail, fmt.Errorf("cannot read index stages for %s", path)
		}
		switch fields[2] {
		case "1":
			detail.Original = fields[1]
		case "2":
			detail.Main = fields[1]
		case "3":
			detail.Goal = fields[1]
		}
	}
	if detail.Original != "" && detail.Main != "" && detail.Goal != "" {
		var diffs [2]string
		for i, blob := range []string{detail.Main, detail.Goal} {
			diffs[i], err = git("diff", "--no-ext-diff", "--no-textconv", "--unified=0", detail.Original, blob)
			if err != nil {
				return detail, err
			}
		}
		detail.FirstLine, detail.LastLine = rebaseOverlap(diffs)
	}
	base, err := git("merge-base", local, req.EndpointTip)
	if err != nil {
		return detail, err
	}
	commit, err := git("log", "-1", "--format=%H", strings.TrimSpace(base)+".."+req.EndpointTip, "--", path)
	if err != nil {
		return detail, err
	}
	detail.MainCommit = strings.TrimSpace(commit)
	if detail.MainCommit != "" {
		body, err := git("show", "-s", "--format=%B", detail.MainCommit)
		if err != nil {
			return detail, err
		}
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "Goal-Unit:") || strings.HasPrefix(line, "Goal-Read:") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					detail.MainGoal = strings.SplitN(fields[1], "/", 2)[0]
					break
				}
			}
		}
		if detail.MainGoal == "" && strings.HasPrefix(body, "Merge commit '") {
			sha, _, _ := strings.Cut(strings.TrimPrefix(body, "Merge commit '"), "'")
			refs, err := git("for-each-ref", "--points-at="+sha, "--format=%(refname)", "refs/heads/goal/", "refs/remotes/")
			if err == nil {
				for _, ref := range strings.Fields(refs) {
					if _, name, ok := strings.Cut(ref, "/goal/"); ok {
						detail.MainGoal = name
						break
					}
				}
			}
		}
	}
	return detail, nil
}

func rebaseOverlap(diffs [2]string) (first, last int) {
	var spans [2][][2]int
	for i, diff := range diffs {
		if strings.Contains(diff, "Binary files ") || strings.Contains(diff, "GIT binary patch") {
			return 0, 0
		}
		for _, line := range strings.Split(diff, "\n") {
			if !strings.HasPrefix(line, "@@ -") {
				continue
			}
			word := strings.Fields(line)[1]
			start, count := 0, 1
			if strings.Contains(word, ",") {
				_, _ = fmt.Sscanf(word, "-%d,%d", &start, &count)
			} else {
				_, _ = fmt.Sscanf(word, "-%d", &start)
			}
			if count > 0 {
				spans[i] = append(spans[i], [2]int{start, start + count - 1})
			}
		}
	}
	for _, a := range spans[0] {
		for _, b := range spans[1] {
			lo, hi := max(a[0], b[0]), min(a[1], b[1])
			if lo <= hi {
				if first == 0 || lo < first {
					first = lo
				}
				last = max(last, hi)
			}
		}
	}
	return first, last
}
