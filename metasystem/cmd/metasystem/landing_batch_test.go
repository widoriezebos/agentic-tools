package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Actual ancestry is the contract here: only real Git can distinguish a
// selected parent from a selected parent that brings an unselected hand-in.
type batchVerbBed struct {
	*plainVerbBed
	trace                 string
	configuration         string
	launches, starts, ids int
	now                   time.Time
}

func newBatchVerbBed(t *testing.T, policy string) *batchVerbBed {
	t.Helper()
	b := &batchVerbBed{plainVerbBed: newPlainVerbBed(t), now: laneTestNow}
	b.cwd = b.checkout
	b.trace = filepath.Join(filepath.Dir(b.checkout), "executions")
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nprintf 'LANDING-CHECKED\\t0\\n'\n"
	if err := testexec.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	b.configuration = "metasystem.template=true\nproof.full=" + shellCommand([]string{script}) + "\nproof.cheap=" + shellCommand([]string{script}) + "\nproof.trunk-every=10h\n"
	b.policy(t, policy)
	b.git(t, b.checkout, "add", "metasystem/metasystem.conf")
	b.git(t, b.checkout, "commit", "--quiet", "-m", "declare batch policy and proof")
	b.git(t, b.checkout, "push", "--quiet", "origin", "main")
	b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
	b.owners.policies = config.PolicyReaders{
		Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: b.checkout}, nil },
		Helm:     func(string) helm.State { return helm.State{} },
	}
	b.owners.lookupEnv = func(key string) (string, bool) {
		if key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" {
			return b.registry, true
		}
		return "", false
	}
	b.owners.landing.plainProve = plain.ProveSeams{
		Now:        func() time.Time { return b.now },
		NewID:      func() string { b.ids++; return fmt.Sprintf("selection-or-proof-%d", b.ids) },
		Executable: func() (string, error) { return "/fixture/engine", nil },
		Launch:     func([]string, string, string) (int64, error) { b.launches++; return 424242, nil },
		Alive:      func(running plain.Running) bool { return running.Pid == int64(os.Getpid()) },
	}
	b.owners.landing.keeper = func(home, root string) lane.AgentKeeper {
		return lane.AgentKeeper{Home: home, Self: root, Now: func() time.Time { return b.now },
			Sources: lane.WakeSources{Reasons: func(string) ([]string, error) {
				return plain.WakeReasons(b.installation, root, time.Time{}, b.now, b.owners.landing.plainProve)
			}},
			Running: func() (string, bool, error) { return "", false, nil },
			Start:   func(string, lane.Wake) (string, error) { b.starts++; return fmt.Sprintf("agent-%d", b.starts), nil },
		}
	}
	return b
}

