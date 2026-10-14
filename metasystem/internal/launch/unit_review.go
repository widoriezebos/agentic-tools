package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UnitSubject binds one completed round's result to the goal-branch unit
// commit that carries it. The run keeps it so a repeated committed-review
// request reaches the same operation, commit and publication instead of
// creating a second subject. A prior round's subject stays for diagnosis
// after a later round amends it.
type UnitDrop struct {
	Decisions, Requirements, Phase string
	Actor, Reason, Impact, At      string
	Revision                       uint64
	Covered                        []string
	PatchDigest, StartingResult    string
	ResultTree, CommitTree         string
	PendingPatch                   []byte
	ConflictTrees                  []string
	Subject                        UnitSubject
}

type UnitSubject struct {
	Drop         *UnitDrop           `json:"drop,omitempty"`
	GateWorktree string              `json:"gateWorktree,omitempty"`
	GateSnapshot *repositorySnapshot `json:"gateSnapshot,omitempty"`
	GateLaunches []string            `json:"gateLaunches,omitempty"`
	GateRunID    string              `json:"gateRunId,omitempty"`
	Conflict     string              `json:"conflict,omitempty"`
	Round        int                 `json:"round"`
	Operation    string              `json:"operation"`
	// ExpectedParent is the publication parent after replay, or the branch
	// tip an amendment replaces. The round retains its original parent.
	ExpectedParent string `json:"expectedParent"`
	// ResultDigest is the SHA-256 of the round's retained result: the raw
	// diff of its proof-after snapshot. It is not a Git tree identifier.
	ResultDigest string   `json:"resultDigest"`
	DiffDigest   string   `json:"diffDigest"`
	Paths        []string `json:"paths"`
	// StagedTree is the Git tree the staged result wrote, retained before
	// the commit so an unknown commit outcome is reconciled exactly.
	StagedTree string `json:"stagedTree,omitempty"`
	// Amends is the earlier round's commit this round's subject replaces.
	Amends       string `json:"amends,omitempty"`
	AmendsParent string `json:"amendsParent,omitempty"`
	// Commit is the unit's own Goal-Unit commit; Tip is the branch tip the
	// commit owner installed (a later replayed commit after an amend).
	Commit    string `json:"commit,omitempty"`
	Tip       string `json:"tip,omitempty"`
	Published string `json:"published,omitempty"`
	// Examination is the critic chain whose completed return examined this
	// subject; ExaminationRound is that return's round. A failed critic
	// round with no return never sets them.
	UnknownRetries   int      `json:"unknownRetries,omitempty"`
	ExaminationJob   string   `json:"examinationJob,omitempty"`
	TransferredTo    []string `json:"transferredTo,omitempty"`
	Examination      string   `json:"examination,omitempty"`
	ExaminationRound int64    `json:"examinationRound,omitempty"`
	// ExaminationReturnPath is the return in the store the review resolved.
	ExaminationReturnPath string `json:"examinationReturnPath,omitempty"`
	PublishedAt           string `json:"publishedAt,omitempty"`
}

// UnitReview is a completed round as a committed review consumes it.
type UnitReview struct {
	AdmitChild func(string) error
	Wait       func(func() error) error
	Whole      bool
	Record     UnitRunRecord
	Round      UnitRound
	// Head is the completed round's observed HEAD; Result is its retained
	// raw diff (proof-after snapshot Tree), compared byte for byte. Base and
	// Diff are the plan's cumulative diff base and the round's retained
	// worktree.diff, whose full-index bytes bind every result format.
	Head, Result string
	Base         string
	Diff         []byte
	DiffDigest   string
	// Legacy marks a snapshot retained before full object ids and exact
	// path bytes; only its retained diff is compared exactly.
	Legacy           bool
	BuildBrief       string
	BuildBriefSHA256 string
	Subject          *UnitSubject
	Prior            *UnitSubject
}

// UnitReviewReadyOutcomes are the round outcomes whose proof passed and
// whose result the proof left unchanged. A clean read by another model than
// the builder's is the unit's read; other reads are feedback and work review
// asks the committed critic. The read never refuses a committed review.
var UnitReviewReadyOutcomes = map[string]bool{"green": true, "read-failed": true, "read-compacted": true}

// ErrRunStillRunning marks the UNIT_REVIEW_NOT_READY refusal of a run whose
// attempt has not finished: the caller offers to wait, decided by errors.Is.
var ErrRunStillRunning = errors.New("the run is still running")

type stillRunningError struct{ error }

func (stillRunningError) Is(target error) bool { return target == ErrRunStillRunning }

func reviewStillRunning(id, state string) error {
	return coded("UNIT_REVIEW_NOT_READY", "run="+id+" state="+state,
		stillRunningError{fmt.Errorf("run %s is still running, so there is no result to review yet", id)})
}

