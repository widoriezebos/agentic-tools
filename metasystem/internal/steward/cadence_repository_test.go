package steward

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	cadenceParentCommit = "1111111111111111111111111111111111111111"
	cadenceNextCommit   = "2222222222222222222222222222222222222222"
	cadenceTestULID     = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
)

var cadenceCommitTime = time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)

// The fixture owns only the two committed snapshots used by these cadence tests.
// Every read returns copied bytes so a mutation must pass through Build.
type cadenceRepository struct {
	t                   *testing.T
	seed, parent, built map[string][]byte
	canonical, accepted string
	opid                string
	captures            []string
	releases            []string
	fileReads           map[string]int
	acceptedReads       int
	builds, publishes   int
	acceptedMoves       int
	ancestryChecks      map[string]int
	trailerChecks       int
	commitTimeReads     int
}

func newCadenceRepository(t *testing.T, file *goal.GoalFile) *cadenceRepository {
	t.Helper()
	files := map[string][]byte{
		"plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{
			Identity: cadenceTestULID, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		}),
		"plans/goals/bounded.md": goal.RenderFile(file),
	}
	tree, problems := goal.ParseTreeFiles(files)
	validation := goal.ValidateTree(tree)
	if len(problems) != 0 || len(validation) != 0 || tree.Live["bounded"] == nil {
		t.Fatalf("initial cadence goal bytes are invalid: parse=%v validate=%v", problems, validation)
	}
	return &cadenceRepository{t: t, seed: copyCadenceFiles(files), parent: copyCadenceFiles(files), canonical: cadenceParentCommit,
		accepted: cadenceParentCommit, opid: goal.Opid(cadenceTestULID, file.Claimed.Machine, file.Claimed.Lineage),
		fileReads: map[string]int{}, ancestryChecks: map[string]int{}}
}

func copyCadenceFiles(files map[string][]byte) map[string][]byte {
	copy := make(map[string][]byte, len(files))
	for path, data := range files {
		copy[path] = append([]byte(nil), data...)
	}
	return copy
}

func (r *cadenceRepository) snapshot(commit string) map[string][]byte {
	r.t.Helper()
	switch commit {
	case cadenceParentCommit:
		return r.parent
	case cadenceNextCommit:
		if r.built != nil {
			return r.built
		}
	}
	r.t.Fatalf("undeclared cadence commit %q", commit)
	return nil
}

func (r *cadenceRepository) Capture(opid string) (string, error) {
	r.t.Helper()
	if len(r.captures) == 0 {
		if opid != r.opid || r.canonical != cadenceParentCommit {
			r.t.Fatalf("first capture: opid=%q canonical=%q", opid, r.canonical)
		}
	} else if len(r.captures) == 1 {
		if opid == "" || opid == r.opid || r.canonical != cadenceNextCommit {
			r.t.Fatalf("confirming capture: opid=%q canonical=%q", opid, r.canonical)
		}
	} else {
		r.t.Fatalf("unexpected third cadence capture: %q", opid)
	}
	r.captures = append(r.captures, opid)
	return r.canonical, nil
}

func (r *cadenceRepository) Accepted() (string, bool, error) {
	r.acceptedReads++
	if r.acceptedReads > 4 {
		r.t.Fatalf("unexpected accepted-ref read %d", r.acceptedReads)
	}
	return r.accepted, true, nil
}

func (r *cadenceRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	r.t.Helper()
	key := strings.Join(prefixes, "|")
	switch key {
	case "plans/goals/|records/goals/", "plans/goals/backlog.md", "plans/goals/done/", goal.ChannelPrefix:
	default:
		r.t.Fatalf("undeclared cadence file query at %s: %v", commit, prefixes)
	}
	r.fileReads[commit+":"+key]++
	files := r.snapshot(commit)
	out := map[string][]byte{}
	for path, data := range files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				out[path] = append([]byte(nil), data...)
				break
			}
		}
	}
	return out, nil
}

func (r *cadenceRepository) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	r.t.Helper()
	if r.builds != 0 || opid != r.opid || parent != cadenceParentCommit ||
		message != "goal defer-findings bounded" || len(changes) != 1 || changes[0].Path != "plans/goals/bounded.md" {
		r.t.Fatalf("unexpected cadence build: opid=%q parent=%q message=%q changes=%+v", opid, parent, message, changes)
	}
	r.builds++
	r.built = copyCadenceFiles(r.snapshot(parent))
	for _, change := range changes {
		if change.Delete {
			delete(r.built, change.Path)
		} else {
			r.built[change.Path] = append([]byte(nil), change.Content...)
		}
	}
	if bytes.Equal(r.parent["plans/goals/bounded.md"], r.built["plans/goals/bounded.md"]) {
		r.t.Fatal("cadence build did not change the goal bytes")
	}
	return cadenceNextCommit, nil
}

func (r *cadenceRepository) Publish(parent, commit string) (goal.CASOutcome, error) {
	r.t.Helper()
	if r.publishes != 0 || parent != cadenceParentCommit || commit != cadenceNextCommit ||
		r.canonical != parent || r.built == nil {
		r.t.Fatalf("unexpected cadence publication: parent=%q commit=%q canonical=%q", parent, commit, r.canonical)
	}
	r.publishes++
	if r.fileReads[cadenceNextCommit+":plans/goals/|records/goals/"] != 1 ||
		r.fileReads[cadenceNextCommit+":"+goal.ChannelPrefix] != 1 {
		r.t.Fatal("cadence publication preceded committed-tree validation")
	}
	r.canonical = commit
	return goal.CASLanded, nil
}

