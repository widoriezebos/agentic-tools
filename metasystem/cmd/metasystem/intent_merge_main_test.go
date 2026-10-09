package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestGoalProjectionFreshFetchUsesInstallationEndpoint(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	installation, err := stateroot.ParseInstallation(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	endpoint := owners.dependencies.endpoint
	reads := 0
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		reads++
		if root != installation.Path() {
			t.Fatalf("endpoint read from %q, want installation %q", root, installation)
		}
		return endpoint(root)
	}
	stateRoot := filepath.Join(bed.root(), "application")
	// The command clock reads the run's state root, as on main; the bed's
	// clock is keyed to the checkout, so check the root and delegate.
	clock := owners.commandNow
	owners.commandNow = func(root string) (time.Time, error) {
		if root != stateRoot {
			t.Fatalf("command clock read from %q, want state root %q", root, stateRoot)
		}
		return clock(bed.root())
	}
	inv := &intentInvocation{owners: owners, stateRoot: stateRoot,
		layout: stateroot.Layout{InstallationRoot: installation}}
	before := bed.repo.captures
	projection, _, problem := inv.projectionWithFetch(true)
	if problem != nil || projection.Tip != bed.repo.accepted || reads != 1 || bed.repo.captures != before+1 {
		t.Fatalf("fresh projection: %+v problem=%+v endpoint reads=%d captures=%d->%d", projection, problem, reads, before, bed.repo.captures)
	}
	if bed.publications() != 0 {
		t.Fatal("a fresh projection published goal changes")
	}
}

func TestWorkRebaseKeepsSubjectProofAndDropPublication(t *testing.T) {
	t.Parallel()
	bed, owners, _ := rebaseIntentBed(t)
	subject := branch.AttestationSubject{Commit: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)}
	checked := false
	owners.connection.subjectCheck = func(dir string, got branch.AttestationSubject) (branch.GateObservation, error) {
		if dir != bed.worktree || got != subject {
			t.Fatalf("proof checked a different subject: %q %+v", dir, got)
		}
		checked = true
		return branch.GateObservation{}, errors.New("the rebased subject failed its declared check")
	}
	owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
		if req.SubjectCheck == nil || req.RecordDrop == nil {
			t.Fatalf("rebase lost subject proof or drop publication: %+v", req)
		}
		_, err := req.SubjectCheck(req.Repo, subject)
		return branch.RebaseResult{}, err
	}
	code, result := bed.runJSON(owners, "work", "rebase", bed.id)
	if code != 1 || result.Outcome != intentRefused || !checked || !strings.Contains(result.Summary, "the rebased subject failed its declared check") {
		t.Fatalf("failed subject proof did not hold the rebase: exit %d %+v checked=%v", code, result, checked)
	}
	if bed.publications() != 0 {
		t.Fatal("a failed rebase proof changed the goal ledger")
	}
}
