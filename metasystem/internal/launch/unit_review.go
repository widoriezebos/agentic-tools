package launch

import (
	"crypto/sha256"
	"encoding/json"
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
type UnitSubject struct {
	Round     int    `json:"round"`
	Operation string `json:"operation"`
	// ExpectedParent is the HEAD the completed round observed after its
	// proof: the tip the unit commit is made on (or amended from).
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
	Examination      string `json:"examination,omitempty"`
	ExaminationRound int64  `json:"examinationRound,omitempty"`
}

// UnitReview is a completed round as a committed review consumes it.
type UnitReview struct {
	Record UnitRunRecord
	Round  UnitRound
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
	Legacy     bool
	BuildBrief string
	Subject    *UnitSubject
	Prior      *UnitSubject
}

// UnitReviewReadyOutcomes are the round outcomes whose proof passed and
// whose result the proof left unchanged. The preliminary read's outcome is
// feedback; it neither admits nor refuses a committed review.
var UnitReviewReadyOutcomes = map[string]bool{"green": true, "read-failed": true, "read-compacted": true}

// ReviewSubject calls bind under the run's lock with its latest completed
// round. retain records that round's subject in the run before the caller's
// next external effect; it is the run's only subject writer.
func (runner *UnitRunner) ReviewSubject(id string, bind func(review UnitReview, retain func(UnitSubject) error) error) error {
	if runner.Manager == nil && runner.Root == "" {
		return fmt.Errorf("unit run store is unavailable")
	}
	if _, err := runner.read(id); err != nil {
		return fmt.Errorf("UNIT_RUN_UNKNOWN run=%s: %v", id, err)
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
		return fmt.Errorf("UNIT_REVIEW_NOT_READY run=%s state=%s: the round is still running", id, record.State)
	}
	round := record.Rounds[len(record.Rounds)-1]
	if !UnitReviewReadyOutcomes[round.Outcome] {
		return fmt.Errorf("UNIT_REVIEW_NOT_READY run=%s round=%d outcome=%s: only a round whose proof passed and left the result unchanged is reviewed", id, round.Number, round.Outcome)
	}
	var after repositorySnapshot
	data, err := os.ReadFile(filepath.Join(round.Directory, "proof-after.json"))
	if err == nil {
		err = json.Unmarshal(data, &after)
	}
	if err != nil {
		return fmt.Errorf("UNIT_REVIEW_NOT_READY run=%s round=%d: no retained result snapshot: %v", id, round.Number, err)
	}
	diff, err := os.ReadFile(filepath.Join(round.Directory, "worktree.diff"))
	if err != nil {
		return fmt.Errorf("UNIT_REVIEW_NOT_READY run=%s round=%d: no retained result diff: %v", id, round.Number, err)
	}
	review := UnitReview{Record: record, Round: round, Head: strings.TrimSpace(after.Head), Result: after.Tree,
		Diff: diff, DiffDigest: digestHex(diff), Legacy: after.Tree != "" && !strings.Contains(after.Tree, "\x00")}
	plan, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return fmt.Errorf("UNIT_REVIEW_NOT_READY run=%s: its plan is unreadable: %v", id, err)
	}
	review.BuildBrief, review.Base = plan.Build.Brief, plan.Base
	for index := range record.Subjects {
		subject := record.Subjects[index]
		if subject.Round == round.Number {
			review.Subject = &subject
		} else if subject.Round < round.Number && subject.Commit != "" && (review.Prior == nil || subject.Round > review.Prior.Round) {
			review.Prior = &subject
		}
	}
	if review.Subject != nil && review.Subject.DiffDigest != review.DiffDigest {
		return fmt.Errorf("UNIT_RESULT_CHANGED run=%s round=%d: the retained result no longer matches its bound subject", id, round.Number)
	}
	retain := func(subject UnitSubject) error {
		if subject.Round != round.Number {
			return fmt.Errorf("a subject binds only the latest completed round %d", round.Number)
		}
		replaced := false
		for index := range record.Subjects {
			if record.Subjects[index].Round == subject.Round {
				record.Subjects[index], replaced = subject, true
			}
		}
		if !replaced {
			record.Subjects = append(record.Subjects, subject)
		}
		return runner.save(record)
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
