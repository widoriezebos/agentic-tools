package plain

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

func TestStewardMainProofUsesGivenClock(t *testing.T) {
	t.Parallel()
	for _, afterGreen := range []bool{true, false} {
		t.Run(map[bool]string{true: "green-written-while-waiting", false: "future-green-refused"}[afterGreen], func(t *testing.T) {
			t.Parallel()
			b := newFlakeMoveBed(t)
			b.base.At = bedNow.Add(8 * time.Second).Format(time.RFC3339)
			b.base.FullAt = b.base.At
			if err := os.Remove(resultsPath(b.install)); err != nil {
				t.Fatal(err)
			}
			b.save(t, b.base)
			given := bedNow.Add(4 * time.Second)
			if afterGreen {
				given = bedNow.Add(20 * time.Second)
			}
			b.move("b", given.Sub(bedNow))
			path := "metasystem/plans/goals/tests-observe-only-what-they-inject-2.md"
			f, problems := goal.ParseFile([]byte(b.trees["a"][path]))
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			f.Intent = "Repair the ordinary goal."
			f.Revision++
			f.History = append(f.History, goal.HistoryLine{At: given.Format(time.RFC3339), Opid: f.History[0].Opid, Actor: "lane+lin-1", Verb: "edit", Targets: []string{f.Id}, Keep: -1})
			b.trees["b"][path] = strings.TrimSpace(string(goal.RenderFile(f)))
			b.trees["b"]["metasystem/plans/goals/trunk-red.json"] = b.trees["a"]["metasystem/plans/goals/trunk-red.json"]
			steward := newStewardBed(t)
			b.seams.Machine, b.seams.Load, b.seams.Steward = steward.seams.Machine, steward.seams.Load, steward.seams.Steward
			b.seams.Steward.Trunk = nil
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "symbolic-ref --quiet --short HEAD":
					return "main", nil
				case "rev-parse --verify HEAD^{commit}", "rev-parse --verify --quiet refs/heads/main^{commit}", "rev-parse --verify --quiet refs/remotes/origin/main^{commit}", "rev-parse --verify refs/remotes/origin/main^{commit}":
					return "commit-" + b.main, nil
				case "rev-parse --verify MERGE_HEAD^{commit}":
					return "", nil
				case "rev-list --parents --ignore-missing commit-b":
					return "commit-b commit-a\ncommit-a", nil
				}
				return git(dir, args...)
			}
			b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{ID: "incident", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: b.base.Commit}}}}, nil
			}
			frozen := b.seams
			frozen.Now = func() time.Time { return bedNow }
			status := ObserveStatus(b.keeper.Home, b.record, frozen, nil)
			_, tree, err := b.seams.subject(b.checkout)
			if err != nil || tree == b.base.Tree || healthRow(status.Health, "unproven-incidents") == nil {
				t.Fatalf("missing moved-main health row: tree=%s status=%+v error=%v", tree, status, err)
			}
			var loads []time.Time
			load := b.seams.Load
			b.seams.Load = func(at time.Time) hostload.Sample {
				loads = append(loads, at)
				return load(at)
			}
			run, err := StewardStep(b.keeper.Home, bedNow, b.seams)
			if err != nil {
				t.Fatalf("steward: %+v %v", run, err)
			}
			running, recorded, _, err := ReadRunning(b.install, b.seams)
			if err != nil {
				t.Fatal(err)
			}
			if afterGreen {
				if b.trunks != 0 || recorded || !strings.Contains(run.Line, "unproven-incidents: "+stewardAlreadyProven) {
					t.Fatalf("green not reused: launches=%d recorded=%v running=%+v admission=%+v line=%s", b.trunks, recorded, running, running.Admission, run.Line)
				}
				b.inherited(t, "b", 1)
			} else {
				if b.trunks != 1 || !recorded || !running.Trunk || len(b.argv) != 1 || !slices.Contains(b.argv[0], "--trunk") || running.Admission == nil || running.Admission.Scope != "full" || running.Admission.Reason != "main has no full green to keep" || running.Admission.Decision == nil || running.Admission.Decision.Scope != "full" || running.Admission.Decision.ScopeReason != "main has no full green to keep" {
					t.Fatalf("future green admitted: launches=%d argv=%v running=%+v admission=%+v", b.trunks, b.argv, running, running.Admission)
				}
				if running.Since != given.Format(time.RFC3339) {
					t.Errorf("proof since=%s, want given clock %s", running.Since, given.Format(time.RFC3339))
				}
				b.inherited(t, "b", 0)
			}
			if len(loads) == 0 || !loads[0].Equal(bedNow) {
				t.Fatalf("step status load clocks=%v, want first %s", loads, bedNow)
			}
		})
	}
}

func TestStewardMainProofScopesWithGivenClock(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	scoped.base.At = bedNow.Add(8 * time.Second).Format(time.RFC3339)
	scoped.base.FullAt = scoped.base.At
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	scoped.save(t, scoped.base)
	scoped.seams.Now = func() time.Time { return bedNow.Add(20 * time.Second) }
	frozen := scoped.seams
	frozen.Now = func() time.Time { return bedNow }
	status := ObserveStatus(b.home, b.record, frozen, nil)
	if healthRow(status.Health, "unproven-incidents") == nil {
		t.Fatalf("missing health row: %+v", status)
	}
	run, err := StewardStep(b.home, bedNow, scoped.seams)
	if err != nil {
		t.Fatalf("steward: %+v %v", run, err)
	}
	running, recorded, _, err := ReadRunning(b.install, scoped.seams)
	if err != nil || !recorded || running.Admission == nil || running.Admission.Scope != "scoped" {
		t.Fatalf("scoped admission: running=%+v admission=%+v error=%v", running, running.Admission, err)
	}
}
