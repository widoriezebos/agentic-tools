package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func proofPermissionBed(t *testing.T) *batchVerbBed {
	t.Helper()
	b := newBatchVerbBed(t, "auto")
	b.configuration += "landing.proof=person\n"
	b.policy(t, "auto")
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("ordinary agent")
	}
	b.owners.processes.question = channel.ReadQuestion
	b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
	a := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, a)
	return b
}

func proofQuestion(t *testing.T, b *batchVerbBed, command string) channel.Question {
	t.Helper()
	questions, unread := channel.WalkOpenQuestions(b.installation)
	if len(unread) != 0 {
		t.Fatal(unread)
	}
	for _, q := range questions {
		if channel.LaneStopCommand(q) == command {
			code, text := b.run(t, "question", "show", "channel:"+q.ID)
			if code != 0 || !strings.Contains(text, command) {
				t.Fatalf("question show: %d %s", code, text)
			}
			code, text = b.plainVerbBed.run(t, "landing", "status", "--verbose")
			if code != 0 || !strings.Contains(text, command) {
				t.Fatalf("status: %d %s", code, text)
			}
			return q
		}
	}
	t.Fatalf("missing proof request %s: %+v", command, questions)
	return channel.Question{}
}

// Both full and trunk commands walk actual enrolled authority at the calling
// checkout. Helm and attorney fallbacks never write a proof admission.
func TestLandingProofPermissionAuthorityMatrix(t *testing.T) {
	t.Parallel()
	for _, trunk := range []bool{false, true} {
		t.Run(fmt.Sprintf("trunk=%v", trunk), func(t *testing.T) {
			t.Parallel()
			b := proofPermissionBed(t)
			// Merge gates need no permission even with proof set to person.
			b.success(t, "landing", "prove", "--gate", "--wait")
			if qs, _ := channel.WalkOpenQuestions(b.installation); len(qs) != 0 {
				t.Fatalf("gate asked: %+v", qs)
			}
			words := []string{"landing", "prove", "--wait"}
			command := "metasystem landing prove"
			if trunk {
				words = append(words, "--trunk")
				command += " --trunk"
			}
			before := b.executions(t)
			code, text := b.run(t, words...)
			if code == 0 || !strings.Contains(text, "LANE_PROOF_PERSON") || b.executions(t) != before {
				t.Fatalf("automatic full: %d %s", code, text)
			}
			q := proofQuestion(t, b, command)
			calling := filepath.Join(t.TempDir(), "caller")
			b.git(t, filepath.Dir(calling), "clone", "--quiet", b.origin, calling)
			calling = realpath.Resolve(calling)
			root := filepath.Join(calling, "metasystem")
			b.cwd = calling
			_, err := humanauthority.Enroll(root, 20, person(), "Wido", b.now)
			helmMust(t, err)
			_, err = helm.Write(calling, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Checkout: calling, Reason: "manual"})
			helmMust(t, err)
			fake := newFakeHelm(calling, filepath.Join(calling, ".git"), "DELEGATE")
			useHelmAdmitter(root, fake.admitter)
			t.Cleanup(func() { helmAdmitters.Delete(resolvedHelmRoot(root)) })
			pid := int64(80)
			var proof humanauthority.Proof
			var proofErr error
			b.owners.prove = func(atRoot string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
				if atRoot != root {
					t.Errorf("authority borrowed destination: %s", atRoot)
				}
				proof, proofErr = humanauthority.Prove(root, pid, person(), at)
				return proof, proofErr
			}
			fake.machinery = true
			if code, text := b.run(t, words...); code == 0 || proofErr == nil {
				t.Fatalf("machinery: %d %s %+v", code, text, proof)
			}
			fake.machinery, pid = false, 60
			if direct, err := humanauthority.Prove(root, pid, person(), b.now); err != nil || direct.Helm == nil {
				t.Fatalf("helm fallback not reached: %+v %v", direct, err)
			}
			if code, text := b.run(t, words...); code == 0 || proofErr != nil || proof.Helm == nil || len(fake.yields) != 1 {
				t.Fatalf("helm caller: %d %s %+v %v", code, text, proof, proofErr)
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, b.now)
			}
			if code, text := b.run(t, words...); code == 0 {
				t.Fatalf("attorney admitted: %s", text)
			}
			if _, recorded, _, _ := plain.ReadRunning(b.installation, b.owners.landing.plainProve); recorded || b.executions(t) != before {
				t.Fatal("unproved caller wrote admission or executed")
			}
			current, err := channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State == "closed" {
				t.Fatalf("refused request closed: %+v %v", current, err)
			}
			_, err = lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			b.owners.prove = enrolledPersonProver(t, root, b.now)
			b.success(t, words...)
			result, found, err := plain.LastResult(b.installation)
			if err != nil || !found || result.Person == nil || result.Person.Root != root || result.Person.Person != "Wido" || result.Trunk != trunk || len(result.Executions) != 1 || result.Executions[0].Scope != "full" || result.Executions[0].Tree != result.Tree || result.Executions[0].State != "claimed" || b.executions(t) != before+1 {
				t.Fatalf("direct exact effect: %+v %v", result, err)
			}
			current, err = channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("admission request open: %+v %v", current, err)
			}
			if !helm.Active(calling).Active {
				t.Fatal("proof removed helm")
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("proof removed pause")
			}
			if !trunk {
				b.success(t, "landing", "prove")
				if b.executions(t) != before+1 || b.launches != 0 {
					t.Fatal("reused green executed or launched")
				}
			}
		})
	}
}

