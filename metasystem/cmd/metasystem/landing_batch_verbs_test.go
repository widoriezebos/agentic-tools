package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// This saved batch is one real lane record. Every destination and queue read
// belongs to the fixture; the saved install is never opened.
func batchVerbFixture(t *testing.T, state string) (*selectionBed, plain.Batch) {
	t.Helper()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	var batch plain.Batch
	data, err := os.ReadFile("testdata/landing-batch.json")
	helmMust(t, err)
	helmMust(t, json.Unmarshal(data, &batch))
	batch.Lane, batch.State = b.record, plain.BatchPrepared
	batch.Selector.Checkout = b.lane
	if state != "open" {
		batch.SealedAt = b.now.Add(-time.Minute).Format(time.RFC3339Nano)
	}
	if state == "running" {
		batch.State = plain.BatchRunning
		writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{BatchID: batch.ID, BatchMembers: batch.Members, Commit: "main", Tree: "tree", Attempt: "proof", Pid: 42, Since: batch.SealedAt})
		b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "batch.json"), batch)
	for _, m := range append(slices.Clone(batch.Members), plain.GoalSHA{Goal: "shared-state-in-the-ledger", SHA: "5b72cf4a72634b7865f4045606852c0ae391bfd1"}) {
		_, _, err := plain.HandIn(b.lane, plain.Line{Goal: m.Goal, SHA: m.SHA, Branch: "goal/" + m.Goal, Seat: "m1e", At: "2026-10-11T20:12:17Z"})
		helmMust(t, err)
	}
	return b, batch
}

func TestLandingBatchVerbsRecordPersonAndClock(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"open", "sealed", "running"} {
		for _, verb := range []string{"show", "seal", "reopen", "restart"} {
			t.Run(state+"/"+verb, func(t *testing.T) {
				t.Parallel()
				b, original := batchVerbFixture(t, state)
				_, err := lane.SetPause(b.home, "Wido", b.now)
				helmMust(t, err)
				words := []string{"landing", "batch"}
				switch verb {
				case "seal":
					words = append(words, "seal", "--by", "Wido")
				case "reopen":
					words = append(words, "reopen", "--with", "shared-state-in-the-ledger", "--by", "Wido")
				case "restart":
					words = []string{"landing", "restart", "--by", "Wido"}
				}
				code, out := b.run(t, b.seat, words...)
				if code != 0 || !strings.Contains(out, original.ID) {
					t.Fatalf("exit %d: %s", code, out)
				}
				batch := b.batch(t)
				if verb == "show" {
					want := "open"
					if state != "open" {
						want = "sealed since"
					}
					for _, text := range []string{want, "lane-rehearsal-before-landing: not merged", "shared-state-in-the-ledger waits:"} {
						if !strings.Contains(out, text) {
							t.Fatalf("missing %q: %s", text, out)
						}
					}
					if len(batch.Actions) != 0 {
						t.Fatal("show mutated the batch")
					}
					return
				}
				if verb == "seal" && state != "open" {
					if batch.SealedAt != original.SealedAt || len(batch.Actions) != 0 || !strings.Contains(out, "already sealed") {
						t.Fatalf("repeated seal wrote a decision: %+v output: %s", batch, out)
					}
					return
				}
				if len(batch.Actions) != 1 {
					t.Fatalf("missing decision: %+v", batch)
				}
				act := batch.Actions[0]
				if act.Kind != verb || act.By != "Wido" || act.At != b.now.UTC().Format(time.RFC3339Nano) || act.Impact == "" || !strings.Contains(out, act.Impact) {
					t.Fatalf("decision: %+v output: %s", act, out)
				}
				if verb == "seal" {
					if batch.SealedAt != act.At || !slices.Equal(batch.Members, original.Members) {
						t.Fatalf("seal changed membership or lost time: %+v", batch)
					}
					seams := b.owners.landing.plainProve
					seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "person"}, nil }
					selected, err := plain.SelectBatch(b.lane, b.lane, b.record, seams)
					if err != nil || selected == nil || !slices.Equal(selected.Members, original.Members) {
						t.Fatalf("policy replaced the person's seal: %+v %v", selected, err)
					}
				} else if state == "running" {
					if !act.Pending || !strings.Contains(out, "recorded; nothing acts on it until the proof ends") || !strings.Contains(out, "run reopen again then") || !slices.Equal(batch.Members, original.Members) {
						t.Fatalf("running proof was changed: %+v", batch)
					}
					if _, present, alive, err := plain.ReadRunning(b.lane, b.owners.landing.plainProve); err != nil || !present || !alive {
						t.Fatalf("fallback lost proof: %v %v %v", present, alive, err)
					}
				} else if act.Pending || batch.State != plain.BatchPrepared || batch.SealedAt != "" || !slices.ContainsFunc(batch.Members, func(m plain.GoalSHA) bool { return m.Goal == "shared-state-in-the-ledger" }) {
					t.Fatalf("goal not admitted: %+v", batch)
				}
				if verb != "seal" && state != "running" {
					if _, err := plain.RecordedPersonBatch(b.lane, b.record, batch.ID); err != nil || plain.PersonBatchContinuation(b.lane, b.record, b.home) != batch.ID {
						t.Fatalf("person's reopened selection cannot continue: %v %+v", err, batch)
					}
				}
				if verb == "restart" && len(act.With) != 3 || verb == "reopen" && len(act.With) != 1 {
					t.Fatalf("wrong requested members: %+v", act.With)
				}
			})
		}
	}
}