func (b *batchVerbBed) policy(t *testing.T, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(b.installation, "metasystem.conf"), []byte(b.configuration+"landing.batch="+value+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func (b *batchVerbBed) batch(t *testing.T) plain.Batch {
	t.Helper()
	data := b.status(t)
	raw, err := json.Marshal(data["batch"])
	if err != nil {
		t.Fatal(err)
	}
	var selected plain.Batch
	if err := json.Unmarshal(raw, &selected); err != nil || selected.ID == "" {
		t.Fatalf("status selection = %s, %v", raw, err)
	}
	return selected
}

func (b *batchVerbBed) success(t *testing.T, words ...string) {
	t.Helper()
	if code, text := b.run(t, words...); code != 0 {
		t.Fatalf("%v exit %d: %s", words, code, text)
	}
}

func (b *batchVerbBed) run(t *testing.T, words ...string) (int, string) {
	t.Helper()
	if !slices.Contains(words, "--json") {
		words = append(words, "--json")
	}
	return b.plainVerbBed.run(t, words...)
}

func (b *batchVerbBed) refused(t *testing.T, words ...string) {
	t.Helper()
	before := b.executions(t)
	launches := b.launches
	code, text := b.run(t, append(words, "--json")...)
	if code == 0 || !strings.Contains(text, plain.CodeBatch) {
		t.Fatalf("%v exit %d: %s; want batch refusal", words, code, text)
	}
	if got := b.executions(t); got != before || b.launches != launches {
		t.Fatalf("a refused batch executed: checks %d -> %d; launches %d -> %d", before, got, launches, b.launches)
	}
}

func (b *batchVerbBed) executions(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(b.trace)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.Fields(string(data)))
}

func (b *batchVerbBed) assemble(t *testing.T, shas ...string) {
	t.Helper()
	b.git(t, b.checkout, "fetch", "--quiet", "origin")
	b.git(t, b.checkout, "checkout", "--quiet", "--detach", "origin/main")
	for _, sha := range shas {
		b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", sha)
	}
}

func TestLandingBatchAdapterCapsGateProofPushAndRetry(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "2")
	a, second, third := b.seat(t, "a"), b.seat(t, "b"), b.seat(t, "c")
	b.success(t, "landing", "run")
	selected := b.batch(t)
	want := []plain.GoalSHA{{Goal: "a", SHA: a}, {Goal: "b", SHA: second}}
	if selected.State != plain.BatchPrepared || selected.Base != b.main || !reflect.DeepEqual(selected.Members, want) || selected.Selector.Value != "2" || selected.Selector.Source != "conf" {
		t.Fatalf("selection = %+v", selected)
	}
	b.assemble(t, a, second, third)
	b.refused(t, "landing", "prove", "--wait")
	b.refused(t, "landing", "prove", "--gate", "--wait")
	b.refused(t, "landing", "prove")
	b.assemble(t, a)
	b.refused(t, "landing", "prove", "--wait")
	b.success(t, "landing", "prove", "--gate", "--wait")
	if b.executions(t) != 2 {
		t.Fatalf("baseline and first merge gate executions = %d", b.executions(t))
	}
	b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", second)
	b.success(t, "landing", "prove", "--gate", "--wait")
	if code, text := b.run(t, "landing", "push"); code == 0 || !strings.Contains(text, plain.CodeUnproven) {
		t.Fatalf("gate green authorized push: %d %s", code, text)
	}
	arrived := false
	b.owners.landing.plainProve.Command = func(command *exec.Cmd) error {
		if !arrived && strings.Contains(strings.Join(command.Env, "\n"), "LANDING_PROOF_SCOPE=full") {
			arrived = true
			if _, recorded, alive, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve); err != nil || !recorded || !alive {
				t.Fatalf("arrival was not during proof: recorded %v alive %v %v", recorded, alive, err)
			}
			b.seat(t, "d")
		}
		return command.Run()
	}
	b.success(t, "landing", "prove", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	if err != nil || !ok || proof.BatchID != selected.ID || !reflect.DeepEqual(proof.BatchMembers, want) || proof.Result != plain.Green {
		t.Fatalf("full proof = %+v, %v", proof, err)
	}
	if !arrived {
		t.Fatal("no hand-in arrived during the full proof")
	}
	advance := filepath.Join(filepath.Dir(b.checkout), "main-writer")
	b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, advance)
	if err := os.WriteFile(filepath.Join(advance, "main-change"), []byte("main moved\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, advance, "add", "main-change")
	b.git(t, advance, "commit", "--quiet", "-m", "advance main")
	b.git(t, advance, "push", "--quiet", "origin", "main")
	if code, text := b.run(t, "landing", "push"); code == 0 || !strings.Contains(text, plain.CodeNotFastForward) {
		t.Fatalf("moved main: %d %s", code, text)
	}
	b.success(t, "landing", "run")
	retry := b.batch(t)
	if retry.ID != selected.ID || retry.Base != selected.Base || !reflect.DeepEqual(retry.Members, want) {
		t.Fatalf("retry selected new waiting work: %+v", retry)
	}
	history, err := plain.Results(b.installation)
	if err != nil || len(history) != 1 || history[0].LoopClosed || history[0].BatchID != selected.ID {
		t.Fatalf("retry reset the original proof history: %+v %v", history, err)
	}
	b.assemble(t, a, second)
	b.success(t, "landing", "prove", "--gate", "--wait")
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")
	closed := b.batch(t)
	if closed.ID != selected.ID || closed.State != plain.BatchClosed || closed.ClosureReason == "" {
		t.Fatalf("push did not close original selection: %+v", closed)
	}
	push, ok, err := plain.LastPush(b.installation)
	if err != nil || !ok || push.BatchID != selected.ID || !reflect.DeepEqual(push.BatchMembers, want) {
		t.Fatalf("push lost selection: %+v %v", push, err)
	}
	states := queueStates(b.status(t))
	if states["a"] != plain.StateLanded || states["b"] != plain.StateLanded || states["c"] != plain.StateWaiting || states["d"] != plain.StateWaiting {
		t.Fatalf("push landed wrong work: %v", states)
	}
}

func TestLandingBatchAdapterTransitiveAndSupersededMembers(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"transitive", "superseded", "push after supersession"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "2")
			a := b.seat(t, "a")
			third := b.seat(t, "c")
			second := b.seat(t, "b")
			if scenario == "transitive" {
				seat := filepath.Join(filepath.Dir(b.checkout), "seat-b")
				b.git(t, seat, "fetch", "--quiet", "origin")
				b.git(t, seat, "merge", "--quiet", "--no-ff", "--no-edit", third)
				b.git(t, seat, "push", "--quiet", "origin", "goal/b")
				second = b.git(t, seat, "rev-parse", "HEAD")
			}
			// Queue order is a,b,c even when b was built on c's content.
			if err := os.Remove(filepath.Join(plain.Dir(b.installation), "queue.jsonl")); err != nil {
				t.Fatal(err)
			}
			for _, member := range []plain.GoalSHA{{Goal: "a", SHA: a}, {Goal: "b", SHA: second}, {Goal: "c", SHA: third}} {
				if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: member.Goal, SHA: member.SHA, At: b.now.Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			}
			b.success(t, "landing", "run")
			b.assemble(t, a, second)
			if scenario == "push after supersession" {
				b.success(t, "landing", "prove", "--wait")
			}
			if scenario != "transitive" {
				seat := filepath.Join(filepath.Dir(b.checkout), "seat-a")
				if err := os.WriteFile(filepath.Join(seat, "a.txt"), []byte("a rebuilt\n"), 0644); err != nil {
					t.Fatal(err)
				}
				b.git(t, seat, "add", "a.txt")
				b.git(t, seat, "commit", "--quiet", "-m", "rebuild a")
				b.git(t, seat, "push", "--quiet", "origin", "goal/a")
				newSHA := b.git(t, seat, "rev-parse", "HEAD")
				b.git(t, b.checkout, "fetch", "--quiet", "origin")
				if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newSHA, At: b.now.Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "push after supersession" {
				b.refused(t, "landing", "push")
			} else {
				b.refused(t, "landing", "prove", "--wait")
				b.refused(t, "landing", "prove")
			}
			if got := b.git(t, b.checkout, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(got, b.main) {
				t.Fatalf("refused candidate published: %s", got)
			}
		})
	}
}

