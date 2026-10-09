package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingResolveBatchConflictWaitsThenReturnsAgainstMain(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	b.owners.landing.view = func(string) lane.View {
		return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "the landing lane is idle"}
	}
	for _, goal := range []string{"A", "B"} {
		if _, _, err := plain.HandIn(b.install, plain.Line{Goal: goal, SHA: goal + "-sha"}); err != nil {
			t.Fatal(err)
		}
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	trunk, head, aborts, probes := "main-sha", "batch-sha", 0, 0
	git := func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "ls-tree --name-only main-sha -- metasystem/plans/goals/trunk-red.json", "ls-tree --name-only A-sha -- metasystem/plans/goals/trunk-red.json":
			return "", nil
		case "show HEAD:metasystem/testing.json":
			return b.contract, nil
		case "diff --name-only --diff-filter=U -z":
			return "metasystem/src/list.go\x00", nil
		case "rev-parse --verify HEAD^{commit}":
			return head, nil
		case "rev-parse --verify refs/remotes/origin/main^{commit}", "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return trunk, nil
		case "rev-parse --verify MERGE_HEAD^{commit}":
			return "B-sha", nil
		case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z":
			return "", nil
		case "merge-tree --write-tree main-sha B-sha":
			probes++
			return "clean-tree", nil
		case "ls-files --unmerged -z -- metasystem/src/list.go":
			return "100644 base 1\tsrc\x00100644 main 2\tsrc\x00100644 goal 3\tsrc\x00", nil
		case "merge --abort":
			aborts++
			return "", nil
		}
		if args[0] == "cat-file" {
			return "", nil
		}
		if args[0] == "merge-base" {
			if args[2] == "A-sha" && (args[3] == "batch-sha" || args[3] == "A-sha") {
				return "", nil
			}
			return "", notAncestor
		}
		if args[0] == "diff" {
			return "@@ -2,0 +3 @@\n+insert\n", nil
		}
		t.Fatalf("unexpected Git: %v", args)
		return "", nil
	}
	b.owners.landing.plainResolve.Git = git
	b.owners.landing.plainProve.Git = git
	code, output := b.run(t, b.root, "resolve", "--json")
	var result struct{ Data plain.ResolveOutcome }
	if json.Unmarshal([]byte(output), &result) != nil || code != 0 || !result.Data.Held || result.Data.Outcome != "held" || aborts != 1 || probes != 1 {
		t.Fatalf("batch conflict: exit=%d output=%s aborts=%d probes=%d", code, output, aborts, probes)
	}
	entry, _, err := plain.Latest(b.install, "B")
	if err != nil || entry.State != plain.StateWaiting || entry.ReturnedAt != "" || !reflect.DeepEqual(entry.After, []plain.GoalSHA{{Goal: "A", SHA: "A-sha"}}) {
		t.Fatalf("batch conflict returned or lost dependency: %+v %v", entry, err)
	}
	statusCode, statusOutput := b.run(t, b.root, "status", "--json")
	var status struct{ Data plain.Status }
	if json.Unmarshal([]byte(statusOutput), &status) != nil || statusCode != 0 || !status.Data.Queue[1].Held || !strings.Contains(status.Data.Queue[1].Reason, "same batch") {
		t.Fatalf("held status: exit=%d %s", statusCode, statusOutput)
	}
	// Once A reaches main, B may be merged again and its conflict is its own.
	trunk, head = "A-sha", "A-sha"
	status = struct{ Data plain.Status }{}
	statusCode, statusOutput = b.run(t, b.root, "status", "--json")
	if json.Unmarshal([]byte(statusOutput), &status) != nil || statusCode != 0 || status.Data.Queue[1].Held || status.Data.Queue[0].State != plain.StateLanded {
		t.Fatalf("dependency did not release: exit=%d %s", statusCode, statusOutput)
	}
	code, output = b.run(t, b.root, "resolve", "--json")
	if json.Unmarshal([]byte(output), &result) != nil || code != 0 || result.Data.Outcome != "returned" || result.Data.Cause.Kind != "own" || result.Data.Conflict.Main != "A-sha" || !strings.Contains(result.Data.Entry.Reason, "work rebase B") || aborts != 2 || probes != 1 {
		t.Fatalf("main conflict: exit=%d %s", code, output)
	}
}