func TestLandingBatchBadInputAndUnprovedCaller(t *testing.T) {
	t.Parallel()
	for _, words := range [][]string{{"landing", "batch", "wrong"}, {"landing", "batch", "reopen"}, {"landing", "batch", "seal", "--with", "a"}, {"landing", "batch", "reopen", "--with", "missing"}, {"landing", "batch", "seal", "--by", "SomeoneElse"}} {
		t.Run(strings.Join(words, "/"), func(t *testing.T) {
			t.Parallel()
			b, original := batchVerbFixture(t, "open")
			code, out := b.run(t, b.seat, words...)
			if code == 0 {
				t.Fatalf("bad input succeeded: %s", out)
			}
			batch := b.batch(t)
			if len(batch.Actions) != 0 || !slices.Equal(batch.Members, original.Members) {
				t.Fatal("bad input changed the batch")
			}
		})
	}
	t.Run("unproved caller", func(t *testing.T) {
		t.Parallel()
		b, original := batchVerbFixture(t, "open")
		b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{}, nil
		}
		code, out := b.run(t, b.seat, "landing", "batch", "seal")
		batch := b.batch(t)
		if code == 0 || len(batch.Actions) != 0 || !slices.Equal(batch.Members, original.Members) {
			t.Fatalf("unproved caller changed batch: exit %d %s", code, out)
		}
	})
}

func TestHelpLandingNamesBatchVerbs(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "open")
	var help strings.Builder
	writeIntentObjectHelp(&help, "landing")
	if !strings.Contains(help.String(), "batch") || !strings.Contains(help.String(), "restart") {
		t.Fatalf("missing verbs: %s", &help)
	}
	work := strings.SplitN(strings.SplitN(help.String(), "Work", 2)[1], "Shape", 2)[0]
	for _, verb := range []string{"batch seal", "batch reopen"} {
		if !strings.Contains(work, verb) {
			t.Fatalf("writing verb %s outside Work: %s", verb, &help)
		}
	}
	for _, words := range [][]string{{"landing", "batch", "--help"}, {"landing", "restart", "--help"}} {
		code, out := b.run(t, b.seat, words...)
		if code != 0 {
			t.Fatalf("help exit %d: %s", code, out)
		}
		for _, word := range words[1 : len(words)-1] {
			if !strings.Contains(out, word) {
				t.Fatalf("help missing %s: %s", word, out)
			}
		}
		if len(words) == 2 && (!strings.Contains(out, "batch") || !strings.Contains(out, "restart")) || len(words) == 3 && words[1] == "batch" && (!strings.Contains(out, "seal") || !strings.Contains(out, "reopen --with G")) {
			t.Fatalf("missing verbs: %s", out)
		}
	}
}

func TestLandingBatchStatusLaneSaysProving(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "running")
	code, out := b.run(t, b.seat, "landing", "status")
	if code != 0 || !strings.Contains(out, "Lane: proving; the agent waits for the keeper") || strings.Contains(out, "Lane: idle") {
		t.Fatalf("running lane: exit %d %s", code, out)
	}
}