func TestLandingBatchAdapterDescendantRehandIn(t *testing.T) {
	t.Parallel()
	for _, state := range []string{plain.StateSuperseded, plain.StateReturned} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			oldSHA := b.seat(t, "a")
			b.success(t, "landing", "run")
			if state == plain.StateReturned {
				b.success(t, "landing", "return", "a", "--cause", "unclassified", "--by", "Wido", "--reason", "fix forward")
			}
			seat := filepath.Join(filepath.Dir(b.checkout), "seat-a")
			if err := os.WriteFile(filepath.Join(seat, "a.txt"), []byte("a fixed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			b.git(t, seat, "add", "a.txt")
			b.git(t, seat, "commit", "--quiet", "-m", "fix a")
			newSHA := b.git(t, seat, "rev-parse", "HEAD")
			if parent := b.git(t, seat, "rev-parse", "HEAD^"); parent != oldSHA {
				t.Fatalf("replacement parent = %s, want old hand-in %s", parent, oldSHA)
			}
			b.git(t, seat, "push", "--quiet", "origin", "goal/a")
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			if _, added, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newSHA, At: b.now.Format(time.RFC3339)}); err != nil || !added {
				t.Fatalf("re-hand-in = %v, %v", added, err)
			}
			entries, err := plain.Entries(b.installation)
			if err != nil || len(entries) != 2 || entries[0].SHA != oldSHA || entries[0].State != state || entries[1].SHA != newSHA || entries[1].State != plain.StateWaiting {
				t.Fatalf("re-hand-in queue = %+v, %v", entries, err)
			}
			b.success(t, "landing", "run")
			if selected := b.batch(t); !reflect.DeepEqual(selected.Members, []plain.GoalSHA{{Goal: "a", SHA: newSHA}}) {
				t.Fatalf("replacement selection = %+v", selected)
			}
			b.assemble(t, newSHA)
			b.success(t, "landing", "prove", "--gate", "--wait")
			b.success(t, "landing", "prove", "--wait")
			candidate := b.git(t, b.checkout, "rev-parse", "HEAD")
			b.success(t, "landing", "push")
			if remote := b.git(t, b.checkout, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(remote, candidate+"\t") {
				t.Fatalf("replacement was not published: %s", remote)
			}
			if states := queueStates(b.status(t)); states["a"] != plain.StateLanded {
				t.Fatalf("replacement did not land: %v", states)
			}
		})
	}
}

