package batch

// landing begin (lane runtime design r10 K4): the landing agent composes a
// batch's members on a base B into a linear series B..C in the lane
// checkout; begin checks that series against every member's pinned
// contribution, writes it again as the canonical series carrying the
// landing metadata (held's ownership and revision trailers, recovery's
// provenance trailers), and durably records {batch, op id, members, B, C,
// candidate, tree} before anything executes. What is proven is what is
// published: prove and publish read only the recorded opening.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

// The trailers the agent marks its own commits with (K4): a member replay
// carrying its resolution, and the one final integration commit.
const (
	LaneResolvedTrailer    = "Lane-Resolved"
	LaneIntegrationTrailer = "Lane-Integration"
)

// SeamCapLines is the one aggregate cap over every agent-authored deviation
// of a series (Wido D3: about 40 lines).
const SeamCapLines = 40

// The kinds of commit a canonical series holds (K4 (a), (b), (c)).
const (
	SeriesReplay      = "replay"
	SeriesResolved    = "resolved"
	SeriesIntegration = "integration"
)

// The kinds of composition evidence begin records (K8): what a conflict or
// seam-too-large return of a member stands on.
const (
	CompositionConflict     = "conflict"
	CompositionSeamTooLarge = "seam-too-large"
)

// Codes of begin's refusals: a series it cannot take, and the two
// composition refusals it records as evidence.
const (
	CodeBeginRefused      = "LANE_BEGIN_REFUSED"
	CodeBeginConflict     = "LANE_BEGIN_CONFLICT"
	CodeBeginSeamTooLarge = "LANE_BEGIN_SEAM_TOO_LARGE"
)

// Opening is one durable landing begin: the canonical series of a batch.
type Opening struct {
	OpID    string   `json:"opId"`
	At      string   `json:"at"`
	Actor   string   `json:"actor"`
	Members []string `json:"members"`
	// Base is B, the commit the series starts on, and BaseTree its tree.
	Base     string `json:"base"`
	BaseTree string `json:"baseTree"`
	// Head is the agent's composed tip C; Candidate is the canonical series'
	// tip, the commit that is proven and published, and Tree their one tree.
	Head      string         `json:"head"`
	Candidate string         `json:"candidate"`
	Tree      string         `json:"tree"`
	Series    []SeriesCommit `json:"series"`
	// Deviation is the aggregate agent-authored deviation, in lines.
	Deviation int `json:"deviation"`
}

// SeriesCommit is one commit of a canonical series.
type SeriesCommit struct {
	Commit string `json:"commit"`
	// Source is the agent's commit it was written from.
	Source string `json:"source"`
	Kind   string `json:"kind"`
	// Member and Pin name the member and the pinned step it replays: the
	// change commit, the build commit, or the chain id. Empty for the
	// integration commit.
	Member    string `json:"member,omitempty"`
	Pin       string `json:"pin,omitempty"`
	Deviation int    `json:"deviation,omitempty"`
	Resolved  string `json:"resolved,omitempty"`
}