// The gate admits a candidate containing the real merge parent before the
// seal is inspected; this exercises the command's Git admission adapter.
func TestLandingBatchGateLeavesSealForFullProofGitAdapter(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "1")
	a := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, a)
	b.success(t, "landing", "prove", "--gate", "--wait")
	batch := b.batch(t)
	if batch.State != plain.BatchRunning || batch.SealedAt != "" {
		t.Fatalf("gate stamped the full proof's seal: %+v", batch)
	}
}

func TestLandingBatchStatusJSONCarriesDecision(t *testing.T) {
	t.Parallel()
	b, original := batchVerbFixture(t, "running")
	if code, out := b.run(t, b.seat, "landing", "batch", "reopen", "--with", "b"); code != 0 {
		t.Fatalf("reopen %d: %s", code, out)
	}
	code, out := b.run(t, b.seat, "landing", "status", "--json")
	var status struct {
		Data struct {
			Batch *plain.Batch `json:"batch"`
		}
	}
	helmMust(t, json.Unmarshal([]byte(out), &status))
	batch := status.Data.Batch
	if code != 0 || batch == nil || batch.ID != original.ID || batch.State != plain.BatchRunning || batch.SealedAt == "" || len(batch.Actions) != 1 || !batch.Actions[0].Pending || batch.Actions[0].By != "Wido" {
		t.Fatalf("batch missing: exit %d %s", code, out)
	}
}

func TestLandingBatchReopenReplacesSupersededMember(t *testing.T) {
	t.Parallel()
	b, original := batchVerbFixture(t, "sealed")
	member := original.Members[0]
	_, _, err := plain.HandIn(b.lane, plain.Line{Goal: member.Goal, SHA: "new-hand-in", At: b.now.Format(time.RFC3339Nano)})
	helmMust(t, err)
	code, out := b.run(t, b.seat, "landing", "batch", "reopen", "--with", member.Goal)
	if code != 0 {
		t.Fatalf("reopen %d: %s", code, out)
	}
	batch := b.batch(t)
	if len(batch.Members) != 1 || batch.Members[0].SHA != "new-hand-in" || batch.Members[0].Goal != member.Goal {
		t.Fatalf("superseded member retained or duplicated: %+v", batch)
	}
}

func TestLandingBatchRepeatedVerbsWriteNothing(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"open", "sealed", "running", "ended"} {
		for _, verb := range []string{"seal", "reopen", "restart"} {
			t.Run(state+"/"+verb, func(t *testing.T) {
				t.Parallel()
				fixtureState := state
				if state == "ended" {
					fixtureState = "running"
				}
				b, original := batchVerbFixture(t, fixtureState)
				if state == "ended" {
					b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
				}
				words := []string{"landing", "batch", verb}
				want := "already sealed"
				if verb == "reopen" {
					words = append(words, "--with", original.Members[0].Goal)
					want = "already a member"
				} else if verb == "restart" {
					words = []string{"landing", "restart"}
					want = "request already pending"
					if state != "running" {
						want = "already a member"
					}
				}
				code, out := b.run(t, b.seat, words...)
				if code != 0 {
					t.Fatalf("first verb: exit %d %s", code, out)
				}
				path := filepath.Join(plain.Dir(b.lane), "batch.json")
				before, err := os.ReadFile(path)
				helmMust(t, err)
				beforeInfo, err := os.Stat(path)
				helmMust(t, err)
				b.now = b.now.Add(time.Minute)
				code, out = b.run(t, b.seat, words...)
				after, err := os.ReadFile(path)
				helmMust(t, err)
				afterInfo, err := os.Stat(path)
				helmMust(t, err)
				if code != 0 || !strings.Contains(out, want) || string(before) != string(after) || !os.SameFile(beforeInfo, afterInfo) {
					t.Fatalf("repeat rewrote batch: exit %d %s\nbefore %s\nafter %s", code, out, before, after)
				}
				if verb == "reopen" && b.batch(t).State != original.State {
					t.Fatal("existing member changed the batch state")
				}
			})
		}
	}
}

