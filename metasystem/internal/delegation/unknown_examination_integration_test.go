package delegation_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestUnknownExaminationFollowUpAdmitsFullGoalAllowanceOnce(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	home := testprovider.Register(t, b.root)
	b.useRealGoalOwner()
	b.completeOnWait()
	now := b.alignClock()
	file := claimedGoal("unknown-read", now, &goal.Budget{
		ElapsedLimit: "1d", AttemptLimit: 1, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
	})
	b.goalWorld(file)
	commit := b.git("rev-parse", "HEAD")
	brief := b.brief("unknown-read.md", "implement", "Review the accepted commit.")
	requireExit(t, b.runEnv(b.budgetEnv("fresh"), "dispatch", "--role", "code-critic", "--brief", brief,
		"--reviews", "commit:"+commit, "--runtime", "fake", "--goal", file.Id,
		"--destructive-reach", "MECHANICAL", "--job-id", "unknown-source", "--wait"), 0, b.stderr.String())
	prior := b.record("unknown-source")
	if pid, err := strconv.Atoi(fmt.Sprint(prior["pid"])); err == nil && pid > 0 {
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Kill(); err != nil {
			t.Fatal(err)
		}
		prior["groupDeathProvenAt"] = now.Format(time.RFC3339)
	}
	b.writeRecord("unknown-source", prior)
	if err := os.WriteFile(filepath.Join(b.roundDir("unknown-source"), "return.md"), []byte("VERDICT: REVISE material=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch.CollectExamination(b.root, "unknown-source"); err == nil {
		t.Fatal("disagreeing structured and prose verdicts became readable")
	}
	before := dispatch.ProjectBudget(b.root, file, now, home)
	if before.Attempts != 1 {
		t.Fatalf("fixture did not reach the goal attempt allowance: %+v", before)
	}
	if err := dispatch.ReserveUnknownExaminationRetry(b.root, "unknown-source"); err != nil {
		t.Fatal(err)
	}
	message := b.brief("retry-read.md", "implement", "Examine the same accepted commit with readable stop inputs.")
	result := b.runEnv(b.budgetEnv("follow-up"), "follow-up", "--job", "unknown-source", "--message", message)
	requireExit(t, result, 0, b.stderr.String())
	fresh := b.record("unknown-source-r2")
	if fresh["examinationRetryOf"] != "unknown-source" || fresh["parentJob"] != "unknown-source" || fresh["reviews"] != "commit:"+commit {
		t.Fatalf("fresh read lost its reserved original subject: %v", fresh)
	}
	after := dispatch.ProjectBudget(b.root, file, now, home)
	if after.Status != dispatch.BudgetKnown || after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes || after.ActiveJobs != 1 {
		t.Fatalf("fresh read spent goal allowance or lost its active slot: before=%+v after=%+v", before, after)
	}
	b.completeJob("unknown-source-r2")
	if err := dispatch.ReserveUnknownExaminationRetry(b.root, "unknown-source-r2"); err == nil {
		t.Fatal("a second unavailable read granted another automatic retry")
	}
	third := b.runEnv(b.budgetEnv("follow-up"), "follow-up", "--job", "unknown-source", "--message", message)
	if third.ExitCode == 0 {
		t.Fatalf("unavailable retry admitted a third physical examination: %s", third.Stdout)
	}
	if _, err := os.Stat(b.recordPath("unknown-source-r3")); !os.IsNotExist(err) {
		t.Fatalf("held read published a third job: %v", err)
	}
	if !strings.Contains(string(result.Stdout), "unknown-source-r2") {
		t.Fatalf("follow-up did not identify its actual examination: %s", result.Stdout)
	}
}