// CompositionEvidence is begin's recorded composition refusal: the members
// it names go back to their seats as Kind (K8).
type CompositionEvidence struct {
	OpID      string   `json:"opId"`
	At        string   `json:"at"`
	Actor     string   `json:"actor"`
	Kind      string   `json:"kind"`
	Members   []string `json:"members"`
	Base      string   `json:"base,omitempty"`
	Onto      string   `json:"onto,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Deviation int      `json:"deviation,omitempty"`
	Detail    string   `json:"detail"`
}

// BeginRequest is one landing begin.
type BeginRequest struct {
	BatchID string
	Members []string
	// Base and Head are commits of the lane checkout: B and the agent's C.
	Base, Head string
	// LedgerRoot is the lane's installation, whose plans/goals is the
	// ledger read at B.
	LedgerRoot string
	// Actor is the lane's claim identity (machine+lineage): who must hold
	// every goal member at B, and the Machine trailer of what the lane
	// authors.
	Actor string
	At    time.Time
	OpID  string
}

// BeginRefusal refuses a series begin cannot take; nothing is recorded.
type BeginRefusal struct {
	Code, Reason, Next string
}

func (refusal *BeginRefusal) Error() string { return refusal.Reason }

// RefusalCode is the registered code, for --verbose, --json and records.
func (refusal *BeginRefusal) RefusalCode() string { return refusal.Code }

// CompositionRefusal refuses a series and is recorded as composition
// evidence for the members it names.
type CompositionRefusal struct {
	Code     string
	Evidence CompositionEvidence
}

func (refusal *CompositionRefusal) Error() string { return refusal.Evidence.Detail }

// RefusalCode is the registered code, for --verbose, --json and records.
func (refusal *CompositionRefusal) RefusalCode() string { return refusal.Code }

func beginRefused(reason, next string) error {
	return &BeginRefusal{Code: CodeBeginRefused, Reason: reason, Next: next}
}

// pinStep is one pinned step of a member's admitted contribution: one
// commit of the canonical series replays it.
type pinStep struct {
	unit  Unit
	build *BranchBuild
}

func (step pinStep) pin() string {
	switch {
	case step.unit.IsChange():
		return step.unit.Change.Commit
	case step.build != nil:
		return step.build.Commit
	}
	return step.unit.Chain
}

// memberSteps are a member's pinned steps in order: a change's commit, each
// selected build of a goal branch (its folds with it), or a certified chain.
func memberSteps(unit Unit) []pinStep {
	if unit.IsChange() || len(unit.Builds) == 0 {
		return []pinStep{{unit: unit}}
	}
	steps := make([]pinStep, 0, len(unit.Builds))
	for index := range unit.Builds {
		steps = append(steps, pinStep{unit: unit, build: &unit.Builds[index]})
	}
	return steps
}

// memberConflict is a member whose contribution does not apply on a tree.
type memberConflict struct {
	member string
	paths  []string
	cause  error
}

func (conflict *memberConflict) Error() string { return conflict.cause.Error() }

// composeMember applies a member's admitted contribution, and nothing else,
// on baseTree: every selected build with its folds (a --through prefix
// holds only the builds it selected), a change's commit, or a chain's
// certified patch. It returns the tree after each step, trees[0] being
// baseTree. A step that does not apply is a *memberConflict.
func composeMember(root, baseTree string, unit Unit) (trees []string, err error) {
	if _, err := batchMergeDriverArgs(); err != nil {
		return nil, err
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(baseTree)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	workspace := detached.Workspace()
	trees = []string{baseTree}
	conflict := func(cause error) error {
		return &memberConflict{member: unit.GoalID, paths: unmergedPaths(workspace.Dir), cause: cause}
	}
	for _, step := range memberSteps(unit) {
		switch {
		case unit.IsChange():
			if err := applyBranchCommit(root, workspace.Dir, unit.Change.Commit); err != nil {
				var applyConflict *patchApplyConflict
				if errors.As(err, &applyConflict) {
					return nil, conflict(err)
				}
				return nil, err
			}
		case step.build != nil:
			for _, fold := range step.build.Folds {
				if err := applyBranchCommit(root, workspace.Dir, fold.ID); err != nil {
					var applyConflict *patchApplyConflict
					if errors.As(err, &applyConflict) {
						return nil, conflict(err)
					}
					return nil, err
				}
			}
			before, err := workspace.Snapshot("HEAD")
			if err != nil {
				return nil, err
			}
			if err := applyBranchCommit(root, workspace.Dir, step.build.Commit); err != nil {
				var applyConflict *patchApplyConflict
				if errors.As(err, &applyConflict) {
					return nil, conflict(err)
				}
				return nil, err
			}
			after, err := workspace.Snapshot("HEAD")
			if err != nil {
				return nil, err
			}
			digest, matches, err := transitionMatchesBuild(root, before, after, step.build.Commit, step.build.Digest)
			if err != nil {
				return nil, err
			}
			if !matches {
				return nil, refuseBatch("BATCH_JOIN_REREAD", fmt.Sprintf("goal %s build %s changed since its review (%s, not %s); metasystem work review %s reviews it again",
					unit.GoalID, strings.Join(step.build.Units, "+"), digest, step.build.Digest, unit.GoalID))
			}
		default:
			if err := applyChainPatch(root, workspace.Dir, unit); err != nil {
				var applyConflict *patchApplyConflict
				if errors.As(err, &applyConflict) {
					return nil, conflict(err)
				}
				return nil, err
			}
		}
		tree, err := workspace.Snapshot("HEAD")
		if err != nil {
			return nil, err
		}
		trees = append(trees, tree)
	}
	return trees, nil
}

// applyChainPatch stages a certified chain member's patch, as the assembly
// does; a patch that does not apply is a *patchApplyConflict.
func applyChainPatch(root, worktree string, unit Unit) error {
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", unit.Chain, "diff.patch"))
	if err != nil {
		return err
	}
	workspace := gittree.Workspace{Dir: worktree}
	before, err := workspace.StagedTree()
	if err != nil {
		return err
	}
	if err := contractgit.PreflightPatchAttributes(worktree, before, "unit "+unit.GoalID, patch); err != nil {
		return err
	}
	if output, applyErr := runBatchMergeGit(worktree, patch, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-"); applyErr != nil {
		if contractgit.IsRefusal(applyErr) {
			return applyErr
		}
		cause := fmt.Errorf("apply unit %s: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), applyErr)
		if patchCompositionConflict(worktree, output, applyErr) {
			return &patchApplyConflict{cause: cause}
		}
		return cause
	}
	after, err := workspace.StagedTree()
	if err != nil {
		return err
	}
	return contractgit.CheckPatchContract(worktree, before, after, patch, "unit "+unit.GoalID)
}

func unmergedPaths(dir string) []string {
	command := exec.Command("git", "-C", dir, "diff", "--name-only", "--diff-filter=U", "-z")
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	raw, _ := command.Output()
	if len(raw) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
}

// MemberSubjectTree is the tree of prove --subject member:M (K6): the base
// tree plus exactly M's admitted contribution, built from its retained
// member metadata. It holds no other member, no build past a --through
// prefix, and no agent edit.
func MemberSubjectTree(root, baseTree string, unit Unit) (string, error) {
	trees, err := composeMember(root, baseTree, unit)
	if err != nil {
		return "", err
	}
	return trees[len(trees)-1], nil
}

// PlanOpening checks the agent's series against the batch's pinned members
// and writes the canonical series. It executes nothing and records nothing:
// RecordOpening records what it returns, and RecordComposition what a
// *CompositionRefusal carries.
func PlanOpening(root string, record Record, request BeginRequest) (Opening, error) {
	git := func(args ...string) (string, error) { return landingGitOutput(root, args...) }
	base, err := git("rev-parse", "--verify", "--quiet", request.Base+"^{commit}")
	if err != nil || base == "" {
		return Opening{}, beginRefused(fmt.Sprintf("the base %s is not a commit of the lane checkout", request.Base),
			"name the base as origin/main's commit: git -C LANE rev-parse origin/main")
	}
	head, err := git("rev-parse", "--verify", "--quiet", request.Head+"^{commit}")
	if err != nil || head == "" {
		return Opening{}, beginRefused(fmt.Sprintf("the head %s is not a commit of the lane checkout", request.Head),
			"name the tip of the composed series: git -C LANE rev-parse HEAD")
	}
	if onMain, err := isAncestor(root, base, "refs/remotes/origin/main"); err != nil || !onMain {
		return Opening{}, beginRefused(fmt.Sprintf("the base %s is not on origin/main", short(base)),
			"fetch origin and compose on origin/main: git -C LANE fetch origin main")
	}
	members, err := pinnedMembers(record, request.Members)
	if err != nil {
		return Opening{}, err
	}
	series, err := linearSeries(root, base, head)
	if err != nil {
		return Opening{}, err
	}
	baseTree, err := git("rev-parse", base+"^{tree}")
	if err != nil {
		return Opening{}, err
	}
	var steps []pinStep
	for _, unit := range members {
		steps = append(steps, memberSteps(unit)...)
	}
	integration := len(series) == len(steps)+1
	if len(series) != len(steps) && !integration {
		return Opening{}, beginRefused(fmt.Sprintf("the series has %d commits, but its members need %d plus at most one %s",
			len(series), len(steps), LaneIntegrationTrailer), "compose one commit per change, build or chain, in member order, then run landing begin again")
	}
	opening := Opening{OpID: request.OpID, At: request.At.UTC().Format(time.RFC3339Nano), Actor: request.Actor, Members: slices.Clone(request.Members),
		Base: base, BaseTree: baseTree, Head: head}
	// Each member's own contribution on B: the pin every replay is judged
	// against, and the member subject prove runs.
	pinTrees := map[string][]string{}
	for _, unit := range members {
		trees, err := composeMember(root, baseTree, unit)
		var conflict *memberConflict
		if errors.As(err, &conflict) {
			return Opening{}, &CompositionRefusal{Code: CodeBeginConflict, Evidence: CompositionEvidence{OpID: request.OpID, At: opening.At, Actor: request.Actor,
				Kind: CompositionConflict, Members: []string{unit.GoalID}, Base: base, Paths: conflict.paths,
				Detail: fmt.Sprintf("%s does not apply on main %s (%s)", unit.GoalID, short(base), pathsWord(conflict.paths))}}
		}
		if err != nil {
			return Opening{}, err
		}
		pinTrees[unit.GoalID] = trees
	}
	stepIndex := map[string]int{}
	var resolvedMembers []string
	for index, entry := range series {
		marks, err := laneMarks(root, entry.commit)
		if err != nil {
			return Opening{}, err
		}
		if index == len(steps) {
			if marks.integration == "" {
				return Opening{}, beginRefused(fmt.Sprintf("commit %s comes after every member's pinned steps but is not marked %s", short(entry.commit), LaneIntegrationTrailer),
					"mark the final seam fix with a "+LaneIntegrationTrailer+" trailer, or drop it, then run landing begin again")
			}
			lines, err := changedLines(root, entry.parent, entry.commit)
			if err != nil {
				return Opening{}, err
			}
			opening.Series = append(opening.Series, SeriesCommit{Source: entry.commit, Kind: SeriesIntegration, Deviation: lines.total(), Resolved: marks.integration})
			opening.Deviation += lines.total()
			continue
		}
		if marks.integration != "" {
			return Opening{}, beginRefused(fmt.Sprintf("commit %s is marked %s but is not the last commit of the series", short(entry.commit), LaneIntegrationTrailer),
				"keep one integration commit, last, then run landing begin again")
		}
		step := steps[index]
		trees := pinTrees[step.unit.GoalID]
		at := stepIndex[step.unit.GoalID]
		stepIndex[step.unit.GoalID] = at + 1
		commit := SeriesCommit{Source: entry.commit, Kind: SeriesReplay, Member: step.unit.GoalID, Pin: step.pin(), Resolved: marks.resolved}
		replayID, err := patchID(root, entry.parent, entry.commit)
		if err != nil {
			return Opening{}, err
		}
		pinID, err := patchID(root, trees[at], trees[at+1])
		if err != nil {
			return Opening{}, err
		}
		if replayID != pinID {
			if marks.resolved == "" {
				return Opening{}, beginRefused(fmt.Sprintf("commit %s differs from %s's pinned %s and is not marked %s", short(entry.commit), step.unit.GoalID, short(step.pin()), LaneResolvedTrailer),
					"replay the pin exactly, or mark the resolution with a "+LaneResolvedTrailer+" trailer, then run landing begin again")
			}
			replay, err := changedLines(root, entry.parent, entry.commit)
			if err != nil {
				return Opening{}, err
			}
			pin, err := changedLines(root, trees[at], trees[at+1])
			if err != nil {
				return Opening{}, err
			}
			commit.Kind, commit.Deviation = SeriesResolved, replay.against(pin)
			opening.Deviation += commit.Deviation
			if commit.Deviation != 0 && !slices.Contains(resolvedMembers, step.unit.GoalID) {
				resolvedMembers = append(resolvedMembers, step.unit.GoalID)
			}
		}
		opening.Series = append(opening.Series, commit)
	}
	if opening.Deviation > SeamCapLines {
		// Over the one aggregate cap: the work goes back to the seats. A
		// resolved replay is its member's; the integration commit is the
		// combination's, every member's.
		named := resolvedMembers
		if integration && opening.Series[len(opening.Series)-1].Deviation != 0 {
			named = slices.Clone(request.Members)
		}
		return Opening{}, &CompositionRefusal{Code: CodeBeginSeamTooLarge, Evidence: CompositionEvidence{OpID: request.OpID, At: opening.At, Actor: request.Actor,
			Kind: CompositionSeamTooLarge, Members: named, Base: base, Deviation: opening.Deviation,
			Detail: fmt.Sprintf("the lane's own edits come to %d lines, over the %d-line cap", opening.Deviation, SeamCapLines)}}
	}
	if err := writeCanonicalSeries(root, record.BatchID, request, members, &opening); err != nil {
		return Opening{}, err
	}
	return opening, nil
}

// pinnedMembers are the units begin pins, in the series' order: every joined
// member of the batch, each named once. A member left out would stay joined
// and never land, so the set must be the batch's joined set exactly.
func pinnedMembers(record Record, names []string) ([]Unit, error) {
	joined := joinedUnits(record.Units)
	var members []Unit
	for _, name := range names {
		index := slices.IndexFunc(joined, func(unit Unit) bool { return unit.GoalID == name })
		if index < 0 {
			return nil, beginRefused(fmt.Sprintf("%s is not a joined member of batch %s", name, record.BatchID),
				"name the members landing status lists for the batch, then run landing begin again")
		}
		if slices.ContainsFunc(members, func(unit Unit) bool { return unit.GoalID == name }) {
			return nil, beginRefused(fmt.Sprintf("%s is named twice", name), "name each member once, then run landing begin again")
		}
		members = append(members, joined[index])
	}
	if len(members) == 0 || len(members) != len(joined) {
		var missing []string
		for _, unit := range joined {
			if !slices.Contains(names, unit.GoalID) {
				missing = append(missing, unit.GoalID)
			}
		}
		return nil, beginRefused(fmt.Sprintf("batch %s pins every joined member; missing: %s", record.BatchID, strings.Join(missing, ", ")),
			"compose every member, or return the one that cannot land first, then run landing begin again")
	}
	return members, nil
}

type seriesEntry struct{ commit, parent string }

// linearSeries is B..C as a first-parent chain starting at B; a merge, or a
// head that does not descend from B, is refused.
func linearSeries(root, base, head string) ([]seriesEntry, error) {
	if base == head {
		return nil, beginRefused("the series is empty: the head is the base", "compose the members on the base, then run landing begin again")
	}
	out, err := landingGitOutput(root, "rev-list", "--first-parent", "--parents", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	var series []seriesEntry
	previous := base
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, beginRefused(fmt.Sprintf("commit %s is a merge; a series is linear", short(fields[0])), "rebase the series into single-parent commits, then run landing begin again")
		}
		if fields[1] != previous {
			return nil, beginRefused(fmt.Sprintf("the head %s does not descend from the base %s in one line", short(head), short(base)),
				"compose the series on the base, then run landing begin again")
		}
		series = append(series, seriesEntry{commit: fields[0], parent: fields[1]})
		previous = fields[0]
	}
	if previous != head || len(series) == 0 {
		return nil, beginRefused(fmt.Sprintf("the head %s does not descend from the base %s", short(head), short(base)),
			"compose the series on the base, then run landing begin again")
	}
	return series, nil
}

type laneMarkings struct{ resolved, integration string }

func laneMarks(root, commit string) (laneMarkings, error) {
	read := func(key string) (string, error) {
		out, err := landingGitOutput(root, "show", "-s", "--format=%(trailers:key="+key+",valueonly,separator=%x20)", commit)
		return strings.TrimSpace(out), err
	}
	resolved, err := read(LaneResolvedTrailer)
	if err != nil {
		return laneMarkings{}, err
	}
	integration, err := read(LaneIntegrationTrailer)
	return laneMarkings{resolved: resolved, integration: integration}, err
}

// patchID is git's stable patch id of the change from one tree-ish to
// another; empty for no change.
func patchID(root, from, to string) (string, error) {
	diff := exec.Command("git", "-C", root, "diff-tree", "-r", "-p", "--no-renames", "--no-color", "--no-ext-diff", "--full-index", "--binary", from, to)
	diff.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	patch, err := diff.Output()
	if err != nil {
		return "", fmt.Errorf("diff %s..%s: %w", short(from), short(to), err)
	}
	if len(bytes.TrimSpace(patch)) == 0 {
		return "", nil
	}
	id := exec.Command("git", "-C", root, "patch-id", "--stable")
	id.Env, id.Stdin = gittree.ScrubbedEnviron("LC_ALL=C"), bytes.NewReader(patch)
	out, err := id.Output()
	if err != nil {
		return "", fmt.Errorf("patch id of %s..%s: %w", short(from), short(to), err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

// lineCounts are a change's added and removed lines, per file, as a
// multiset: file header plus sign plus content.
type lineCounts map[string]int

func (counts lineCounts) total() int {
	sum := 0
	for _, count := range counts {
		sum += count
	}
	return sum
}

// against is the deviation of one change from another: the lines either
// adds or removes that the other does not.
func (counts lineCounts) against(other lineCounts) int {
	deviation := 0
	for key, count := range counts {
		if difference := count - other[key]; difference > 0 {
			deviation += difference
		}
	}
	for key, count := range other {
		if difference := count - counts[key]; difference > 0 {
			deviation += difference
		}
	}
	return deviation
}

func changedLines(root, from, to string) (lineCounts, error) {
	diff := exec.Command("git", "-C", root, "diff-tree", "-r", "-p", "-U0", "--no-renames", "--no-color", "--no-ext-diff", from, to)
	diff.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	out, err := diff.Output()
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", short(from), short(to), err)
	}
	counts := lineCounts{}
	file, inHunk := "", false
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file, inHunk = line, false
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case strings.HasPrefix(line, "Binary files "):
			counts[file+"\x00binary"]++
		case inHunk && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")):
			counts[file+"\x00"+line]++
		}
	}
	return counts, nil
}

func isAncestor(root, ancestor, descendant string) (bool, error) {
	command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", ancestor, descendant)
	command.Env = gittree.ScrubbedEnviron()
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func pathsWord(paths []string) string {
	if len(paths) == 0 {
		return "the patch does not apply"
	}
	return "conflicts in " + strings.Join(paths, ", ")
}

// commitIdentity is who a canonical commit is by, and when.
type commitIdentity struct {
	authorName, authorEmail, authorDate, committerName, committerEmail, committerDate string
}

func readIdentity(root, commit string) (commitIdentity, error) {
	out, err := landingGitOutput(root, "show", "-s", "--format=%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI", commit)
	fields := strings.Split(out, "\x00")
	if err != nil || len(fields) != 6 {
		return commitIdentity{}, fmt.Errorf("read the identity of %s: %v", short(commit), err)
	}
	return commitIdentity{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]}, nil
}

// writeCanonicalSeries writes each series commit again on the canonical
// parent with the landing metadata the producers make: the tree of every
// commit is the agent's, so the candidate's tree is the head's.
func writeCanonicalSeries(root, batchID string, request BeginRequest, members []Unit, opening *Opening) error {
	byID := map[string]Unit{}
	for _, unit := range members {
		byID[unit.GoalID] = unit
	}
	parent := opening.Base
	for index := range opening.Series {
		entry := &opening.Series[index]
		source, err := readIdentity(root, entry.Source)
		if err != nil {
			return err
		}
		tree, err := landingGitOutput(root, "rev-parse", entry.Source+"^{tree}")
		if err != nil {
			return err
		}
		message, identity, err := canonicalMessage(root, batchID, request, byID, *entry, source)
		if err != nil {
			return err
		}
		command := exec.Command("git", "-C", root, "commit-tree", tree, "-p", parent)
		command.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_NAME="+identity.authorName, "GIT_AUTHOR_EMAIL="+identity.authorEmail,
			"GIT_AUTHOR_DATE="+identity.authorDate, "GIT_COMMITTER_NAME="+identity.committerName, "GIT_COMMITTER_EMAIL="+identity.committerEmail,
			"GIT_COMMITTER_DATE="+identity.committerDate)
		command.Stdin = strings.NewReader(message)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("write the canonical commit for %s: %s: %w", short(entry.Source), strings.TrimSpace(string(output)), err)
		}
		entry.Commit = strings.TrimSpace(string(output))
		parent = entry.Commit
	}
	opening.Candidate = parent
	candidateTree, err := landingGitOutput(root, "rev-parse", parent+"^{tree}")
	if err != nil {
		return err
	}
	headTree, err := landingGitOutput(root, "rev-parse", opening.Head+"^{tree}")
	if err != nil {
		return err
	}
	if candidateTree != headTree {
		return fmt.Errorf("the canonical series' tree %s is not the head's %s", short(candidateTree), short(headTree))
	}
	opening.Tree = candidateTree
	return nil
}

// canonicalMessage is one canonical commit's message and identity: the
// landing metadata producers' (BranchLandingMessage, ChangeLandingMessage,
// a chain's provenance) with held's ownership and revision trailers for a
// goal, and the agent's marks kept.
func canonicalMessage(root, batchID string, request BeginRequest, members map[string]Unit, entry SeriesCommit, source commitIdentity) (string, commitIdentity, error) {
	resolved := ""
	if entry.Resolved != "" && entry.Kind != SeriesIntegration {
		resolved = LaneResolvedTrailer + ": " + entry.Resolved + "\n"
	}
	if entry.Kind == SeriesIntegration {
		subject, err := landingGitOutput(root, "show", "-s", "--format=%s", entry.Source)
		if err != nil {
			return "", commitIdentity{}, err
		}
		if strings.TrimSpace(subject) == "" {
			subject = "integrate batch " + batchID
		}
		return fmt.Sprintf("%s\n\n%s\n\n%s: %s\nMachine: %s\n", strings.TrimSpace(subject), entry.Resolved, LaneIntegrationTrailer, batchID, request.Actor), source, nil
	}
	unit := members[entry.Member]
	if unit.IsChange() {
		original, err := branchCommitMessage(root, unit.Change.Commit)
		if err != nil {
			return "", commitIdentity{}, fmt.Errorf("%s: change %s message: %w", codeChangeUnreadable, unit.GoalID, err)
		}
		asker, err := readIdentity(root, unit.Change.Commit)
		if err != nil {
			return "", commitIdentity{}, err
		}
		asker.committerDate = source.committerDate
		return ChangeLandingMessage(string(original), unit.GoalID) + resolved, asker, nil
	}
	if unit.AuthorName == "" || unit.AuthorEmail == "" {
		return "", commitIdentity{}, beginRefused(fmt.Sprintf("goal %s has no approver identity to author its landing", unit.GoalID),
			"the approver sets goal.human.NAME in metasystem.conf, then the goal joins again")
	}
	revision, err := laneHeldRevision(request.LedgerRoot, request.Base, unit.GoalID, request.Actor)
	if err != nil {
		return "", commitIdentity{}, err
	}
	held := fmt.Sprintf("Machine: %s\nGoal-Item: %s\nGoal-Revision: %d\n", request.Actor, unit.GoalID, revision)
	var message string
	if len(unit.Builds) != 0 {
		build := unitBuild(unit, entry.Pin)
		last := unit.GoalLast && len(unit.CommitIDs) != 0 && build.Commit == unit.CommitIDs[len(unit.CommitIDs)-1]
		message = BranchLandingMessage(unit.GoalID, build, last) + held + resolved
	} else {
		message = fmt.Sprintf("land %s in batch %s\n\nLanding-Provenance: chain=%s\n", unit.GoalID, batchID, unit.Chain) + held + resolved
	}
	approver := commitIdentity{authorName: unit.AuthorName, authorEmail: unit.AuthorEmail, authorDate: source.authorDate,
		committerName: unit.AuthorName, committerEmail: unit.AuthorEmail, committerDate: source.committerDate}
	return message, approver, nil
}

func unitBuild(unit Unit, commit string) BranchBuild {
	for _, build := range unit.Builds {
		if build.Commit == commit {
			return build
		}
	}
	return BranchBuild{}
}

// laneHeldRevision is the claimed revision of goalID on the ledger at base,
// which must be held by actor, the lane: held checks the same at the push.
func laneHeldRevision(ledgerRoot, base, goalID, actor string) (uint64, error) {
	workspace := gittree.Workspace{Dir: ledgerRoot}
	tree, err := workspace.TreeOf(base)
	if err != nil {
		return 0, err
	}
	for _, path := range []string{"plans/goals/" + goalID + ".md", "records/goals/" + goalID + ".md"} {
		data, present, err := workspace.FileAt(tree, path)
		if err != nil {
			return 0, err
		}
		if !present {
			continue
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 || file.Id != goalID {
			break
		}
		if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine+"+"+file.Claimed.Lineage != actor || file.Claimed.Revision == 0 {
			return 0, beginRefused(fmt.Sprintf("goal %s is not held by the landing lane on main %s", goalID, short(base)),
				"wait for its handover to reach main, then run landing begin again")
		}
		return file.Claimed.Revision, nil
	}
	return 0, beginRefused(fmt.Sprintf("goal %s is not on the ledger at main %s", goalID, short(base)),
		"fetch origin and name its main as the base, then run landing begin again")
}

// RecordOpening durably records opening as batch id's current series, under
// the store lock, after checking again that its members are still the
// batch's joined members. An open batch is sealed by it: joins go to the
// next batch. The same series again changes nothing (false).
func RecordOpening(store Store, id string, opening Opening, at time.Time) (Record, bool, error) {
	var recorded Record
	changed := false
	err := store.Update(id, func(record *Record) error {
		if record.State != StateOpen && record.State != StateSealed {
			return beginRefused(fmt.Sprintf("batch %s is %s, so it takes no begin", id, record.State),
				"run landing status to see the batch")
		}
		joined := joinedUnits(record.Units)
		if len(joined) != len(opening.Members) || slices.ContainsFunc(joined, func(unit Unit) bool { return !slices.Contains(opening.Members, unit.GoalID) }) {
			return beginRefused(fmt.Sprintf("batch %s's members changed while the series was checked", id), "run landing begin again")
		}
		if current, ok := record.CurrentOpening(); ok && current.Candidate == opening.Candidate && current.Base == opening.Base && slices.Equal(current.Members, opening.Members) {
			recorded = *record
			return nil
		}
		record.Openings = append(record.Openings, opening)
		to := record.State
		if to == StateOpen {
			to = StateSealed
		}
		record.Transition(to, at, "begin", opening.Actor, "op "+opening.OpID+" candidate "+opening.Candidate)
		recorded, changed = *record, true
		return nil
	})
	return recorded, changed, err
}

// RecordComposition durably records composition evidence on batch id.
func RecordComposition(store Store, id string, evidence CompositionEvidence, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		record.Compositions = append(record.Compositions, evidence)
		record.Transition(record.State, at, "composition", evidence.Actor, evidence.Kind+" "+strings.Join(evidence.Members, ",")+": "+evidence.Detail)
		return nil
	})
}

// CurrentOpening is the batch's latest landing begin.
func (record Record) CurrentOpening() (Opening, bool) {
	if len(record.Openings) == 0 {
		return Opening{}, false
	}
	return record.Openings[len(record.Openings)-1], true
}

// ConflictRequest is landing begin --record-conflict: the agent's failed
// cherry-pick of Member onto Onto, a commit of its series on Base.
type ConflictRequest struct {
	BatchID, Member, Base, Onto, Actor, OpID string
	At                                       time.Time
}

// CheckConflict verifies a conflict the agent reports: Member's admitted
// contribution must fail to apply on Onto, which descends from Base in one
// line. It returns the evidence to record; a contribution that applies is
// refused, so no conflict is recorded that the kernel did not see.
func CheckConflict(root string, record Record, request ConflictRequest) (CompositionEvidence, error) {
	joined := joinedUnits(record.Units)
	index := slices.IndexFunc(joined, func(unit Unit) bool { return unit.GoalID == request.Member })
	if index < 0 {
		return CompositionEvidence{}, beginRefused(fmt.Sprintf("%s is not a joined member of batch %s", request.Member, record.BatchID),
			"name a member landing status lists for the batch")
	}
	base, err := landingGitOutput(root, "rev-parse", "--verify", "--quiet", request.Base+"^{commit}")
	if err != nil || base == "" {
		return CompositionEvidence{}, beginRefused(fmt.Sprintf("the base %s is not a commit of the lane checkout", request.Base), "name origin/main's commit as the base")
	}
	onto, err := landingGitOutput(root, "rev-parse", "--verify", "--quiet", request.Onto+"^{commit}")
	if err != nil || onto == "" {
		return CompositionEvidence{}, beginRefused(fmt.Sprintf("%s is not a commit of the lane checkout", request.Onto), "name the series commit the member failed on")
	}
	if onMain, err := isAncestor(root, base, "refs/remotes/origin/main"); err != nil || !onMain {
		return CompositionEvidence{}, beginRefused(fmt.Sprintf("the base %s is not on origin/main", short(base)), "fetch origin and compose on origin/main")
	}
	if onto != base {
		if _, err := linearSeries(root, base, onto); err != nil {
			return CompositionEvidence{}, err
		}
	}
	ontoTree, err := landingGitOutput(root, "rev-parse", onto+"^{tree}")
	if err != nil {
		return CompositionEvidence{}, err
	}
	_, err = composeMember(root, ontoTree, joined[index])
	var conflict *memberConflict
	if !errors.As(err, &conflict) {
		if err != nil {
			return CompositionEvidence{}, err
		}
		return CompositionEvidence{}, beginRefused(fmt.Sprintf("%s applies on %s without a conflict, so none is recorded", request.Member, short(onto)),
			"compose it on the series, then run landing begin")
	}
	return CompositionEvidence{OpID: request.OpID, At: request.At.UTC().Format(time.RFC3339Nano), Actor: request.Actor, Kind: CompositionConflict,
		Members: []string{request.Member}, Base: base, Onto: onto, Paths: conflict.paths,
		Detail: fmt.Sprintf("%s does not apply on %s (%s)", request.Member, short(onto), pathsWord(conflict.paths))}, nil
}