func TestLandingBatchPendingRequestBelongsToAttempt(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"reopen", "restart"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b, original := batchVerbFixture(t, "running")
			words := []string{"landing", "batch", "reopen", "--with", "b"}
			if verb == "restart" {
				words = []string{"landing", "restart"}
			}
			for _, attempt := range []string{"proof", "later-proof"} {
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{BatchID: original.ID, Attempt: attempt, Pid: 42, Since: b.now.Format(time.RFC3339Nano)})
				code, out := b.run(t, b.seat, words...)
				if code != 0 || !strings.Contains(out, "run reopen again then") {
					t.Fatalf("request: exit %d %s", code, out)
				}
				path := filepath.Join(plain.Dir(b.lane), "batch.json")
				before, err := os.ReadFile(path)
				helmMust(t, err)
				beforeInfo, err := os.Stat(path)
				helmMust(t, err)
				code, out = b.run(t, b.seat, words...)
				after, err := os.ReadFile(path)
				helmMust(t, err)
				afterInfo, err := os.Stat(path)
				helmMust(t, err)
				if code != 0 || !strings.Contains(out, "request already pending") || string(before) != string(after) || !os.SameFile(beforeInfo, afterInfo) {
					t.Fatalf("repeated pending request wrote again: exit %d %s", code, out)
				}
				code, out = b.run(t, b.seat, "landing", "status")
				want := verb + " requested by Wido at " + b.now.Local().Format("15:04") + " for attempt " + attempt + "; waits for the proof to end"
				if code != 0 || !strings.Contains(out, want) {
					t.Fatalf("pending status: exit %d %s; want %s", code, out, want)
				}
			}
			data, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "batch.json"))
			helmMust(t, err)
			var saved struct{ Actions []struct{ Attempt string } }
			helmMust(t, json.Unmarshal(data, &saved))
			if len(saved.Actions) != 2 || saved.Actions[0].Attempt != "proof" || saved.Actions[1].Attempt != "later-proof" {
				t.Fatalf("attempts not stored on their requests: %s", data)
			}
			b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
			if code, out := b.run(t, b.seat, words...); code != 0 || b.batch(t).State != plain.BatchPrepared || b.batch(t).Actions[2].Pending {
				t.Fatalf("ended attempt blocked retry: exit %d %s", code, out)
			}
		})
	}
}

func TestLandingBatchRestartWithoutNewMembers(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"sealed", "running", "ended"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			fixtureState := state
			if state == "ended" {
				fixtureState = "running"
			}
			b, original := batchVerbFixture(t, fixtureState)
			member := original.Members[0]
			writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "queue.jsonl"), plain.Line{Goal: member.Goal, SHA: member.SHA, At: b.now.Format(time.RFC3339Nano)})
			if state == "ended" {
				b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
			}
			code, out := b.run(t, b.seat, "landing", "restart")
			batch := b.batch(t)
			if code != 0 || len(batch.Actions) != 1 || !slices.Equal(batch.Members, original.Members) || batch.Actions[0].Pending != (state == "running") {
				t.Fatalf("restart with no newcomers: exit %d %s batch %+v", code, out, batch)
			}
			if state != "running" && (batch.State != plain.BatchPrepared || batch.SealedAt != "") {
				t.Fatalf("restart left the window closed: %+v", batch)
			}
			path := filepath.Join(plain.Dir(b.lane), "batch.json")
			before, err := os.ReadFile(path)
			helmMust(t, err)
			code, out = b.run(t, b.seat, "landing", "restart")
			after, err := os.ReadFile(path)
			helmMust(t, err)
			if code != 0 || string(before) != string(after) {
				t.Fatalf("repeated empty restart wrote again: exit %d %s", code, out)
			}
		})
	}
}

func TestLandingBatchReopenRefusalNamesGoalState(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"returned", "landed", "never handed in"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, _ := batchVerbFixture(t, "open")
			if state != "never handed in" {
				_, _, err := plain.HandIn(b.lane, plain.Line{Goal: "unavailable", SHA: "sha", At: b.now.Format(time.RFC3339Nano)})
				helmMust(t, err)
				if state == "returned" {
					_, _, err = plain.ReturnProven(b.lane, "unavailable", "own", "fixture", true, "Wido", b.now, plain.ProveSeams{Person: &plain.ActProvenance{Person: "Wido"}})
					helmMust(t, err)
				} else {
					git := b.owners.landing.plainProve.Git
					b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
						if args[0] == "merge-base" && args[2] == "sha" {
							return "", nil
						}
						return git(dir, args...)
					}
				}
			}
			code, out := b.run(t, b.seat, "landing", "batch", "reopen", "--with", "unavailable")
			if code == 0 || !strings.Contains(out, state) || len(b.batch(t).Actions) != 0 {
				t.Fatalf("refusal lost state or changed batch: exit %d %s", code, out)
			}
		})
	}
}

