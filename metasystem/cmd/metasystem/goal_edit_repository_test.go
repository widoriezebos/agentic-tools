package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type goalEditRepositoryFixture struct {
	*obligationCommandFixture
	now       time.Time
	seedID    string
	seedFiles map[string][]byte
}

func newGoalEditRepositoryFixture(t *testing.T, now time.Time, prepare func(*goal.GoalFile)) *goalEditRepositoryFixture {
	t.Helper()
	base := newObligationCommandFixture(t)
	writeFixtureEnrollment(t, base.root(), "Wido")
	file, _ := base.acceptedGoal()
	prepare(file)
	goalPath := "plans/goals/standing-validation.md"
	goalBytes := goal.RenderFile(file)
	if err := os.WriteFile(filepath.Join(base.root(), filepath.FromSlash(goalPath)), goalBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	initial := base.repo.commit(base.repo.accepted)
	files := obligationFilesCopy(initial.files)
	files[goalPath] = bytes.Clone(goalBytes)
	seedID := base.repo.accepted
	base.repo = &obligationRepository{
		t: t, commits: map[string]obligationCommit{
			seedID: {files: obligationFilesCopy(files), at: initial.at},
		}, canonical: seedID, accepted: seedID, serial: 1,
	}
	return &goalEditRepositoryFixture{obligationCommandFixture: base, now: now, seedID: seedID, seedFiles: files}
}

func (f *goalEditRepositoryFixture) commandNow(root string) (time.Time, error) {
	if _, err := f.facts.commandNow(root); err != nil {
		return time.Time{}, err
	}
	return f.now, nil
}

func (f *goalEditRepositoryFixture) run(args []string) (string, string, int) {
	f.t.Helper()
	var stdout string
	stderr, code := captureStderr(f.t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(f.t, func() int {
			return runGoalEditWithDependencies(args, f.commandNow, f.dependencies())
		})
		return innerCode
	})
	return stdout, stderr, code
}

func (f *goalEditRepositoryFixture) acceptedText() string {
	f.t.Helper()
	_, data := f.acceptedGoal()
	return string(data)
}

func (f *goalEditRepositoryFixture) expectTransactions(publications, rejectedMutations int) {
	f.t.Helper()
	f.obligationCommandFixture.expectTransactions(publications, rejectedMutations)
	seed := f.repo.commit(f.seedID)
	if len(seed.files) != len(f.seedFiles) {
		f.t.Fatal("seeded repository files changed")
	}
	for path, initial := range f.seedFiles {
		if !bytes.Equal(seed.files[path], initial) {
			f.t.Fatalf("seeded repository file %s changed", path)
		}
	}
}

func (f *goalEditRepositoryFixture) expectFacts(requests, authorityClocks int) {
	f.t.Helper()
	sequence := make([]string, 0, requests*6+authorityClocks)
	for range requests {
		sequence = append(sequence, "repository top", "ledger identity", "guard", "endpoint", "machine", "clock")
	}
	for range authorityClocks {
		sequence = append(sequence, "clock")
	}
	f.facts.expect(requests, requests, 0, requests+authorityClocks, 0, 0, sequence...)
}
