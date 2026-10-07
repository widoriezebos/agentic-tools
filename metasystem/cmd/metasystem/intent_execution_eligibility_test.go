package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// approveExecutionBed uses the approval command's publication path so the
// work fixture carries the same approval evidence as an ordinary caller.
func approveExecutionBed(t *testing.T, bed *workBed, relayed bool) {
	t.Helper()
	owners := bed.owners()
	args := []string{"goal", "approve", bed.id, "--by", "Wido", "--lineage", "m1"}
	if relayed {
		owners.prove = fixedTemporaryGoalAuthority
		args = append(args, "--temporary-human-word", "Wido authorizes this goal approval", "--review-by", "2026-09-06")
	} else {
		args = append(args, "--fixture-human-authority")
	}
	if code, result := bed.runJSON(owners, args...); code != 0 {
		t.Fatalf("approve execution fixture: code=%d %+v", code, result)
	}
}

func TestIntentBuildRequiresCurrentExecutionAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, cause string
		relayed     bool
	}{
		{name: "proven approval"},
		{name: "current relayed approval", relayed: true},
		{name: "missing capability", cause: "no stop capability"},
		{name: "missing approval", cause: "awaits a person's approval"},
		{name: "missing approval and budget", cause: "not approved with a budget yet"},
		{name: "expired relayed approval", cause: "review date 2026-09-06 has passed", relayed: true},
		{name: "budget above norm", cause: "over its tier's"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBedWith(t, func(file *goal.GoalFile) {
				file.Approved, file.Budget = nil, nil
			})
			approveExecutionBed(t, bed, row.relayed)
			file := bed.goalFile(bed.id)
			switch row.name {
			case "missing capability":
				file.StopCapability = nil
			case "missing approval":
				file.Approved = nil
			case "missing approval and budget":
				file.Approved, file.Budget = nil, nil
			case "budget above norm":
				file.Budget.ReservedJobMinutesLimit++
				file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
			}
			bed.addGoal(file)
			before := goal.RenderFile(bed.goalFile(bed.id))
			owners := bed.workOwners()
			if row.name == "expired relayed approval" {
				owners.commandNow = func(string) (time.Time, error) {
					return time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC), nil
				}
			}
			args := append([]string{"work", "build", bed.id, "authority", "--brief", bed.brief("authority.md", "Build it.\n"), "--lines", "5"}, workCheck...)
			build := func(owners intentOwners) (int, intentResult) {
				t.Helper()
				command, rest, _ := resolveIntentArgv(args)
				var stdout, stderr bytes.Buffer
				code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, bed.root(), owners)
				var result intentResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("build result: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
				}
				bed.recordReadDirs(result)
				return code, result
			}
			code, result := build(owners)
			if row.cause == "" {
				if code != 0 || result.Outcome != intentConfirmed || len(bed.starter.launched()) == 0 {
					t.Fatalf("current approval did not admit owned work: code=%d %+v", code, result)
				}
				return
			}
			if code != 1 || result.Outcome != intentRefused || !strings.Contains(resultWords(result), row.cause) || len(bed.starter.launched()) != 0 || !bytes.Equal(before, goal.RenderFile(bed.goalFile(bed.id))) {
				t.Fatalf("invalid execution authority launched or changed owned work: code=%d %+v launches=%v", code, result, bed.starter.launched())
			}
			if strings.HasPrefix(row.name, "missing approval") || row.name == "expired relayed approval" {
				approveExecutionBed(t, bed, false)
				code, result = build(bed.workOwners())
				if code != 0 || result.Outcome != intentConfirmed || len(bed.starter.launched()) == 0 {
					t.Fatalf("approval remedy did not admit the same owned work: code=%d %+v", code, result)
				}
			}
		})
	}
}
