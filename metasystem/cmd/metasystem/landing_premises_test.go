package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingProveBudgetTerminalRemedy(t *testing.T) {
	t.Parallel()
	for _, actor := range []string{"unenrolled tty", "enrolled tty", "agent tty"} {
		t.Run(actor, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			b.owners.processes.question = channel.ReadQuestion
			a := b.seat(t, "a")
			b.success(t, "landing", "run")
			b.assemble(t, a)
			selected := b.batch(t)
			head := b.git(t, b.checkout, "rev-parse", "HEAD")
			tree := b.git(t, b.checkout, "rev-parse", "HEAD^{tree}")
			var proofs []plain.Result
			for _, id := range []string{"first", "second"} {
				proofs = append(proofs, plain.Result{Commit: head, Tree: tree, BatchBase: selected.Base, Result: plain.Red, Attempt: id, CountedFull: true, Scope: "full", Repeat: "allowed", Goals: selected.Members, BatchID: selected.ID, BatchMembers: selected.Members})
			}
			writeCauseProof(t, b.installation, "results.jsonl", proofs...)
			b.owners.helm.stdinTerminal = func() bool { return true }
			switch actor {
			case "unenrolled tty":
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil)
				}
			case "agent tty":
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeAgent, errors.New("agent"))
				}
			case "enrolled tty":
				b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			}
			before := b.executions(t)
			code, text := b.run(t, "landing", "prove", "--wait")
			switch actor {
			case "unenrolled tty":
				if code == 0 || !strings.Contains(oneSpaced(text), "this terminal is not enrolled for the landing install; type: metasystem system enroll --name") || strings.Contains(text, "ask a person to run") || b.executions(t) != before {
					t.Fatalf("terminal refusal: %d %s", code, text)
				}
			case "agent tty":
				if code == 0 || !strings.Contains(oneSpaced(text), "two full checks went red on this base") || !strings.Contains(oneSpaced(text), "metasystem landing prove") || b.executions(t) != before {
					t.Fatalf("agent hold: %d %s", code, text)
				}
				proofQuestion(t, b, "metasystem landing prove")
			case "enrolled tty":
				if code != 0 || b.executions(t) != before+1 {
					t.Fatalf("person check: %d %s", code, text)
				}
				last, ok, err := plain.LastResult(b.installation)
				if err != nil || !ok || last.BatchBase != selected.Base || len(last.Executions) == 0 || !last.Executions[0].Override || !strings.Contains(last.Executions[0].Warning, "exceeds the automatic full-check allowance") {
					t.Fatalf("override warning/base: %+v err=%v", last, err)
				}
				if strings.Contains(last.Executions[0].Warning, (&plain.ProofBudget{}).Error()) || strings.Contains(text, "goals hold until a person decides") {
					t.Fatalf("the person's override quoted the automatic hold: %s %s", last.Executions[0].Warning, text)
				}
			}
		})
	}
}

func TestLandingStatusShowsTerminalEnrollment(t *testing.T) {
	t.Parallel()
	for _, actor := range []string{"enrolled", "not enrolled", "agent", "helm"} {
		t.Run(actor, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			b.owners.helm.stdinTerminal = func() bool { return true }
			want := "terminal: enrolled for Wido"
			switch actor {
			case "not enrolled":
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil)
				}
				want = "terminal: not enrolled; type: metasystem system enroll --name Wido"
			case "agent":
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{Outcome: humanauthority.OutcomeAgent}, humanauthority.AgentRefused("codex")
				}
				want = ""
			case "helm":
				b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
					return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido"}, now)
				}
				want = "terminal: enrolled for Wido (helm active)"
			}
			code, text := b.run(t, b.root, "status")
			if code != 0 || want != "" && !strings.Contains(oneSpaced(text), want) || want == "" && strings.Contains(text, "terminal:") {
				t.Fatalf("status: %d %s want %q", code, text, want)
			}
			if actor == "helm" && strings.Contains(text, "system enroll") {
				t.Fatalf("the enrolled helm holder got an enroll hint: %s", text)
			}
		})
	}
}

func TestLandingStatusTerminalObservationPreservesRemedy(t *testing.T) {
	t.Parallel()
	for _, actor := range []string{"not enrolled", "agent"} {
		t.Run(actor, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			b.owners.helm.stdinTerminal = func() bool { return true }
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				outcome := humanauthority.OutcomeTerminalMissing
				if actor == "agent" {
					outcome = humanauthority.OutcomeAgent
				}
				return humanauthority.Proof{Outcome: outcome}, humanauthority.Refused(outcome, nil)
			}
			b.owners.landing.status = func(_ string, _ lane.Record, view lane.View) landingStatusData {
				view.Owner.Fix = []string{"metasystem", "landing", "restart"}
				return plain.Status{View: view}
			}
			code, text := b.run(t, b.root, "status")
			if code != 0 || !strings.Contains(oneSpaced(text), "metasystem landing resume") || strings.Contains(text, "metasystem landing restart") {
				t.Fatalf("terminal observation changed the remedy: %d %s", code, text)
			}
		})
	}
}

func TestLandingProveBudgetFreshAfterMovedMainRecovery(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"resume", "restart"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			a := b.seat(t, "a")
			b.success(t, "landing", "run")
			b.assemble(t, a)
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.success(t, "landing", "batch", "seal")
			selected := b.batch(t)
			oldHead := b.git(t, b.checkout, "rev-parse", "HEAD")
			oldTree := b.git(t, b.checkout, "rev-parse", "HEAD^{tree}")
			var proofs []plain.Result
			for _, id := range []string{"a1", "a2"} {
				proofs = append(proofs, plain.Result{Commit: oldHead, Tree: oldTree, BatchBase: selected.Base, Result: plain.Red, Attempt: id, CountedFull: true, Scope: "full", Repeat: "allowed", Goals: selected.Members, BatchID: selected.ID, BatchMembers: selected.Members})
			}
			writeCauseProof(t, b.installation, "results.jsonl", proofs...)
			writer := filepath.Join(t.TempDir(), "main-writer")
			b.git(t, filepath.Dir(writer), "clone", "--quiet", b.origin, writer)
			helmMust(t, os.WriteFile(filepath.Join(writer, "new-main.txt"), []byte("new main\n"), 0600))
			b.git(t, writer, "add", "new-main.txt")
			b.git(t, writer, "commit", "--quiet", "-m", "advance main fixture")
			b.git(t, writer, "push", "--quiet", "origin", "HEAD:main")
			newMain := b.git(t, writer, "rev-parse", "HEAD")
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.success(t, "landing", action)
			next := b.batch(t)
			if next.Base != newMain || next.Base == selected.Base {
				t.Fatalf("recovered base: %+v", next)
			}
			b.assemble(t, a)
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeAgent, nil)
			}
			b.success(t, "landing", "prove", "--wait")
			last, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || last.BatchBase != newMain || last.Person != nil || len(last.Executions) == 0 || last.Executions[0].Override {
				t.Fatalf("automatic new-base check: %+v err=%v", last, err)
			}
		})
	}
}

func TestLandingStatusDoesNotTreatNullAsTerminal(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	file, err := os.Open(os.DevNull)
	helmMust(t, err)
	defer file.Close()
	b.owners.helm.stdin = file
	code, text := b.run(t, b.root, "status")
	if code != 0 || strings.Contains(text, "terminal:") {
		t.Fatalf("non-terminal status: %d %s", code, text)
	}
}