// The launch callback observes durable pending admission. A child already
// entering the public handler waits on the parent's queue lock, then claims
// once without enrollment. A repeat during execution starts nothing.
func TestLandingProofPermissionDetachedAdmissionRace(t *testing.T) {
	t.Parallel()
	b := proofPermissionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	childOwners := b.owners
	childOwners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		t.Error("child re-proved terminal")
		return humanauthority.Proof{}, errors.New("child")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	childRead := make(chan bool, 1)
	readDeclaration := false
	childOwners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if !readDeclaration && len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
			readDeclaration = true
			running, _, _, _ := plain.ReadRunning(b.installation, childOwners.landing.plainProve)
			childRead <- running.Admission != nil && running.Admission.State == "pending"
		}
		return plain.Git(dir, args...)
	}
	childOwners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		close(entered)
		<-release
		return cmd.Run()
	}
	completed := make(chan int, 1)
	var childWords []string
	b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
		pending, recorded, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
		if err != nil || !recorded || pending.Admission == nil || pending.Admission.State != "pending" || pending.Person == nil || pending.Admission.Command == "" {
			t.Errorf("launch before admission: %+v %v", pending, err)
		}
		childWords = slices.Clone(argv[1:])
		go func() {
			cmd, rest, _ := resolveIntentArgv(childWords)
			var out bytes.Buffer
			completed <- runIntentIn(cmd, rest, &out, &out, b.cwd, childOwners)
		}()
		if pending := <-childRead; !pending {
			t.Error("child did not reach its pending admission before launch recording")
		}
		return int64(os.Getpid()), nil
	}
	b.success(t, "landing", "prove")
	<-entered
	running, recorded, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
	if err != nil || !recorded || running.Admission.State != "claimed" {
		t.Fatalf("child claim: %+v %v", running, err)
	}
	b.success(t, "landing", "prove")
	if code, text := b.run(t, childWords...); code == 0 {
		t.Fatalf("claimed admission replay: %s", text)
	}
	close(release)
	if code := <-completed; code != 0 {
		t.Fatalf("child exit %d", code)
	}
	if b.executions(t) != 1 {
		t.Fatalf("execution count %d", b.executions(t))
	}
	if code, text := b.run(t, childWords...); code == 0 {
		t.Fatalf("completed admission replay: %s", text)
	}
	for _, id := range []string{"fabricated", running.Attempt} {
		if code, text := b.run(t, "landing", "prove", "--wait", "--attempt", id); code == 0 {
			t.Fatalf("bare admission %s: %s", id, text)
		}
	}
	if b.executions(t) != 1 {
		t.Fatal("replay executed")
	}
}