func TestLandingBatchAdapterRefusesOlderContentOfUnselectedGoal(t *testing.T) {
	t.Parallel()
	for _, state := range []string{plain.StateReturned, plain.StateSuperseded} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "1")
			b1 := b.seat(t, "b")
			if state == plain.StateReturned {
				b.success(t, "landing", "return", "b", "--cause", "unclassified", "--by", "Wido", "--reason", "fix forward")
			}

			// Goal a hands in only after its branch has merged b's old content.
			seatA := filepath.Join(filepath.Dir(b.checkout), "seat-a")
			b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, seatA)
			b.git(t, seatA, "checkout", "--quiet", "-b", "goal/a")
			if err := os.WriteFile(filepath.Join(seatA, "a.txt"), []byte("a\n"), 0644); err != nil {
				t.Fatal(err)
			}
			b.git(t, seatA, "add", "a.txt")
			b.git(t, seatA, "commit", "--quiet", "-m", "a")
			b.git(t, seatA, "merge", "--quiet", "--no-ff", "--no-edit", b1)
			b.git(t, seatA, "push", "--quiet", "origin", "goal/a")
			a := b.git(t, seatA, "rev-parse", "HEAD")
			if _, added, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: a, At: b.now.Format(time.RFC3339)}); err != nil || !added {
				t.Fatalf("hand in a: %v %v", added, err)
			}

			seatB := filepath.Join(filepath.Dir(b.checkout), "seat-b")
			if err := os.WriteFile(filepath.Join(seatB, "b.txt"), []byte("b fixed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			b.git(t, seatB, "add", "b.txt")
			b.git(t, seatB, "commit", "--quiet", "-m", "fix b")
			b.git(t, seatB, "push", "--quiet", "origin", "goal/b")
			b2 := b.git(t, seatB, "rev-parse", "HEAD")
			if _, added, err := plain.HandIn(b.installation, plain.Line{Goal: "b", SHA: b2, At: b.now.Format(time.RFC3339)}); err != nil || !added {
				t.Fatalf("re-hand-in b: %v %v", added, err)
			}
			entries, err := plain.Entries(b.installation)
			if err != nil || len(entries) != 3 || entries[0].SHA != b1 || entries[0].State != state || entries[1].Goal != "a" || entries[1].SHA != a || entries[2].SHA != b2 || entries[2].State != plain.StateWaiting {
				t.Fatalf("queue = %+v, %v", entries, err)
			}

			b.success(t, "landing", "run")
			if selected := b.batch(t); !reflect.DeepEqual(selected.Members, []plain.GoalSHA{{Goal: "a", SHA: a}}) {
				t.Fatalf("selection = %+v; want only a", selected)
			}
			b.assemble(t, a)
			if got := b.git(t, b.checkout, "rev-list", "--first-parent", "--parents", b.main+"..HEAD"); got != b.git(t, b.checkout, "rev-parse", "HEAD")+" "+b.main+" "+a {
				t.Fatalf("candidate is not main plus one merge of a: %s", got)
			}
			if got := b.git(t, b.checkout, "rev-list", "HEAD.."+b1); got != "" {
				t.Fatalf("candidate omits b1: %s", got)
			}
			if got := b.git(t, b.checkout, "rev-list", "HEAD.."+b2); got == "" {
				t.Fatal("candidate contains b2")
			}
			b.refused(t, "landing", "prove", "--wait")
			pushes := 0
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "push" {
					pushes++
				}
				return plain.Git(dir, args...)
			}
			if code, text := b.run(t, "landing", "push"); code == 0 || pushes != 0 {
				t.Fatalf("unselected old content published: %d %s; pushes %d", code, text, pushes)
			}
			if got := b.git(t, b.checkout, "ls-remote", "origin", "refs/heads/main"); got != b.main+"\trefs/heads/main" {
				t.Fatalf("refused candidate moved main: %s", got)
			}
		})
	}
}