// ReviewSubject calls bind under the run's lock with its latest completed
// round. retain records that round's subject in the run before the caller's
// next external effect; it is the run's only subject writer.
func (runner *UnitRunner) ReviewSubject(id string, bind func(review UnitReview, retain func(UnitSubject) error) error) error {
	if runner.Manager == nil && runner.Root == "" {
		return fmt.Errorf("unit run store is unavailable")
	}
	current, err := runner.read(id)
	if err != nil {
		return coded("UNIT_RUN_UNKNOWN", "run="+id, fmt.Errorf("there is no work run %s: %v", id, err))
	}
	if runner.Manager != nil && runner.tree == nil {
		_, err := treeCall(runner, current.Worktree, func(bound *UnitRunner) (struct{}, error) {
			return struct{}{}, bound.ReviewSubject(id, bind)
		})
		return err
	}
	if runner.Manager != nil {
		if err := runner.GateTree(current.Worktree, id, nil); err != nil {
			return err
		}
	}
	if runner.Manager != nil {
		_, key, err := namedUnitIdentity(UnitPlan{Worktree: current.Worktree, Goal: current.Goal, Unit: current.Unit})
		if err != nil {
			return err
		}
		held, err := runner.namedLock(key, UnitPlan{Goal: current.Goal, Unit: current.Unit})
		if err != nil {
			return err
		}
		defer releaseUnitLock(held)
	}
	lock, err := runner.lock(id)
	if err != nil {
		return err
	}
	defer releaseUnitLock(lock)
	record, err := runner.read(id)
	if err != nil {
		return err
	}
	if record.State != "awaiting-judgement" || len(record.Rounds) == 0 {
		return reviewStillRunning(id, string(record.State))
	}
	round := record.Rounds[len(record.Rounds)-1]
	if !UnitReviewReadyOutcomes[round.Outcome] {
		return coded("UNIT_REVIEW_NOT_READY", fmt.Sprintf("run=%s round=%d outcome=%s", id, round.Number, round.Outcome),
			fmt.Errorf("attempt %d ended %s; only an attempt whose checks passed with the result unchanged is reviewed", round.Number, round.Outcome))
	}
	var after repositorySnapshot
	data, err := os.ReadFile(filepath.Join(round.Directory, "proof-after.json"))
	if err == nil {
		err = json.Unmarshal(data, &after)
	}
	if err != nil {
		return coded("UNIT_REVIEW_NOT_READY", fmt.Sprintf("run=%s round=%d", id, round.Number), fmt.Errorf("the result of attempt %d was not kept, so it cannot be reviewed: %v", round.Number, err))
	}
	diff, err := os.ReadFile(filepath.Join(round.Directory, "worktree.diff"))
	if err != nil {
		return coded("UNIT_REVIEW_NOT_READY", fmt.Sprintf("run=%s round=%d", id, round.Number), fmt.Errorf("the changes of attempt %d were not kept, so they cannot be reviewed: %v", round.Number, err))
	}
	review := UnitReview{Wait: runner.CommandWait, Record: record, Round: round, Head: strings.TrimSpace(after.Head), Result: after.Tree,
		Diff: diff, DiffDigest: digestHex(diff), Legacy: after.Tree != "" && !strings.Contains(after.Tree, "\x00")}
	review.AdmitChild = func(id string) error {
		if runner.tree == nil {
			return fmt.Errorf("the publication check has no worktree owner")
		}
		runner.tree.owner.Children = append(runner.tree.owner.Children, id)
		return writeUnitJSON(runner.tree.path, *runner.tree.owner, runner.root())
	}
	plan, err := readUnitPlan(filepath.Join(round.Directory, "plan.json"), record.PlanDirectory)
	if errors.Is(err, os.ErrNotExist) {
		plan, err = readUnitPlan(record.Plan, record.PlanDirectory)
	}
	if err != nil {
		return coded("UNIT_REVIEW_NOT_READY", "run="+id, fmt.Errorf("the plan of run %s cannot be read: %v", id, err))
	}
	review.BuildBrief, review.Base = plan.Build.Brief, plan.Base
	review.Whole = plan.Whole
	review.BuildBriefSHA256 = round.BuildBriefSHA256
	if round.FollowUp != "" && plan.Check == nil {
		// A corrected attempt was built from its correction brief alone, so
		// it is reviewed against that brief.
		review.BuildBrief = round.FollowUp
	}
	if frozen, frozenErr := readUnitPlan(filepath.Join(round.Directory, "plan.json"), record.PlanDirectory); frozenErr == nil && frozen.Check != nil {
		// The declared-check instructions are part of the brief the builder
		// received; review uses the same complete brief and its digest.
		review.BuildBrief = frozen.Build.Brief
	} else if frozenErr != nil && !errors.Is(frozenErr, os.ErrNotExist) {
		return coded("UNIT_REVIEW_NOT_READY", "run="+id, fmt.Errorf("the frozen plan of attempt %d cannot be read: %v", round.Number, frozenErr))
	}
	for index := range record.Subjects {
		subject := record.Subjects[index]
		if subject.Round == round.Number {
			review.Subject = &subject
		} else if subject.Round < round.Number && subject.Commit != "" && (review.Prior == nil || subject.Round > review.Prior.Round) {
			review.Prior = &subject
		}
	}
	if review.Subject != nil && review.Subject.DiffDigest != review.DiffDigest {
		return coded("UNIT_RESULT_CHANGED", fmt.Sprintf("run=%s round=%d", id, round.Number), fmt.Errorf("the kept result of attempt %d changed since it was recorded for review", round.Number))
	}
	retain := func(subject UnitSubject) error {
		if subject.Round != round.Number {
			return fmt.Errorf("a subject binds only the latest completed round %d", round.Number)
		}
		replaced := false
		sameExamination := false
		for index := range record.Subjects {
			if record.Subjects[index].Round == subject.Round {
				previous := record.Subjects[index]
				sameExamination = previous.Examination == subject.Examination && previous.ExaminationRound == subject.ExaminationRound &&
					previous.ExaminationReturnPath == subject.ExaminationReturnPath && previous.ExaminationJob == subject.ExaminationJob
				record.Subjects[index], replaced = subject, true
			}
		}
		if !replaced {
			record.Subjects = append(record.Subjects, subject)
		}
		if subject.UnknownRetries > record.Rounds[len(record.Rounds)-1].UnknownRetries {
			record.Rounds[len(record.Rounds)-1].UnknownRetries = subject.UnknownRetries
		}
		if len(subject.TransferredTo) > 0 {
			record.Rounds[len(record.Rounds)-1].Transferred = true
		}
		if subject.Examination != "" {
			latest := &record.Rounds[len(record.Rounds)-1]
			var err error
			if sameExamination && len(latest.Reads) > 0 && latest.Stop != nil &&
				(latest.Stop.Handoff == "stopped unreadable-policy" || latest.Stop.Handoff == "stopped unreadable-inherited-findings") {
				// The examination is retained; only its decision inputs need another read.
				err = runner.decideRound(&record, latest, "")
			} else {
				err = runner.CollectExamination(&record, latest, subject)
			}
			if err != nil {
				return err
			}
			material, err := runner.roundMaterial(record, record.Rounds[len(record.Rounds)-1])
			if err != nil {
				record.Notes = append(record.Notes, fmt.Sprintf("attempt %d: material count is unknown: %v", round.Number, err))
			}
			record.Rounds[len(record.Rounds)-1].Material = material
		}
		if err := runner.save(record); err != nil {
			return err
		}
		if subject.Drop != nil {
			runner.publishJudgement(record, round.Number)
		}
		return nil
	}
	return bind(review, retain)
}

