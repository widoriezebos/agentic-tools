package goal

// The tree validator: the full read-set, revalidated on
// every transaction tree and on every read-side advance. A tree
// passes WHOLE or names the file and the rule — the projection
// stays at the accepted tree on any refusal, so a torn or tampered
// tip can never become the world other machines act on. The verb
// transition table's per-verb rows live with the verbs; THIS layer
// owns what must hold on any tree at rest.

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// TreeGoals is one tree's parsed live ledger and concluded-goal records.
type TreeGoals struct {
	Root *RootRecord
	// Live and Done are keyed by goal id; placement is recorded at
	// parse so the placement/State agreement rule can name both.
	Live      map[string]*GoalFile
	Done      map[string]*GoalFile
	DonePaths map[string]string
}

var (
	goalsRoot          = relativeStateRoot(stateroot.Goals)
	recordsRoot        = relativeStateRoot(stateroot.Records)
	goalsPrefix        = goalsRoot + "/"
	recordsGoalsRoot   = path.Join(recordsRoot, "goals")
	recordsGoalsPrefix = recordsGoalsRoot + "/"
	legacyDonePrefix   = goalsPrefix + "done/"
)

func protectedGoalDirectories() []string {
	return []string{
		path.Dir(goalsRoot), goalsRoot, strings.TrimSuffix(legacyDonePrefix, "/"),
		recordsRoot, recordsGoalsRoot,
	}
}

func relativeStateRoot(kind stateroot.Kind) string {
	root, err := stateroot.RelativeRoot(kind)
	if err != nil {
		panic(fmt.Sprintf("goal: resolve %s state root: %v", kind, err))
	}
	return root
}

// ParseTreeFiles builds the parsed subtree from raw bytes keyed by
// repository-relative path. Every problem names its file; a tree
// with problems is refused whole.
func ParseTreeFiles(files map[string][]byte) (*TreeGoals, []Problem) {
	t := &TreeGoals{Live: map[string]*GoalFile{}, Done: map[string]*GoalFile{}, DonePaths: map[string]string{}}
	var problems []Problem
	addf := func(format string, args ...any) {
		problems = append(problems, Problem(fmt.Sprintf(format, args...)))
	}
	for _, p := range sortedKeys(files) {
		data := files[p]
		if strings.HasPrefix(p, recordsGoalsPrefix) {
			rel := strings.TrimPrefix(p, recordsGoalsPrefix)
			if strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "/") {
				parseDoneAt(t, p, data, addf)
			} else {
				addf("%s: not a concluded goal record", p)
			}
			continue
		}
		if !strings.HasPrefix(p, goalsPrefix) {
			addf("%s: outside the goal ledger", p)
			continue
		}
		rel := strings.TrimPrefix(p, goalsPrefix)
		switch {
		case rel == "backlog.md":
			root, rootProblems := ParseRoot(data)
			for _, rp := range rootProblems {
				addf("%s: %s", p, rp)
			}
			t.Root = root
		case strings.HasPrefix(rel, "done/") && strings.HasSuffix(rel, ".md") && !strings.Contains(strings.TrimPrefix(rel, "done/"), "/"):
			// PERMANENT, not soak (named reason per Ruling N):
			// historical git trees carry plans/goals/done immutably,
			// and one uniform parser reads every tree in history.
			// Engine WRITES never target this prefix.
			parseDoneAt(t, p, data, addf)
		case strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "/"):
			f, ok := parseGoalAt(p, data, addf)
			if ok {
				t.Live[f.Id] = f
			}
		default:
			addf("%s: not a goal file, the archive, or the root record", p)
		}
	}
	if t.Root == nil {
		addf("%sbacklog.md: the root record is missing", goalsPrefix)
	}
	return t, problems
}

func parseDoneAt(t *TreeGoals, p string, data []byte, addf func(string, ...any)) {
	f, ok := parseGoalAt(p, data, addf)
	if !ok {
		return
	}
	if prior, exists := t.DonePaths[f.Id]; exists {
		addf("%s: goal %s is also present at %s", p, f.Id, prior)
		return
	}
	t.Done[f.Id] = f
	t.DonePaths[f.Id] = p
}

func parseGoalAt(p string, data []byte, addf func(string, ...any)) (*GoalFile, bool) {
	// ParseFile owns the Integrity check: a missing or mismatched
	// digest arrives here as a named problem.
	f, fileProblems := ParseFile(data)
	if len(fileProblems) > 0 {
		for _, fp := range fileProblems {
			addf("%s: %s", p, fp)
		}
		return nil, false
	}
	want := strings.TrimSuffix(path.Base(p), ".md")
	if f.Id != want {
		addf("%s: file name and Id disagree (%s)", p, f.Id)
		return nil, false
	}
	return f, true
}

