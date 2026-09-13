package goal

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

var (
	abandonBuildStamp = func() string { return "dev" }
	abandonIsAncestor = func(root, ancestor, descendant string) (bool, error) {
		_, err := gitIn(root, "merge-base", "--is-ancestor", ancestor, descendant)
		if err == nil {
			return true, nil
		}
		var commandErr *gitError
		if errors.As(err, &commandErr) && commandErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	abandonRegistryProblems = func(string, func(a, b string) (bool, error), time.Time) ([]string, error) {
		return nil, fmt.Errorf("the fleet engine registry checker is not configured")
	}
	abandonBeforePush func(attempt int) error
)

// ConfigureAbandonFleetFloor joins the ledger policy to the executing
// engine's supervision view. The command entrypoint calls it before invoking
// Abandon; keeping the join there avoids making the ledger package depend on
// dispatch through supervision.
func ConfigureAbandonFleetFloor(buildStamp func() string, registryProblems func(string, func(a, b string) (bool, error), time.Time) ([]string, error)) {
	abandonBuildStamp = buildStamp
	abandonRegistryProblems = registryProblems
}

// EngineFloor records the oldest engine commit every enrolled seat is known
// to run. It changes only root history, keeping the record readable by older
// engines until the first abandoned goal introduces the new grammar.
func EngineFloor(r VerbRequest, commit string, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("engine-floor is a human act and names its human (--by)")
	}
	if proof == nil || !proof.ValidFor(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("engine-floor requires freshly observed enrolled-terminal human authority")
	}
	if !hexCommit(commit) {
		return PublishResult{}, fmt.Errorf("engine-floor line without a commit")
	}
	return Publish(r.Endpoint, engineFloorRequest(r, commit))
}

func engineFloorRequest(r VerbRequest, commit string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "engine-floor", Args: intentArgs(r, map[string]string{"commit": commit})},
		Message: "goal engine-floor " + commit,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if tree.Root == nil {
				return nil, fmt.Errorf("the ledger is not adopted")
			}
			if rootOpidLanded(tree.Root, r) {
				return nil, AlreadyApplied{}
			}
			tree.Root.Revision++
			tree.Root.History = append(tree.Root.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "engine-floor", Actor: r.Actor.historyActor(),
				Keep: -1, Reason: commit + " every enrolled seat runs this engine or newer",
			})
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(tree.Root)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// AbandonSpec carries the reason and the complete disposition of every live
// dependent of the goal set being abandoned.
type AbandonSpec struct {
	Because string
	Carried string
	Waive   []string
	Also    []string
}

type abandonArguments struct {
	waive map[string]string
	also  []string
	set   map[string]bool
}

