package steward

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
)

var remainingRemedyOutput = flag.String("remaining-remedy-output", "", "write the real health bed verdicts for system check")

func TestHealthRemedyTableOwnsEveryRole(t *testing.T) {
	t.Parallel()
	causes := []RemedyCause{"", "unknown-cause"}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			if spec, ok := node.(*ast.ValueSpec); ok {
				for i, id := range spec.Names {
					if strings.HasPrefix(id.Name, "Cause") {
						value := spec.Values[i].(*ast.BasicLit)
						cause, err := strconv.Unquote(value.Value)
						if err != nil {
							t.Fatal(err)
						}
						causes = append(causes, RemedyCause(cause))
					}
				}
			}
			if fn, ok := node.(*ast.FuncDecl); ok && (fn.Name.Name == "roleDead" || fn.Name.Name == "roleUnknown") {
				return false
			}
			if field, ok := node.(*ast.KeyValueExpr); ok {
				if id, ok := field.Key.(*ast.Ident); ok && id.Name == "Remedy" {
					t.Errorf("%s: remedy field words must come from the table", name)
				}
			}
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for i, lhs := range assignment.Lhs {
					if field, ok := lhs.(*ast.SelectorExpr); ok && field.Sel.Name == "Remedy" && i < len(assignment.Rhs) {
						if raw, ok := assignment.Rhs[i].(*ast.BasicLit); ok && raw.Value != `""` {
							t.Errorf("%s: remedy assignment owns words outside the table", name)
						}
					}
				}
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if ok && (id.Name == "roleDead" || id.Name == "roleUnknown") {
				if len(call.Args) < 4 {
					t.Errorf("%s: %s leaves its cause with the old string", name, id.Name)
				} else if diagnostic, ok := call.Args[2].(*ast.BasicLit); !ok || diagnostic.Value != `""` {
					t.Errorf("%s: %s still owns remedy words outside the table", name, id.Name)
				}
			}
			return true
		})
	}
	for _, role := range append(slices.Clone(healthRoleOrder), RoleRetroDebt) {
		for _, cause := range causes {
			fact := RemedyFact{Cause: cause, Goal: "g", Record: "record", Stop: "stop", Job: "job", Incident: "incident", Command: "metasystem system start --repo /fixture"}
			remedy := remedyFor(role, fact)
			if remedy.Clears == "" || len(remedy.Act) == 0 && remedy.Plain == "" {
				t.Errorf("%s/%s has no owned clearing effect: %+v", role, cause, remedy)
			}
			for _, audience := range []string{"human", "agent"} {
				act, plain := remedy.Render(audience, nil)
				if role == RoleProofAdmission && (cause == "" || cause == CauseUnavailable) && (len(act) != 0 || plain != "a person follows the lease owner's instruction: "+fact.Command) {
					t.Errorf("%s/%s changed the lease owner's exact act: %v %q", role, cause, act, plain)
				}
				if len(act) > 0 && (len(act) < 3 || act[0] != "metasystem" || slices.Contains([]string{"list", "show", "check", "status"}, act[2]) || audience == "human" && act[1] == "session") {
					t.Errorf("%s/%s for %s is no public repairing act: %v", role, cause, audience, act)
				}
				if len(act) == 0 && !strings.Contains(plain, "person") && !strings.Contains(plain, "steward") && !strings.Contains(plain, "claiming session") && !strings.Contains(plain, "landing lane") && !strings.Contains(plain, "job reaper") && !strings.Contains(plain, "waiting heavy proof") && !strings.Contains(plain, "evidence producer") {
					t.Errorf("%s/%s names no actor who clears it: %q", role, cause, plain)
				}
				if len(act) == 0 && plain == "" {
					t.Errorf("%s/%s for %s has no line 2", role, cause, audience)
				}
			}
		}
	}

	type observation struct {
		Root    string
		Verdict HealthVerdict
	}
	var verdicts []observation
	for _, status := range []HealthStatus{HealthDead, HealthUnknown} {
		b := newHealthBed(t, EnrollmentFixture, "")
		for _, ref := range []identity.Ref{b.runner, b.owner, b.watcher, b.main} {
			b.probe[ref.Pid] = struct {
				exact identity.Exact
				state identity.Liveness
				err   error
			}{state: identity.Dead}
			if status == HealthUnknown {
				b.probe[ref.Pid] = struct {
					exact identity.Exact
					state identity.Liveness
					err   error
				}{state: identity.Unknown}
			}
		}
		now := b.base.Add(time.Hour)
		b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\n"))
		elapsed := int64(57)
		recordStopCompletion(t, b.root, &elapsed, "DEADLINE_EXPIRED")
		b.lookPath = func(string) (string, error) { return "/fixture/claude", nil }
		b.writeJSON("artifacts/agents/jobs/job.json", map[string]any{"jobId": "job", "status": "running", "pid": b.runner.Pid, "pidStartedAt": b.runner.StartedAtSec})
		b.writeJSON("artifacts/agents/proof-runs/attempts/proof.json", map[string]any{"attemptId": "proof", "launcher": map[string]any{"pid": b.runner.Pid, "pidStartedAt": b.runner.StartedAtSec}})
		if err := saveLedgerAttentionState(b.root, ledgerAttentionState{RemoteTip: "remote", ExaminedTip: "base", MovedAt: b.base.Format(time.RFC3339Nano)}); err != nil {
			t.Fatal(err)
		}
		if status == HealthUnknown {
			for _, relative := range []string{"artifacts/agents/supervision/last-census.json", "artifacts/agents/steward/components/narrator.json", "artifacts/agents/steward/components/supervision-hook.json", "artifacts/agents/capabilities/claude-broken.json"} {
				b.writeFile(relative, []byte("{broken"))
			}
			if err := os.WriteFile(ledgerAttentionStatePath(b.root), []byte("{broken"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		evaluate := func(_, _ string, _ time.Time, _ identity.Prober, _ bool) ([]RoleVerdict, SpendObservation) {
			roles, spend := b.evaluate(b.root, b.root, now, b.probe, false)
			var filtered []RoleVerdict
			filtered = append(filtered, checkLedgerAttention(b.root, now), checkProofAttempts(b.root, b.probe))
			projection, projectionErr := goal.Projection{}, errors.New("fixture unreadable ledger")
			if status == HealthDead {
				epoch := newRoleHealthProjectionBed(t, now, map[string]*goal.GoalFile{"bounded-goal": stopCapabilityHealthGoal()}, nil)
				projection, projectionErr = epoch.project()
				writeHealthLease(t, b.root, "coordinator", 5)
			}
			filtered = append(filtered, checkStopCapabilityEpochFromProjection(b.root, now, projection, projectionErr, func(string) (string, error) { return "bed-m1", nil }))
			if status == HealthDead {
				file := structuredHealthGoal()
				file.Budget = nil
				root, budgetProjection, budgetErr := budgetHealthProjectionBed(t, now, map[string]*goal.GoalFile{"bounded-goal": file})
				trunk := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("incident", "", b.base)}, nil)
				filtered = append(filtered, checkClaimedGoalBudgetsFromProjection(root, now, budgetProjection, true, budgetErr, nil), trunk.trunkRedWithoutBatch())
			} else {
				filtered = append(filtered, checkClaimedGoalBudgetsFromProjection(b.root, now, projection, true, projectionErr, nil), checkTrunkRedFromProjection(b.root, now, projection, projectionErr))
			}
			filtered = append(filtered, checkProofAdmission(b.root, now, func(string) ([]proofrun.HostLeaseReport, error) {
				if status == HealthUnknown {
					return nil, errors.New("fixture unreadable lease")
				}
				return []proofrun.HostLeaseReport{{Lease: "lease", Since: b.base, State: proofrun.HostLeaseDead, Reason: "owner ended"}}, nil
			}))
			for _, role := range filtered {
				i := slices.IndexFunc(roles, func(candidate RoleVerdict) bool { return candidate.Role == role.Role })
				roles[i] = role
			}
			return roles, spend
		}
		open := previewHealthAtWithEvaluation(b.root, b.root, now, b.probe, evaluate)
		seen := make(map[HealthRole]HealthStatus)
		for _, role := range open.Roles {
			seen[role.Role] = role.Status
			if role.Status == HealthAlive {
				continue
			}
			act, plain := role.PublicRemedy("human", nil)
			if role.Role == RoleProofAdmission && status == HealthDead && (len(act) != 0 || plain != "nothing to do: a waiting heavy proof reclaims a dead lease on its next admission pass") {
				t.Errorf("dead lease without an owner instruction requires no human act: %v %q", act, plain)
			}
			want := strings.TrimSpace(strings.Join(act, " ") + " " + plain)
			if len(role.RemedyFacts) == 0 || role.Remedy == "" || role.Remedy != want || !strings.Contains(role.Line(), want) {
				t.Errorf("real %s verdict lost the table words: %+v", status, role)
			}
		}
		for _, role := range []HealthRole{RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher, RoleCensusFreshness, RoleNarratorFreshness, RoleSessionMain, RoleHookFreshness, RoleStopHookDuration, RoleLedgerAttention, RoleClaimedGoalBudget, RoleStopCapabilityEpoch, RoleTrunkRed, RoleNonterminalJobs, RoleCapabilitySnapshots, RoleProofAttempts, RoleProofAdmission} {
			if seen[role] != status {
				t.Errorf("the real %s fixture did not drive %s: %s", status, role, seen[role])
			}
		}
		hook := NewHookHealthPreview(open)
		if status == HealthDead && !strings.Contains(hook.Line, "remedy: a person runs: metasystem goal budget bounded-goal BOX") {
			t.Errorf("Stop hook gave the agent a person's budget act: %s", hook.Line)
		}
		for _, role := range open.Roles {
			if role.Status == HealthDead && processHealthRole(role.Role) {
				if !strings.Contains(hook.Line, string(role.Role)+"=dead ("+role.Reason+"; remedy: metasystem session start)") {
					t.Errorf("Stop hook did not give %s the agent act: %s", role.Role, hook.Line)
				}
			}
		}
		verdicts = append(verdicts, observation{b.root, open})
		closeProcessFence(t, b.root, 1)
		closed := previewHealthAtWithEvaluation(b.root, b.root, now, b.probe, evaluate)
		for i, role := range closed.Roles {
			if role.Status != HealthAlive && processHealthRole(role.Role) {
				if role.RemedyFacts[0].Cause != CauseStopFenceClosed || !strings.Contains(role.Remedy, "metasystem system start --repo "+b.root) {
					t.Errorf("process role under the fence lost its repair: %+v", role)
				}
			} else if role.Remedy != open.Roles[i].Remedy || !slices.Equal(role.RemedyFacts, open.Roles[i].RemedyFacts) {
				t.Errorf("fence rewrote a non-process role: %+v", role)
			}
		}
		// A typed non-process cause can mention session start without joining
		// the process class. An obsolete process diagnostic must not decide it.
		probe := []RoleVerdict{{Role: RoleDisk, Status: HealthDead, Remedy: "metasystem session start is mentioned in diagnostic evidence"}, {Role: RoleSupervisionOwner, Status: HealthUnknown, Remedy: "obsolete words"}}
		diskDiagnostic := probe[0].Remedy
		fenced, err := healthStopped(b.root, b.root, now, probe, SpendObservation{}, HealthObservationState{})
		if err != nil || fenced == nil {
			t.Fatalf("fenced health: %v", err)
		}
		if fenced.Roles[0].Remedy != diskDiagnostic || len(fenced.Roles[1].RemedyFacts) != 1 || fenced.Roles[1].RemedyFacts[0].Cause != CauseStopFenceClosed {
			t.Fatalf("the fence used remedy text instead of role class: %+v %v", fenced, err)
		}
		verdicts = append(verdicts, observation{b.root, closed})
		standing := open.Roles[0]
		standing.Standing = true
		var message string
		if err := fileStandingDefects(b.root, HealthVerdict{Roles: []RoleVerdict{standing}}, now, func(_, text string) error { message = text; return nil }); err != nil || !strings.Contains(message, standing.Remedy) {
			t.Fatalf("standing defect lost the table words: %q %v", message, err)
		}
	}
	b := newHealthBed(t, EnrollmentFixture, "")
	if _, err := BeginHookAttempt(b.root, b.main, "pending", b.base); err != nil {
		t.Fatal(err)
	}
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=invalid-runtime\n"))
	settings, _ := capabilitySnapshotStatus(b.root, b.root, b.base, b.lookPath)
	spendRole, _ := checkSpendFenceWithMeasureAndMachine(b.root, b.base,
		func(string, string, time.Time) (spend.Ledger, error) {
			return spend.Ledger{}, errors.New("fixture spend evidence unreadable")
		},
		func(string) (string, error) { return "bed-m1", nil })
	for _, item := range []struct {
		role  RoleVerdict
		cause RemedyCause
		now   time.Time
	}{
		{checkHookFreshnessAt(b.root, b.base, false), CauseObservationPending, b.base},
		{checkHookFreshnessAt(b.root, b.base.Add(-time.Second), false), CauseClockRegressed, b.base.Add(-time.Second)},
		{settings, CauseSettingsInvalid, b.base}, {spendRole, CauseUnreadable, b.base},
	} {
		if item.role.Status != HealthUnknown || len(item.role.RemedyFacts) != 1 || item.role.RemedyFacts[0].Cause != item.cause {
			t.Fatalf("the real %s cause was not observed: %+v", item.cause, item.role)
		}
		if item.cause == CauseSettingsInvalid && !strings.Contains(item.role.Reason, "metasystem.runtimes") {
			t.Fatal("the settings remedy lost the key a person must correct")
		}
		verdict := previewHealthAtWithEvaluation(b.root, b.root, item.now, b.probe,
			func(string, string, time.Time, identity.Prober, bool) ([]RoleVerdict, SpendObservation) {
				roles, spend := b.evaluate(b.root, b.root, item.now, b.probe, false)
				i := slices.IndexFunc(roles, func(role RoleVerdict) bool { return role.Role == item.role.Role })
				roles[i] = item.role
				return roles, spend
			})
		verdicts = append(verdicts, observation{b.root, verdict})
	}
	if *remainingRemedyOutput != "" {
		data, err := json.Marshal(verdicts)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(*remainingRemedyOutput, data, 0600); err != nil {
			t.Fatal(err)
		}
		// The public command reads these same temporary checkouts before cleanup.
		fmt.Fprintln(os.Stdout, "health beds ready")
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			t.Fatal(err)
		}
	}
}