// ValidateTree runs every at-rest rule over a parsed subtree.
func ValidateTree(t *TreeGoals) []Problem {
	var problems []Problem
	addf := func(format string, args ...any) {
		problems = append(problems, Problem(fmt.Sprintf(format, args...)))
	}

	// Placement and State agree — path never silently implies state
	//.
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		if f.State == StateDone {
			addf("%s%s.md: State done outside the archive", goalsPrefix, id)
		}
	}
	for _, id := range sortedGoalIds(t.Done) {
		f := t.Done[id]
		if f.State != StateDone {
			addf("%s: State %s inside the archive", doneLocation(t, id), f.State)
		}
		if f.Conclude == "" {
			addf("%s: done without a conclusion", doneLocation(t, id))
		}
	}

	// One id, one file: the live set and the archive never both
	// carry a goal.
	for _, id := range sortedGoalIds(t.Live) {
		if _, both := t.Done[id]; both {
			addf("%s%s.md: also present in the archive", goalsPrefix, id)
		}
	}
	if t.Root != nil {
		for _, entry := range t.Root.Decomposed {
			if _, live := t.Live[entry.Id]; live {
				addf("%s%s.md: decomposed parent %s is live again", goalsPrefix, entry.Id, entry.Id)
			}
		}
	}

	exists := func(id string) bool {
		_, live := t.Live[id]
		_, done := t.Done[id]
		return live || done
	}
	stateOf := func(id string) string {
		if f, ok := t.Live[id]; ok {
			return f.State
		}
		if _, ok := t.Done[id]; ok {
			return StateDone
		}
		return ""
	}

	// Referential integrity: every edge and arc names a real goal.
	forAll(t, func(where string, f *GoalFile) {
		for _, dep := range f.Blocked {
			if !exists(dep) {
				addf("%s: blockedBy names a goal that does not exist: %s", where, dep)
			}
		}
		canonical, err := canonicalLabels(f.Labels)
		if err != nil {
			addf("%s: %v", where, err)
		} else if strings.Join(canonical, ",") != strings.Join(f.Labels, ",") {
			addf("%s: Labels must be sorted and deduplicated on a published tree", where)
		}
		if f.Budget != nil && f.NormApproval != nil && f.NormApproval.Minutes < f.Budget.ReservedJobMinutesLimit {
			addf("%s: NormApproval proves %d minutes but Budget reserves %d minutes", where,
				f.NormApproval.Minutes, f.Budget.ReservedJobMinutesLimit)
		}
		if f.Budget != nil && f.NormApproval != nil && f.NormApproval.ReviewRounds < f.Budget.ReviewRoundLimit {
			addf("%s: NormApproval proves %d review rounds but Budget permits %d review rounds", where,
				f.NormApproval.ReviewRounds, f.Budget.ReviewRoundLimit)
		}
	})
	// Acyclicity over the COMPOSED blocked graph (live and done
	// edges together): a cycle anywhere wedges the frontier.
	if cycle := findCycle(t); cycle != "" {
		addf("blockedBy cycle: %s", cycle)
	}

	// A claimed goal's blockers are all done — tree-wide, arcs
	// included: claiming ahead of an unfinished blocker is the exact
	// order violation the frontier exists to prevent.
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		if f.State != StateClaimed {
			continue
		}
		if f.Claimed == nil {
			addf("%s%s.md: claimed without a claim record", goalsPrefix, id)
			continue
		}
		for _, dep := range f.Blocked {
			if stateOf(dep) != StateDone {
				addf("%s%s.md: claimed while blocker %s is not done", goalsPrefix, id, dep)
			}
		}
		if f.Pinned != "" && f.Claimed.Machine != f.Pinned {
			addf("%s%s.md: pinned to machine %s but claimed by %s; ownership contradicts the pin", goalsPrefix, id, f.Pinned, f.Claimed.Machine)
		}
		if f.Approved == nil && t.Root != nil && t.Root.ApprovalGate != nil {
			claimedAt, claimErr := time.Parse(time.RFC3339, f.Claimed.At)
			gateAt, gateErr := time.Parse(time.RFC3339, t.Root.ApprovalGate.Since)
			if claimErr == nil && gateErr == nil && !claimedAt.Before(gateAt) {
				addf("%s%s.md: claim at %s landed after the approval gate armed at %s without an Approved record",
					goalsPrefix, id, f.Claimed.At, t.Root.ApprovalGate.Since)
			}
		}
	}

	// A done goal's blockers are done: concluding over an open
	// blocker is the blocked-done refusal.
	for _, id := range sortedGoalIds(t.Done) {
		f := t.Done[id]
		for _, dep := range f.Blocked {
			if stateOf(dep) != StateDone {
				addf("%s: done while blocker %s is not done", doneLocation(t, id), dep)
			}
		}
	}

	// Quota: one claim per machine, tree-wide; the members of ONE
	// arc under one claimant count once.
	claimsByMachine := map[string][]*GoalFile{}
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		if f.State == StateClaimed && f.Claimed != nil {
			claimsByMachine[f.Claimed.Machine] = append(claimsByMachine[f.Claimed.Machine], f)
		}
	}
	machines := make([]string, 0, len(claimsByMachine))
	for m := range claimsByMachine {
		machines = append(machines, m)
	}
	sort.Strings(machines)
	for _, m := range machines {
		claimed := claimsByMachine[m]
		if len(claimed) <= 1 {
			continue
		}
		arc := claimed[0].Arc
		sameArc := arc != ""
		for _, f := range claimed[1:] {
			if f.Arc != arc {
				sameArc = false
			}
		}
		if !sameArc {
			ids := make([]string, len(claimed))
			for i, f := range claimed {
				ids[i] = f.Id
			}
			addf("machine %s claims %s: the quota is one claim per machine (one arc counts once)", m, strings.Join(ids, ", "))
		}
	}

	// Goal-free exclusivity: the declaration coexists with parked
	// and done, never with queued, approved, or claimed.
	if t.Root != nil && t.Root.Free != nil {
		for _, id := range sortedGoalIds(t.Live) {
			switch t.Live[id].State {
			case StateQueued, StateApproved, StateClaimed:
				addf("%sbacklog.md: Goal-free declared while %s is %s", goalsPrefix, id, t.Live[id].State)
			}
		}
	}

	// The ledger identity is the root record's reason to exist.
	if t.Root != nil && t.Root.Identity == "" {
		addf("%sbacklog.md: no adoption identity", goalsPrefix)
	}

	return problems
}