// Real Git containment and the rebuilt production CLI own this adapter proof.
func TestLandingBatchCLIReadGitAdapter(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	record, _, err := lane.Read(b.home)
	helmMust(t, err)
	batch := plain.Batch{ID: "fixture-batch", Lane: record, Base: b.main, CreatedAt: laneTestNow.Format(time.RFC3339Nano), State: plain.BatchPrepared, Members: []plain.GoalSHA{{Goal: "fixture-goal", SHA: b.main}}}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.installation), "batch.json"), batch)
	engine := os.Getenv("METASYSTEM_WAIT_BINARY")
	if engine == "" {
		t.Fatal("the package setup did not build the production CLI")
	}
	for _, args := range [][]string{{"landing", "batch"}, {"landing", "batch", "invalid"}} {
		command := exec.CommandContext(t.Context(), engine, args...)
		command.Dir = b.checkout
		command.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		out, err := command.CombinedOutput()
		if len(args) == 2 {
			if err != nil || !strings.Contains(string(out), "batch fixture-batch: open") || !strings.Contains(string(out), "fixture-goal: merged") {
				t.Fatalf("real CLI read: %v %s", err, out)
			}
		} else {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), "landing batch reopen --with G") {
				t.Fatalf("real CLI boundary: %v %s", err, out)
			}
		}
		t.Logf("production CLI %v: %v\n%s", args, err, out)
	}
	process, live, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	helmMust(t, err)
	if live != identity.Alive {
		t.Fatal("the fixture proof process is not alive")
	}
	ref, err := identity.EncodeRef(process.Ref())
	helmMust(t, err)
	batch.State = plain.BatchRunning
	batch.Actions = []plain.BatchAction{{Kind: "restart", By: "Wido", At: batch.CreatedAt, Pending: true, Attempt: "fixture-proof", Impact: "recorded; nothing acts on it until the proof ends — run reopen again then"}}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.installation), "batch.json"), batch)
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.installation), "running.json"), plain.Running{BatchID: batch.ID, Attempt: "fixture-proof", Commit: b.main, Tree: "fixture-tree", Pid: int64(os.Getpid()), Process: ref, Since: batch.CreatedAt})
	for _, args := range [][]string{{"landing", "status"}, {"landing", "batch"}} {
		command := exec.CommandContext(t.Context(), engine, args...)
		command.Dir, command.Env = b.checkout, fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		out, err := command.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "for attempt fixture-proof; waits for the proof to end") {
			t.Fatalf("real CLI pending request: %v %s", err, out)
		}
		if args[1] == "status" && (!strings.Contains(string(out), "Lane: proving; the agent waits for the keeper") || strings.Contains(string(out), "Lane: idle")) {
			t.Fatalf("real CLI falsely idle: %s", out)
		}
		if args[1] == "batch" && !strings.Contains(string(out), "sealed since") {
			t.Fatalf("real CLI omitted the running proof's seal time: %s", out)
		}
		t.Logf("production CLI %v: %v\n%s", args, err, out)
	}
}

func TestLandingBatchShowsOnlyWaitingQueueEntries(t *testing.T) {
	t.Parallel()
	b, batch := batchVerbFixture(t, "sealed")
	base := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if len(args) > 2 && args[0] == "merge-base" && args[1] == "--is-ancestor" && args[2] == "5b72cf4a72634b7865f4045606852c0ae391bfd1" {
			return "", nil
		}
		return base(dir, args...)
	}
	code, out := b.run(t, b.seat, "landing", "batch")
	if code != 0 || !strings.Contains(out, batch.ID) || strings.Contains(out, "shared-state-in-the-ledger waits:") {
		t.Fatalf("exit %d: %s", code, out)
	}
}
