package goal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// BoundedCapture is one isolated canonical-tip fetch. OperationID names the
// temporary ref that the caller removes after validation and acceptance.
type BoundedCapture struct {
	Tip         string
	OperationID string
}

// LedgerChange is one consecutive first-parent ledger state. Consecutive is
// false when history cannot prove intermediate canonical states and the
// caller must treat the target as one direct accepted-world transition.
type LedgerChange struct {
	Tip         string
	Consecutive bool
}

const boundedCaptureGrace = 5 * time.Second

type waitGraceKey struct{}

// waitCaptureGrace sizes the cleanup grace a wait reserves after its reads
// from the wait's own deadline: the full grace when the deadline allows it,
// otherwise half of what remains, so a short wait still fetches within its
// deadline instead of spending it all on the reserve.
func waitCaptureGrace(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return boundedCaptureGrace
	}
	return min(boundedCaptureGrace, max(time.Until(deadline)/2, 0))
}

func withWaitGrace(ctx context.Context, grace time.Duration) context.Context {
	return context.WithValue(ctx, waitGraceKey{}, grace)
}

func waitGraceOf(ctx context.Context) time.Duration {
	if grace, ok := ctx.Value(waitGraceKey{}).(time.Duration); ok {
		return grace
	}
	return boundedCaptureGrace
}

type attentionGitRun func(context.Context, string, []byte, ...string) (string, error)

type waitGitContextFunc func(context.Context, time.Duration, []string) (context.Context, context.CancelFunc)

type waitGitDependencies struct {
	run              attentionGitRun
	withTimeout      waitGitContextFunc
	withFetchTimeout waitGitContextFunc
	timers           attentionTimerSource
}

type waitGitDependenciesKey struct{}

func withWaitGitDependencies(ctx context.Context, dependencies waitGitDependencies) context.Context {
	return context.WithValue(ctx, waitGitDependenciesKey{}, dependencies)
}

func waitDependencies(ctx context.Context) waitGitDependencies {
	dependencies, _ := ctx.Value(waitGitDependenciesKey{}).(waitGitDependencies)
	if dependencies.run == nil {
		dependencies.run = runAttentionGit
	}
	if dependencies.withTimeout == nil {
		dependencies.withTimeout = func(parent context.Context, budget time.Duration, _ []string) (context.Context, context.CancelFunc) {
			return context.WithTimeout(parent, budget)
		}
	}
	if dependencies.withFetchTimeout == nil {
		dependencies.withFetchTimeout = func(parent context.Context, budget time.Duration, _ []string) (context.Context, context.CancelFunc) {
			return context.WithTimeout(parent, budget)
		}
	}
	if dependencies.timers == nil {
		dependencies.timers = wallAttentionTimerSource{}
	}
	return dependencies
}

func runWaitGit(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
	return waitDependencies(ctx).run(ctx, root, stdin, args...)
}

func runAttentionGit(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
	return runAttentionGitWithEnvironment(ctx, root, stdin, nil, args...)
}

func runAttentionGitWithEnvironment(ctx context.Context, root string, stdin []byte, environment []string, args ...string) (string, error) {
	timers := waitDependencies(ctx).timers
	full := append([]string{"-C", root, "-c", "core.logAllRefUpdates=false"}, args...)
	var cmd *exec.Cmd
	if environment == nil {
		cmd = exec.Command("git", full...)
		cmd.Env = environWithoutGitSteering()
	} else {
		cmd = commandWithEnvironment(environment, "git", full...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start git %s: %w", strings.Join(args, " "), err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			return stdout.String(), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	case <-ctx.Done():
		pgid := cmd.Process.Pid
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		grace := timers.NewTimer(waitGraceOf(ctx))
		poll := timers.NewTicker(10 * time.Millisecond)
		defer grace.Stop()
		defer poll.Stop()
		waitDone := false
		for {
			groupGone, probeErr := captureProcessGroupGone(pgid)
			if probeErr != nil {
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					<-waited
				}
				return "", fmt.Errorf("inspect cancelled git process group: %w", probeErr)
			}
			if waitDone && groupGone {
				return "", ctx.Err()
			}
			select {
			case <-waited:
				waitDone = true
			case <-poll.C():
			case <-grace.C():
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					<-waited
				}
				return "", ctx.Err()
			}
		}
	}
}

func waitGit(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	budget := 10 * time.Second
	grace := waitCaptureGrace(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining-budget < grace {
			budget = remaining - grace
		}
	}
	if budget <= 0 {
		return "", context.DeadlineExceeded
	}
	readCtx, cancel := waitDependencies(ctx).withTimeout(withWaitGrace(ctx, grace), budget, args)
	defer cancel()
	return runWaitGit(readCtx, root, stdin, args...)
}

func resolveWaitEndpoint(ctx context.Context, root string) (Endpoint, error) {
	e := Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
	if out, err := waitGit(ctx, root, nil, "config", "--get", "goal.sync-remote"); err == nil && strings.TrimSpace(out) != "" {
		e.Remote = strings.TrimSpace(out)
	} else if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Endpoint{}, err
	}
	if out, err := waitGit(ctx, root, nil, "config", "--get", "goal.sync-branch"); err == nil && strings.TrimSpace(out) != "" {
		e.Branch = strings.TrimSpace(out)
	} else if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Endpoint{}, err
	}
	if !strings.HasPrefix(e.Branch, "refs/") {
		return Endpoint{}, fmt.Errorf("goal.sync-branch must be fully qualified (refs/...), got %q", e.Branch)
	}
	return e, nil
}