func TestLandingProofPermissionRejectsMismatchedAdmissionAndFailedLaunch(t *testing.T) {
	t.Parallel()
	b := proofPermissionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.success(t, "landing", "prove")
	running, _, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
	helmMust(t, err)
	running.Admission.Tree = "different"
	raw, err := json.Marshal(running)
	helmMust(t, err, os.WriteFile(filepath.Join(plain.Dir(b.installation), "running.json"), raw, 0600))
	if code, text := b.run(t, "landing", "prove", "--wait", "--attempt", running.Attempt); code == 0 || b.executions(t) != 0 {
		t.Fatalf("mismatch: %d %s", code, text)
	}
	helmMust(t, os.Remove(filepath.Join(plain.Dir(b.installation), "running.json")))
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { return 0, errors.New("launcher unavailable") }
	if code, text := b.run(t, "landing", "prove"); code == 0 {
		t.Fatalf("failed launch: %s", text)
	}
	failed, recorded, alive, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
	if err != nil || !recorded || alive || failed.Admission.State != "failed" || b.executions(t) != 0 {
		t.Fatalf("failed admission: %+v %v", failed, err)
	}
	if code, text := b.run(t, "landing", "prove", "--wait", "--attempt", failed.Attempt); code == 0 {
		t.Fatalf("failed admission claimed: %s", text)
	}
	b.success(t, "landing", "prove", "--wait")
	if b.executions(t) != 1 {
		t.Fatal("failed launch prevented fresh exact act")
	}
}