// Abandon records that one goal and any named dependent goals will never be
// worked. The proof and argument checks deliberately precede every read.
func Abandon(r VerbRequest, id string, spec AbandonSpec, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("abandon is a human act and names its human (--by)")
	}
	if proof == nil || !proof.ValidFor(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("abandon requires freshly observed enrolled-terminal human authority")
	}
	if strings.TrimSpace(spec.Because) == "" || strings.ContainsAny(spec.Because, "\r\n") {
		return PublishResult{}, fmt.Errorf("abandon needs its reason on one line; a goal that will never be worked owes the reader why")
	}
	arguments, err := validateAbandonArguments(id, spec)
	if err != nil {
		return PublishResult{}, err
	}

	projection, err := Project(r.Endpoint, false, r.Now)
	if err != nil {
		return PublishResult{}, err
	}
	if err := requireAbandonFleetFloor(r, projection.Tree); err != nil {
		return PublishResult{}, err
	}

	revisions := map[string]uint64{}
	var held []*goalrevision.Held
	for _, goalID := range sortedSet(arguments.set) {
		file := projection.Tree.Live[goalID]
		if file == nil {
			file, _ = projection.Tree.Archived(goalID)
		}
		if file == nil {
			continue
		}
		revisions[goalID] = file.Revision
		if projection.Tree.Live[goalID] == nil {
			continue
		}
		lockRevision := file.Revision
		if file.State == StateClaimed && file.Claimed != nil {
			lockRevision = file.Claimed.Revision
		}
		lock, err := goalrevision.Acquire(r.Endpoint.Root, goalID, lockRevision, "goal-abandon")
		if err != nil {
			for index := len(held) - 1; index >= 0; index-- {
				_ = held[index].Release()
			}
			return PublishResult{}, fmt.Errorf("goal abandon could not acquire the goal-revision lock: %w", err)
		}
		held = append(held, lock)
	}
	defer func() {
		for index := len(held) - 1; index >= 0; index-- {
			_ = held[index].Release()
		}
	}()

	jobs, err := nonTerminalGoalJobs(r.Endpoint.Root, arguments.set)
	if err != nil {
		return PublishResult{}, err
	}
	if len(jobs) != 0 {
		return PublishResult{}, fmt.Errorf("goal abandon refuses while non-terminal jobs name the abandoned set: %s; cancel each one with metasystem delegate --cancel <job>", strings.Join(jobs, ", "))
	}
	debtDetail := abandonedReviewDebtDetail(projection.Tree, arguments.set)
	result, err := Publish(r.Endpoint, abandonRequest(r, id, spec, arguments, revisions))
	if err == nil && result.Outcome == OutcomeConfirmed && debtDetail != "" {
		if result.Detail != "" {
			result.Detail += "\n"
		}
		result.Detail += debtDetail
	}
	return result, err
}