func TestLandingBatchAdapterPolicyAdmissionBoundaries(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"cap changes", "1844674407370955161600000000000000000000000", "person", "unknown"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "2")
			a, second, third := b.seat(t, "a"), b.seat(t, "b"), b.seat(t, "c")
			if policy == "unknown" {
				b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
					return config.PolicyRegistry{}, fmt.Errorf("policy registry unreadable")
				}
			} else if policy != "cap changes" {
				b.policy(t, policy)
			}
			if policy == "person" || policy == "unknown" {
				if code, text := b.run(t, "landing", "run"); code == 0 || b.starts != 0 {
					t.Fatalf("%s launched automation: %d %s", policy, code, text)
				}
				if policy == "person" {
					if got := b.batch(t); got.State != plain.BatchPrepared || len(got.Members) != 3 || got.Selector.Value != "person" {
						t.Fatalf("person proposal: %+v", got)
					}
					b.assemble(t, a, second, third)
					if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || b.executions(t) != 0 || !strings.Contains(text, "human-selection consumer is pending") {
						t.Fatalf("person proposal admitted: %d %s", code, text)
					}
				} else if batch, err := plain.ReadBatch(b.installation); err != nil || batch != nil {
					t.Fatalf("unreadable policy wrote selection: %+v %v", batch, err)
				}
				return
			}
			b.success(t, "landing", "run")
			selected := b.batch(t)
			if policy != "cap changes" {
				if len(selected.Members) != 3 {
					t.Fatalf("lawful large cap lost work: %+v", selected)
				}
				b.assemble(t, a, second, third)
				b.success(t, "landing", "prove", "--wait")
				return
			}
			b.policy(t, "1")
			b.assemble(t, a, second)
			b.refused(t, "landing", "prove", "--gate", "--wait")
			reselected := b.batch(t)
			if reselected.ID == selected.ID || reselected.State != plain.BatchPrepared || len(reselected.Members) != 1 {
				t.Fatalf("cap changed before admission: %+v", reselected)
			}
			b.assemble(t, a)
			b.success(t, "landing", "prove", "--gate", "--wait")
			running := b.batch(t)
			b.policy(t, "person")
			b.success(t, "landing", "run")
			if got := b.batch(t); got.ID != running.ID || got.State != plain.BatchRunning || len(got.Members) != 1 {
				t.Fatalf("running cap changed: %+v", got)
			}
			b.success(t, "landing", "prove", "--wait")
			b.success(t, "landing", "push")
			if code, _ := b.run(t, "landing", "run"); code == 0 || b.batch(t).Selector.Value != "person" {
				t.Fatal("next selection ignored later person policy")
			}
		})
	}
}

func TestLandingBatchAdapterStatusDoesNotSelectAndSeatEnvironmentCannotOverrideLane(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "2")
	b.seat(t, "a")
	b.seat(t, "b")
	b.seat(t, "c")
	for i := 0; i < 2; i++ {
		if data := b.status(t); data["batch"] != nil {
			t.Fatalf("status secretly selected: %+v", data["batch"])
		}
	}
	// --repo routes the act to the lane while authority/configuration still
	// belongs to the original seat checkout.
	b.cwd = filepath.Join(filepath.Dir(b.checkout), "seat-a")
	previous := b.owners.lookupEnv
	b.owners.lookupEnv = func(key string) (string, bool) {
		if key == config.EnvName("landing.batch") {
			return "100", true
		}
		return previous(key)
	}
	b.success(t, "landing", "run", "--repo", b.checkout)
	if got := b.batch(t); len(got.Members) != 2 || got.Selector.Source != "conf" || got.Selector.Checkout != b.checkout {
		t.Fatalf("seat environment selected lane: %+v", got)
	}
}