func TestLandingResolveGeneratedReturnShowsPathsInStatus(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	b.owners.landing.view = func(string) lane.View {
		return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "the landing lane is idle"}
	}
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	b.sourceConflictGit(t, func() {})
	git := b.owners.landing.plainResolve.Git
	b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "diff --name-only --diff-filter=U -z" {
			return "metasystem/out/result\x00", nil
		}
		return git(dir, args...)
	}
	b.owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "rev-parse" {
			return "main-sha", nil
		}
		if args[0] == "merge-base" {
			return "", exec.Command("/usr/bin/false").Run()
		}
		if args[0] == "ls-tree" || args[0] == "cat-file" {
			return "", nil
		}
		t.Fatalf("unexpected status Git: %v", args)
		return "", nil
	}
	if code, text := b.run(t, b.root, "resolve"); code != 0 {
		t.Fatalf("return=%d %s", code, text)
	}
	if code, text := b.run(t, b.root, "status"); code != 0 || !strings.Contains(oneSpaced(text), "metasystem/out/result (generated)") || !strings.Contains(oneSpaced(text), "work rebase goal") || !strings.Contains(text, "returned") {
		t.Fatalf("status=%d %s", code, text)
	}
}

func TestLandingResolveProbesMainBeforeReturningSourceConflict(t *testing.T) {
	t.Parallel()
	for _, probe := range []string{"conflicts", "unavailable"} {
		t.Run(probe, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
				t.Fatal(err)
			}
			aborts := 0
			b.sourceConflictGit(t, func() { aborts++ })
			base := b.owners.landing.plainResolve.Git
			b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "ls-tree --name-only main-sha -- metasystem/plans/goals/trunk-red.json":
					return "", nil
				case "rev-parse --verify HEAD^{commit}":
					return "batch-sha", nil
				case "merge-tree --write-tree main-sha goal-sha":
					if probe == "conflicts" {
						return "conflict tree", exec.Command("/usr/bin/false").Run()
					}
					return "", errors.New("main cannot be probed")
				}
				return base(dir, args...)
			}
			code, output := b.run(t, b.root, "resolve", "--json")
			var result struct{ Data plain.ResolveOutcome }
			if json.Unmarshal([]byte(output), &result) != nil {
				t.Fatal(output)
			}
			entry, _, err := plain.Latest(b.install, "goal")
			if err != nil {
				t.Fatal(err)
			}
			if probe == "conflicts" {
				if code != 0 || aborts != 1 || entry.State != plain.StateReturned || entry.Conflict.Main != "main-sha" || entry.Cause.Kind != "own" {
					t.Fatalf("main conflict: exit=%d aborts=%d %+v", code, aborts, entry)
				}
			} else if code != 1 || aborts != 0 || entry.State != plain.StateWaiting {
				t.Fatalf("unreadable main was attributed: exit=%d aborts=%d %+v", code, aborts, entry)
			}
		})
	}
}

func TestWorkLandBatchConflictNamesDependencyAndLeavesQueueAlone(t *testing.T) {
	t.Parallel()
	b := newLandRebaseBed(t)
	tip := b.landing.status.BranchTip
	line := plain.Line{Goal: "standing-validation", Branch: "goal/standing-validation", SHA: tip, Delivered: "Makes landing reliable"}
	if _, _, err := plain.HandIn(b.lane, line); err != nil {
		t.Fatal(err)
	}
	line.Outcome, line.Reason = plain.StateWaiting, "conflicts with goal A, which is in the same batch; it is merged again when A has landed"
	line.After = []plain.GoalSHA{{Goal: "A", SHA: "A-sha"}}
	queue := filepath.Join(plain.Dir(b.lane), "queue.jsonl")
	before, err := os.ReadFile(queue)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	before = append(before, append(data, '\n')...)
	b.writeFile(queue, string(before))
	code, result := b.runJSON(b.owners, "work", "land", line.Goal, "--delivered", "Makes landing reliable")
	after, err := os.ReadFile(queue)
	want := "goal standing-validation at " + plain.Short(tip) + " waits in the landing lane. It " + line.Reason + "."
	if code != 0 || result.Outcome != intentUnchanged || result.Summary != want || result.Next != nil || b.calls != 0 || b.records != 0 || err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("waiting seat: exit=%d result=%+v err=%v", code, result, err)
	}
}