func validateWaitLandingDestination(ctx context.Context, root string, endpoint Endpoint) error {
	codeBranch := ""
	if out, err := waitGit(ctx, root, nil, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref"); err == nil && strings.TrimSpace(out) != "" {
		landingRef := strings.TrimSpace(out)
		tail := strings.TrimPrefix(landingRef, "refs/remotes/")
		parts := strings.SplitN(tail, "/", 2)
		if tail == landingRef || len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.HasPrefix(parts[1], "/") {
			return fmt.Errorf("the configured code landing ref %s is not refs/remotes/<remote>/<branch>", landingRef)
		}
		codeBranch = "refs/heads/" + parts[1]
	} else if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("the configured code landing branch cannot be read: %w", err)
		}
	}
	if codeBranch == "" {
		out, err := waitGit(ctx, root, nil, "symbolic-ref", "--quiet", "HEAD")
		if err != nil {
			return fmt.Errorf("the code landing branch cannot be resolved from the checkout: %w", err)
		}
		codeBranch = strings.TrimSpace(out)
	}
	ledgerBranch := endpoint.Branch
	if endpoint.LocalMode() {
		ledgerBranch = LocalLedgerBranch
	}
	if codeBranch != ledgerBranch {
		return fmt.Errorf("landings go to %s; this ledger endpoint watches %s", codeBranch, ledgerBranch)
	}
	return nil
}

func captureWaitTip(ctx context.Context, e Endpoint, budget time.Duration) (BoundedCapture, error) {
	if budget <= 0 {
		return BoundedCapture{}, fmt.Errorf("goal ledger fetch budget must be positive")
	}
	ulid, err := NewOperationULID()
	if err != nil {
		return BoundedCapture{}, err
	}
	opid := "read-" + ulid
	fail := func(cause error) (BoundedCapture, error) {
		cleanupWaitTip(ctx, e, opid)
		return BoundedCapture{}, cause
	}
	fetchArgs := []string{"fetch", "--no-tags", "--refmap=", e.Remote, "+" + e.Branch + ":" + fetchRefFor(opid)}
	fetchCtx, cancel := waitDependencies(ctx).withFetchTimeout(ctx, budget, fetchArgs)
	defer cancel()
	if e.LocalMode() {
		tip, readErr := runWaitGit(fetchCtx, e.Root, nil, "rev-parse", "--verify", LocalLedgerBranch)
		if readErr != nil && fetchCtx.Err() == nil {
			tip, readErr = runWaitGit(fetchCtx, e.Root, nil, "rev-parse", "--verify", "HEAD")
		}
		if readErr != nil {
			return fail(readErr)
		}
		return BoundedCapture{Tip: strings.TrimSpace(tip), OperationID: opid}, nil
	}
	_, err = runWaitGit(fetchCtx, e.Root, nil, fetchArgs...)
	if err != nil {
		return fail(err)
	}
	tip, err := runWaitGit(fetchCtx, e.Root, nil, "rev-parse", "--verify", fetchRefFor(opid))
	if err != nil {
		return fail(err)
	}
	return BoundedCapture{Tip: strings.TrimSpace(tip), OperationID: opid}, nil
}

func cleanupWaitTip(parent context.Context, e Endpoint, opid string) {
	if e.LocalMode() || opid == "" {
		return
	}
	grace := waitGraceOf(parent)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	cleanupCtx = withWaitGrace(withWaitGitDependencies(cleanupCtx, waitDependencies(parent)), grace)
	_, _ = runWaitGit(cleanupCtx, e.Root, nil, "update-ref", "-d", fetchRefFor(opid))
}

func waitTreeIdentity(ctx context.Context, root, commit string) (string, error) {
	out, err := waitGit(ctx, root, nil, "cat-file", "-p", commit+":./"+goalsPrefix+"backlog.md")
	if err != nil {
		return "", fmt.Errorf("no root record at %s: %w", short(commit), err)
	}
	record, problems := ParseRoot([]byte(out))
	if len(problems) > 0 || record.Identity == "" {
		return "", fmt.Errorf("the root record at %s does not parse to an identity", short(commit))
	}
	return record.Identity, nil
}

