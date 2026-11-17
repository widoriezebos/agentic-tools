package goal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// waitObservationTranscript declares the Git facts visible to one wait test.
// A command is valid only at its declared position and with its exact input.
type waitObservationTranscript struct {
	t         *testing.T
	root      string
	store     *fakeGoalStore
	commits   map[string]fakeGoalCommit
	steps     []waitObservationStep
	calls     [][]string
	ref       string
	cancelled int
}

type waitObservationStep struct {
	args       []string
	input      string
	output     string
	err        error
	cancel     bool
	capture    bool
	captureTip string
}

const waitCaptureRef = "{capture-ref}"

func newWaitObservationTranscript(t *testing.T, endpoint Endpoint, client *fakeGoalRepository) *waitObservationTranscript {
	t.Helper()
	transcript := &waitObservationTranscript{t: t, root: endpoint.Root, store: client.store, commits: make(map[string]fakeGoalCommit)}
	t.Cleanup(transcript.done)
	return transcript
}

func (tr *waitObservationTranscript) declare(tip, parent string) {
	tr.t.Helper()
	tr.store.mu.Lock()
	commit, ok := tr.store.commits[tip]
	tr.store.mu.Unlock()
	if !ok || commit.parent != parent {
		tr.t.Fatalf("undeclared ledger ancestry at %s: exists=%t parent=%s, want %s", tip, ok, commit.parent, parent)
	}
	commit.files = copyFakeFiles(commit.files)
	tr.commits[tip] = commit
}

func (tr *waitObservationTranscript) expect(output string, args ...string) {
	tr.steps = append(tr.steps, waitObservationStep{args: args, output: output})
}

func (tr *waitObservationTranscript) expectError(err error, args ...string) {
	tr.steps = append(tr.steps, waitObservationStep{args: args, err: err})
}

func (tr *waitObservationTranscript) cancel(args ...string) {
	tr.steps = append(tr.steps, waitObservationStep{args: args, cancel: true})
}

func (tr *waitObservationTranscript) endpoint(remote, branch string) {
	tr.expect(remote+"\n", "config", "--get", "goal.sync-remote")
	tr.expect(branch+"\n", "config", "--get", "goal.sync-branch")
}

func (tr *waitObservationTranscript) capture(tip string) {
	tr.t.Helper()
	if _, ok := tr.commits[tip]; !ok {
		tr.t.Fatalf("capture of undeclared ledger commit %s", tip)
	}
	tr.steps = append(tr.steps, waitObservationStep{args: []string{"fetch", "--no-tags", "--refmap=", "origin", "+refs/heads/main:" + waitCaptureRef}, capture: true, captureTip: tip})
	tr.expect(tip+"\n", "rev-parse", "--verify", waitCaptureRef)
}

func (tr *waitObservationTranscript) cleanup() {
	tr.expect("", "update-ref", "-d", waitCaptureRef)
}

func (tr *waitObservationTranscript) acceptance(before, after string) {
	tr.t.Helper()
	beforeCommit, beforeOK := tr.commits[before]
	afterCommit, afterOK := tr.commits[after]
	if !beforeOK || !afterOK {
		tr.t.Fatalf("acceptance uses undeclared commits: before=%s after=%s", before, after)
	}
	for tip := after; tip != before; {
		commit, ok := tr.commits[tip]
		if !ok || commit.parent == "" {
			tr.t.Fatalf("accepted tip %s does not descend from %s", after, before)
		}
		tip = commit.parent
	}
	for path, body := range beforeCommit.files {
		if strings.HasPrefix(path, legacyDonePrefix) && !reflect.DeepEqual(body, afterCommit.files[path]) {
			tr.t.Fatalf("legacy concluded-goal path changed between %s and %s: %s", before, after, path)
		}
	}
	for path := range afterCommit.files {
		if strings.HasPrefix(path, legacyDonePrefix) && !reflect.DeepEqual(beforeCommit.files[path], afterCommit.files[path]) {
			tr.t.Fatalf("legacy concluded-goal path changed between %s and %s: %s", before, after, path)
		}
	}
	for _, tip := range []string{before, after} {
		commit, ok := tr.commits[tip]
		if !ok {
			tr.t.Fatalf("acceptance of undeclared ledger commit %s", tip)
		}
		root, ok := commit.files[goalsPrefix+"backlog.md"]
		if !ok {
			tr.t.Fatalf("declared commit %s has no ledger root", tip)
		}
		tr.expect(string(root), "cat-file", "-p", tip+":./"+goalsPrefix+"backlog.md")
	}
	tr.expect("", "merge-base", "--is-ancestor", before, after)
	tr.expect("", "diff", "--name-status", "--no-renames", before, after, "--", legacyDonePrefix)
}

func (tr *waitObservationTranscript) rewind(before, after string) {
	tr.t.Helper()
	for _, tip := range []string{before, after} {
		commit, ok := tr.commits[tip]
		if !ok {
			tr.t.Fatalf("rewind of undeclared ledger commit %s", tip)
		}
		root, ok := commit.files[goalsPrefix+"backlog.md"]
		if !ok {
			tr.t.Fatalf("declared commit %s has no ledger root", tip)
		}
		tr.expect(string(root), "cat-file", "-p", tip+":./"+goalsPrefix+"backlog.md")
	}
	tr.expectError(fmt.Errorf("the fetched tip is not a descendant"), "merge-base", "--is-ancestor", before, after)
}