func abandonedReviewDebtDetail(tree *TreeGoals, set map[string]bool) string {
	var lines []string
	for _, id := range sortedSet(set) {
		file := tree.Live[id]
		if file == nil {
			file = tree.Abandoned[id]
		}
		if file == nil {
			continue
		}
		for _, obligation := range file.ReviewObligations {
			if obligation.State == "open" {
				lines = append(lines, fmt.Sprintf("goal %s has open review obligation finding=%s chain=%s; it still needs discharge", id, obligation.Finding, obligation.Chain))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func validateAbandonArguments(id string, spec AbandonSpec) (abandonArguments, error) {
	args := abandonArguments{waive: map[string]string{}, set: map[string]bool{id: true}}
	var invalid []string
	if !validId(id) {
		invalid = append(invalid, id)
	}
	alsoSeen := map[string]bool{}
	for _, goalID := range spec.Also {
		if !validId(goalID) {
			invalid = append(invalid, goalID)
		}
		alsoSeen[goalID] = true
		args.set[goalID] = true
	}
	if spec.Carried != "" && !validId(spec.Carried) {
		invalid = append(invalid, spec.Carried)
	}
	if len(invalid) != 0 {
		return abandonArguments{}, fmt.Errorf("abandon needs valid goal ids: %s", strings.Join(invalid, ", "))
	}
	args.also = sortedSet(alsoSeen)
	for _, entry := range spec.Waive {
		dependent, reason, found := strings.Cut(entry, "=")
		if !found || !validId(dependent) || strings.TrimSpace(reason) == "" || strings.ContainsAny(reason, "\r\n") {
			return abandonArguments{}, fmt.Errorf("waive names its dependent and its reason: --waive <dependent>=<reason>")
		}
		if _, duplicate := args.waive[dependent]; duplicate {
			return abandonArguments{}, fmt.Errorf("dependent %s is waived twice; name it once", dependent)
		}
		args.waive[dependent] = reason
	}
	var conflicts []string
	for dependent := range args.waive {
		if alsoSeen[dependent] {
			conflicts = append(conflicts, dependent)
		}
	}
	if len(conflicts) != 0 {
		sort.Strings(conflicts)
		lines := make([]string, 0, len(conflicts))
		for _, dependent := range conflicts {
			lines = append(lines, fmt.Sprintf("dependent %s is both waived and abandoned; choose one", dependent))
		}
		return abandonArguments{}, fmt.Errorf("%s", strings.Join(lines, "\n"))
	}
	if alsoSeen[id] || args.waive[id] != "" || spec.Carried == id || alsoSeen[spec.Carried] {
		return abandonArguments{}, fmt.Errorf("carried must name a live successor")
	}
	return args, nil
}

func requireAbandonFleetFloor(r VerbRequest, tree *TreeGoals) error {
	floor := ""
	if tree != nil {
		floor = newestEngineFloor(tree.Root)
	}
	stamp := abandonBuildStamp()
	if floor == "" {
		return fmt.Errorf("abandon writes a state that engines older than this build refuse, and the ledger has no record that the fleet runs it; after every enrolled seat has rebuilt and re-armed (metasystem supervise status --repo <checkout> on each machine), a human records the floor: metasystem goal engine-floor --root . --commit %s --by <name>", stamp)
	}
	commit, dirty, ok := enginebuild.StampCommit(stamp)
	if !ok {
		return fmt.Errorf("this engine's build (%s) cannot be placed against the fleet floor %s; build it with scripts/agents/go-build.sh", stamp, floor)
	}
	atOrAbove, err := abandonIsAncestor(r.Endpoint.Root, floor, commit)
	if err != nil {
		return fmt.Errorf("place this engine's build against fleet floor %s: %w", floor, err)
	}
	if !atOrAbove {
		dirtyText := ""
		if dirty {
			dirtyText = " (dirty build)"
		}
		return fmt.Errorf("this engine's build (%s)%s is below the fleet floor %s; build it with scripts/agents/go-build.sh", stamp, dirtyText, floor)
	}
	problems, err := abandonRegistryProblems(floor, func(ancestor, descendant string) (bool, error) {
		return abandonIsAncestor(r.Endpoint.Root, ancestor, descendant)
	}, r.Now)
	if err != nil {
		return err
	}
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func abandonRequest(r VerbRequest, id string, spec AbandonSpec, arguments abandonArguments, projectedRevisions map[string]uint64) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "abandon", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"because": spec.Because, "carried": spec.Carried, "waive": strings.Join(spec.Waive, ";"), "also": strings.Join(spec.Also, ","),
		})},
		Message: "goal abandon " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if archived, found := tree.Archived(id); found {
				if opidLanded(archived, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(archived)}
			}
			if tree.Live[id] == nil {
				return nil, fmt.Errorf("goal %s is not live; nothing to abandon", id)
			}
			if _, projected := projectedRevisions[id]; !projected {
				return nil, fmt.Errorf("goal %s is not live; nothing to abandon", id)
			}
			for _, goalID := range arguments.also {
				if _, projected := projectedRevisions[goalID]; !projected {
					if file := tree.Live[goalID]; file != nil && transitiveLiveDependents(tree, id)[goalID] {
						return nil, fmt.Errorf("goal %s changed under abandon's lock (it was not live in the projection and is now revision %d); re-read and retry", goalID, file.Revision)
					}
					return nil, fmt.Errorf("abandon takes one goal; --also names only its live dependents")
				}
			}
			for _, goalID := range sortedSet(arguments.set) {
				if file := tree.Live[goalID]; file != nil && file.Revision != projectedRevisions[goalID] {
					return nil, fmt.Errorf("goal %s changed under abandon's lock (revision %d is now %d); re-read and retry", goalID, projectedRevisions[goalID], file.Revision)
				}
			}
			liveDependents := transitiveLiveDependents(tree, id)
			for _, goalID := range arguments.also {
				if tree.Live[goalID] == nil || !liveDependents[goalID] {
					return nil, fmt.Errorf("abandon takes one goal; --also names only its live dependents")
				}
			}
			if spec.Carried != "" && (tree.Live[spec.Carried] == nil || arguments.set[spec.Carried]) {
				return nil, fmt.Errorf("carried must name a live successor")
			}

			dependents := directLiveDependents(tree, arguments.set)
			var uncovered []string
			for _, dependent := range sortedSliceMapKeys(dependents) {
				if arguments.set[dependent] || arguments.waive[dependent] != "" || spec.Carried != "" {
					continue
				}
				for _, blocker := range dependents[dependent] {
					uncovered = append(uncovered, fmt.Sprintf("goal %s is blocked by %s; re-point it with --carried, waive it with --waive %s=<reason>, or abandon it with --also %s", dependent, blocker, dependent, dependent))
				}
			}
			if len(uncovered) != 0 {
				return nil, fmt.Errorf("%s", strings.Join(uncovered, "\n"))
			}
			var badWaivers []string
			for dependent := range arguments.waive {
				if tree.Live[dependent] == nil || len(dependents[dependent]) == 0 {
					badWaivers = append(badWaivers, dependent)
				}
			}
			if len(badWaivers) != 0 {
				sort.Strings(badWaivers)
				return nil, fmt.Errorf("--waive names %s, which is not a live dependent of the abandoned set", strings.Join(badWaivers, ", "))
			}
			for _, goalID := range sortedSet(arguments.set) {
				if err := abandonCarryRefusal(r.Endpoint.Root, tree, carryCodeTip(r.Endpoint, tip), goalID, tree.Live[goalID], r.Now); err != nil {
					return nil, err
				}
			}

			abandonedIDs := sortedSet(arguments.set)
			departed := make([]*GoalFile, 0, len(abandonedIDs))
			for _, goalID := range abandonedIDs {
				if file := tree.Live[goalID]; file != nil {
					departed = append(departed, file)
				}
			}
			for _, goalID := range arguments.also {
				if tree.Live[goalID] == nil {
					return nil, fmt.Errorf("abandon takes one goal; --also names only its live dependents")
				}
			}
			for _, file := range departed {
				delete(tree.Live, file.Id)
			}
			compactions := compactDepartedPriorities(tree.Live, departed)
			changes := map[string]Change{}
			for _, file := range departed {
				displaced := ""
				if file.Claimed != nil {
					displaced = pairMarker(file.Claimed)
				}
				stopID := ""
				if file.StopFence != nil {
					stopID = file.StopFence.StopID
				} else {
					file.StopCapability = nil
				}
				file.State = StateAbandoned
				file.Claimed = nil
				file.Obligation = nil
				file.Landing = nil
				file.Parked = nil
				targets := abandonPriorityTargets(file.Id, abandonedIDs, compactions, file.Priority)
				file.Abandoned = &AbandonRecord{By: r.Actor.historyActor(), At: r.stamp(), Revision: file.Revision + 1, Opid: r.opid(), Displaced: displaced, StopID: stopID, Carried: spec.Carried, Because: spec.Because}
				touchDisplaced(file, r, "abandon", targets, displaced)
				event := &file.History[len(file.History)-1]
				event.StopID = stopID
				event.Carried = spec.Carried
				event.Reason = spec.Because
				tree.Abandoned[file.Id] = file
				changes[livePath(file.Id)] = Change{Path: livePath(file.Id), Delete: true}
				changes[donePath(file.Id)] = Change{Path: donePath(file.Id), Content: RenderFile(file)}
			}

			for _, compaction := range compactions {
				targets := append(append([]string(nil), compaction.Departed...), compactionSurvivors(compaction)...)
				for _, priorityChange := range compaction.Changed {
					mergeAbandonEvent(priorityChange.File, r, targets, fmt.Sprintf("priority-order from=%s to=%s", renderRank(priorityChange.Before), renderRank(priorityChange.After)))
					changes[livePath(priorityChange.File.Id)] = Change{Path: livePath(priorityChange.File.Id), Content: RenderFile(priorityChange.File)}
				}
			}

			for _, dependent := range sortedSliceMapKeys(dependents) {
				if arguments.set[dependent] {
					continue
				}
				file := tree.Live[dependent]
				removed := dependents[dependent]
				var reasons []string
				repointedParkTo := ""
				if waiver, waived := arguments.waive[dependent]; waived {
					file.Blocked = removeBlockers(file.Blocked, arguments.set)
					for _, blocker := range removed {
						reasons = append(reasons, fmt.Sprintf("blocker %s waived: %s", blocker, waiver))
					}
				} else {
					file.Blocked = repointBlockers(file.Blocked, arguments.set, spec.Carried)
					repointedParkTo = spec.Carried
					for _, blocker := range removed {
						reasons = append(reasons, fmt.Sprintf("blockedBy %s re-pointed to %s", blocker, spec.Carried))
					}
				}
				mergeAbandonEvent(file, r, removed, strings.Join(reasons, "; "))
				if repairAbandonedBlockerPark(tree, file, repointedParkTo) {
					file.State = restingState(file)
					file.Parked = nil
					touch(file, r, "unpark", []string{file.Id})
					file.History[len(file.History)-1].Reason = fmt.Sprintf("blocker %s was abandoned and waived; the park lifts", strings.Join(removed, ", "))
				}
				changes[livePath(file.Id)] = Change{Path: livePath(file.Id), Content: RenderFile(file)}
			}

			var paths []string
			for path := range changes {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			ordered := make([]Change, 0, len(paths))
			for _, path := range paths {
				change := changes[path]
				if !change.Delete && strings.HasPrefix(path, goalsPrefix) {
					goalID := strings.TrimSuffix(strings.TrimPrefix(path, goalsPrefix), ".md")
					if file := tree.Live[goalID]; file != nil {
						change.Content = RenderFile(file)
					}
				}
				ordered = append(ordered, change)
			}
			return ackDisplacements(tree, r, ordered), nil
		},
		Validate:   func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
		BeforePush: abandonBeforePush,
	}
}

