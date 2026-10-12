package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// Real first-parent merges are the admission boundary: a gate can prove a
// prefix, but the full proof must contain every selected commit.
func TestLandingOpenBatchGateNewcomerAndSealGitAdapter(t *testing.T) {
	t.Parallel()
	for _, detached := range []bool{false, true} {
		name := "wait"
		if detached {
			name = "detached"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			a := b.seat(t, "a")
			b.success(t, "landing", "run")
			first := b.batch(t)
			b.assemble(t, a)
			b.success(t, "landing", "prove", "--gate", "--wait")
			gate := b.batch(t)
			if !gate.Open || gate.State != plain.BatchRunning || gate.SealedAt != "" {
				t.Fatalf("gate sealed the batch: %+v", gate)
			}
			b.now = b.now.Add(20 * time.Second)
			second := b.seat(t, "b")
			observed := b.batch(t)
			want := []plain.GoalSHA{{Goal: "a", SHA: a}, {Goal: "b", SHA: second}}
			if observed.ID != first.ID || !observed.Open || !reflect.DeepEqual(observed.Members, first.Members) {
				t.Fatalf("status changed membership: %+v", observed)
			}
			if code, text := b.run(t, "landing", "status"); code != 0 || b.batch(t).Open != true || strings.Contains(text, "sealed window") {
				t.Fatalf("open status: exit=%d %s", code, text)
			}
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.refused(t, "landing", "prove", "--wait")
			if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || !strings.Contains(text, "landing prove --gate --wait, then metasystem landing prove --wait") {
				t.Fatalf("newcomer refusal omitted the merge gate: exit=%d %s", code, text)
			}
			expanded := b.batch(t)
			if !expanded.Open || !reflect.DeepEqual(expanded.Members, want) {
				t.Fatalf("proof admission did not retain the newcomer in the open batch: %+v", expanded)
			}
			b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", second)
			b.success(t, "landing", "prove", "--gate", "--wait")
			if !b.batch(t).Open {
				t.Fatal("the newcomer's gate sealed the batch")
			}
			if detached {
				launches := b.launches
				b.success(t, "landing", "prove")
				if b.launches != launches+1 {
					t.Fatalf("full proof launches = %d, want 1", b.launches-launches)
				}
			} else {
				b.success(t, "landing", "prove", "--wait")
			}
			sealed := b.batch(t)
			if sealed.Open || sealed.State != plain.BatchRunning || sealed.SealedAt != b.now.Format(time.RFC3339Nano) {
				t.Fatalf("full proof did not persist the seal: %+v", sealed)
			}
			b.seat(t, "c")
			if late := b.batch(t); late.Open || !reflect.DeepEqual(late.Members, want) || late.SealedAt != sealed.SealedAt {
				t.Fatalf("sealed batch changed for a late arrival: %+v", late)
			}
			if code, text := b.run(t, "landing", "status"); code != 0 || b.batch(t).Open || b.batch(t).SealedAt != sealed.SealedAt || !strings.Contains(text, "c waits for the next batch") {
				t.Fatalf("sealed status: exit=%d %s", code, text)
			}
		})
	}
}

func TestSkillLandingAgentMergesOpenBatchNewcomerBeforeGate(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	sentence := "Before the landing merge gate and before the full proof, re-read `landing status --json` and merge any member that joined the open batch since the last read, then run its landing merge gate."
	if !strings.Contains(string(data), sentence) {
		t.Fatal("the merge loop must re-read status and merge newcomers before the gate and full proof")
	}
}

func TestLandingHeldFullProofKeepsCandidateMembershipGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"person", "budget"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := proofPermissionBed(t)
			b.success(t, "landing", "prove", "--gate", "--wait")
			selected := b.batch(t)
			codeWanted := "LANE_PROOF_PERSON"
			if mode == "budget" {
				head := b.git(t, b.checkout, "rev-parse", "HEAD")
				tree := b.git(t, b.checkout, "rev-parse", "HEAD^{tree}")
				var spent []plain.Result
				for _, id := range []string{"first", "second"} {
					spent = append(spent, plain.Result{Attempt: id, Commit: head, Tree: tree, Result: plain.Red, Repeat: "allowed", Scope: "full", CountedFull: true, Goals: selected.Members, BatchID: selected.ID, BatchMembers: selected.Members})
				}
				writeCauseProof(t, b.installation, "results.jsonl", spent...)
				b.configuration = strings.ReplaceAll(b.configuration, "landing.proof=person", "landing.proof=auto")
				b.policy(t, "auto")
				codeWanted = "LANE_PROOF_BUDGET"
			}
			before := b.executions(t)
			if code, text := b.run(t, "landing", "prove"); code == 0 || !strings.Contains(text, codeWanted) || b.executions(t) != before {
				t.Fatalf("full proof hold: exit=%d %s", code, text)
			}
			q := proofQuestion(t, b, "metasystem landing prove")
			b.seat(t, "late")
			// The keeper's next preparation must preserve the person's candidate.
			b.success(t, "landing", "run")
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.success(t, "landing", "prove", "--wait")
			result, found, err := plain.LastResult(b.installation)
			if err != nil || !found || result.Result != plain.Green || result.Person == nil || !reflect.DeepEqual(result.BatchMembers, selected.Members) || b.executions(t) != before+1 {
				t.Fatalf("person's candidate proof: %+v error=%v", result, err)
			}
			sealed := b.batch(t)
			if sealed.Open || sealed.SealedAt != b.now.Format(time.RFC3339Nano) || !reflect.DeepEqual(sealed.Members, selected.Members) {
				t.Fatalf("held membership changed: %+v", sealed)
			}
			if code, text := b.run(t, "landing", "status"); code != 0 || !strings.Contains(text, "late waits for the next batch") {
				t.Fatalf("held newcomer status: exit=%d %s", code, text)
			}
			current, err := channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("person's proof request remains open: %+v %v", current, err)
			}
		})
	}
}