func waitAcceptanceGates(ctx context.Context, root, accepted, fetched string) error {
	acceptedIdentity, err := waitTreeIdentity(ctx, root, accepted)
	if err != nil {
		return fmt.Errorf("the accepted tree's identity cannot be read: %w", err)
	}
	fetchedIdentity, fetchedErr := waitTreeIdentity(ctx, root, fetched)
	if fetchedErr != nil && (ctx.Err() != nil || errors.Is(fetchedErr, context.DeadlineExceeded) || errors.Is(fetchedErr, context.Canceled)) {
		return fetchedErr
	}
	if fetchedIdentity != "" && fetchedIdentity != acceptedIdentity {
		return fmt.Errorf("foreign ledger refused: the fetched tree's identity %s is not this ledger's %s — config cannot silently change what the ledger is", fetchedIdentity, acceptedIdentity)
	}
	if _, err := waitGit(ctx, root, nil, "merge-base", "--is-ancestor", accepted, fetched); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("rewound canonical branch refused: %s does not descend from the accepted tip %s; the projection stays pinned — repair --accept-remote is the deliberate path", short(fetched), short(accepted))
	}
	out, err := waitGit(ctx, root, nil, "diff", "--name-status", "--no-renames", accepted, fetched, "--", legacyDonePrefix)
	if err != nil {
		return fmt.Errorf("cannot compare the legacy concluded-goal location: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		status, path, found := strings.Cut(line, "\t")
		if !found {
			return fmt.Errorf("cannot classify legacy concluded-goal change %q", line)
		}
		switch status {
		case "D":
		case "A":
			return fmt.Errorf("%s: the legacy concluded-goal location is read-only; new conclusions belong under %s", path, recordsGoalsRoot)
		default:
			return fmt.Errorf("%s: the legacy concluded-goal location is read-only; reopen or prune the standing record through its verb", path)
		}
	}
	return nil
}