// abandonCarryRefusal keeps every carry permission reachable until it is
// closed or expires, and keeps an in-flight landing bound to a live goal.
func abandonCarryRefusal(root string, tree *TreeGoals, codeTip, id string, file *GoalFile, now time.Time) error {
	refs := map[string]bool{}
	for _, history := range file.History {
		if history.Verb == "carrying" && history.ApprovedRef != "" {
			refs[history.ApprovedRef] = true
		}
	}
	goalOnly := &TreeGoals{Root: tree.Root, Live: map[string]*GoalFile{id: file}, Done: map[string]*GoalFile{}, Abandoned: map[string]*GoalFile{}}
	for _, word := range carryWords(goalOnly) {
		if !CarryWordProven(root, word) {
			continue
		}
		consumption, err := CarryConsumptionAt(root, tree, codeTip, word)
		if err != nil {
			return err
		}
		if consumption.Kind == "origin" {
			return fmt.Errorf("goal %s has carried commit %s without its ledger row; close it with land.sh --carried %s, even if the word has expired; then retry abandon", id, consumption.ID, word.History.Opid)
		}
		if reservation := CarryReservationAt(tree, id, word.History.Opid, now); reservation.State == "open" {
			seat, err := OpidMachine(reservation.History.Opid)
			if err != nil {
				return err
			}
			return fmt.Errorf("carry reservation %s on goal %s is in flight on %s; use goal carrying --abandon %s on that seat or wait for its expiry; then retry abandon", reservation.History.Opid, id, seat, reservation.History.Opid)
		}
		delete(refs, word.History.Opid)
		if now.Before(word.Expires) && (consumption.Kind == "none" || consumption.Kind == "missing-anchor") {
			return fmt.Errorf("goal %s has open carry word %s; finish its landing, supersede it on a live goal with goal carry --supersede %s, or let it expire at %s; then retry abandon", id, word.History.Opid, word.History.Opid, word.Expires.UTC().Format(time.RFC3339))
		}
	}
	for _, ref := range sortedSet(refs) {
		reservation := CarryReservationAt(tree, id, ref, now)
		if reservation.State != "open" {
			continue
		}
		seat, err := OpidMachine(reservation.History.Opid)
		if err != nil {
			return err
		}
		return fmt.Errorf("carry reservation %s on goal %s is in flight on %s; use goal carrying --abandon %s on that seat or wait for its expiry; then retry abandon", reservation.History.Opid, id, seat, reservation.History.Opid)
	}
	return nil
}

