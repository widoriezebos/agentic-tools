package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func checkActBuild(t *testing.T, b *workBed, owners intentOwners, args ...string) (int, intentResult) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatalf("no command %v", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, b.root(), owners)
	raw := stdout.String()
	var result intentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("exit %d: %s %s: %v", code, raw, stderr.String(), err)
	}
	b.recordReadDirs(result)
	return code, result
}

func checkBuildArgs(b *workBed, unit, brief string, check []string) []string {
	return append([]string{"work", "build", b.id, "--work", unit, "--brief", brief, "--lines", "5", "--check"}, check...)
}

func checkNextWorktree(t *testing.T, b *workBed) {
	t.Helper()
	parent := t.TempDir()
	b.worktree = filepath.Join(parent, "work")
	for _, path := range []string{b.worktree, filepath.Join(parent, "objects")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "index"), []byte("index"), 0600); err != nil {
		t.Fatal(err)
	}
}

func checkActs(t *testing.T, b *workBed) []processchange.ProcessAct {
	t.Helper()
	acts, unknown, err := processchange.ReadActs(b.root(), b.id)
	if err != nil || len(unknown) != 0 {
		t.Fatalf("acts: %v %v", err, unknown)
	}
	return acts
}

func checkActLaunches(t *testing.T, b *workBed, result intentResult, id string) {
	t.Helper()
	plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || plan.Check == nil || plan.Check.ProcessAct != id {
		t.Fatalf("consumed plan: %+v %v", plan.Check, err)
	}
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, result)["run"].(string))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, step := range record.Rounds[0].Steps {
		if step.Kind != "build" && step.Kind != "attest" {
			continue
		}
		seen++
		physical, err := b.manager.Store.Read(step.LaunchID)
		var consumed string
		_ = json.Unmarshal(physical.AdapterData["checkAct"], &consumed)
		if err != nil || consumed != id {
			t.Fatalf("%s consumed %q, want %q: %v", step.Kind, consumed, id, err)
		}
	}
	if seen < 2 {
		t.Fatalf("missing physical build or proof launch: %+v", record.Rounds[0].Steps)
	}
}

