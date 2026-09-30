package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

// The layout goldens of group G1a (status and helm): the process verbs,
// work status, questions and the helm, each driven in-process on its own
// bed.
func g1aLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "system-status", args: []string{"system", "status"}, bed: statusLayoutBed(false)},
		{name: "system-start", args: []string{"system", "start"}, bed: processLayoutBed()},
		{name: "system-start-again", args: []string{"system", "start"}, bed: processLayoutBed("system", "start")},
		{name: "system-start-refusal", args: []string{"system", "start", "--if-down", "--temporary-human-word", "yes"}, bed: processLayoutBed()},
		{name: "system-stop", args: []string{"system", "stop"}, bed: processLayoutBed("system", "start")},
		{name: "system-stop-again", args: []string{"system", "stop"}, bed: processLayoutBed("system", "stop")},
		{name: "system-restart", args: []string{"system", "restart"}, bed: processLayoutBed("system", "start")},
		{name: "work-status-empty", args: []string{"work", "status"}, bed: processLayoutBed()},
		{name: "work-status", args: []string{"work", "status"}, bed: jobsLayoutBed},
		{name: "work-status-all", args: []string{"work", "status", "--all"}, bed: jobsLayoutBed},
		{name: "work-stop", args: []string{"work", "stop", "j2:design-r2-4f1c"}, bed: jobsLayoutBed},
		{name: "work-stop-already", args: []string{"work", "stop", "j2:impl-01"}, bed: jobsLayoutBed},
		{name: "work-stop-launch-already", args: []string{"work", "stop", "j1:build-trial-91e2"}, bed: jobsLayoutBed},
		{name: "work-stop-refusal", args: []string{"work", "stop"}, bed: jobsLayoutBed},
		{name: "system-setup", args: []string{"system", "setup"}, bed: setupLayoutBed(false)},
		{name: "system-setup-again", args: []string{"system", "setup"}, bed: setupLayoutBed(true)},
		{name: "system-setup-refusal", args: []string{"system", "setup", "--runtimes", ""}, bed: setupLayoutBed(false)},
		{name: "system-check", args: []string{"system", "check"}, bed: checkLayoutBed, measured: "each health role's durationMillis"},
		{name: "question-list-empty", args: []string{"question", "list"}, bed: processLayoutBed()},
		{name: "question-list", args: []string{"question", "list"}, bed: questionsLayoutBed},
		{name: "helm-take", args: []string{"helm", "take", "--reason", "coordinating the verb batches"}, bed: helmLayoutBed(true)},
		{name: "helm-take-enrolls", args: []string{"helm", "take", "--reason", "coordinating the verb batches"}, bed: helmLayoutBed(false)},
		{name: "helm-take-again", args: []string{"helm", "take", "--reason", "coordinating the verb batches"}, bed: helmLayoutBed(true, "helm", "take", "--reason", "coordinating the verb batches")},
		{name: "helm-take-refusal", args: []string{"helm", "take"}, bed: helmLayoutBed(true)},
	}
}

// helmLayoutBed is a seat whose person runs zsh at the terminal login leads,
// enrolled or not, at the helm tests' clock, after the acts given.
func helmLayoutBed(enrolled bool, before ...string) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newHelmBed(t, 20, enrolled)
		b.owners.helm.zone = layoutZone(t)
		if len(before) > 0 {
			if code, out := b.run(before...); code != 0 {
				t.Fatalf("%v = %d %s", before, code, out)
			}
		}
		root := realpath.Resolve(b.root)
		return layoutBed{owners: b.owners, cwd: b.root, now: helmNow, replace: layoutPaths(b.root, root, "/Users/wido/GitHub/agentic-tools-m1e")}
	}
}

// processLayoutBed is a checkout named m1e with nothing running, its clock
// at the goldens' time, after the acts given (a start before a stop).
func processLayoutBed(before ...string) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b, owners := processLayoutOwners(t)
		b.helpersRun = true
		if len(before) > 0 {
			if code, stdout, stderr := b.run(owners, before...); code != 0 {
				t.Fatalf("%v = %d %s%s", before, code, stdout, stderr)
			}
		}
		root := realpath.Resolve(b.root())
		return layoutBed{owners: owners, cwd: b.root(), replace: processLayoutPaths(b, root)}
	}
}