// repairAbandonedBlockerPark keeps a blocker-created park bound to the
// rewritten dependency list. The caller lifts the park when that list has no
// unfinished blocker, even when its old marker names an earlier completed one.
func repairAbandonedBlockerPark(tree *TreeGoals, file *GoalFile, repointedTo string) bool {
	if file.State != StateParked || file.Parked == nil || file.Parked.Blocker == "" {
		return false
	}
	if repointedTo != "" && contains(file.Blocked, repointedTo) && depState(tree, repointedTo) != StateDone {
		file.Parked.Blocker = repointedTo
		file.Parked.Because = "blocked by " + repointedTo + "; returns when it is done"
		return false
	}
	for _, blocker := range file.Blocked {
		if depState(tree, blocker) != StateDone {
			file.Parked.Blocker = blocker
			file.Parked.Because = "blocked by " + blocker + "; returns when it is done"
			return false
		}
	}
	return true
}

func transitiveLiveDependents(tree *TreeGoals, id string) map[string]bool {
	closure := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for goalID, file := range tree.Live {
			if closure[goalID] {
				continue
			}
			for _, blocker := range file.Blocked {
				if closure[blocker] {
					closure[goalID] = true
					changed = true
					break
				}
			}
		}
	}
	return closure
}

func directLiveDependents(tree *TreeGoals, abandoned map[string]bool) map[string][]string {
	dependents := map[string][]string{}
	for goalID, file := range tree.Live {
		for _, blocker := range file.Blocked {
			if abandoned[blocker] {
				dependents[goalID] = append(dependents[goalID], blocker)
			}
		}
		dependents[goalID] = sortedUnique(dependents[goalID])
	}
	return dependents
}