func TestProcessCheckPublicRemedy(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.lineage = "builder"
	b.declaredCheap = "true"
	brief := b.brief("check.md", "Build this unit.\n")
	owners := b.workOwners()
	code, unchanged := checkActBuild(t, b, owners, checkBuildArgs(b, "initial", brief, []string{"true"})...)
	if code != 0 || len(checkActs(t, b)) != 0 {
		t.Fatalf("unchanged selection: %d %+v", code, unchanged)
	}
	beforeLaunches := len(b.starter.launched())
	checkNextWorktree(t, b)
	script := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	argv := []string{script, "two words"}
	args := checkBuildArgs(b, "changed", brief, argv)
	code, held := checkActBuild(t, b, owners, args...)
	act := processAct(t, held)
	if code != 1 || act.Status != "proposed" || act.Class != "widened-check" || act.Actor != "agent" || act.Lineage != "builder" || !slices.Equal(act.BeforeArgv, []string{"true"}) || !slices.Equal(act.AfterArgv, argv) || len(b.starter.launched()) != beforeLaunches {
		t.Fatalf("held before launch: %d %+v", code, held)
	}
	_, repeated := checkActBuild(t, b, owners, args...)
	if again := processAct(t, repeated); again.ID != act.ID || again.Question != act.Question || len(checkActs(t, b)) != 1 {
		t.Fatalf("duplicate proposal: %+v", again)
	}
	q, err := channel.ReadQuestion(b.root(), act.Question)
	if err != nil || q.ProcessAct != act.ID || q.Wants != shellCommand(held.Next.Argv) {
		t.Fatalf("exact question: %+v %v", q, err)
	}
	code, spoofed := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
	if code != 1 || len(b.starter.launched()) != beforeLaunches {
		t.Fatalf("--act granted person power: %d %+v", code, spoofed)
	}
	actPath := filepath.Join(b.root(), "process", "acts", act.ID+".json")
	original, err := os.ReadFile(actPath)
	if err != nil {
		t.Fatal(err)
	}
	damaged := act
	damaged.Status = "unknown effect"
	body, _ := json.Marshal(damaged)
	if err := os.WriteFile(actPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	if code, result := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 1 || len(b.starter.launched()) != beforeLaunches {
		t.Fatalf("unknown effect admitted agent: %d %+v", code, result)
	}
	if err := os.WriteFile(actPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	code, launched := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
	if code != 0 {
		t.Fatalf("person remedy: %d %+v", code, launched)
	}
	checkActLaunches(t, b, launched, act.ID)
	if !slices.Equal(starter.calls, []int{0, 0}) {
		t.Fatalf("real check calls: %v", starter.calls)
	}
	for _, result := range starter.results {
		data, _ := json.Marshal(result.Data)
		if !strings.Contains(string(data), "two words") {
			t.Fatalf("exact argv was not executed: %s", data)
		}
	}
	applied := checkActs(t, b)[0]
	q, _ = channel.ReadQuestion(b.root(), act.Question)
	if applied.Status != "applied" || applied.AppliedProof.Helm != nil || applied.AppliedProof.TerminalGeneration == 0 || q.State != "closed" {
		t.Fatalf("exact completion: %+v %+v", applied, q)
	}
	count := len(b.starter.launched())
	if code, replay := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 0 || len(b.starter.launched()) != count {
		t.Fatalf("duplicate launch: %d %+v", code, replay)
	}
	owners.prove = fixedFixtureGoalAuthority
	checkNextWorktree(t, b)
	code, next := checkActBuild(t, b, owners, checkBuildArgs(b, "same", brief, argv)...)
	if code != 0 || len(checkActs(t, b)) != 1 {
		t.Fatalf("unchanged replay created act: %d %+v", code, next)
	}
	checkActLaunches(t, b, next, act.ID)
	t.Log("work build held before launch; exact person command closed its question, retained argv, and bound build/proof launches; unchanged selection reused the act")
}

func TestProcessCheckFirstAndPersonSelection(t *testing.T) {
	t.Parallel()
	t.Run("person-first", func(t *testing.T) {
		t.Parallel()
		b := newWorkBed(t)
		b.declaredCheap = "true"
		owners := b.workOwners()
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		args := []string{"work", "build", b.id, "--work", "person-first", "--brief", b.brief("person.md", "Build.\n"), "--lines", "5", "--reason", "Choose this check", "--by", "Wido", "--check", "false"}
		if code, result := checkActBuild(t, b, owners, args...); code != 0 {
			t.Fatalf("person first selection: %d %+v", code, result)
		}
		acts := checkActs(t, b)
		if len(acts) != 1 || acts[0].BeforeArgv != nil || acts[0].Class != "added-check" || acts[0].Actor != "direct-person" {
			t.Fatalf("person first before-value: %+v", acts)
		}
	})
	for _, explicitAct := range []bool{true, false} {
		t.Run(map[bool]string{true: "exact-act", false: "own-command"}[explicitAct], func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			b.lineage, b.declaredCheap = "builder", "true"
			brief := b.brief("first.md", "Build.\n")
			owners := b.workOwners()
			owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
				return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "grant-42", Class: "DELEGATE"}, at)
			}
			args := checkBuildArgs(b, "first", brief, []string{"false"})
			code, held := checkActBuild(t, b, owners, args...)
			act := processAct(t, held)
			if code != 1 || act.Class != "added-check" || act.Proof.Helm == nil || act.Proof.Helm.Grant != "grant-42" || !slices.Equal(act.BeforeArgv, []string{"true"}) || len(b.starter.launched()) != 0 {
				t.Fatalf("first grant-backed change: %d %+v", code, held)
			}
			owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
			personArgs := held.Next.Argv[1:]
			if !explicitAct {
				personArgs = []string{"work", "build", b.id, "--work", "first", "--brief", brief, "--lines", "5", "--reason", "Use this exact check", "--by", "Wido", "--check", "false"}
			}
			code, built := checkActBuild(t, b, owners, personArgs...)
			if code != 0 {
				t.Fatalf("person selection: %d %+v", code, built)
			}
			checkActLaunches(t, b, built, act.ID)
			q, _ := channel.ReadQuestion(b.root(), act.Question)
			if q.State != "closed" || len(checkActs(t, b)) != 1 {
				t.Fatalf("person selection did not consume pending act: %+v", q)
			}
		})
	}
}

