package batch

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// AdmissionCarried is a change member's admission: the seat's commit
// boundary proved its tree before the change joined, so the lane runs no
// admission of its own for it.
const AdmissionCarried = "carried"

// ReturnRecorded settles a change member's return: there is no claim to hand
// back, so the outcome and its reason stay on the unit, where landing status
// and the repeat of the asker's work land print them.
const ReturnRecorded = "recorded"

// LandingChangeTrailer names a replayed change on main; recovery after a push
// finds the change by it.
const LandingChangeTrailer = "Landing-Change"

// ChangeMember is work made without a goal (U11b): one commit the asker made
// through the commit boundary on its seat, with the boundary's trailers.
// Goal and GoalRevision are the commit's Goal-Item and Goal-Revision when it
// was made in a goal's name; a change never holds or spends that goal.
type ChangeMember struct {
	Commit       string `json:"commit"`
	Parent       string `json:"parent"`
	AskedBy      string `json:"askedBy"`
	Subject      string `json:"subject"`
	Goal         string `json:"goal,omitempty"`
	GoalRevision uint64 `json:"goalRevision,omitempty"`
	// GateTip is the tip the goal's landing gate was read at on the seat.
	GateTip string `json:"gateTip,omitempty"`
}

// ChangeID is a change member's stable id: change:<first 12 of its commit>.
func ChangeID(commit string) string {
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return "change:" + commit
}

// NewChangeUnit is a change member as the lane records it: keyed by its
// change id (goal and chain fields alike), its claim naming only the asker's
// seat and lineage, with no epoch and no revisions.
func NewChangeUnit(change ChangeMember, seatRoot, machine, lineage string, changedPaths []string, closure *adapter.Closure) Unit {
	id := ChangeID(change.Commit)
	paths := slices.Clone(changedPaths)
	slices.Sort(paths)
	return Unit{GoalID: id, Chain: id, SeatRoot: seatRoot, Claim: Claim{Machine: machine, Lineage: lineage}, State: UnitJoining,
		ChangedPaths: paths, Closure: closure, Change: &change}
}

// IsChange reports whether the unit is a change member.
func (unit Unit) IsChange() bool { return unit.Change != nil }

// ChargeMember is the member a batch's proofs are accounted to: its last
// goal member. Every proof is charged to a goal and a change has none, so a
// change rides on this member's proof; false when the units hold no goal.
func ChargeMember(units []Unit) (Unit, bool) {
	for index := len(units) - 1; index >= 0; index-- {
		if !units[index].IsChange() {
			return units[index], true
		}
	}
	return Unit{}, false
}

// ChangeJoin is one change joining the lane: the prepared unit, the fetched
// origin tree the join addresses, and the id a new batch takes when no batch
// is open.
type ChangeJoin struct {
	Unit            Unit
	BaseTree, NewID string
	Actor           string
	At              time.Time
}

// liveChangeStates are the states in which a change is still the lane's.
var liveChangeStates = []string{UnitJoining, UnitJoined, UnitReturnPending, UnitLanded}