func sortedSliceMapKeys(set map[string][]string) []string {
	ids := make([]string, 0, len(set))
	for id, values := range set {
		if len(values) != 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func compactionSurvivors(compaction priorityCompaction) []string {
	ids := make([]string, 0, len(compaction.Changed))
	for _, change := range compaction.Changed {
		ids = append(ids, change.File.Id)
	}
	return ids
}

func abandonPriorityTargets(id string, allDeparted []string, compactions []priorityCompaction, priority uint8) []string {
	result := []string{id}
	for _, departed := range allDeparted {
		if departed != id {
			result = append(result, departed)
		}
	}
	for _, compaction := range compactions {
		if compaction.Priority == priority {
			result = append(result, compactionSurvivors(compaction)...)
		}
	}
	return result
}

func mergeAbandonEvent(file *GoalFile, r VerbRequest, targets []string, reason string) {
	index := len(file.History) - 1
	if index < 0 || file.History[index].Opid != r.opid() {
		touch(file, r, "abandon", append([]string(nil), targets...))
		index = len(file.History) - 1
	} else {
		seen := map[string]bool{}
		for _, target := range file.History[index].Targets {
			seen[target] = true
		}
		for _, target := range targets {
			if !seen[target] {
				file.History[index].Targets = append(file.History[index].Targets, target)
				seen[target] = true
			}
		}
	}
	if reason != "" {
		if file.History[index].Reason != "" {
			file.History[index].Reason += "; "
		}
		file.History[index].Reason += reason
	}
}

func removeBlockers(blocked []string, abandoned map[string]bool) []string {
	var result []string
	for _, blocker := range blocked {
		if !abandoned[blocker] {
			result = append(result, blocker)
		}
	}
	return sortedUnique(result)
}

func repointBlockers(blocked []string, abandoned map[string]bool, successor string) []string {
	var result []string
	for _, blocker := range blocked {
		if abandoned[blocker] {
			result = append(result, successor)
		} else {
			result = append(result, blocker)
		}
	}
	return sortedUnique(result)
}

func newestEngineFloor(root *RootRecord) string {
	if root == nil {
		return ""
	}
	for i := len(root.History) - 1; i >= 0; i-- {
		line := root.History[i]
		if line.Verb != "engine-floor" {
			continue
		}
		words := strings.Fields(line.Reason)
		if len(words) > 0 {
			return words[0]
		}
	}
	return ""
}

func sortedSet(set map[string]bool) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
