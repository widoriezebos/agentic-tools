package main

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// U6b follow-up 3: the resident runner defers its breach-stop pass until it
// leads its own session and the classifier names the runner itself STEWARD
// (no recognized ancestor). The first-tick transient is driven through the
// facts seam, never a clock.
func TestStewardRunnerCustodianReadyOnlyOnceItsStandingIsItsOwn(t *testing.T) {
	t.Parallel()
	const runner = int64(4242)
	leader := false
	var answer lease.Classification
	var answerErr error
	classifications := 0
	ready := stewardRunnerCustodianReady("/fixture/repo", stewardCustodianFacts{
		pid:           runner,
		sessionLeader: func() bool { return leader },
		classify: func(root string, pid int64) (lease.Classification, error) {
			if root != "/fixture/repo" || pid != runner {
				t.Fatalf("classified %s pid %d, want the runner itself", root, pid)
			}
			classifications++
			return answer, answerErr
		},
	})
	if ready() || classifications != 0 {
		t.Fatalf("a runner that does not lead its session was ready (classifications=%d)", classifications)
	}
	leader = true
	for _, step := range []struct {
		name   string
		answer lease.Classification
		err    error
	}{
		{"the arming main is still the recognized ancestor", lease.Classification{Class: lease.ClassMain, MainId: "armer"}, nil},
		{"a runtime above the armer is the recognized ancestor", lease.Classification{Class: lease.ClassDelegate, Pid: 77}, nil},
		{"the steward-family armer claims the runner", lease.Classification{Class: lease.ClassSteward, Pid: 77}, nil},
		{"a terminal-bearing runner is a person", lease.Classification{Class: lease.ClassHuman}, nil},
		{"classification failed", lease.Classification{}, errors.New("announcements unreadable")},
	} {
		answer, answerErr = step.answer, step.err
		if ready() {
			t.Fatalf("%s: the runner was ready", step.name)
		}
	}
	answer, answerErr = lease.Classification{Class: lease.ClassSteward, Pid: runner}, nil
	if !ready() {
		t.Fatal("a session-leading runner the classifier names STEWARD itself was not ready")
	}
	before := classifications
	answer = lease.Classification{Class: lease.ClassUntrusted}
	if !ready() || classifications != before {
		t.Fatalf("readiness did not latch: classifications %d -> %d", before, classifications)
	}
}
