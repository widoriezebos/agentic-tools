package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLaneMergeBuildCollectsOneJobOnBatchTree(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	layout, err := b.owners().resolver.ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	root, checkout := layout.InstallationRoot.Path(), layout.GitRoot
	pid := int64(os.Getpid())
	exact, _, err := (identity.KernelProber{}).Probe(pid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lease.AnnounceWithPair(root, "merge-session", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "merge-session", "metasystem", lane.AgentLineage); err != nil {
		t.Fatal(err)
	}
	if holder, err := lease.RequireHolder(root, pid, nil); err != nil || !holder.Holder {
		t.Fatalf("holder=%+v %v", holder, err)
	}
	home := t.TempDir()
	registered := lane.Record{Root: checkout, Install: root, CustodyEpoch: 1}
	data, _ := json.Marshal(registered)
	if err := os.MkdirAll(filepath.Dir(lane.RecordPath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	fix := &plain.Fix{Goal: b.id, Units: []string{"lane-merge-1"}, Commit: "batch", Tip: "member", State: "resolving", Attempt: "merge-attempt", Paths: []conflict.Path{{Path: "source.go", Class: conflict.Builder}}}
	if err := plain.WriteFix(root, fix); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(root, plain.Line{Goal: b.id, SHA: "member"}); err != nil {
		t.Fatal(err)
	}
	brief := b.brief("merge.md", "| Unit | Lines |\n| --- | ---: |\n| lane-merge-1 | 5 |\nResolve both sides.\n")
	message := filepath.Join(t.TempDir(), "MERGE_MSG")
	if err := os.WriteFile(message, []byte("Merge member into batch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	owners.landing.home = func() (string, error) { return home, nil }
	head, commits := "batch", 0
	owners.landing.plainResolve = plain.ResolveSeams{
		Git: func(dir string, args ...string) (string, error) {
			if dir != checkout {
				t.Fatalf("merge left lane checkout: %s", dir)
			}
			switch strings.Join(args, " ") {
			case "rev-parse --verify HEAD^{commit}":
				return head, nil
			case "rev-parse --verify MERGE_HEAD^{commit}":
				return "member", nil
			case "diff --binary AUTO_MERGE --":
				return "diff --git a/source.go b/source.go\n+resolved\n", nil
			case "rev-parse --path-format=absolute --git-path MERGE_MSG":
				return message, nil
			case "diff --name-only --diff-filter=U -z", "diff --quiet --", "grep --cached -n -e ^<<<<<<<  -e ^======= -e ^>>>>>>>  -e ^||||||| -- :(literal)source.go":
				return "", nil
			}
			if args[0] == "commit" && args[1] == "-m" && args[2] == "Merge member into batch\n\nGoal-Unit: "+b.id+"/lane-merge-1" {
				commits++
				head = "merge-commit"
				return "", nil
			}
			t.Fatalf("unexpected merge call: %v", args)
			return "", nil
		}, Run: func(argv []string, dir string, log *os.File, started func(int64) error) error {
			if strings.Join(argv, " ") != "env LANDING_PROOF_BASE=batch /bin/sh -c metasystem test impact --check" || dir != root {
				t.Fatalf("check=%v cwd=%s", argv, dir)
			}
			return started(0)
		},
	}
	b.starter.hold = "build"
	command, _ := findIntentAction("work", "build")
	lastOutput := ""
	invoke := func() int {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, []string{b.id, "--work", "lane-merge-1", "--brief", brief, "--check", "metasystem test impact"}, &stdout, &stderr, checkout, owners)
		lastOutput = stdout.String() + stderr.String()
		t.Logf("exit=%d %s", code, lastOutput)
		return code
	}
	for range 2 {
		if code := invoke(); code != 0 {
			t.Fatalf("build exit=%d", code)
		}
	}
	if jobs := b.starter.launched(); len(jobs) != 1 || jobs[0] != "build" || commits != 0 {
		t.Fatalf("jobs=%v commits=%d", jobs, commits)
	}
	fix, err = plain.ReadFix(root)
	if err != nil || fix == nil {
		t.Fatalf("fix=%+v %v", fix, err)
	}
	if _, err := b.manager.Store.Update(fix.Job, func(record *launch.Record) error { record.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	if code := invoke(); code != 0 {
		t.Fatalf("collection exit=%d", code)
	}
	if code := invoke(); code != 0 {
		t.Fatalf("repeat exit=%d", code)
	}
	if commits != 1 || len(b.starter.launched()) != 1 {
		t.Fatalf("repeated build jobs=%v commits=%d", b.starter.launched(), commits)
	}
	fix, err = plain.ReadFix(root)
	if err != nil || fix == nil || fix.State != "reviewing" || fix.Commit != "merge-commit" {
		t.Fatalf("collected=%+v %v", fix, err)
	}
	resolutionPatch := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".patch")
	if _, err := os.ReadFile(resolutionPatch); err != nil {
		t.Fatal(err)
	}
	brief = fix.Brief
	// Exercise the read with a real merge whose resolution diff cannot apply to its first parent.
	merge, handResolution, contextDiff := laneMergeReadCommit(t, checkout)
	fix.Commit = merge
	if err := plain.WriteFix(root, fix); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolutionPatch, handResolution, 0o600); err != nil {
		t.Fatal(err)
	}
	resolution := handResolution
	normalGit := owners.work.git
	owners.work.git = func(dir string, args ...string) ([]byte, error) { return (launch.OSGitRunner{}).Run(dir, nil, args...) }
	units := owners.work.units
	owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		runner := units(layout)
		runner.Git = launch.OSGitRunner{}
		return runner
	}
	review, _ := findIntentAction("work", "review")
	reviewMerge := func() (int, string) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(review, []string{b.id, "--work", "lane-merge-1"}, &stdout, &stderr, checkout, owners)
		return code, stdout.String() + stderr.String()
	}
	// A read error must preserve the committed resolution and offer the same review again.
	for _, missing := range []string{brief, resolutionPatch} {
		if err := os.Rename(missing, missing+".saved"); err != nil {
			t.Fatal(err)
		}
		if code, output := reviewMerge(); code != 1 || !strings.Contains(output, "work review") {
			t.Fatalf("failed read exit=%d %s", code, output)
		}
		fix, err = plain.ReadFix(root)
		if err != nil || fix == nil || fix.State != "resolved" || !strings.Contains(fix.Reason, missing) || fix.Commit != merge {
			t.Fatalf("failed read left no retryable resolution: %+v %v", fix, err)
		}
		if _, _, err := plain.Start(root, checkout, plain.ProveSeams{}); err == nil || !strings.Contains(err.Error(), fix.Reason) {
			t.Fatalf("proof refusal lost the read error: %v", err)
		}
		if err := os.Rename(missing+".saved", missing); err != nil {
			t.Fatal(err)
		}
	}
	b.starter.hold = "read"
	if code, output := reviewMerge(); code != 124 {
		t.Fatalf("resolution read exit=%d %s", code, output)
	}
	requests, err := filepath.Glob(filepath.Join(b.unitRoot, ".reads", "*", "request.json"))
	if err != nil || len(requests) != 1 {
		t.Fatalf("read requests=%v %v", requests, err)
	}
	data, err = os.ReadFile(requests[0])
	var request launch.ReadRequestRecord
	if err != nil || json.Unmarshal(data, &request) != nil {
		t.Fatalf("request=%s %v", data, err)
	}
	if request.Kind != "commit" || request.Base != merge || request.DiffSHA256 != fmt.Sprintf("%x", sha256.Sum256(contextDiff)) || len(request.Inputs) != 2 {
		t.Fatalf("read did not select the merge tree with both reference diffs: %+v", request)
	}
	frozenBrief, err := os.ReadFile(request.Brief)
	if err != nil || !strings.Contains(string(frozenBrief), "INPUT merge-attempt.patch") || !strings.Contains(string(frozenBrief), "INPUT merge-attempt.context.patch") {
		t.Fatalf("read brief does not name both inputs: %s %v", frozenBrief, err)
	}
	for index, want := range [][]byte{resolution, contextDiff} {
		if data, err := os.ReadFile(request.Inputs[index].Path); err != nil || !bytes.Equal(data, want) {
			t.Fatalf("read input %d=%s %v", index, data, err)
		}
	}
	runner := owners.work.units(layout)
	read, err := runner.InspectRead(request.Ref)
	if err != nil || read.Attempt.Limitation != "" || read.Attempt.Checkout == "" {
		t.Fatalf("real standalone read context=%+v %v", read, err)
	}
	for _, name := range strings.Split(connectionGit(t, checkout, "ls-tree", "-r", "--name-only", merge), "\n") {
		want, err := (launch.OSGitRunner{}).Run(checkout, nil, "show", merge+":"+name)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(read.Attempt.Checkout, name)); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("read checkout differs from merge at %s: %q want %q: %v", name, got, want, err)
		}
	}
	privateIndex := filepath.Join(t.TempDir(), "read.index")
	environment := []string{"GIT_INDEX_FILE=" + privateIndex, "GIT_DIR=" + filepath.Join(checkout, ".git"), "GIT_WORK_TREE=" + read.Attempt.Checkout}
	for _, args := range [][]string{{"read-tree", "--empty"}, {"add", "-A", "--", "."}} {
		if _, err := (launch.OSGitRunner{}).Run(read.Attempt.Checkout, environment, args...); err != nil {
			t.Fatal(err)
		}
	}
	readTree, err := (launch.OSGitRunner{}).Run(read.Attempt.Checkout, environment, "write-tree")
	mergeTree := connectionGit(t, checkout, "rev-parse", merge+"^{tree}")
	if err != nil || strings.TrimSpace(string(readTree)) != mergeTree {
		t.Fatalf("read tree=%s merge tree=%s: %v", readTree, mergeTree, err)
	}
	t.Logf("reader checkout tree equals merge tree %s; no context limitation", mergeTree)
	for _, report := range read.Reports {
		job, err := b.manager.Store.Read(report.Launch)
		if err != nil || strings.Contains(string(job.AdapterData["unitReadPacket"]), "Context limitation") {
			t.Fatalf("read launch limitation: %+v %v", job, err)
		}
		if _, err := b.manager.Store.Update(report.Launch, func(record *launch.Record) error {
			record.State, record.Reason = launch.Failed, "reader unavailable"
			record.Supervisor, record.Child = nil, nil
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if code, output := reviewMerge(); code != 1 {
		t.Fatalf("failed reader collection exit=%d %s", code, output)
	}
	fix, err = plain.ReadFix(root)
	if err != nil || fix == nil || fix.State != "resolved" || !strings.Contains(fix.Reason, "reader unavailable") || fix.Read != request.Ref {
		t.Fatalf("failed reader stayed reviewing: %+v %v", fix, err)
	}
	if _, _, err := plain.Start(root, checkout, plain.ProveSeams{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("proof refusal lost the reader failure: %v", err)
	}
	b.starter.hold = ""
	if code, output := reviewMerge(); code != 0 || !strings.Contains(output, "Reviewed the resolved merge") {
		t.Fatalf("read collection exit=%d %s", code, output)
	}
	if active, err := plain.ReadFix(root); err != nil || active != nil {
		t.Fatalf("read did not finish the repair: %+v %v", active, err)
	}
	if retried, err := runner.InspectRead(request.Ref); err != nil || !retried.Complete || retried.Attempt.Number != 2 {
		t.Fatalf("failed reader was not retried: %+v %v", retried, err)
	}
	if data, err := os.ReadFile(resolutionPatch); err != nil || !bytes.Equal(data, resolution) {
		t.Fatalf("read replaced hand resolution: %s %v", data, err)
	}
	fix = &plain.Fix{Goal: b.id, Units: []string{"lane-merge-1"}, Commit: merge, Tip: "member", Job: "build", Brief: brief, Attempt: "merge-attempt", Paths: []conflict.Path{{Path: "source.go", Class: conflict.Builder}}}
	// A disappeared merge closes the build's repair instead of repeating collection.
	fix.State = "resolving"
	if err := plain.WriteFix(root, fix); err != nil {
		t.Fatal(err)
	}
	missing := exec.Command("/bin/sh", "-c", "exit 128").Run()
	owners.landing.plainResolve.Git = func(string, ...string) (string, error) { return "", missing }
	if code := invoke(); code != 0 || !strings.Contains(lastOutput, "resolution was abandoned") {
		t.Fatalf("missing merge exit=%d output=%s", code, lastOutput)
	}
	if active, err := plain.ReadFix(root); err != nil || active != nil {
		t.Fatalf("abandoned build active=%+v %v", active, err)
	}
	owners.work.git = normalGit
	if code := invoke(); !strings.Contains(lastOutput, "continuing with the normal goal build") {
		t.Fatalf("inactive merge exit=%d output=%s", code, lastOutput)
	}

}

func TestSkillLaneConflictBuildPreservesBothSides(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../skills/landing-agent/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{"landing resolve --as-seat", "merge-brief.md", "paths by class", "<base>..origin/main -- PATH", "<base>..<goal tip> -- PATH", "keep both sides whole", "never drop a test", "never hand-edited", "ONE resolution job", "--work lane-merge-1", "Goal-Unit: GOAL/lane-merge-1", "metasystem work review GOAL", "metasystem landing prove", "job id"} {
		if !strings.Contains(string(data), words) {
			t.Errorf("skill omits %q", words)
		}
	}
	if strings.Contains(string(data), "never edit a conflicted file") {
		t.Fatal("skill forbids its resolution job")
	}
}

func laneMergeReadCommit(t *testing.T, checkout string) (string, []byte, []byte) {
	t.Helper()
	git := func(args ...string) string { return connectionGit(t, checkout, args...) }
	git("init", "-q", "-b", "main")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("config", "commit.gpgsign", "false")
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(checkout, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		git("add", "--", name)
	}
	write("source.go", "base\n")
	git("commit", "-qm", "base")
	git("checkout", "-qb", "goal")
	write("source.go", "goal\n")
	write("goal-only.go", "clean goal addition\n")
	git("commit", "-qm", "goal")
	git("checkout", "-q", "main")
	write("source.go", "main\n")
	git("commit", "-qm", "main")
	if err := exec.Command("git", "-C", checkout, "merge", "--no-commit", "goal").Run(); err == nil {
		t.Fatal("fixture merge did not conflict")
	}
	write("source.go", "main\ngoal\n")
	patch, err := (launch.OSGitRunner{}).Run(checkout, nil, "diff", "--binary", "AUTO_MERGE", "--")
	if err != nil {
		t.Fatal(err)
	}
	git("commit", "-qm", "resolved merge")
	merge := git("rev-parse", "HEAD")
	diff, err := (launch.OSGitRunner{}).Run(checkout, nil, "diff", "--binary", merge+"^", merge)
	if err != nil {
		t.Fatal(err)
	}
	return merge, patch, diff
}