func TestProcessCheckHistoryAndRecovery(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.lineage, b.declaredCheap = "builder", "true"
	brief := b.brief("history.md", "Build.\n")
	owners := b.workOwners()
	_, held := checkActBuild(t, b, owners, checkBuildArgs(b, "pending", brief, []string{"false"})...)
	pendingTree := b.worktree
	checkNextWorktree(t, b)
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	args := []string{"work", "build", b.id, "--work", "later", "--brief", brief, "--lines", "5", "--reason", "Replacement", "--by", "Wido", "--check", "third"}
	if code, result := checkActBuild(t, b, owners, args...); code != 0 {
		t.Fatalf("replacement: %d %+v", code, result)
	}
	count := len(b.starter.launched())
	b.worktree = pendingTree
	code, stale := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
	if code != 1 || processAct(t, stale).Status != "superseded" || len(b.starter.launched()) != count {
		t.Fatalf("older proposal hid current selection: %d %+v", code, stale)
	}
	if code, result := checkActBuild(t, b, owners, stale.Next.Argv[1:]...); code != 0 {
		t.Fatalf("superseded remedy: %d %+v", code, result)
	}
	count = len(b.starter.launched())
	// Damage only synthetic retained history, leaving the person's explicit route.
	paths, err := filepath.Glob(filepath.Join(b.root(), "process", "check-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("selection path: %v %v", paths, err)
	}
	if err := os.WriteFile(paths[0], []byte("damaged history"), 0600); err != nil {
		t.Fatal(err)
	}
	owners.prove = fixedFixtureGoalAuthority
	checkNextWorktree(t, b)
	if code, result := checkActBuild(t, b, owners, checkBuildArgs(b, "unknown", brief, []string{"true"})...); code == 0 || len(b.starter.launched()) != count {
		t.Fatalf("unknown history admitted agent: %d %+v", code, result)
	}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	args[4] = "unknown"
	if code, result := checkActBuild(t, b, owners, args...); code == 0 || !strings.Contains(result.Summary, "request") {
		t.Fatalf("changed request replaced retained unit: %d %+v", code, result)
	}
	personArgs := []string{"work", "build", b.id, "--work", "unknown", "--brief", brief, "--lines", "5", "--reason", "Explicit repair", "--by", "Wido", "--check", "true"}
	if code, result := checkActBuild(t, b, owners, personArgs...); code != 0 {
		t.Fatalf("person recovery: %d %+v", code, result)
	}
	archived, _ := filepath.Glob(paths[0] + ".damaged-*")
	if len(archived) != 1 {
		t.Fatal("damaged history was discarded")
	}
	if body, err := os.ReadFile(archived[0]); err != nil || string(body) != "damaged history" {
		t.Fatalf("damaged bytes: %s %v", body, err)
	}
	acts := checkActs(t, b)
	if !slices.ContainsFunc(acts, func(act processchange.ProcessAct) bool {
		return act.Unit == "unknown" && act.Status == "applied" && act.BeforeArgv == nil
	}) {
		t.Fatal("person repair fabricated an available previous selection")
	}
	if err := os.Rename(paths[0], paths[0]+".retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths[0], 0700); err != nil {
		t.Fatal(err)
	}
	checkNextWorktree(t, b)
	observed := []string{"work", "build", b.id, "--work", "observed", "--brief", brief, "--lines", "5"}
	if code, result := checkActBuild(t, b, owners, observed...); code != 0 || len(checkActs(t, b)) != len(acts) {
		t.Fatalf("observation failure held ordinary work: %d %+v", code, result)
	}
	count = len(b.starter.launched())
	checkNextWorktree(t, b)
	personArgs[4] = "preservation-failure"
	if code, result := checkActBuild(t, b, owners, personArgs...); code != 1 || len(b.starter.launched()) != count || !strings.Contains(result.Summary, "preserved") {
		t.Fatalf("physical preservation failure launched: %d %+v", code, result)
	}
}

func TestProcessCheckOrdinaryContentionUpdatesSelection(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap = "true"
	brief := b.brief("contention.md", "Build.\n")
	owners := b.workOwners()
	if code, result := checkActBuild(t, b, owners, checkBuildArgs(b, "initial", brief, []string{"true"})...); code != 0 {
		t.Fatalf("initial selection: %d %s", code, result.Summary)
	}
	held, err := lock.File(filepath.Join(b.root(), "process", "lock"), 0600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() })
	b.declaredCheap = "false"
	checkNextWorktree(t, b)
	done := make(chan struct{})
	var code int
	var result intentResult
	go func() {
		defer close(done)
		code, result = checkActBuild(t, b, owners, "work", "build", b.id, "--work", "ordinary", "--brief", brief, "--lines", "5")
	}()
	// Observe the actual blocking flock before releasing its owner.
	stack := make([]byte, 1<<20)
waiting:
	for {
		select {
		case <-done:
			t.Fatalf("ordinary admission skipped the busy lock: %d %s", code, result.Summary)
		default:
		}
		count := runtime.Stack(stack, true)
		for _, goroutine := range strings.Split(string(stack[:count]), "\n\n") {
			if strings.Contains(goroutine, "processchange.AdmitDeclaration") && strings.Contains(goroutine, "unix.Flock") {
				break waiting
			}
		}
		runtime.Gosched()
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	<-done
	if code != 0 || len(checkActs(t, b)) != 0 {
		t.Fatalf("ordinary admission: %d %s", code, result.Summary)
	}
	count := len(b.starter.launched())
	checkNextWorktree(t, b)
	code, result = checkActBuild(t, b, owners, checkBuildArgs(b, "changed", brief, []string{"true"})...)
	act := processAct(t, result)
	if code != 1 || act.Status != "proposed" || !slices.Equal(act.BeforeArgv, []string{"false"}) || len(b.starter.launched()) != count {
		t.Fatalf("ordinary selection was not retained: %d %s", code, result.Summary)
	}
}

func TestProcessCheckFailedObservationHoldsNextAgent(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap = "true"
	brief := b.brief("failed-record.md", "Build.\n")
	owners := b.workOwners()
	if code, result := checkActBuild(t, b, owners, checkBuildArgs(b, "initial", brief, []string{"true"})...); code != 0 {
		t.Fatalf("initial selection: %d %s", code, result.Summary)
	}
	paths, err := filepath.Glob(filepath.Join(b.root(), "process", "check-*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("selection path: %v %v", paths, err)
	}
	before, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	process := filepath.Dir(paths[0])
	// Selection publication fails after the round's declaration is durable.
	// Reads, the existing lock and the acts directory remain usable.
	now := b.manager.Now
	b.manager.Now = func() time.Time {
		declarations, _ := filepath.Glob(filepath.Join(process, "declaration-*.json"))
		for _, path := range declarations {
			data, _ := os.ReadFile(path)
			var reference struct{ Operation string }
			if json.Unmarshal(data, &reference) == nil && reference.Operation != "" {
				if err := os.Chmod(process, 0500); err != nil {
					t.Fatal(err)
				}
				b.manager.Now = now
			}
		}
		return now()
	}
	t.Cleanup(func() { _ = os.Chmod(process, 0700) })
	b.declaredCheap = "false"
	checkNextWorktree(t, b)
	code, result := checkActBuild(t, b, owners, "work", "build", b.id, "--work", "ordinary", "--brief", brief, "--lines", "5")
	if code != 0 {
		t.Fatalf("failed observation blocked ordinary work: %d %s", code, result.Summary)
	}
	if err := os.Chmod(process, 0700); err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(before, retained) {
		t.Fatalf("selection write did not fail with its prior bytes intact: %s %v", retained, err)
	}
	count := len(b.starter.launched())
	checkNextWorktree(t, b)
	code, result = checkActBuild(t, b, owners, checkBuildArgs(b, "unknown", brief, []string{"true"})...)
	if code != 1 || !strings.Contains(result.Summary, "unavailable") || len(b.starter.launched()) != count {
		t.Fatalf("stale selection admitted the next agent: %d %s", code, result.Summary)
	}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	args := []string{"work", "build", b.id, "--work", "unknown", "--brief", brief, "--lines", "5", "--reason", "Repair unknown selection", "--by", "Wido", "--check", "true"}
	if code, result := checkActBuild(t, b, owners, args...); code != 0 {
		t.Fatalf("person could not repair failed observation: %d %s", code, result.Summary)
	}
	acts := checkActs(t, b)
	if len(acts) != 1 || acts[0].BeforeArgv != nil || acts[0].Status != "applied" {
		t.Fatalf("repair used stale before-values: %+v", acts)
	}
	checkNextWorktree(t, b)
	owners.prove = fixedFixtureGoalAuthority
	if code, result := checkActBuild(t, b, owners, checkBuildArgs(b, "repaired", brief, []string{"true"})...); code != 0 || len(checkActs(t, b)) != 1 {
		t.Fatalf("repaired selection remained unknown: %d %s", code, result.Summary)
	}
}

func TestProcessCheckPublicRechecksAppliedArgv(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed-agent", true: "longer-person"}[person], func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			b.declaredCheap = "make d"
			owners := b.workOwners()
			argv := []string{"rm", "-rf", "/tmp/zzz"}
			if person {
				argv = []string{"go", "test", "./x", "-count=1"}
			}
			args := checkBuildArgs(b, "rechecked", b.brief("rechecked.md", "Build.\n"), argv)
			code, result := checkActBuild(t, b, owners, args...)
			pending := processAct(t, result)
			pendingRemedy := result.Next.Argv[1:]
			if code != 1 || pending.Status != "proposed" {
				t.Fatalf("initial agent proposal: %d %s", code, result.Summary)
			}
			// Retain an intervening observation and person admission for the
			// same operation before its pending public request is retried.
			c := processchange.Check{Root: b.root(), ProcessAct: pending, Observation: true, Now: b.manager.Now()}
			c.ID, c.Question, c.AfterArgv = "", "", []string{"make", "d"}
			if _, err := processchange.AdmitCheck(c); err != nil {
				t.Fatal(err)
			}
			c.Observation, c.Person, c.Actor, c.AfterArgv = false, true, "direct-person", []string{"go", "test", "./x"}
			applied, err := processchange.AdmitCheck(c)
			if err != nil || applied.Status != "applied" {
				t.Fatalf("intervening person admission: %+v %v", applied, err)
			}
			count := len(b.starter.launched())
			if person {
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				check := slices.Index(args, "--check")
				args = slices.Insert(args, check, "--reason", "Choose a longer check", "--by", "Wido")
			}
			code, result = checkActBuild(t, b, owners, args...)
			if person {
				if code != 0 || len(b.starter.launched()) <= count {
					t.Fatalf("person's longer argv was refused: %d %s", code, result.Summary)
				}
				plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
				if err != nil || plan.Check.ProcessAct == applied.ID || plan.Check.Cheap != shellCommand(argv) {
					t.Fatalf("person's new selection was not consumed: %+v %v", plan.Check, err)
				}
				acts := checkActs(t, b)
				if !slices.ContainsFunc(acts, func(act processchange.ProcessAct) bool {
					return act.ID == plan.Check.ProcessAct && act.Actor == "direct-person" && slices.Equal(act.BeforeArgv, []string{"go", "test", "./x"})
				}) {
					t.Fatalf("person's new act used an older proposal instead of the current admission: %+v", acts)
				}
			} else {
				if code != 1 || processAct(t, result).Status != "proposed" || len(b.starter.launched()) != count || !slices.Equal(argv, []string{"rm", "-rf", "/tmp/zzz"}) {
					t.Fatalf("changed agent check launched or rewrote argv: %d %s; argv %q", code, result.Summary, argv)
				}
				current := processAct(t, result)
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				code, stale := checkActBuild(t, b, owners, pendingRemedy...)
				if code != 1 || processAct(t, stale).Status != "superseded" || len(b.starter.launched()) != count {
					t.Fatalf("older same-operation proposal hid the current admission: %d %s", code, stale.Summary)
				}
				code, repaired := checkActBuild(t, b, owners, stale.Next.Argv[1:]...)
				if code != 0 {
					t.Fatalf("same-operation superseded remedy: %d %s", code, repaired.Summary)
				}
				checkActLaunches(t, b, repaired, current.ID)
			}
		})
	}
}

