# The carried fold for goal dispatch-admission-refuses-on-a-fenced-sibling

Round 7 of chain bsws-build1b-20260909 (job bsws-build1b-20260909-r7, 2026-09-10 08:04 to 08:59Z, codex gpt-5.6-sol, xhigh) built the fold for finding BSW-12 of the closing read (records/misc/breach-stop-wedges-seat-critique-r3.md): in EvaluateGoalAdmission the machine-wide claim scan no longer refuses a dispatch for another goal because a fenced sibling is held; dispatch against the fenced goal itself stays refused by the exact-revision gate. Fixtures: one and two fenced siblings do not refuse live-goal admission; the fenced goal itself still returns the stop-batch and ELAPSED_LIMIT refusal. Proven in the round: dispatch, steward and channel packages green; the goal package green in isolation; the fast gate green; dispatch-fixtures green except two sandbox process-enumeration scenarios. The chain had reached goal breach-stop-wedges-seat's three-read ceiling (R-42-m0) when this fold was built, and the chain's own law admits no closing read of an earlier round once a later one exists. So the read of round 7 is charged to goal dispatch-admission-refuses-on-a-fenced-sibling, opened for this finding, and the chain lands whole under breach-stop-wedges-seat; the new goal concludes on that landing. The diff below is the fold as built, kept here so the finding's record is complete.

```diff
diff --git a/metasystem/internal/dispatch/admission.go b/metasystem/internal/dispatch/admission.go
index a07bff6b2..a985c9275 100644
--- a/metasystem/internal/dispatch/admission.go
+++ b/metasystem/internal/dispatch/admission.go
@@ -133,13 +133,9 @@ func EvaluateGoalAdmission(repoRoot, stopLineage string, now time.Time) (GoalAdm
            file.Claimed.Machine != machine || file.Claimed.Lineage != stopLineage {
            continue
        }
-       if file.StopFence != nil {
-           verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
-               GoalID: id, GoalRevision: file.Claimed.Revision,
-               Unknown: &BudgetUnknownEvidence{Code: BudgetUnknown, Record: goalRecordPath(id),
-                   Reason: fmt.Sprintf("launch fence closed by stop batch %s", file.StopFence.StopID)},
-               LiveStopReason: file.StopFence.Reason,
-           })
+       // A breach-stopped goal is waiting on a human and must not keep the
+       // machine from working the next item.
+       if file.IsFencedClaim() {
            continue
        }
        budget := ProjectBudget(repoRoot, file, now)
diff --git a/metasystem/internal/dispatch/admission_test.go b/metasystem/internal/dispatch/admission_test.go
new file mode 100644
index 000000000..b2cee4452
--- /dev/null
+++ b/metasystem/internal/dispatch/admission_test.go
@@ -0,0 +1,96 @@
+package dispatch
+
+import (
+   "os"
+   "os/exec"
+   "path/filepath"
+   "testing"
+   "time"
+
+   "github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
+)
+
+func addFencedAdmissionGoal(t *testing.T, root, id, stopID string, opids [3]string) {
+   t.Helper()
+   openedAt := "2026-08-28T08:00:00Z"
+   claimedAt := "2026-08-28T09:00:00Z"
+   closedAt := "2026-08-28T09:30:00Z"
+   file := &goal.GoalFile{
+       Id: id, State: goal.StateClaimed, Intent: "Hold the stopped work", Origin: goal.OriginMain,
+       NextStep: "Wait for a human resume.", OpenedAt: openedAt, Revision: 3,
+       Budget: &goal.Budget{
+           ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 3,
+       },
+       Claimed:        &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: claimedAt, Revision: 2},
+       StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1},
+       StopFence: &goal.StopFence{
+           StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2,
+           ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit,
+       },
+       History: []goal.HistoryLine{
+           {At: openedAt, Opid: goal.Opid(opids[0], "bed-m1", "coordinator"), Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
+           {At: claimedAt, Opid: goal.Opid(opids[1], "bed-m1", "coordinator"), Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
+           {At: closedAt, Opid: goal.Opid(opids[2], "bed-m1", "coordinator"), Verb: "breach-stop", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
+       },
+   }
+   path := filepath.Join(root, "plans", "goals", id+".md")
+   if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
+       t.Fatal(err)
+   }
+   for _, args := range [][]string{
+       {"add", "plans/goals"},
+       {"commit", "-q", "-m", "add fenced admission goal"},
+       {"update-ref", goal.LocalLedgerBranch, "HEAD"},
+       {"update-ref", goal.AcceptedRef, "HEAD"},
+   } {
+       command := exec.Command("git", append([]string{"-C", root}, args...)...)
+       command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
+       if output, err := command.CombinedOutput(); err != nil {
+           t.Fatalf("git %v: %v: %s", args, err, output)
+       }
+   }
+}
+
+func TestGoalAdmissionIgnoresSiblingFencedClaim(t *testing.T) {
+   root := revisionBindingBed(t, 2)
+   addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
+       "01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
+   })
+
+   verdict, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
+   if err != nil || verdict.Refused() {
+       t.Fatalf("the fenced sibling kept admission closed for the live goal: %+v %v", verdict, err)
+   }
+}
+
+func TestGoalRevisionAdmissionStillRefusesTheFencedGoal(t *testing.T) {
+   root := revisionBindingBed(t, 2)
+   addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
+       "01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
+   })
+
+   verdict, err := EvaluateGoalRevisionAdmission(root, "stopped-a", 2, 5,
+       time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
+   if err != nil || !verdict.Refused() || verdict.Refusal == nil ||
+       verdict.Refusal.Unknown == nil ||
+       verdict.Refusal.Unknown.Reason != "launch fence closed by stop batch stop-stopped-a-r2-f1" ||
+       verdict.LiveStopReason != goal.StopReasonElapsedLimit ||
+       verdict.Refusal.LiveStopReason != goal.StopReasonElapsedLimit {
+       t.Fatalf("the fenced goal did not retain its live-stop refusal: %+v %v", verdict, err)
+   }
+}
+
+func TestGoalAdmissionIgnoresMultipleSiblingFencedClaims(t *testing.T) {
+   root := revisionBindingBed(t, 2)
+   addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
+       "01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
+   })
+   addFencedAdmissionGoal(t, root, "stopped-c", "stop-stopped-c-r2-f1", [3]string{
+       "01ARZ3NDEKTSV4RRFFQ69G5FAD", "01ARZ3NDEKTSV4RRFFQ69G5FAE", "01ARZ3NDEKTSV4RRFFQ69G5FAF",
+   })
+
+   verdict, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
+   if err != nil || verdict.Refused() {
+       t.Fatalf("multiple fenced siblings kept admission closed for the live goal: %+v %v", verdict, err)
+   }
+}
```
