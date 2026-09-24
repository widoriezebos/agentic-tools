package dispatch

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// acceptedAbsentGoalReads lets setup reach the real accepted-tree projection
// without supplying any committed goal. No other repository fact is available.
func acceptedAbsentGoalReads(t *testing.T, root string, expected int) goalAdmissionReads {
	t.Helper()
	repository := &acceptedAbsentRepository{t: t}
	endpointCalls := 0
	t.Cleanup(func() {
		if endpointCalls != expected || repository.acceptedCalls != expected {
			t.Errorf("goal endpoint/Accepted calls = %d/%d, want %d/%d", endpointCalls, repository.acceptedCalls, expected, expected)
		}
	})
	return goalAdmissionReads{
		ResolveEndpoint: func(got string) (goal.Endpoint, error) {
			if got != root {
				t.Fatalf("goal endpoint root = %q, want %q", got, root)
			}
			endpointCalls++
			repository.endpointCalls = endpointCalls
			return goal.Endpoint{Root: root, Repository: repository}, nil
		},
		NewWorld: func(string) bool {
			t.Fatal("unexpected NewWorld read")
			return false
		},
		ResolveMachine: func(string) (string, error) {
			t.Fatal("unexpected ResolveMachine read")
			return "", nil
		},
		Receipt: receiptAdmissionReads{
			AcceptedLedgerTip: func(string) (string, bool, error) {
				t.Fatal("unexpected receipt accepted-tip read")
				return "", false, nil
			},
			TopLevel: func(string) (string, error) {
				t.Fatal("unexpected receipt top-level read")
				return "", nil
			},
			FileAt: func(string, string, string) ([]byte, bool, error) {
				t.Fatal("unexpected receipt file read")
				return nil, false, nil
			},
		},
	}
}

type acceptedAbsentRepository struct {
	t             *testing.T
	endpointCalls int
	acceptedCalls int
}

func (r *acceptedAbsentRepository) unexpected(name string) {
	r.t.Helper()
	r.t.Fatalf("unexpected goal repository %s", name)
}

func (r *acceptedAbsentRepository) Accepted() (string, bool, error) {
	r.acceptedCalls++
	if r.acceptedCalls > r.endpointCalls {
		r.unexpected("Accepted without endpoint")
	}
	return "", false, nil
}

func (r *acceptedAbsentRepository) Capture(string) (string, error) {
	r.unexpected("Capture")
	return "", nil
}
func (r *acceptedAbsentRepository) Files(string, ...string) (map[string][]byte, error) {
	r.unexpected("Files")
	return nil, nil
}
func (r *acceptedAbsentRepository) Build(string, string, []goal.Change, string) (string, error) {
	r.unexpected("Build")
	return "", nil
}
func (r *acceptedAbsentRepository) Publish(string, string) (goal.CASOutcome, error) {
	r.unexpected("Publish")
	return "", nil
}
func (r *acceptedAbsentRepository) AcceptedCAS(string, string) error {
	r.unexpected("AcceptedCAS")
	return nil
}
func (r *acceptedAbsentRepository) IsAncestor(string, string) (bool, error) {
	r.unexpected("IsAncestor")
	return false, nil
}
func (r *acceptedAbsentRepository) TrailerPresent(string, string) (bool, error) {
	r.unexpected("TrailerPresent")
	return false, nil
}
func (r *acceptedAbsentRepository) CommitWithTrailer(string, string, string) (string, error) {
	r.unexpected("CommitWithTrailer")
	return "", nil
}
func (r *acceptedAbsentRepository) CommitTime(string) (time.Time, error) {
	r.unexpected("CommitTime")
	return time.Time{}, nil
}
func (r *acceptedAbsentRepository) Release(string) error {
	r.unexpected("Release")
	return nil
}