func readWaitFiles(ctx context.Context, root, commit string, prefixes ...string) (map[string][]byte, error) {
	args := []string{"ls-tree", "-r", "--name-only", commit, "--"}
	args = append(args, prefixes...)
	out, err := waitGit(ctx, root, nil, args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	files := make(map[string][]byte, len(paths))
	if len(paths) == 0 {
		return files, nil
	}
	var input strings.Builder
	for _, path := range paths {
		input.WriteString(commit + ":./" + path + "\n")
	}
	out, err = waitGit(ctx, root, []byte(input.String()), "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("read ledger blobs at %s: %w", commit, err)
	}
	reader := bufio.NewReader(strings.NewReader(out))
	for _, path := range paths {
		header, readErr := reader.ReadString('\n')
		if readErr != nil {
			return nil, fmt.Errorf("cannot read %s at %s: malformed git cat-file header: %w", path, commit, readErr)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("cannot read %s at %s: unexpected git cat-file header %q", path, commit, strings.TrimSpace(header))
		}
		size, parseErr := strconv.ParseInt(fields[2], 10, 64)
		if parseErr != nil || size < 0 {
			return nil, fmt.Errorf("cannot read %s at %s: invalid git cat-file size %q", path, commit, fields[2])
		}
		content := make([]byte, size)
		if _, readErr := io.ReadFull(reader, content); readErr != nil {
			return nil, fmt.Errorf("cannot read %s at %s: truncated git cat-file content: %w", path, commit, readErr)
		}
		if separator, readErr := reader.ReadByte(); readErr != nil || separator != '\n' {
			return nil, fmt.Errorf("cannot read %s at %s: malformed git cat-file separator", path, commit)
		}
		files[path] = content
	}
	return files, nil
}

func validateWaitChannelFiles(files, goalFiles map[string][]byte) []Problem {
	goalIDs := channelGoalIDs(goalFiles)
	var problems []Problem
	questions := make(map[string]channelQuestionAtPath)
	var inbounds []channelInboundAtPath
	for _, filePath := range sortedKeys(files) {
		location, ok := classifyChannelPath(filePath)
		if !ok {
			problems = append(problems, channelProblem("channel-unknown-path", filePath, "the path is not a question, inbox record, or listener record"))
			continue
		}
		switch location.kind {
		case "question":
			q, raw, err := decodeChannelQuestion(files[filePath])
			if err != nil {
				problems = append(problems, channelProblem("channel-json", filePath, err.Error()))
				continue
			}
			questions[location.id] = channelQuestionAtPath{path: filePath, raw: raw, q: q}
			problems = append(problems, validateChannelQuestion(filePath, location.id, q, raw, goalIDs)...)
		case "inbound":
			in, err := decodeChannelInbound(files[filePath])
			if err != nil {
				problems = append(problems, channelProblem("channel-json", filePath, err.Error()))
				continue
			}
			inbounds = append(inbounds, channelInboundAtPath{path: filePath, in: in})
			problems = append(problems, validateChannelInbound(filePath, location, in)...)
		case "listener":
			listener, err := decodeChannelListener(files[filePath])
			if err != nil {
				problems = append(problems, channelProblem("channel-json", filePath, err.Error()))
				continue
			}
			problems = append(problems, validateChannelListener(filePath, location.id, listener)...)
		}
	}
	for _, record := range inbounds {
		in := record.in
		if in.Outcome != "verified" || in.Question == "unbound" || in.Question == "unmatched" {
			continue
		}
		question, exists := questions[in.Question]
		if exists && question.q.Lineage == "migrated" {
			continue
		}
		if !exists || question.q.Answer == nil || question.q.Answer.InboxID != channelInboxID(in) {
			problems = append(problems, channelProblem("channel-answer-state", record.path, fmt.Sprintf("verified inbox record %s is not the recorded answer of question %s", channelInboxID(in), in.Question)))
		}
	}
	return problems
}

func validateWaitCommit(ctx context.Context, root, commit string) (*TreeGoals, error) {
	allFiles, err := readWaitFiles(ctx, root, commit, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	if err != nil {
		return nil, err
	}
	goalFiles := map[string][]byte{}
	channelFiles := map[string][]byte{}
	for path, body := range allFiles {
		if strings.HasPrefix(path, ChannelPrefix) {
			channelFiles[path] = body
		} else {
			goalFiles[path] = body
		}
	}
	tree, problems := ParseTreeFiles(goalFiles)
	if len(problems) == 0 {
		problems = ValidateTree(tree)
	}
	if len(problems) == 0 {
		problems = append(problems, validateWaitChannelFiles(channelFiles, goalFiles)...)
	}
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, problem := range problems {
			lines[i] = string(problem)
		}
		return nil, fmt.Errorf("the ledger tree at %s does not validate:\n%s", commit, strings.Join(lines, "\n"))
	}
	return tree, nil
}

func waitLedgerChanges(ctx context.Context, root, before, after string) ([]LedgerChange, error) {
	if before == after {
		return nil, nil
	}
	out, err := waitGit(ctx, root, nil, "rev-list", "--reverse", "--first-parent", before+".."+after)
	if err != nil {
		return nil, fmt.Errorf("walk accepted ledger changes: %w", err)
	}
	var tips []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if tip := strings.TrimSpace(line); tip != "" {
			tips = append(tips, tip)
		}
	}
	if len(tips) == 0 {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	parent, err := waitGit(ctx, root, nil, "rev-parse", "--verify", tips[0]+"^1")
	if err != nil {
		return nil, fmt.Errorf("read first parent of accepted ledger change: %w", err)
	}
	if strings.TrimSpace(parent) != before {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	changes := make([]LedgerChange, 0, len(tips))
	for _, tip := range tips {
		changes = append(changes, LedgerChange{Tip: tip, Consecutive: true})
	}
	return changes, nil
}

func projectWaitAt(ctx context.Context, root, tip string) (Projection, error) {
	files, err := readWaitFiles(ctx, root, tip, goalsPrefix, recordsGoalsPrefix)
	if err != nil {
		return Projection{}, err
	}
	tree, problems := ParseTreeFiles(files)
	if len(problems) > 0 {
		return Projection{}, &TreeReadError{Tip: tip, Problems: problems, Files: files}
	}
	return Projection{Root: root, Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, time.Now().UTC())}, nil
}

// WaitClaimableGoals returns the revisioned claimable frontier from the
// waiter's already validated private tip. The caller compares it with the
// registration snapshot; this function never turns an existing backlog into
// an event.
func WaitClaimableGoals(ctx context.Context, root, tip, waitingGoal, ownerLineage string) ([]metarun.ClaimableGoal, error) {
	projection, err := projectWaitAt(ctx, root, tip)
	if err != nil {
		return nil, err
	}
	machine, err := ResolveMachine(root)
	if err != nil {
		return nil, err
	}
	frontier, err := Next(projection, machine)
	if err != nil {
		return nil, err
	}
	goals := make([]metarun.ClaimableGoal, 0, len(frontier.Ready))
	for _, id := range frontier.Ready {
		if id == waitingGoal {
			continue
		}
		file := projection.Tree.Live[id]
		if file == nil || (file.Claimed != nil && file.Claimed.Lineage == ownerLineage) {
			continue
		}
		goals = append(goals, metarun.ClaimableGoal{ID: id, Revision: file.Revision})
	}
	return goals, nil
}

func waitClaimableGoalsFromTree(ctx context.Context, root, tip string, tree *TreeGoals, waitingGoal, ownerLineage string) ([]metarun.ClaimableGoal, error) {
	out, err := waitGit(ctx, root, nil, "config", "--get", "metasystem.goal.machine")
	if err != nil || strings.TrimSpace(out) == "" {
		if err != nil && (ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			return nil, err
		}
		return nil, fmt.Errorf("the claimable frontier cannot be read because this checkout has no enrolled machine nickname")
	}
	projection := Projection{Root: root, Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, time.Now().UTC())}
	frontier, err := Next(projection, strings.TrimSpace(out))
	if err != nil {
		return nil, err
	}
	goals := make([]metarun.ClaimableGoal, 0, len(frontier.Ready))
	for _, id := range frontier.Ready {
		if id == waitingGoal {
			continue
		}
		file := tree.Live[id]
		if file == nil || (file.Claimed != nil && file.Claimed.Lineage == ownerLineage) {
			continue
		}
		goals = append(goals, metarun.ClaimableGoal{ID: id, Revision: file.Revision})
	}
	return goals, nil
}

// AcceptedLedgerTip distinguishes an absent pre-bootstrap world from a broken
// accepted ref and from a readable migrated ledger.
func AcceptedLedgerTip(root string) (tip string, exists bool, err error) {
	tip, resolved, err := acceptedTipForGates(root)
	if err != nil || !resolved {
		return tip, false, err
	}
	hasLedger, err := tipCarriesLedger(Endpoint{Root: root}, tip)
	if err != nil {
		return "", false, err
	}
	if !hasLedger {
		return tip, false, nil
	}
	return tip, true, nil
}