// An agent naming the declared check with --check would take the person's
// no-audits repair; it runs with the declared audits and deadline instead.
func TestProcessCheckAgentDeclaredCheckKeepsAudits(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap, b.declaredAudits, b.declaredDeadline = "make d", "make audits", "40"
	owners := b.workOwners()
	args := checkBuildArgs(b, "declared", b.brief("declared.md", "Build.\n"), []string{"make", "d"})
	code, result := checkActBuild(t, b, owners, args...)
	if code != 0 {
		t.Fatalf("agent's --check of the declared command: %d %s", code, result.Summary)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || plan.Check.Audits != "make audits" || plan.Check.Minutes != 40 {
		t.Fatalf("agent's --check of the declared command dropped its audits or deadline: %+v %v", plan.Check, err)
	}
}

// A person's applied selection does not carry the no-audits repair to an
// agent's later round with the same argv: committed declarations apply.
func TestProcessCheckAgentReusingAppliedActKeepsAudits(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap, b.declaredAudits, b.declaredDeadline = "make d", "make audits", "40"
	owners := b.workOwners()
	argv := []string{"go", "test", "./x"}
	args := checkBuildArgs(b, "reused", b.brief("reused.md", "Build.\n"), argv)
	code, result := checkActBuild(t, b, owners, args...)
	pending := processAct(t, result)
	if code != 1 || pending.Status != "proposed" {
		t.Fatalf("agent proposal: %d %s", code, result.Summary)
	}
	c := processchange.Check{Root: b.root(), ProcessAct: pending, Person: true, Now: b.manager.Now()}
	c.ID, c.Question, c.Actor, c.AfterArgv = "", "", "direct-person", argv
	if applied, err := processchange.AdmitCheck(c); err != nil || applied.Status != "applied" {
		t.Fatalf("person admission: %+v %v", applied, err)
	}
	code, result = checkActBuild(t, b, owners, args...)
	if code != 0 {
		t.Fatalf("agent's round with the applied argv: %d %s", code, result.Summary)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || plan.Check.Audits != "make audits" || plan.Check.Minutes != 40 {
		t.Fatalf("agent reused the person's no-audits repair: %+v %v", plan.Check, err)
	}
}
