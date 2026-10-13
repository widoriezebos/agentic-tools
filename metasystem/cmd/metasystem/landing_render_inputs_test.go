package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStatusRepeatedDuringProofWritesNothing(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.install), "running.json"), plain.Running{Attempt: "proof", Commit: "main", Tree: "tree", Trunk: true, Pid: 42, Since: b.now.Format(time.RFC3339Nano)})
	code, output := b.run(t, b.root, "status")
	if code != 0 || !strings.Contains(output, "proving main") {
		t.Fatalf("first status: %d %s", code, output)
	}
	paths := []string{"state.json", "events.jsonl", "running.json"}
	saved := map[string][]byte{}
	for _, name := range paths {
		data, err := os.ReadFile(filepath.Join(plain.Dir(b.install), name))
		helmMust(t, err)
		saved[name] = data
	}
	b.now = b.now.Add(5 * time.Minute)
	code, output = b.run(t, b.root, "status")
	if code != 0 || !strings.Contains(output, "5.0 min") {
		t.Fatalf("repeat status: %d %s", code, output)
	}
	for _, name := range paths {
		data, err := os.ReadFile(filepath.Join(plain.Dir(b.install), name))
		helmMust(t, err)
		if !bytes.Equal(data, saved[name]) {
			t.Errorf("status rewrote %s", name)
		}
	}
}

func TestLandingPersonGatesKeepEnrollmentReason(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"batch", "return", "resume", "deploy"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{Outcome: humanauthority.OutcomeNotEnrolled, Grade: humanauthority.GradeTerminal, TerminalRef: humanauthority.ProcessRef{PID: 42}}, humanauthority.Refused(humanauthority.OutcomeNotEnrolled, nil)
			}
			if action == "return" {
				_, _, err := plain.HandIn(b.install, plain.Line{Goal: "G", SHA: "sha", At: b.now.Format(time.RFC3339Nano)})
				helmMust(t, err)
			}
			args := []string{}
			switch action {
			case "batch":
				args = []string{"seal"}
			case "return":
				args = []string{"G", "--cause", "own"}
			}
			code, output := b.run(t, b.root, action, args...)
			if code != 1 || !strings.Contains(output, "this terminal is not enrolled for the landing install; type: metasystem system enroll --name") || strings.Contains(output, "ask a person to run: metasystem landing "+action) {
				t.Fatalf("enrollment refusal: %d %s", code, output)
			}
		})
	}
}