func TestLandingBatchAdapterRefusesMissingUnknownAndChangedAuthority(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing", "corrupt", "registration", "queue"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			sha := b.seat(t, "a")
			b.assemble(t, sha)
			if scenario == "missing" {
				b.refused(t, "landing", "prove", "--wait")
				b.refused(t, "landing", "prove")
				return
			}
			b.success(t, "landing", "run")
			selection := b.batch(t)
			switch scenario {
			case "corrupt":
				if err := os.WriteFile(filepath.Join(plain.Dir(b.installation), "batch.json"), []byte("{unknown"), 0600); err != nil {
					t.Fatal(err)
				}
			case "registration":
				record := selection.Lane
				record.CustodyEpoch++
				data, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(lane.RecordPath(b.home), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "queue":
				file, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "queue.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := file.WriteString("{torn\n")
				if err := file.Close(); writeErr != nil || err != nil {
					t.Fatalf("damage fixture: %v %v", writeErr, err)
				}
			}
			if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || b.executions(t) != 0 || b.launches != 0 {
				t.Fatalf("%s admitted unknown content: %d %s", scenario, code, text)
			}
			starts := b.starts
			if code, text := b.run(t, "landing", "run"); code == 0 || b.starts != starts {
				t.Fatalf("%s overwrote unknown authority: %d %s", scenario, code, text)
			}
			if scenario == "corrupt" || scenario == "queue" {
				data := b.status(t)
				if problems, _ := data["problems"].([]any); len(problems) == 0 {
					t.Fatalf("status hid %s: %+v", scenario, data)
				}
			}
		})
	}
}

func TestLandingBatchAdapterReconcilesSuccessfulPushAfterRecordFailure(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	sha := b.seat(t, "a")
	b.success(t, "landing", "run")
	selected := b.batch(t)
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--wait")
	pushes := 0
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "push" {
			pushes++
		}
		return plain.Git(dir, args...)
	}
	if err := os.Mkdir(filepath.Join(plain.Dir(b.installation), "pushes.jsonl"), 0700); err != nil {
		t.Fatal(err)
	}
	if code, text := b.run(t, "landing", "push"); code == 0 || pushes != 1 || !strings.Contains(text, "partial") {
		t.Fatalf("push record failure: %d %s; executions %d", code, text, pushes)
	}
	if batch, err := plain.ReadBatch(b.installation); err != nil || batch.ID != selected.ID || batch.State != plain.BatchRunning {
		t.Fatalf("unrecorded push lost selection: %+v %v", batch, err)
	}
	b.success(t, "landing", "push")
	if batch := b.batch(t); batch.ID != selected.ID || batch.State != plain.BatchClosed || pushes != 1 {
		t.Fatalf("reconciliation republished or lost selection: %+v, pushes %d", batch, pushes)
	}
}

func TestLandingBatchAdapterNoSelectionForEmptyQueueOrTrunk(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{"empty", "trunk with waiting goals", "trunk with running batch"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "2")
			var selected *plain.Batch
			if subject != "empty" {
				a := b.seat(t, "a")
				b.seat(t, "b")
				b.seat(t, "c")
				if subject == "trunk with running batch" {
					b.success(t, "landing", "run")
					b.assemble(t, a)
					b.success(t, "landing", "prove", "--gate", "--wait")
					var err error
					selected, err = plain.ReadBatch(b.installation)
					if err != nil || selected == nil || selected.State != plain.BatchRunning {
						t.Fatalf("running selection: %+v %v", selected, err)
					}
				}
			}
			before := b.executions(t)
			words := []string{"landing", "prove", "--wait"}
			if subject != "empty" {
				words = append(words, "--trunk")
			}
			b.success(t, words...)
			proof, present, err := plain.LastResult(b.installation)
			if err != nil || !present || proof.Result != plain.Green || proof.BatchID != "" || len(proof.BatchMembers) != 0 || b.executions(t) != before+1 {
				t.Fatalf("independent proof: %+v present %v executions %d: %v", proof, present, b.executions(t)-before, err)
			}
			if subject != "empty" && (!proof.Trunk || proof.Commit != b.main || proof.Scope != "full") {
				t.Fatalf("trunk proof checked the batch instead of fetched main: %+v", proof)
			}
			if got, err := plain.ReadBatch(b.installation); err != nil || !reflect.DeepEqual(got, selected) {
				t.Fatalf("independent proof changed selection: %+v; want %+v: %v", got, selected, err)
			}
		})
	}
}