// ProjectAt reads one already-identified commit without moving any ref.
func ProjectAt(root, tip string, at ...time.Time) (Projection, error) {
	return projectAtForEndpoint(Endpoint{Root: root}, tip, at...)
}

// ProjectAtEndpoint reads one already-identified commit through the endpoint's repository.
func ProjectAtEndpoint(endpoint Endpoint, tip string, at ...time.Time) (Projection, error) {
	return projectAtForEndpoint(endpoint, tip, at...)
}

func projectAtForEndpoint(endpoint Endpoint, tip string, at ...time.Time) (Projection, error) {
	tree, err := loadTreeFor(endpoint, tip)
	if err != nil {
		return Projection{}, err
	}
	now := time.Now().UTC()
	if len(at) > 0 {
		now = at[0]
	}
	return Projection{Root: endpoint.Root, Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, now)}, nil
}

// LedgerChanges returns the proven consecutive first-parent states from
// before to after. A merge that reaches before through another parent, or a
// sanctioned accepted-ref rewind, is represented as one direct transition;
// inventing intermediate canonical states would surface changes that may
// never have occupied the shared branch.
func LedgerChanges(root, before, after string) ([]LedgerChange, error) {
	return ledgerChangesWithReader(root, before, after, goalGit)
}

func ledgerChangesWithReader(root, before, after string, reader func(string, []string, ...string) (string, error)) ([]LedgerChange, error) {
	if before == after {
		return nil, nil
	}
	out, err := reader(root, nil, "rev-list", "--reverse", "--first-parent", before+".."+after)
	if err != nil {
		return nil, fmt.Errorf("walk accepted ledger changes: %w", err)
	}
	var tips []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if tip := strings.TrimSpace(line); tip != "" {
			tips = append(tips, tip)
		}
	}
	if len(tips) == 0 {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	parent, parentErr := reader(root, nil, "rev-parse", "--verify", tips[0]+"^1")
	if parentErr != nil {
		return nil, fmt.Errorf("read first parent of accepted ledger change: %w", parentErr)
	}
	if strings.TrimSpace(parent) != before {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	changes := make([]LedgerChange, 0, len(tips))
	for _, tip := range tips {
		changes = append(changes, LedgerChange{Tip: tip, Consecutive: true})
	}
	return changes, nil
}

// IsAncestor reports whether ancestor is equal to or precedes descendant.
func IsAncestor(root, ancestor, descendant string) (bool, error) {
	return isAncestorWithEnvironment(root, ancestor, descendant, nil)
}

func isAncestorWithEnvironment(root, ancestor, descendant string, environment []string) (bool, error) {
	if ancestor == descendant {
		return true, nil
	}
	_, err := goalGitWithEnvironment(root, environment, nil, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// CaptureTipBounded fetches the canonical branch on an owned process group.
// A deadline kills the whole group before returning, so a transport helper
// cannot outlive the steward pass or advance anything beyond its temporary
// ref.
func CaptureTipBounded(e Endpoint, budget time.Duration) (BoundedCapture, error) {
	if budget <= 0 {
		return BoundedCapture{}, fmt.Errorf("goal ledger fetch budget must be positive")
	}
	ulid, err := NewOperationULID()
	if err != nil {
		return BoundedCapture{}, err
	}
	opid := "read-" + ulid
	fail := func(cause error) (BoundedCapture, error) {
		CleanupRefs(e, opid)
		return BoundedCapture{}, cause
	}
	if e.LocalMode() {
		tip, err := captureLocalTipBounded(e, budget)
		if err != nil {
			return fail(err)
		}
		return BoundedCapture{Tip: tip, OperationID: opid}, nil
	}

	args := []string{
		"-C", e.Root, "-c", "core.logAllRefUpdates=false",
		"fetch", "--no-tags", "--refmap=", e.Remote,
		"+" + e.Branch + ":" + fetchRefFor(opid),
	}
	cmd := commandWithEnvironment(e.commandEnv, "git", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fail(fmt.Errorf("start goal ledger fetch: %w", err))
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timers := e.captureTimers
	if timers == nil {
		timers = wallAttentionTimerSource{}
	}
	timer := timers.NewTimer(budget)
	defer timer.Stop()
	select {
	case waitErr := <-waited:
		if waitErr != nil {
			return fail(fmt.Errorf("git fetch --no-tags --refmap=: %v (%s)", waitErr, strings.TrimSpace(stderr.String())))
		}
	case <-timer.C():
		pgid := cmd.Process.Pid
		termErr := syscall.Kill(-pgid, syscall.SIGTERM)
		grace := timers.NewTimer(boundedCaptureGrace)
		poll := timers.NewTicker(10 * time.Millisecond)
		defer grace.Stop()
		defer poll.Stop()
		waitDone := false
		var waitErr error
		for {
			groupGone, probeErr := captureProcessGroupGone(pgid)
			if probeErr != nil {
				killErr := syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					waitErr = <-waited
				}
				return fail(fmt.Errorf("goal ledger fetch timed out and its process group could not be inspected: %v (TERM: %v, KILL: %v, wait: %v)", probeErr, termErr, killErr, waitErr))
			}
			if waitDone && groupGone {
				return fail(fmt.Errorf("goal ledger fetch timed out after %s", budget))
			}
			select {
			case waitErr = <-waited:
				waitDone = true
			case <-poll.C():
				// Recheck the process-group condition above; cooperative Git
				// transports return without spending the grace ceiling.
			case <-grace.C():
				killErr := syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					waitErr = <-waited
				}
				if killErr != nil && killErr != syscall.ESRCH {
					return fail(fmt.Errorf("goal ledger fetch timed out and its process group could not be killed after %s grace: %v (TERM: %v, wait: %v)", boundedCaptureGrace, killErr, termErr, waitErr))
				}
				return fail(fmt.Errorf("goal ledger fetch timed out after %s", budget))
			}
		}
	}

	tipOut, err := goalGitWithEnvironment(e.Root, e.commandEnv, nil, "rev-parse", "--verify", fetchRefFor(opid))
	if err != nil {
		return fail(err)
	}
	return BoundedCapture{Tip: strings.TrimSpace(tipOut), OperationID: opid}, nil
}

func captureLocalTipBounded(e Endpoint, budget time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	read := func(ref string) (string, error) {
		base := commandWithEnvironment(e.commandEnv, "git", "-C", e.Root, "-c", "core.logAllRefUpdates=false", "rev-parse", "--verify", ref)
		cmd := exec.CommandContext(ctx, base.Path, base.Args[1:]...)
		cmd.Env = base.Env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("local goal ledger read timed out after %s", budget)
			}
			return "", fmt.Errorf("read local goal ledger: %v (%s)", err, strings.TrimSpace(stderr.String()))
		}
		return strings.TrimSpace(stdout.String()), nil
	}
	if tip, err := read(LocalLedgerBranch); err == nil {
		return tip, nil
	}
	return read("HEAD")
}