// jobsLayoutBed is that checkout with a review launch and a builder job
// running, and a launch and a job that ended.
func jobsLayoutBed(t *testing.T) layoutBed {
	b, owners := processLayoutOwners(t)
	store := launch.Store{Root: b.launchDir}
	started := layoutNow.Add(-12 * time.Minute).Format(time.RFC3339)
	for _, record := range []launch.Record{
		{ID: "review-verbs-3a7c", Kind: "review", Goal: "verbs-match-intent", State: launch.Running, StartedAt: started},
		{ID: "build-trial-91e2", Kind: "build", Goal: "switch-on-trial", State: launch.Completed, StartedAt: layoutNow.Add(-3 * time.Hour).Format(time.RFC3339)},
	} {
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	jobs := filepath.Join(b.root(), "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, record := range map[string]string{
		"design-r2-4f1c": `{"status":"running","role":"builder","goalId":"verbs-match-intent","startedAt":"` + started + `"}`,
		"impl-01":        `{"status":"completed","role":"implementer","goalId":"switch-on-trial","startedAt":"2026-09-29T15:02:00Z"}`,
	} {
		if err := os.WriteFile(filepath.Join(jobs, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	owners.processes.cancelDispatch = func(string, string) (map[string]any, int, error) {
		return map[string]any{"outcome": "CANCELLED"}, 0, nil
	}
	root := realpath.Resolve(b.root())
	return layoutBed{owners: owners, cwd: b.root(), replace: processLayoutPaths(b, root)}
}

// questionsLayoutBed is that checkout with two channel questions open and
// one answered.
func questionsLayoutBed(t *testing.T) layoutBed {
	b, owners := processLayoutOwners(t)
	dir := filepath.Join(b.root(), "artifacts", "agents", "channel", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, record := range map[string]string{
		"q-20260930-1": `{"id":"q-20260930-1","goal":"verbs-match-intent","openedAt":"2026-09-30T08:31:00Z","state":"open","facts":["Land slice 2 of the verb redesign now, or wait for the second read?"]}`,
		"q-20260929-4": `{"id":"q-20260929-4","goal":"switch-on-trial","openedAt":"2026-09-29T19:05:00Z","state":"open","facts":["The trial run found one flaky test in the landing lane; quarantine it and continue, or stop the trial until it is fixed?"]}`,
		"q-20260929-2": `{"id":"q-20260929-2","goal":"switch-on-trial","openedAt":"2026-09-29T11:00:00Z","state":"answered","facts":["Start the trial?"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := realpath.Resolve(b.root())
	return layoutBed{owners: owners, cwd: b.root(), replace: processLayoutPaths(b, root)}
}

// processLayoutPaths replaces the bed's checkout and its home, which holds
// the default evidence root.
func processLayoutPaths(b *processBed, root string) []string {
	home := realpath.Resolve(b.home)
	return layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e", home, "/Users/wido", b.home, "/Users/wido")
}

// setupLayoutBed is a real Git checkout from before the engine guard, its
// Claude settings running the old stub, set up once already when again.
func setupLayoutBed(again bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := newSystemSetupBed(t)
		if again {
			if code, result := bed.run(t); code != 0 {
				t.Fatalf("first setup = %d %+v", code, result)
			}
		}
		owners := intentOwners{resolver: stateroot.NewResolver(stateroot.RepositoryTop, noExecutable)}
		return layoutBed{owners: owners, cwd: bed.repo, replace: layoutPaths(bed.repo, bed.repo, "/Users/wido/GitHub/agentic-tools-m1e")}
	}
}

// checkLayoutBed is that checkout as a Git repository with its skills
// folder, its machinery never started.
func checkLayoutBed(t *testing.T) layoutBed {
	b, owners := processLayoutOwners(t)
	if output, err := systemSetupGit(t, b.root(), "init", "-q"); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(b.root(), "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) (string, error) { return systemSetupGit(t, b.root(), args...) }
	if _, err := contractgit.Register(b.root(), "testing.json", filepath.Join(realpath.Resolve(b.root()), "bin", "metasystem"), git); err != nil {
		t.Fatal(err)
	}
	root := realpath.Resolve(b.root())
	return layoutBed{owners: owners, cwd: b.root(), replace: processLayoutPaths(b, root)}
}

func processLayoutOwners(t *testing.T) (*processBed, intentOwners) {
	b := newProcessBed(t)
	owners := b.owners()
	owners.helm.machine = func(string) (string, error) { return "m1e", nil }
	owners.helm.zone = layoutZone(t)
	owners.commandNow = func(string) (time.Time, error) { return layoutNow, nil }
	transition := owners.processes.process.transition
	owners.processes.process.transition = func(scope processScope, scale int) *stoptransition.Transition {
		made := transition(scope, scale)
		made.Now = func() time.Time { return layoutNow.Add(-5 * time.Minute) }
		return made
	}
	return b, owners
}