// Every followed stop command preserves spent automatic history and unrelated
// requests. Gate remedies execute cheap checks without a full-policy question.
func TestLandingProofPermissionFollowScopeStopCommands(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"gate", "trunk", "full", "budget"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := proofPermissionBed(t)
			if mode == "gate" {
				b.success(t, "landing", "prove", "--gate", "--wait")
				path := filepath.Join(plain.Dir(b.installation), "gates.jsonl")
				raw, err := os.ReadFile(path)
				helmMust(t, err)
				lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
				helmMust(t, os.WriteFile(path, append(bytes.Join(lines[:len(lines)-1], []byte("\n")), '\n'), 0600))
			}
			script := filepath.Join(filepath.Dir(b.checkout), "environment-check")
			body := "#!/bin/sh\nprintf '%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nprintf 'LANDING-NOT-RUN\\tdisk full\\n'\nexit 2\n"
			helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
			// Declarations are pinned at the commit; execution's environment can fail.
			command := b.owners.landing.plainProve.Command
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				cmd.Args = []string{"sh", "-c", shellCommand([]string{script})}
				return cmd.Run()
			}
			words := []string{"landing", "prove", "--wait"}
			remedy := "metasystem landing prove"
			file := "results.jsonl"
			if mode == "gate" {
				words = append(words, "--gate")
				remedy += " --gate"
				file = "gates.jsonl"
			}
			if mode == "trunk" {
				words = append(words, "--trunk")
				remedy += " --trunk"
			}
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			if mode == "budget" {
				head := b.git(t, b.checkout, "rev-parse", "HEAD")
				tree := b.git(t, b.checkout, "rev-parse", "HEAD^{tree}")
				selected := b.batch(t)
				var proofs []plain.Result
				for _, id := range []string{"first", "second"} {
					proofs = append(proofs, plain.Result{Commit: head, Tree: tree, Result: plain.Red, Attempt: id, CountedFull: true, Scope: "full", Repeat: "allowed", Goals: selected.Members, BatchID: selected.ID, BatchMembers: selected.Members})
				}
				writeCauseProof(t, b.installation, file, proofs...)
				b.configuration = strings.ReplaceAll(b.configuration, "landing.proof=person", "landing.proof=auto")
				b.policy(t, "auto")
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("agent")
				}
				if code, text := b.run(t, words...); code == 0 || !strings.Contains(text, "LANE_PROOF_BUDGET") {
					t.Fatalf("budget hold: %d %s", code, text)
				}
			} else {
				// Independently reach the repeated environment stop on each scope.
				for range 2 {
					if code, text := b.run(t, words...); code == 0 {
						t.Fatalf("environment check passed: %s", text)
					}
				}
			}
			q := proofQuestion(t, b, remedy)
			unrelated := plain.Stop{Loop: "lane-return", Subject: "other-goal", Attempt: 2, Budget: 2, Decision: "stop", Handoff: "ask lane", Cause: &plain.Cause{Kind: "unclassified"}, At: b.now.Format(time.RFC3339)}
			raw, err := json.Marshal(unrelated)
			helmMust(t, err)
			stopPath := filepath.Join(plain.Dir(b.installation), "stops.jsonl")
			f, err := os.OpenFile(stopPath, os.O_APPEND|os.O_WRONLY, 0600)
			helmMust(t, err)
			_, err = f.Write(append(raw, '\n'))
			helmMust(t, err, f.Close(), plain.SyncPolicyQuestion(b.installation, b.owners.landing.machine, b.now))
			prior, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), file))
			helmMust(t, err)
			b.success(t, "landing", "run")
			current, err := channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State == "closed" {
				t.Fatal("generic run closed proof request")
			}
			retained, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), file))
			if err != nil || !bytes.Equal(prior, retained) {
				t.Fatal("generic run reset proof history")
			}
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.owners.landing.plainProve.Command = command
			_, err = lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			if mode == "trunk" || mode == "full" {
				b.advanceMain(t, "metasystem/trunk-moved.md", "main moved after its stop\n")
			}
			before := b.executions(t)
			followed := append(strings.Fields(remedy)[1:], "--wait")
			b.success(t, followed...)
			if b.executions(t) != before+1 {
				t.Fatalf("matching execution count %d -> %d", before, b.executions(t))
			}
			current, err = channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("matching stop stayed open: %+v %v", current, err)
			}
			stop, err := plain.NewestStop(b.installation)
			if err != nil || stop == nil || stop.Subject != "other-goal" {
				t.Fatalf("unrelated stop lost: %+v %v", stop, err)
			}
			after, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), file))
			if err != nil || !bytes.HasPrefix(after, prior) {
				t.Fatal("person proof erased history")
			}
			last, found, err := plain.LastResult(b.installation)
			if mode == "budget" && (err != nil || !found || !last.Executions[0].Override) {
				t.Fatalf("budget exception missing: %+v %v", last, err)
			}
		})
	}
}

// Corrupt advice cannot veto the known target or erase its bytes. Admission
// failures never close a request or run a command.
func TestLandingProofPermissionCorruptAdviceAndWriteFailure(t *testing.T) {
	t.Parallel()
	b := proofPermissionBed(t)
	_, _ = b.run(t, "landing", "prove", "--wait")
	q := proofQuestion(t, b, "metasystem landing prove")
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	runningPath := filepath.Join(plain.Dir(b.installation), "running.json")
	helmMust(t, os.MkdirAll(runningPath+".tmp", 0755))
	if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || b.executions(t) != 0 {
		t.Fatalf("write failure: %d %s", code, text)
	}
	current, err := channel.ReadQuestion(b.installation, q.ID)
	if err != nil || current.State == "closed" {
		t.Fatal("write failure closed request")
	}
	helmMust(t, os.Remove(runningPath+".tmp"))
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, errors.New("unreadable") }
	index := filepath.Join(plain.Dir(b.installation), "stop-question")
	helmMust(t, os.WriteFile(index, []byte("corrupt advisory index"), 0600))
	_, err = lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	b.success(t, "landing", "prove", "--wait")
	last, found, err := plain.LastResult(b.installation)
	if err != nil || !found || last.Person == nil || len(last.Executions) != 1 || !strings.Contains(last.Executions[0].Warning, "policy") || b.executions(t) != 1 {
		t.Fatalf("corrupt advice refused person: %+v %v", last, err)
	}
	if _, paused := lane.ReadPause(b.home); !paused {
		t.Fatal("direct proof removed pause")
	}

}