func captureProcessGroupGone(pgid int) (bool, error) {
	err := syscall.Kill(-pgid, 0)
	if err == nil || errors.Is(err, syscall.EPERM) {
		return false, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	return false, err
}

var (
	landingProvenanceRe         = regexp.MustCompile(`^chain=([^ ]+) change=([0-9a-f]{64})(?: .*)?$`)
	directFixProvenanceRe       = regexp.MustCompile(`^direct-fix [^\n]*\bchange=([0-9a-f]{64})(?: |$)`)
	carriedLandingProvenanceRe  = regexp.MustCompile(`^carried opid=[^ ]+(?: .*)?$`)
	attestedLandingProvenanceRe = regexp.MustCompile(`^attested=[0-9a-f]{40} goal=[^ ]+ unit=[^ ]+ critic=[^ /]+/[1-9][0-9]* change=[0-9a-f]{64}$`)
)

// ObserveLedger validates a private canonical tip from the original cursor
// and evaluates one landing or human-history predicate. It never advances the
// accepted ref and never substitutes a newer cursor for the one registered.
func ObserveLedger(ctx context.Context, root string, selector metarun.WaitSelector, pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, error) {
	return ObserveLedgerForWait(ctx, root, selector, pinned, lastTip, "")
}

// ObserveLedgerForWait keeps the claimable frontier in the same private-tip
// lifetime as event inspection, so cleanup cannot invalidate the actionable
// read and every Git process remains under the observation context.
func ObserveLedgerForWait(ctx context.Context, root string, selector metarun.WaitSelector, pinned metarun.WaiterTarget, lastTip, ownerLineage string) (metarun.SourceObservation, error) {
	select {
	case <-ctx.Done():
		return metarun.SourceObservation{Temporary: true}, ctx.Err()
	default:
	}
	endpoint, err := resolveWaitEndpoint(ctx, root)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return metarun.SourceObservation{Temporary: true}, err
		}
		return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "the configured goal endpoint is invalid", Outcome: "invalid-source"}, nil
	}
	if selector.Event == "landing" && pinned == (metarun.WaiterTarget{}) {
		if err := validateWaitLandingDestination(ctx, root, endpoint); err != nil {
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return metarun.SourceObservation{Temporary: true}, err
			}
			return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: err.Error(), Outcome: "invalid-source"}, nil
		}
	}
	budget := 10 * time.Second
	grace := waitCaptureGrace(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline) - grace
		if remaining < budget {
			budget = remaining
		}
	}
	if budget <= 0 {
		return metarun.SourceObservation{Temporary: true}, fmt.Errorf("not enough wait budget remains for goal fetch and cleanup")
	}
	captured, err := captureWaitTip(withWaitGrace(ctx, grace), endpoint, budget)
	if err != nil {
		return metarun.SourceObservation{Temporary: true}, err
	}
	defer cleanupWaitTip(withWaitGrace(ctx, grace), endpoint, captured.OperationID)
	incarnation := metarun.WaiterTarget{StartedAt: "ledger:" + selector.After, ProofDigest: ledgerEndpointIdentity(endpoint)}
	if pinned != (metarun.WaiterTarget{}) && lastTip == captured.Tip {
		return metarun.SourceObservation{Pending: true, Incarnation: incarnation, Outcome: "pending", Evidence: "ledger:" + captured.Tip, LedgerTip: captured.Tip}, nil
	}
	changeFloor := lastTip
	if changeFloor == "" {
		changeFloor = selector.After
	}
	// A successfully recorded last tip is itself an accepted projection. Using
	// it as the next gate floor preserves the original selector as the match
	// floor without re-reading ledger states already inspected by this wait.
	if err := waitAcceptanceGates(ctx, root, changeFloor, captured.Tip); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return metarun.SourceObservation{Temporary: true}, err
		}
		return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
	}
	validatedTree, err := validateWaitCommit(ctx, root, captured.Tip)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return metarun.SourceObservation{Temporary: true}, err
		}
		return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "the fetched ledger tip is invalid: " + err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
	}
	changes, err := waitLedgerChanges(ctx, root, changeFloor, captured.Tip)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return metarun.SourceObservation{Temporary: true}, err
		}
		return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
	}
	observation := metarun.SourceObservation{Pending: true, Incarnation: incarnation, Outcome: "pending", Evidence: "ledger:" + captured.Tip, LedgerTip: captured.Tip}
	if selector.Event == "human-act" {
		observation.ClaimableGoals, err = waitClaimableGoalsFromTree(ctx, root, captured.Tip, validatedTree, selector.GoalID, ownerLineage)
		if err != nil {
			return metarun.SourceObservation{Temporary: true, Incarnation: incarnation, LedgerTip: captured.Tip}, err
		}
		observation.ClaimableRead = true
	}
	previous := Projection{Root: root, Tip: captured.Tip, Tree: validatedTree, Horizon: approvalHorizon(validatedTree, time.Now().UTC())}
	if changeFloor != captured.Tip {
		previous, err = projectWaitAt(ctx, root, changeFloor)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return metarun.SourceObservation{Temporary: true, Incarnation: incarnation, LedgerTip: captured.Tip}, err
			}
			return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "the last checked ledger tip is unreadable: " + err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
		}
	}
	if goalFileAt(previous.Tree, selector.GoalID) == nil {
		return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "the goal did not exist at the saved ledger cursor", Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
	}
	for _, change := range changes {
		if !change.Consecutive {
			return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "accepted ledger history is not consecutive from the saved cursor", Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: captured.Tip}, nil
		}
		if selector.Event == "landing" {
			matched, provenance, err := landingWaitAt(ctx, root, change.Tip, selector.GoalID, selector.Chain)
			if err != nil {
				return metarun.SourceObservation{Temporary: true, Incarnation: incarnation, LedgerTip: change.Tip}, err
			}
			if matched {
				observation.Pending = false
				observation.ExitCode = metarun.ExitGreen
				observation.Reason = "matching goal landing became reachable"
				observation.Outcome = "landing"
				observation.Evidence = "commit:" + change.Tip + ":" + provenance
				observation.LedgerTip = change.Tip
				return observation, nil
			}
		} else {
			current := Projection{Root: root, Tip: captured.Tip, Tree: validatedTree, Horizon: approvalHorizon(validatedTree, time.Now().UTC())}
			if change.Tip != captured.Tip {
				current, err = projectWaitAt(ctx, root, change.Tip)
				if err != nil {
					if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						return metarun.SourceObservation{Temporary: true, Incarnation: incarnation, LedgerTip: change.Tip}, err
					}
					return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "an intervening accepted ledger state is unreadable: " + err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: change.Tip}, nil
				}
			}
			rows, err := appendedGoalHistory(previous.Tree, current.Tree, selector.GoalID)
			if err != nil {
				return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: err.Error(), Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: change.Tip}, nil
			}
			for _, row := range rows {
				if !acceptedHumanAct(row) || (selector.Verb != "" && row.Verb != selector.Verb) || (selector.Question != "" && row.Question != selector.Question) {
					continue
				}
				if row.Verb != "answer" {
					transaction, transactionErr := goalTransactionWaitAt(ctx, root, change.Tip)
					if transactionErr != nil {
						return metarun.SourceObservation{Temporary: true, Incarnation: incarnation, LedgerTip: change.Tip}, transactionErr
					}
					if transaction != row.Opid {
						return metarun.SourceObservation{ExitCode: metarun.ExitNoRecord, Reason: "the matching History operation does not join the commit's Goal-Transaction trailer", Outcome: "invalid-source", Incarnation: incarnation, LedgerTip: change.Tip}, nil
					}
				}
				observation.Pending = false
				observation.ExitCode = metarun.ExitGreen
				observation.Reason = fmt.Sprintf("accepted human act %s matched operation %s", row.Verb, row.Opid)
				observation.Outcome = row.Verb
				observation.Evidence = "ledger:" + change.Tip + ":operation:" + row.Opid
				observation.LedgerTip = change.Tip
				observation.TerminalStamp = row.At
				return observation, nil
			}
			previous = current
		}
	}
	return observation, nil
}

