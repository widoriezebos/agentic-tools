package plain

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestStandingResolvedRedAdmission(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"fresh", "expired", "landed", "unlanded", "nonrun", "inherited", "scoped", "closed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			b.red.At = bedNow.Add(-48 * time.Hour).Format(time.RFC3339)
			b.git.changed[[2]string{b.red.Commit, b.git.commit}] = "metasystem/internal/other/code.go"
			if err := os.Remove(resultsPath(b.install)); err != nil {
				t.Fatal(err)
			}
			b.save(t, b.red)
			green := b.base
			green.Trunk = mode != "landed" && mode != "unlanded"
			green.Environment = b.environment
			green.Commit, green.Tree, green.FullTree = b.git.commit, b.git.tree, b.git.tree
			if mode == "expired" {
				green.At = bedNow.Add(-2 * time.Hour).Format(time.RFC3339)
				green.FullAt = green.At
			}
			if mode == "nonrun" {
				green.Cause = &Cause{Kind: "environment", Name: "lost-process"}
			}
			if mode == "inherited" {
				green.Scope = "inherited"
			}
			if mode == "scoped" {
				green.Scope = "scoped"
			}
			if mode == "closed" {
				for i := range b.incidents {
					b.incidents[i].Closed = &goal.TrunkRedClosure{How: "hand"}
				}
			} else {
				b.save(t, green)
			}
			if mode == "fresh" || mode == "expired" || mode == "landed" {
				if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil || b.launches != 0 {
					t.Fatalf("unmoved green restarted: %v launches=%d", err, b.launches)
				}
			}
			old := b.git.commit
			b.git.commit, b.git.tree = "records-target", "records-tree"
			b.git.changed[[2]string{b.red.Commit, b.git.commit}] = "metasystem/internal/other/code.go"
			b.git.changed[[2]string{green.FullTree, b.git.tree}] = "metasystem/records/shared/page.md"
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if mode == "unlanded" && len(args) == 4 && args[0] == "merge-base" && args[2] == old {
					return "", exec.Command("/usr/bin/false").Run()
				}
				return git(dir, args...)
			}
			b.setContract(t)
			if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
				t.Fatal(err)
			}
			running, _, _, err := ReadRunning(b.install, b.seams)
			if err != nil || running.Admission == nil {
				t.Fatalf("admission %+v %v", running, err)
			}
			a := running.Admission
			if a.Decision.Standing != nil {
				t.Fatalf("resolved red retained: %+v", a)
			}
			switch mode {
			case "fresh", "landed":
				if a.Scope != "scoped" || a.Reason != fmt.Sprintf("main moved by records only; 1 check run, the rest holds from the %s check", bedNow.Add(-10*time.Minute).Local().Format("15:04")) {
					t.Fatalf("fresh green scope %+v", a)
				}
			case "expired":
				if a.Scope != "scoped" || a.Reason != fmt.Sprintf("main moved by records only; 1 check run, the rest holds from the %s check", bedNow.Add(-2*time.Hour).Local().Format("15:04")) {
					t.Fatalf("expired green scope %+v", a)
				}
			case "closed":
				if a.Scope != "full" || strings.Contains(a.Reason, "standing red") {
					t.Fatalf("closed red scope %+v", a)
				}
			default:
				if a.Scope != "full" || a.Reason != "an executable input changed since the standing red" {
					t.Fatalf("ineligible stop %+v", a)
				}
			}
		})
	}
}
func TestStandingRedPersonWithoutFreshUsesAdmission(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	b.seams.Person = &ActProvenance{Person: "Wido", Root: b.install}
	r := b.detached(t)
	running, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || r.Fresh || r.Scope != "scoped" || running.Admission.Decision.Standing == nil || len(r.Failed) != 2 || len(b.calls) != 1 {
		t.Fatalf("person admission %+v %+v %v", r, running, err)
	}
}
func TestStandingOutcomeRetentionEveryConfirmationRow(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"full pass", "full confirm", "scoped confirm", "scoped confirm fail"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			full := strings.HasPrefix(mode, "full")
			b.seams.Fresh = full
			if full {
				b.incidents = nil
			} else {
				b.incidents = b.incidents[1:]
			}
			if !full {
				b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/plans/designs/page.md"
			}
			calls := 0
			b.seams.RecordFlake = func(FlakeRecord) (FlakeRecorded, error) { return FlakeRecorded{Seen: 1}, nil }
			b.seams.Command = func(cmd *exec.Cmd) error {
				calls++
				status := "pass"
				if strings.Contains(mode, "confirm") && (calls == 1 || strings.HasSuffix(mode, "fail")) {
					status = "fail"
				}
				fmt.Fprintln(cmd.Stdout, "landing environment "+b.environment)
				for i := 0; i < 20000; i++ {
					fmt.Fprintf(cmd.Stdout, "{\"Action\":\"pass\",\"Package\":\"github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem\",\"Test\":\"TestUnrelated%d\"}\n", i)
				}
				fmt.Fprintf(cmd.Stdout, "{\"Action\":%q,\"Package\":\"github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem\",\"Test\":\"TestX\"}\n{\"Action\":%q,\"Package\":\"github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem\"}\n", status, status)
				fmt.Fprintf(cmd.Stdout, "landing group plans %s 1\n", status)
				if status == "fail" {
					fmt.Fprintln(cmd.Stdout, "LANDING-FAILED\tmetasystem/cmd/metasystem\tTestX\nLANDING-CHECKED\t1")
					return exec.Command("/usr/bin/false").Run()
				}
				fmt.Fprintln(cmd.Stdout, "LANDING-CHECKED\t0")
				return nil
			}
			r := b.detached(t)
			rows, err := Results(b.install)
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			for _, row := range append(rows, r) {
				if row.Attempt != r.Attempt {
					continue
				}
				checked++
				want := 1
				if full {
					want = 0
				}
				if (row.Scope == "full") != full || len(row.Outcomes) != want {
					t.Fatalf("%s retained %d outcomes, want %d", row.Repeat, len(row.Outcomes), want)
				}
				for _, o := range row.Outcomes {
					if o.Unit != "metasystem/cmd/metasystem" || o.Test != "TestX" || row.RepeatComplete && o.Status != "pass" {
						t.Fatalf("unrelated outcome %+v", o)
					}
				}
			}
			if strings.Contains(mode, "confirm") && (calls < 2 || checked < 4 || len(r.FlakeRepeats) == 0) {
				t.Fatalf("confirmation not exercised calls=%d rows=%d %+v", calls, checked, r)
			}
			if !strings.HasSuffix(mode, "fail") && r.Result != Green {
				t.Fatalf("passing execution %+v", r)
			}
		})
	}
}
func TestStandingBatchPassRetainedWithoutCoverage(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"full", "scoped"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			b.red.Failed = b.red.Failed[:1]
			b.save(t, b.red)
			b.seams.Trunk = false
			b.git.batches[b.git.commit] = "unlanded-batch"
			b.seams.Fresh = scope == "full"
			if scope == "scoped" {
				base := b.base
				base.Commit = "batch-base"
				b.git.batches[base.Commit] = "unlanded-batch"
				b.save(t, base)
				b.git.changed[[2]string{base.Tree, b.git.tree}] = "metasystem/plans/designs/page.md"
			}
			passed := b.detached(t)
			if passed.Scope != scope || len(passed.Outcomes) != 1 || passed.Outcomes[0] != (testOutcome{Unit: "plans", Status: "pass"}) {
				t.Fatalf("batch evidence %+v", passed)
			}
			b.git.commit, b.git.tree = "after-batch", "after-batch-tree"
			b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/records/shared/page.md"
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if len(args) == 4 && args[0] == "merge-base" && (args[2] == passed.Commit || args[2] == "batch-base") {
					return "", exec.Command("/usr/bin/false").Run()
				}
				return git(dir, args...)
			}
			b.seams.Trunk, b.seams.Fresh = true, false
			b.seams.NewID = func() string { return "after-batch" }
			b.setContract(t)
			next := b.detached(t)
			if next.Result != Green || len(next.Failed) != 0 || next.Scope != "scoped" || next.FullTree != "" || len(next.Unproved) == 0 || next.reusableGreen(bedNow) {
				t.Fatalf("batch pass lost or granted coverage %+v", next)
			}
		})
	}
}