func TestLandingKeeperEmptyQueueNeedsNoSelectionPolicy(t *testing.T) {
	t.Parallel()
	b, _, _, starts, _ := landingRestartBed(t)
	keeper := newLandingAgentKeeper(b.landingA, b.home, landingAgent{
		now:     func() time.Time { return laneTestNow },
		machine: func(string) (string, error) { return "lane-fixture", nil },
	})
	keeper.Running = func() (string, bool, error) { return "", false, nil }
	keeper.Fingerprint = nil
	keeper.Holds, keeper.Reap = nil, nil
	keeper.Sources.Reasons = func(string) ([]string, error) { return nil, nil }
	keeper.Start = func(string, lane.Wake) (string, error) { t.Fatal("an empty queue launched an agent"); return "", nil }
	// There is no repository or policy file to resolve in this empty lane.
	// Only SelectBatch's absence of work should decide preparation.
	if run := keeper.Run(); run.Outcome != lane.AgentIdle || *starts != 0 {
		t.Fatalf("empty lane: %+v; starts %d", run, *starts)
	}
	if selected, err := plain.ReadBatch(b.landingA); err != nil || selected != nil {
		t.Fatalf("empty lane recorded a selection: %+v %v", selected, err)
	}
}

// These adapter cases exercise commit ancestry and squash topology that a Git stub cannot establish.
func TestLandingBatchAdapterSameSHARehandIn(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{"merged", "selection stays prepared", "cannot skip"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			sha := b.seat(t, "a")
			if _, changed, err := plain.Return(b.installation, "a", "rebuild requested", b.now); err != nil || !changed {
				t.Fatalf("return: %v %v", changed, err)
			}
			var prefix string
			if subject == "cannot skip" {
				prefix = b.seat(t, "b")
			}
			if _, added, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: sha, Again: true, At: b.now.Format(time.RFC3339)}); err != nil || !added {
				t.Fatalf("re-hand-in: %v %v", added, err)
			}
			b.success(t, "landing", "run")
			selected := b.batch(t)
			b.assemble(t)
			if subject == "selection stays prepared" {
				b.success(t, "landing", "run")
				if got := b.batch(t); got.ID != selected.ID || got.State != plain.BatchPrepared {
					t.Fatalf("re-hand-in closed a prepared batch: %+v", got)
				}
			}
			if prefix != "" {
				b.assemble(t, prefix)
				b.success(t, "landing", "prove", "--gate", "--wait")
				b.refused(t, "landing", "prove", "--wait")
				b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", sha)
			} else {
				b.assemble(t, sha)
			}
			b.success(t, "landing", "prove", "--wait")
		})
	}
}

func TestLandingBatchAdapterRefusesUnselectedSquash(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "2")
	a, second, third := b.seat(t, "a"), b.seat(t, "b"), b.seat(t, "c")
	b.success(t, "landing", "run")
	b.assemble(t, a, second)
	b.git(t, b.checkout, "merge", "--quiet", "--squash", third)
	b.git(t, b.checkout, "commit", "--quiet", "-m", "squash c")
	before := b.executions(t)
	code, words := b.run(t, "landing", "prove", "--wait")
	if code == 0 || !strings.Contains(words, plain.CodeBatch) || !strings.Contains(words, "goal c") || b.executions(t) != before {
		t.Fatalf("unselected squash admitted or not named: %d %s checks=%d", code, words, b.executions(t))
	}
	b.refused(t, "landing", "prove")
	b.refused(t, "landing", "prove", "--gate", "--wait")
}

func TestLandingBatchAdapterProvesMainWithoutSelectionDespiteHistory(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{"waiting", "landed"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			sha := b.seat(t, "a")
			if subject == "landed" {
				b.assemble(t, sha)
				b.git(t, b.checkout, "push", "--quiet", "origin", "HEAD:main")
			}
			b.assemble(t)
			b.success(t, "landing", "prove", "--wait")
			if batch, err := plain.ReadBatch(b.installation); err != nil || batch != nil {
				t.Fatalf("main proof selected work: %+v %v", batch, err)
			}
			if proof, ok, err := plain.LastResult(b.installation); err != nil || !ok || proof.Trunk || proof.Result != plain.Green {
				t.Fatalf("plain main proof: %+v %v", proof, err)
			}
		})
	}
}