func goalTransactionWaitAt(ctx context.Context, root, tip string) (string, error) {
	output, err := waitGit(ctx, root, nil, "show", "-s", "--format=%(trailers:key=Goal-Transaction,valueonly)", tip)
	if err != nil {
		return "", fmt.Errorf("read Goal-Transaction trailer at %s: %w", short(tip), err)
	}
	fields := strings.Fields(output)
	if len(fields) != 1 {
		return "", nil
	}
	return fields[0], nil
}

func ledgerEndpointIdentity(endpoint Endpoint) string {
	sum := sha256.Sum256([]byte(endpoint.Root + "\x00" + endpoint.Remote + "\x00" + endpoint.Branch))
	return fmt.Sprintf("%x", sum[:])
}

func goalFileAt(tree *TreeGoals, goalID string) *GoalFile {
	if tree == nil {
		return nil
	}
	if file := tree.Live[goalID]; file != nil {
		return file
	}
	return tree.Done[goalID]
}

func appendedGoalHistory(before, after *TreeGoals, goalID string) ([]HistoryLine, error) {
	prior, current := goalFileAt(before, goalID), goalFileAt(after, goalID)
	if current == nil {
		return nil, fmt.Errorf("goal %s vanished from accepted ledger history", goalID)
	}
	priorLength := 0
	if prior != nil {
		priorLength = len(prior.History)
		if len(current.History) < priorLength {
			return nil, fmt.Errorf("goal %s history was shortened", goalID)
		}
		for i := 0; i < priorLength; i++ {
			if RenderHistoryLine(prior.History[i]) != RenderHistoryLine(current.History[i]) {
				return nil, fmt.Errorf("goal %s history was rewritten", goalID)
			}
		}
	}
	return current.History[priorLength:], nil
}