// JoinChange joins a change to the open batch a join addresses (a new batch
// on the base when none is open), under the store lock: membership, the whole
// series re-assembled with the change last, and the change's own tree on the
// batch base as its carried admission. A change already the lane's (joined,
// being returned, or landed) is returned unchanged. Any record that cannot be
// read refuses the join; it never reads as "not a member".
func JoinChange(store Store, join ChangeJoin) (Record, error) {
	unit := join.Unit
	if unit.Change == nil || unit.GoalID != ChangeID(unit.Change.Commit) || unit.Chain != unit.GoalID || unit.Claim.Machine == "" || unit.Claim.Lineage == "" {
		return Record{}, fmt.Errorf("BATCH_CHANGE_UNREADABLE: change member %s is incomplete", unit.GoalID)
	}
	var joined Record
	err := store.locked(func() error {
		records, err := store.Records()
		if err != nil {
			return err
		}
		for _, record := range records {
			for _, existing := range record.Units {
				if existing.GoalID == unit.GoalID && slices.Contains(liveChangeStates, existing.State) {
					joined = record
					return nil
				}
			}
		}
		record, found := JoinableOpen(records, join.BaseTree)
		if !found {
			record = Record{Schema: 1, BatchID: join.NewID, BaseTree: join.BaseTree, TipTree: join.BaseTree}
			record.Transition(StateOpen, join.At, "open", join.Actor, "")
		} else if err := joinRefusal(record); err != nil {
			return err
		}
		live := slices.DeleteFunc(slices.Clone(record.Units), func(existing Unit) bool {
			return existing.State != UnitJoined && existing.State != UnitJoining
		})
		unit.State = UnitJoined
		prefixes, err := store.reassembly.assemble(record.BaseTree, append(live, unit))
		if err != nil {
			return err
		}
		if len(prefixes) != len(live)+1 {
			return fmt.Errorf("assemble change %s: prefixes=%d", unit.GoalID, len(prefixes))
		}
		// A change stacked on a change of this batch has no tree of its own
		// on the base: its admitted tree is the series through it.
		ownTree := prefixes[len(prefixes)-1]
		if !slices.ContainsFunc(live, func(member Unit) bool { return member.IsChange() && member.Change.Commit == unit.Change.Parent }) {
			own, err := store.reassembly.assemble(record.BaseTree, []Unit{unit})
			if err != nil || len(own) != 1 {
				return fmt.Errorf("assemble change %s: own=%d: %w", unit.GoalID, len(own), err)
			}
			ownTree = own[0]
		}
		unit.Admission = &JoinAdmission{Tree: ownTree, Status: AdmissionCarried}
		mutate := func(current *Record) error {
			current.Units = append(current.Units, unit)
			current.PrefixTrees, current.TipTree = prefixes, prefixes[len(prefixes)-1]
			appendUnitHistory(current, join.At, "join", join.Actor, unit.GoalID, "", UnitJoined)
			joined = *current
			return nil
		}
		if !found {
			if err := mutate(&record); err != nil {
				return err
			}
			return store.write(record)
		}
		return store.updateLocked(record.BatchID, mutate)
	})
	return joined, err
}

// assembleChangeUnit applies a change's commit alone on base in a detached
// worktree; a change that does not apply is the member's composition
// conflict, with the files named.
func assembleChangeUnit(root, base string, unit Unit) (next string, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(base)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	workspace := detached.Workspace()
	if err := applyBranchCommit(root, workspace.Dir, unit.Change.Commit); err != nil {
		var conflict *patchApplyConflict
		if errors.As(err, &conflict) {
			paths := exec.Command("git", "-C", workspace.Dir, "diff", "--name-only", "--diff-filter=U", "-z")
			paths.Env = gittree.ScrubbedEnviron()
			raw, _ := paths.Output()
			files := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
			if len(raw) == 0 {
				files = nil
			}
			return "", &assemblyConflict{GoalID: unit.GoalID, Paths: files, Cause: refuseBatch("BATCH_JOIN_CONFLICT", "change "+unit.GoalID+" does not apply: "+err.Error())}
		}
		return "", err
	}
	return workspace.Snapshot("HEAD")
}

var trailerLine = regexp.MustCompile(`^[A-Za-z0-9-]+: `)

// ChangeLandingMessage is the asker's message with the Landing-Change
// trailer appended to its trailer block (a new block when it has none).
func ChangeLandingMessage(message, id string) string {
	message = strings.TrimRight(message, "\n \t")
	paragraphs := strings.Split(message, "\n\n")
	last := paragraphs[len(paragraphs)-1]
	separator := "\n\n"
	if len(paragraphs) > 1 && !slices.ContainsFunc(strings.Split(last, "\n"), func(line string) bool { return !trailerLine.MatchString(line) }) {
		separator = "\n"
	}
	return message + separator + LandingChangeTrailer + ": " + id + "\n"
}