// WorktreeResult is the worktree's current HEAD and raw result diff,
// computed as the proof-after snapshot computed them, without touching the
// caller's index.
func (runner *UnitRunner) WorktreeResult(worktree string) (head, result string, err error) {
	snapshot, err := runner.snapshotRepository(worktree)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(snapshot.Head), snapshot.Tree, nil
}

// UnitResultDigest names a retained raw result diff.
func UnitResultDigest(result string) string { return digestHex([]byte(result)) }

// UnitResultPaths are the exact paths a NUL-delimited raw result diff
// changes, in its order; a rename or copy names both of its paths.
func UnitResultPaths(result string) ([]string, error) {
	fields := strings.Split(result, "\x00")
	var paths []string
	for index := 0; index < len(fields); {
		header := fields[index]
		if header == "" && index == len(fields)-1 {
			break
		}
		if !strings.HasPrefix(header, ":") {
			return nil, fmt.Errorf("malformed raw result entry %q", header)
		}
		parts := strings.Split(header, " ")
		count := 1
		if len(parts) == 5 && parts[4] != "" && (parts[4][0] == 'R' || parts[4][0] == 'C') {
			count = 2
		}
		if index+count >= len(fields) {
			return nil, fmt.Errorf("truncated raw result entry %q", header)
		}
		for _, path := range fields[index+1 : index+1+count] {
			if path == "" {
				return nil, fmt.Errorf("raw result entry %q names an empty path", header)
			}
			paths = append(paths, path)
		}
		index += 1 + count
	}
	return paths, nil
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}