func acceptedHumanAct(row HistoryLine) bool {
	if row.AuthorityOutcome == AuthorityOutcomePowerOfAttorney {
		return false
	}
	if strings.HasPrefix(row.Actor, "human:") {
		return true
	}
	switch row.AuthorityOutcome {
	case AuthorityOutcomeAuthenticatedChannelWord, AuthorityOutcomeVerifiedChannelAnswer, AuthorityOutcomeTemporaryHumanWord,
		AuthorityOutcomeHumanAuthorityProven, AuthorityOutcomeSignedInSession:
		return true
	}
	return false
}

func landingAt(ctx context.Context, root, tip, goalID, chain string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "-c", "core.logAllRefUpdates=false", "show", "-s", "--format=%B", tip)
	cmd.Env = environWithoutGitSteering()
	output, err := cmd.Output()
	if err != nil {
		return false, "", fmt.Errorf("read landing commit %s: %w", short(tip), err)
	}
	return matchLandingMessage(string(output), goalID, chain)
}

func landingWaitAt(ctx context.Context, root, tip, goalID, chain string) (bool, string, error) {
	output, err := waitGit(ctx, root, nil, "show", "-s", "--format=%B", tip)
	if err != nil {
		return false, "", fmt.Errorf("read landing commit %s: %w", short(tip), err)
	}
	matched, provenance, err := matchLandingMessage(output, goalID, chain)
	if matched || err != nil || chain != "" {
		return matched, provenance, err
	}
	return VerifiedGoalBranchLanding(ctx, root, tip, goalID)
}

// GoalBranchLandingVerifier verifies that commit is the last landing of
// goalID from its goal branch and returns that landing's provenance. The goal
// branch owner registers it; this package cannot import that owner.
type GoalBranchLandingVerifier func(ctx context.Context, root, commit, goalID string) (bool, string, error)

var goalBranchLandingVerifier GoalBranchLandingVerifier

func RegisterGoalBranchLandingVerifier(verifier GoalBranchLandingVerifier) {
	goalBranchLandingVerifier = verifier
}

// VerifiedGoalBranchLanding asks the registered goal branch owner whether
// commit is goalID's verified last landing; without an owner nothing matches.
func VerifiedGoalBranchLanding(ctx context.Context, root, commit, goalID string) (bool, string, error) {
	if goalBranchLandingVerifier == nil {
		return false, "", nil
	}
	return goalBranchLandingVerifier(ctx, root, commit, goalID)
}

func matchLandingMessage(message, goalID, chain string) (bool, string, error) {
	goalMatches := 0
	var provenance string
	var provenanceVerdict string
	for _, line := range strings.Split(message, "\n") {
		if strings.TrimSpace(line) == "Goal-Item: "+goalID {
			goalMatches++
		}
		if strings.HasPrefix(line, "Landing-Provenance: ") {
			if provenance != "" {
				return false, "", nil
			}
			provenance = strings.TrimPrefix(line, "Landing-Provenance: ")
		}
		if strings.HasPrefix(line, "Landing-Provenance-Verdict: ") {
			if provenanceVerdict != "" {
				return false, "", nil
			}
			provenanceVerdict = strings.TrimPrefix(line, "Landing-Provenance-Verdict: ")
		}
	}
	if goalMatches != 1 || provenance == "" || (provenanceVerdict != "pass" && !strings.HasPrefix(provenanceVerdict, "pass ")) {
		return false, "", nil
	}
	match := landingProvenanceRe.FindStringSubmatch(provenance)
	if chain != "" {
		if len(match) != 3 || match[1] != chain {
			return false, "", nil
		}
		return true, provenance, nil
	}
	if len(match) != 3 && !directFixProvenanceRe.MatchString(provenance) && !carriedLandingProvenanceRe.MatchString(provenance) && !attestedLandingProvenanceRe.MatchString(provenance) {
		return false, "", nil
	}
	return true, provenance, nil
}