func TestLandingProofPermissionScopedEscalationNeedsFreshAct(t *testing.T) {
	t.Parallel()
	for _, directScoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("directScoped=%v", directScoped), func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			b.configuration += "testing.contract=testing.json\nlanding.proof=person\n"
			b.policy(t, "auto")
			contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"metasystem/**"}, Standard: []string{"plans"}}}, Unknown: []string{"plans"},
				Groups: []testpolicy.Group{{ID: "plans", Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/plans/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"app-tests"}, Format: "exit-status"}}}
			raw, err := json.Marshal(contract)
			helmMust(t, err, os.WriteFile(filepath.Join(b.installation, "testing.json"), raw, 0600))
			helmMust(t, os.MkdirAll(filepath.Join(b.installation, "plans"), 0755), os.WriteFile(filepath.Join(b.installation, "plans", "page.md"), []byte("initial record\n"), 0600))
			b.git(t, b.checkout, "add", "metasystem/metasystem.conf", "metasystem/testing.json", "metasystem/plans/page.md")
			b.git(t, b.checkout, "commit", "--quiet", "-m", "declare scoped proof contract")
			b.git(t, b.checkout, "push", "--quiet", "origin", "main")
			b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
			script := filepath.Join(filepath.Dir(b.checkout), "check")
			writeCheck := func(environment string) {
				t.Helper()
				body := "#!/bin/sh\nprintf '%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nprintf 'landing environment " + environment + "\\nLANDING-CHECKED\\t0\\n'\n"
				helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
			}
			writeCheck("base")
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.owners.processes.question = channel.ReadQuestion
			b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
			a := b.seat(t, "a")
			b.success(t, "landing", "run")
			b.assemble(t, a)
			b.success(t, "landing", "prove", "--wait")
			b.advanceMain(t, "metasystem/plans/page.md", "record refresh\n")
			b.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) {
				return adapter.Closure{Unowned: []string{"metasystem/plans/page.md"}}, nil
			}
			if !directScoped {
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("ordinary agent")
				}
			}
			writeCheck("changed")
			before := b.executions(t)
			if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || !strings.Contains(text, "LANE_PROOF_PERSON") || b.executions(t) != before+1 {
				t.Fatalf("scoped escalation: %d %s checks=%d", code, text, b.executions(t))
			}
			last, found, err := plain.LastResult(b.installation)
			if err != nil || !found || last.Result != "held" || last.Cause != nil || !strings.HasPrefix(last.ScopeReason, "full check pending:") || last.Scope != "scoped" || len(last.Executions) != 1 || last.Executions[0].Scope != "scoped" || last.CountedFull {
				t.Fatalf("scoped evidence: %+v %v", last, err)
			}
			q := proofQuestion(t, b, "metasystem landing prove")
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.success(t, "landing", "prove", "--wait")
			last, found, err = plain.LastResult(b.installation)
			if err != nil || !found || last.Scope != "full" || len(last.Executions) != 1 || last.Executions[0].Scope != "full" || b.executions(t) != before+2 {
				t.Fatalf("fresh full act: %+v %v checks=%d", last, err, b.executions(t))
			}
			current, err := channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("fresh full did not close request: %+v %v", current, err)
			}
		})
	}
}