func (tr *waitObservationTranscript) files(tip string, prefixes ...string) {
	tr.t.Helper()
	commit, ok := tr.commits[tip]
	if !ok {
		tr.t.Fatalf("file read from undeclared ledger commit %s", tip)
	}
	var paths []string
	for path := range commit.files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				paths = append(paths, path)
				break
			}
		}
	}
	slices.Sort(paths)
	args := append([]string{"ls-tree", "-r", "--name-only", tip, "--"}, prefixes...)
	tr.expect(strings.Join(paths, "\n")+"\n", args...)
	if len(paths) == 0 {
		return
	}
	var input, output strings.Builder
	for i, path := range paths {
		body := commit.files[path]
		fmt.Fprintf(&input, "%s:./%s\n", tip, path)
		fmt.Fprintf(&output, "%040x blob %d\n", i+1, len(body))
		output.Write(body)
		output.WriteByte('\n')
	}
	tr.steps = append(tr.steps, waitObservationStep{args: []string{"cat-file", "--batch"}, input: input.String(), output: output.String()})
}

func (tr *waitObservationTranscript) changes(before string, tips ...string) {
	tr.t.Helper()
	parent := before
	for _, tip := range tips {
		commit, ok := tr.commits[tip]
		if !ok || commit.parent != parent {
			tr.t.Fatalf("nonconsecutive declared ledger change %s after %s", tip, parent)
		}
		parent = tip
	}
	if len(tips) == 0 {
		tr.t.Fatal("a change range needs at least one declared commit")
	}
	tr.expect(strings.Join(tips, "\n")+"\n", "rev-list", "--reverse", "--first-parent", before+".."+tips[len(tips)-1])
	tr.expect(before+"\n", "rev-parse", "--verify", tips[0]+"^1")
}

func (tr *waitObservationTranscript) message(tip, body string) {
	tr.t.Helper()
	if _, ok := tr.commits[tip]; !ok {
		tr.t.Fatalf("message read from undeclared ledger commit %s", tip)
	}
	tr.expect(body, "show", "-s", "--format=%B", tip)
}

func (tr *waitObservationTranscript) dependencies() waitGitDependencies {
	return waitGitDependencies{
		run: tr.run,
		withTimeout: func(parent context.Context, _ time.Duration, args []string) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			if len(tr.steps) > 0 && tr.steps[0].cancel && reflect.DeepEqual(args, tr.expandedArgs(tr.steps[0].args)) {
				cancel()
			}
			return ctx, cancel
		},
		withFetchTimeout: func(parent context.Context, _ time.Duration, _ []string) (context.Context, context.CancelFunc) {
			return context.WithCancel(parent)
		},
	}
}

func (tr *waitObservationTranscript) expandedArgs(args []string) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		expanded[i] = strings.ReplaceAll(arg, waitCaptureRef, tr.ref)
	}
	return expanded
}

func (tr *waitObservationTranscript) run(ctx context.Context, root string, input []byte, args ...string) (string, error) {
	tr.t.Helper()
	tr.calls = append(tr.calls, append([]string(nil), args...))
	if root != tr.root || len(tr.steps) == 0 {
		err := fmt.Errorf("unexpected wait Git command: root=%q args=%q input=%q", root, args, input)
		tr.t.Error(err)
		return "", err
	}
	step := tr.steps[0]
	tr.steps = tr.steps[1:]
	if step.capture {
		tr.store.mu.Lock()
		canonical := tr.store.canonical
		tr.store.mu.Unlock()
		if canonical != step.captureTip {
			err := fmt.Errorf("captured ledger tip %s, want %s", canonical, step.captureTip)
			tr.t.Error(err)
			return "", err
		}
		const prefix = "+refs/heads/main:refs/metasystem/goals/fetch/read-"
		if len(args) != 5 || !strings.HasPrefix(args[4], prefix) || len(strings.TrimPrefix(args[4], prefix)) != 26 || strings.IndexFunc(strings.TrimPrefix(args[4], prefix), func(r rune) bool { return !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) }) >= 0 {
			err := fmt.Errorf("invalid temporary capture ref in %q", args)
			tr.t.Error(err)
			return "", err
		}
		tr.ref = strings.TrimPrefix(args[4], "+refs/heads/main:")
	}
	if !reflect.DeepEqual(args, tr.expandedArgs(step.args)) || string(input) != step.input {
		err := fmt.Errorf("wait Git command mismatch: got args=%q input=%q; want args=%q input=%q", args, input, tr.expandedArgs(step.args), step.input)
		tr.t.Error(err)
		return "", err
	}
	if step.cancel {
		if ctx.Err() == nil {
			err := fmt.Errorf("deadline stage %q ran with a live context", args)
			tr.t.Error(err)
			return "", err
		}
		tr.cancelled++
		return "", ctx.Err()
	}
	if ctx.Err() != nil {
		if step.err != nil && errors.Is(step.err, ctx.Err()) {
			return "", step.err
		}
		err := fmt.Errorf("command %q ran after cancellation: %w", args, ctx.Err())
		tr.t.Error(err)
		return "", err
	}
	return step.output, step.err
}

func (tr *waitObservationTranscript) done() {
	tr.t.Helper()
	if len(tr.steps) != 0 {
		tr.t.Errorf("unused required wait Git commands: %+v", tr.steps)
	}
}