// forAll visits every goal with its path, live first, sorted.
func forAll(t *TreeGoals, visit func(where string, f *GoalFile)) {
	for _, id := range sortedGoalIds(t.Live) {
		visit(goalsPrefix+id+".md", t.Live[id])
	}
	for _, id := range sortedGoalIds(t.Done) {
		visit(doneLocation(t, id), t.Done[id])
	}
}

func doneLocation(t *TreeGoals, id string) string {
	if t.DonePaths != nil && t.DonePaths[id] != "" {
		return t.DonePaths[id]
	}
	return recordsGoalsPrefix + id + ".md"
}

// findCycle walks the composed blocked graph and names the first
// cycle it proves, as a path, or returns "".
func findCycle(t *TreeGoals) string {
	edges := map[string][]string{}
	collect := func(f *GoalFile) {
		if len(f.Blocked) > 0 {
			edges[f.Id] = append([]string(nil), f.Blocked...)
		}
	}
	for _, f := range t.Live {
		collect(f)
	}
	for _, f := range t.Done {
		collect(f)
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var found string
	var walk func(id string) bool
	walk = func(id string) bool {
		color[id] = grey
		stack = append(stack, id)
		for _, dep := range edges[id] {
			switch color[dep] {
			case grey:
				start := 0
				for i, s := range stack {
					if s == dep {
						start = i
					}
				}
				found = strings.Join(append(stack[start:], dep), " -> ")
				return true
			case white:
				if walk(dep) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return false
	}
	ids := make([]string, 0, len(edges))
	for id := range edges {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if color[id] == white {
			if walk(id) {
				return found
			}
		}
	}
	return ""
}

// ReadCommitGoals lists and reads the live ledger plus both concluded-goal
// locations of one commit — the validator's and the projection's shared
// reader.
func ReadCommitGoals(root, commit string) (map[string][]byte, error) {
	out, err := gitIn(root, "ls-tree", "-r", "--name-only", commit, "--", goalsPrefix, recordsGoalsPrefix)
	if err != nil {
		return nil, fmt.Errorf("cannot list the ledger tree of %s: %w", commit, err)
	}
	files := map[string][]byte{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		content, err := gitIn(root, "cat-file", "-p", commit+":./"+p)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s at %s: %w", p, commit, err)
		}
		files[p] = []byte(content)
	}
	return files, nil
}

// ValidateCommit is the one-call seam Publish and the read-side
// advance share: parse the commit's ledger subtree and run every
// at-rest rule; the first problem set refuses the whole tree.
func ValidateCommit(root, commit string) error {
	files, err := ReadCommitGoals(root, commit)
	if err != nil {
		return err
	}
	t, problems := ParseTreeFiles(files)
	if len(problems) == 0 {
		problems = ValidateTree(t)
	}
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = string(p)
		}
		return fmt.Errorf("the ledger tree at %s does not validate:\n%s", commit, strings.Join(lines, "\n"))
	}
	return nil
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedGoalIds is the deterministic iteration order every reader
// shares.
func SortedGoalIds(m map[string]*GoalFile) []string { return sortedGoalIds(m) }

func sortedGoalIds(m map[string]*GoalFile) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