// ReplayChange lands a change member's commit on the landing branch checked
// out in root: its patch applied with the assembly's own apply and contract
// checks, then one commit that keeps the asker's message, trailers, author
// and committer and adds the Landing-Change trailer. It returns the commit.
func ReplayChange(root string, unit Unit) (string, error) {
	if unit.Change == nil {
		return "", fmt.Errorf("BATCH_CHANGE_UNREADABLE: unit %s is not a change", unit.GoalID)
	}
	if _, err := batchMergeDriverArgs(); err != nil {
		return "", err
	}
	before, err := landingGitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if err := applyBranchCommit(root, root, unit.Change.Commit); err != nil {
		return "", err
	}
	tree, err := landingGitOutput(root, "write-tree")
	if err != nil {
		return "", err
	}
	commit, err := commitChange(root, unit, tree, before)
	if err != nil {
		return "", err
	}
	if _, err := landingGitOutput(root, "update-ref", "-m", "replay "+unit.GoalID, "HEAD", commit, before); err != nil {
		return "", fmt.Errorf("replay %s: move the landing branch: %w", unit.GoalID, err)
	}
	return commit, nil
}

// commitChange writes the replay commit object of a change on parent.
func commitChange(root string, unit Unit, tree, parent string) (string, error) {
	message, err := branchCommitMessage(root, unit.Change.Commit)
	if err != nil {
		return "", fmt.Errorf("BATCH_CHANGE_UNREADABLE: change %s message: %w", unit.GoalID, err)
	}
	identity, err := landingGitOutput(root, "show", "-s", "--format=%an%x00%ae%x00%aI%x00%cn%x00%ce", unit.Change.Commit)
	fields := strings.Split(identity, "\x00")
	if err != nil || len(fields) != 5 {
		return "", fmt.Errorf("BATCH_CHANGE_UNREADABLE: change %s identity: %v", unit.GoalID, err)
	}
	command := exec.Command("git", "-C", root, "commit-tree", tree, "-p", parent)
	command.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_NAME="+fields[0], "GIT_AUTHOR_EMAIL="+fields[1], "GIT_AUTHOR_DATE="+fields[2],
		"GIT_COMMITTER_NAME="+fields[3], "GIT_COMMITTER_EMAIL="+fields[4])
	command.Stdin = strings.NewReader(ChangeLandingMessage(string(message), unit.GoalID))
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("replay %s: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// UnitClosure asks the checkout's language adapter for a member's changed
// and dependent units at its tree; nil when no adapter can say.
func UnitClosure(root, baseTree, tree string) *adapter.Closure {
	return unitClosure(root, baseTree, tree)
}

// ChargeUnit is the member a batch's proofs are keyed to: its last goal
// member, else its last member, a change, whose proofs the lane owner
// charges to the lane itself (U11b). The zero unit when units is empty.
func ChargeUnit(units []Unit) Unit {
	if charge, ok := ChargeMember(units); ok {
		return charge
	}
	if len(units) == 0 {
		return Unit{}
	}
	return units[len(units)-1]
}

// HeldCommitRefusal is held's refusal of one commit of the series at the
// push; the landing ejects the change whose replay it names.
type HeldCommitRefusal struct {
	Commit string
	Cause  error
}

func (refusal *HeldCommitRefusal) Error() string { return refusal.Cause.Error() }
func (refusal *HeldCommitRefusal) Unwrap() error { return refusal.Cause }

// RecordHold writes why a batch cannot proceed now (a lane that cannot be
// named, an engine that cannot plan on the lane) where every reader shows
// it: one "hold" history entry per distinct reason, the batch's state kept.
func RecordHold(store Store, id, actor, reason string, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		if last := lastHistory(*record); last.Verb == "hold" && last.Detail == reason {
			return nil
		}
		record.Transition(record.State, at, "hold", actor, reason)
		return nil
	})
}

// HoldReason is the plain reason a batch holds, when its last word is a hold
// or a refused proof admission; empty otherwise.
func HoldReason(record Record) string {
	if last := lastHistory(record); last.Verb == "hold" || last.Verb == "prove-refused" {
		return last.Detail
	}
	return ""
}
