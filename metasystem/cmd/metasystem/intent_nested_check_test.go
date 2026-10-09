package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestNestedCheckPublicCostReport(t *testing.T) {
	t.Parallel()
	script := filepath.Join(t.TempDir(), "check")
	// Each fake child reads the actual durable start before returning.
	b := declaredCheckBed(t, "proof.cheap="+shellCommand([]string{script, "child"})+"\nproof.audits=true\nproof.deadline=15\n")
	text := "#!/bin/sh\nfound=no\nfor file in '" + b.unitRoot + "'/*/round-*/*/observation.json '" + b.worktree + "'/artifacts/unit-checks/*/round-*/*/observation.json; do\n if /usr/bin/grep -q '\"End\": \"\"' \"$file\" 2>/dev/null; then found=yes; fi\ndone\ntest \"$found\" = yes || exit 42\nprintf checked\n"
	if err := testexec.WriteFile(script, []byte(text), 0700); err != nil {
		t.Fatal(err)
	}
	hook := b.workOwnersHook
	b.workOwnersHook = func(w *intentWorkOwners) {
		hook(w)
		git := w.git
		w.git = func(root string, args ...string) ([]byte, error) {
			if slices.Equal(args, []string{"rev-parse", "origin/main"}) || args[0] == "merge-base" {
				return []byte("seed-nested"), nil
			}
			if slices.Equal(args, []string{"rev-parse", "seed-nested^{tree}"}) {
				return []byte("seed-tree"), nil
			}
			if args[0] == "show" && strings.HasPrefix(args[1], "seed-nested:") {
				return []byte("proof.cheap=true\nproof.audits=true\nproof.deadline=15\nproof.full=printf full\n"), nil
			}
			data, err := git(root, args...)
			if err == nil && args[0] == "show" {
				data = []byte(strings.ReplaceAll(string(data), "proof.full=true", "proof.full=printf full"))
			}
			return data, err
		}
	}
	start := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	clock := &workClock{now: start}
	b.manager.Now = clock.Now
	b.manager.Prober = identity.KernelProber{}
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	var observations []launch.CheckExecution
	starter.beforeCheck = func(record launch.Record) func() {
		exact, live, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || live != identity.Alive {
			t.Fatal(err, live)
		}
		ref := exact.Ref()
		if _, err := b.manager.Store.Update(record.ID, func(r *launch.Record) error { r.Child = &ref; r.State = launch.Running; return nil }); err != nil {
			t.Fatal(err)
		}
		// Clocks supply retained intervals without real waits; the shell still runs.
		times := []int{2, 5, 4, 6}
		if record.Kind == "proof" {
			times = []int{10, 10, 10, 10}
		}
		calls := 0
		b.manager.Now = func() time.Time {
			index := min(calls, len(times)-1)
			calls++
			return start.Add(time.Duration(times[index]) * time.Minute)
		}
		return func() {
			b.manager.Now = clock.Now
			clock.mu.Lock()
			clock.now = start.Add(10 * time.Minute)
			clock.mu.Unlock()
			if _, err := b.manager.Store.Update(record.ID, func(r *launch.Record) error {
				r.StartedAt = start.Format(time.RFC3339Nano)
				if record.Kind == "proof" {
					r.StartedAt = clock.Now().Format(time.RFC3339Nano)
				}
				r.FinishedAt = clock.Now().Format(time.RFC3339Nano)
				if record.Kind == "build" {
					r.Measurement = launch.Measurement{UsageRead: true, InputTokens: 100}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	code, built, out := b.work("work", "build", b.id, "nested", "--brief", b.brief("nested.md", "Build.\n"), "--lines", "100")
	if code != 1 || processAct(t, built).Status != "proposed" {
		t.Fatalf("declaration admission: %d %+v %s", code, built, out)
	}
	actID := processAct(t, built).ID
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), clock.Now())
	code, built = b.runJSON(owners, built.Next.Argv[1:]...)
	if code != 0 {
		t.Fatalf("person remedy: %d %s", code, built.Summary)
	}
	run := resultData(t, built)["run"].(string)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	round := record.Rounds[0]
	observations, unknown := launch.ReadCheckExecutions(round.Directory)
	if len(unknown) != 0 || len(observations) != 2 {
		t.Fatalf("observations: %+v %v", observations, unknown)
	}
	for _, o := range observations {
		if o.Run != run || o.Goal != b.id || o.Round != filepath.Base(round.Directory) || o.BriefSHA256 == "" || o.SourceSHA256 == "" || len(o.Steps) != 2 {
			t.Fatalf("context: %+v", o)
		}
		for _, step := range o.Steps {
			if step.Act != actID || step.Parent == "" || !step.Terminal || step.Start == "" || step.End == "" || step.Outcome != "passed" {
				t.Fatalf("lost executor boundaries: %+v", step)
			}
		}
	}
	status := func() processmeasure.Projection {
		t.Helper()
		owners := b.workOwners()
		owners.processes.launches = func() *launch.Manager { return b.manager }
		code, result := b.runJSON(owners, "work", "status", "run:"+run)
		if code != 0 {
			t.Fatalf("status: %d %+v", code, result)
		}
		body, _ := json.Marshal(resultData(t, result)["processReport"])
		var p processmeasure.Projection
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	assertMinutes := func(m processmeasure.Measures, build, check float64) {
		t.Helper()
		if m.Hours["build"] == nil || math.Abs(*m.Hours["build"]*60-build) > 1e-8 || m.Hours["attest"] == nil || math.Abs(*m.Hours["attest"]*60-check) > 1e-8 || m.ElapsedHours == nil || *m.ElapsedHours*60 != 10 {
			t.Fatalf("exclusive/check/elapsed: %+v", m)
		}
	}
	p := status()
	assertMinutes(*p.Measures, 6, 5)
	if p.Measures.Tokens == nil || p.Measures.Tokens.Input != 100 || len(p.Measures.Children) != 4 {
		t.Fatalf("parent tokens/children: %+v", p.Measures)
	}
	if !reflect.DeepEqual(p.Measures, status().Measures) {
		t.Fatal("replayed observations changed cost")
	}
	// Serialized child usage is independent evidence, with explicit inclusion.
	var child launch.CheckExecution
	var childPath string
	for _, o := range observations {
		if o.Role == "builder" {
			child = o
			childPath = filepath.Join(round.Directory, "builder-"+o.ExecutionID, "observation.json")
		}
	}
	if childPath == "" {
		t.Fatal("builder custody was not joined")
	}
	child.Steps[0].Usage = &processmeasure.Tokens{Input: 30}
	child.Steps[0].Coverage = "parent"
	write := func() {
		t.Helper()
		body, _ := json.Marshal(child)
		if err := os.WriteFile(childPath, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	p = status()
	if !strings.Contains(strings.Join(p.Lines, "\n"), "tokens {30 0 0 0}; token inclusion parent") {
		t.Fatal("reported child usage was hidden")
	}
	if p.Measures.Tokens == nil || p.Measures.Tokens.Input != 100 {
		t.Fatalf("inclusive child counted twice: %+v", p.Measures)
	}
	child.Steps[0].Coverage = "unknown"
	write()
	p = status()
	if p.Measures.Tokens != nil || !strings.Contains(strings.Join(p.Lines, "\n"), "token inclusion unavailable") {
		t.Fatalf("unknown inclusion became additive: %+v", p)
	}
	// Actual consumption, rather than an act's date, establishes cost attribution.
	write()
	seed := filepath.Join(b.root(), "process", "acts", actID+".json")
	acts := checkActs(t, b)
	if len(acts) != 1 || acts[0].ID != actID {
		t.Fatalf("consumed act unavailable: %+v", acts)
	}
	act := acts[0]
	act.Actor = "direct-person"
	act.AppliedProof.Helm = &humanauthority.HelmGrant{By: "Wido", Grant: "grant-42"}
	body, _ := json.Marshal(act)
	if err := os.WriteFile(seed, body, 0600); err != nil {
		t.Fatal(err)
	}
	p = status()
	if len(p.Acts) != 1 || !reflect.DeepEqual(p.Acts[0].Requester, act.Proof) || p.Acts[0].Applier.Helm == nil || p.Acts[0].Applier.Outcome != humanauthority.OutcomeProven {
		t.Fatalf("requester/applier provenance missing: %+v", p.Acts)
	}
	lines := strings.Join(p.Lines, "\n")
	for _, fragment := range []string{"own process cost", "actor agent", "grant grant-42", "observed 11 minutes", "counterfactual unknown"} {
		if !strings.Contains(lines, fragment) {
			t.Fatalf("missing %q: %s", fragment, lines)
		}
	}
	replay := processmeasure.Report(*p.Measures, append(child.Steps, child.Steps...), []processmeasure.Act{{ID: actID, Actor: "agent"}}, nil, p.Watermark)
	if !strings.Contains(strings.Join(replay.Lines, "\n"), "observed 0 minutes") {
		t.Fatal("report replay counted the child twice")
	}
	// Exact frozen argv is the only full-suite evidence, whatever its name.
	child.Steps[0].FullArgv = slices.Clone(child.Steps[0].Argv)
	write()
	p = status()
	if p.Measures.SuiteMinutes == nil || *p.Measures.SuiteMinutes != 3 {
		t.Fatalf("exact full argv: %+v", p.Measures)
	}
	child.Steps[0].FullArgv = []string{"/bin/sh", "-c", "full"}
	write()
	p = status()
	if p.Measures.SuiteMinutes == nil || *p.Measures.SuiteMinutes != 0 {
		t.Fatalf("suite inferred by name: %+v", p.Measures)
	}
	// A newer unfinished physical execution cannot inherit any old end.
	child.ExecutionID = "check-new"
	child.Steps = child.Steps[:1]
	child.Steps[0].ID = "check-new:cheap"
	child.Steps[0].Start = start.Add(8 * time.Minute).Format(time.RFC3339Nano)
	child.Steps[0].End = ""
	child.Steps[0].Terminal = false
	child.Steps[0].Outcome = ""
	childPath = filepath.Join(round.Directory, child.ExecutionID, "observation.json")
	if err := os.MkdirAll(filepath.Dir(childPath), 0700); err != nil {
		t.Fatal(err)
	}
	write()
	p = status()
	assertMinutes(*p.Measures, 4, 7)
	if !p.Measures.WorkLowerBound {
		t.Fatal("unfinished child borrowed completion")
	}
	// A child beginning before the parent contributes only its clipped interval.
	child.ExecutionID = "check-clipped"
	child.Steps[0].ID = "check-clipped:cheap"
	child.Steps[0].Start = start.Add(-2 * time.Minute).Format(time.RFC3339Nano)
	child.Steps[0].End = start.Add(3 * time.Minute).Format(time.RFC3339Nano)
	child.Steps[0].Terminal = true
	childPath = filepath.Join(round.Directory, child.ExecutionID, "observation.json")
	if err := os.MkdirAll(filepath.Dir(childPath), 0700); err != nil {
		t.Fatal(err)
	}
	write()
	p = status()
	assertMinutes(*p.Measures, 2, 12)
	if err := os.RemoveAll(filepath.Dir(childPath)); err != nil {
		t.Fatal(err)
	}
	// A label without retained native child custody cannot supply a parent.
	if code, result := declaredCheckAt(t, b, b.worktree, "build", "test", "run", "--unit-run", run); code != 0 {
		t.Fatalf("raw public check: %d %+v", code, result)
	}
	p = status()
	if !strings.Contains(strings.Join(p.Lines, "\n"), "check parent unavailable") {
		t.Fatal("kind label manufactured custody")
	}
	t.Run("manual-full", func(t *testing.T) {
		t.Parallel()
		b := declaredCheckBed(t, "proof.cheap=printf declared\nproof.audits=true\nproof.deadline=15\n")
		hook := b.workOwnersHook
		b.workOwnersHook = func(w *intentWorkOwners) {
			hook(w)
			git := w.git
			w.git = func(root string, args ...string) ([]byte, error) {
				data, err := git(root, args...)
				if err == nil && args[0] == "show" {
					data = []byte(strings.ReplaceAll(string(data), "proof.full=true", "proof.full=printf full"))
				}
				return data, err
			}
		}
		s := &declaredCheckStarter{bed: b, t: t}
		b.manager.Supervisor = s
		owners := b.workOwners()
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		code, result := checkActBuild(t, b, owners, "work", "build", b.id, "manual", "--brief", b.brief("manual.md", "Build.\n"), "--lines", "5", "--reason", "Run full once", "--by", "Wido", "--check", "printf", "full")
		if code != 0 {
			t.Fatalf("manual full admission: %d %s", code, result.Summary)
		}
		obs, unknown := launch.ReadCheckExecutions(resultData(t, result)["directory"].(string))
		if len(obs) != 2 || len(unknown) != 0 {
			t.Fatalf("manual execution: %+v %v", obs, unknown)
		}
		for _, o := range obs {
			if !slices.Equal(o.Steps[0].Argv, o.Steps[0].FullArgv) || o.Steps[0].Act == "" {
				t.Fatalf("manual full classification/act lost: %+v", o)
			}
		}
	})
	t.Run("carry", func(t *testing.T) {
		t.Parallel()
		b, owners, args, _ := carryRecoveryBed(t)
		if code, result := b.runJSON(owners, args...); code != 0 {
			t.Fatalf("carry: %d %+v", code, result)
		}
		root := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
		observations, unknown := launch.ReadCheckExecutions(root)
		if len(unknown) != 0 || len(observations) != 1 || observations[0].Role != "carry" || observations[0].Goal != b.id || observations[0].Steps[0].Parent != "" || observations[0].Steps[0].Act == "" {
			t.Fatalf("carry evidence: %+v %v", observations, unknown)
		}
		owners.processes.launches = func() *launch.Manager { return b.manager }
		if code, result := b.runJSON(owners, "work", "status", b.id); code != 0 {
			t.Fatalf("carry report: %d %+v", code, result)
		} else {
			raw, _ := json.Marshal(resultData(t, result)["processReport"])
			var p processmeasure.Projection
			_ = json.Unmarshal(raw, &p)
			if len(p.Measures.Children) != 2 {
				t.Fatalf("carry omitted by report: %+v", p)
			}
		}
		current := filepath.Join(root, "check-current")
		if err := os.MkdirAll(current, 0700); err != nil {
			t.Fatal(err)
		}
		partial := observations[0]
		partial.ExecutionID = "check-current"
		partial.Steps = slices.Clone(partial.Steps[:1])
		started, err := time.Parse(time.RFC3339Nano, partial.Steps[0].Start)
		if err != nil {
			t.Fatal(err)
		}
		partial.Steps[0].ID = "check-current:cheap"
		partial.Steps[0].Start = started.Add(time.Minute).Format(time.RFC3339Nano)
		partial.Steps[0].End, partial.Steps[0].Terminal = "", false
		body, _ := json.Marshal(partial)
		if err := os.WriteFile(filepath.Join(current, "observation.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(current)
		if err != nil {
			t.Fatal(err)
		}
		later := info.ModTime().Add(time.Hour)
		if err := os.Chtimes(filepath.Join(root, observations[0].ExecutionID), later, later); err != nil {
			t.Fatal(err)
		}
		clock := &workClock{now: started.Add(2 * time.Minute)}
		b.manager.Now = clock.Now
		owners.prove = fixedFixtureGoalAuthority
		if code, result := b.runJSON(owners, args...); code == 0 {
			t.Fatalf("older completion hid unfinished carry: %+v", result)
		}
	})
	for _, row := range []struct{ name, command, outcome string }{{"failure", "exit 23", "failed"}, {"cancellation", "kill -TERM $$", "cancelled"}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap="+row.command+"\nproof.audits=true\nproof.deadline=15\n")
			s := &declaredCheckStarter{bed: b, t: t}
			b.manager.Supervisor = s
			code, result, _ := b.work("work", "build", b.id, row.name, "--brief", b.brief(row.name+".md", "Build.\n"), "--lines", "100")
			if code != 1 {
				t.Fatalf("failed child turned green: %+v", result)
			}
			run := resultData(t, result)["run"].(string)
			record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			if err != nil {
				t.Fatal(err)
			}
			obs, unknown := launch.ReadCheckExecutions(record.Rounds[0].Directory)
			if len(unknown) != 0 || len(obs) != 2 {
				t.Fatalf("failed evidence: %+v %v", obs, unknown)
			}
			for _, o := range obs {
				if len(o.Steps) != 2 || o.Steps[0].Outcome != row.outcome || !o.Steps[0].Terminal || o.Steps[0].End == "" {
					t.Fatalf("failure lost: %+v", o)
				}
			}
		})
	}
}