func (r *cadenceRepository) AcceptedCAS(old, next string) error {
	r.t.Helper()
	if r.acceptedMoves != 0 || old != cadenceParentCommit || next != cadenceNextCommit ||
		r.accepted != old || r.canonical != next {
		r.t.Fatalf("unexpected accepted CAS: old=%q next=%q accepted=%q canonical=%q", old, next, r.accepted, r.canonical)
	}
	r.acceptedMoves++
	r.accepted = next
	return nil
}

func (r *cadenceRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	r.t.Helper()
	key := ancestor + ":" + descendant
	switch key {
	case cadenceParentCommit + ":" + cadenceParentCommit,
		cadenceNextCommit + ":" + cadenceParentCommit,
		cadenceParentCommit + ":" + cadenceNextCommit:
	default:
		r.t.Fatalf("undeclared cadence ancestry query %s", key)
	}
	r.ancestryChecks[key]++
	if r.ancestryChecks[key] != 1 {
		r.t.Fatalf("repeated cadence ancestry query %s", key)
	}
	return ancestor == descendant || ancestor == cadenceParentCommit && descendant == cadenceNextCommit, nil
}

func (r *cadenceRepository) TrailerPresent(tip, opid string) (bool, error) {
	r.t.Helper()
	if r.trailerChecks != 0 || tip != cadenceNextCommit || opid != r.opid || r.publishes != 1 {
		r.t.Fatalf("unexpected cadence trailer check: tip=%q opid=%q", tip, opid)
	}
	r.trailerChecks++
	return true, nil
}

func (r *cadenceRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	r.t.Fatalf("unexpected cadence history lookup: %q %q %q", revision, key, value)
	return "", fmt.Errorf("unexpected cadence history lookup")
}

func (r *cadenceRepository) CommitTime(commit string) (time.Time, error) {
	r.t.Helper()
	if r.commitTimeReads == 0 && commit != cadenceParentCommit ||
		r.commitTimeReads == 1 && commit != cadenceNextCommit || r.commitTimeReads > 1 {
		r.t.Fatalf("unexpected cadence commit-time read: %q", commit)
	}
	r.commitTimeReads++
	return cadenceCommitTime, nil
}

func (r *cadenceRepository) Release(opid string) error {
	r.t.Helper()
	want := ""
	switch len(r.releases) {
	case 0:
		if len(r.captures) == 2 {
			want = r.captures[1]
		}
	case 1:
		want = r.opid
	}
	if want == "" || opid != want {
		r.t.Fatalf("unexpected cadence release: opid=%q want=%q", opid, want)
	}
	r.releases = append(r.releases, opid)
	return nil
}

func (r *cadenceRepository) acceptedFiles() map[string][]byte {
	if r.accepted != cadenceNextCommit {
		r.t.Fatalf("cadence accepted ref stayed at %q", r.accepted)
	}
	return copyCadenceFiles(r.snapshot(r.accepted))
}

func (r *cadenceRepository) verify() {
	r.t.Helper()
	if r.canonical != cadenceNextCommit || r.accepted != cadenceNextCommit ||
		r.acceptedReads != 4 || r.builds != 1 || r.publishes != 1 || r.acceptedMoves != 1 ||
		r.trailerChecks != 1 || r.commitTimeReads != 2 || len(r.captures) != 2 || len(r.releases) != 2 {
		r.t.Fatalf("incomplete cadence transaction: %+v", r)
	}
	wantAncestry := map[string]int{
		cadenceParentCommit + ":" + cadenceParentCommit: 1,
		cadenceNextCommit + ":" + cadenceParentCommit:   1,
		cadenceParentCommit + ":" + cadenceNextCommit:   1,
	}
	if !reflect.DeepEqual(r.ancestryChecks, wantAncestry) {
		r.t.Fatalf("cadence ancestry checks: got=%v want=%v", r.ancestryChecks, wantAncestry)
	}
	wantReads := map[string]int{
		cadenceParentCommit + ":plans/goals/|records/goals/": 3,
		cadenceParentCommit + ":plans/goals/backlog.md":      5,
		cadenceParentCommit + ":plans/goals/done/":           3,
		cadenceParentCommit + ":" + goal.ChannelPrefix:       1,
		cadenceNextCommit + ":plans/goals/|records/goals/":   2,
		cadenceNextCommit + ":plans/goals/done/":             1,
		cadenceNextCommit + ":" + goal.ChannelPrefix:         1,
	}
	if !reflect.DeepEqual(r.fileReads, wantReads) {
		r.t.Fatalf("cadence committed-file reads: got=%v want=%v", r.fileReads, wantReads)
	}
	if !reflect.DeepEqual(r.parent, r.seed) {
		r.t.Fatal("cadence parent snapshot was mutated")
	}
	if !bytes.Equal(r.parent["plans/goals/backlog.md"], r.built["plans/goals/backlog.md"]) ||
		len(r.parent) != 2 || len(r.built) != 2 {
		r.t.Fatal("cadence transaction changed the immutable root or file set")
	}
}
