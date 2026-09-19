package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

func configureAbandonFloorTest(t *testing.T, stamp string) {
	t.Helper()
	if stamp != strings.Repeat("a", 40) {
		t.Fatalf("unsupported abandon test build stamp %q", stamp)
	}
}

func recordAbandonFloorTest(t *testing.T, root, ulid string) {
	t.Helper()
	req := verbReq(root, ulid, "mac-a")
	req.Actor.Human = "Wido"
	commit := strings.Repeat("a", 40)
	if result, err := EngineFloor(req, commit, goalHumanProof(t, root, req.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("engine floor: %+v %v", result, err)
	}
}

func TestAbandonOfALandReadyClaimClearsTheLandingBinding(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X00000000000000000M000")

	t0 := time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	if result, err := Open(landingReq(root, "01J5X00000000000000000M010", "mac-a", t0), "land-ready-abandon", "Abandon work in the landing slot.", OriginHuman, "Enter landing."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, landingReq(root, "01J5X00000000000000000M020", "mac-a", t0.Add(time.Minute)), "land-ready-abandon", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	if result, err := LandReady(landingReq(root, "01J5X00000000000000000M030", "mac-a", t0.Add(2*time.Minute)), "land-ready-abandon"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready: %+v %v", result, err)
	}

	req := landingReq(root, "01J5X00000000000000000M040", "mac-a", t0.Add(3*time.Minute))
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "land-ready-abandon", AbandonSpec{Because: "the landing is no longer wanted"}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon land-ready claim: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	abandoned := tree.Abandoned["land-ready-abandon"]
	if abandoned == nil || abandoned.State != StateAbandoned || abandoned.Claimed != nil || abandoned.Obligation != nil || abandoned.Landing != nil {
		t.Fatalf("abandon did not clear the claim and its landing binding: %+v", abandoned)
	}
}

func TestAbandonRepairsBlockerParksForCarriedWaivedAndAlso(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec AbandonSpec
		want string
	}{
		{name: "carried", spec: AbandonSpec{Because: "the blocker moved", Carried: "successor"}, want: "carried"},
		{name: "waived", spec: AbandonSpec{Because: "the blocker is unnecessary", Waive: []string{"dependent=the dependency no longer applies"}}, want: "waived"},
		{name: "also", spec: AbandonSpec{Because: "neither goal will be worked", Also: []string{"dependent"}}, want: "also"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, root := oneClone(t)
			seedLedger(t, root)
			configureAbandonFloorTest(t, strings.Repeat("a", 40))
			recordAbandonFloorTest(t, root, fmt.Sprintf("01J5X00000000000000000P0%d0", index))
			if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000000P1%d0", index), "mac-a"), "dependent", "Wait for a blocker.", OriginHuman, "Resume after the blocker."); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("open dependent: %+v %v", result, err)
			}
			risk := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "abandon blocker park fixture"}
			blockerReq := verbReq(root, fmt.Sprintf("01J5X00000000000000000P2%d0", index), "mac-a")
			blockerReq.Actor.Human = "Wido"
			if result, err := OpenRisked(blockerReq, "blocker", "Block the dependent.", OriginHuman, "Resolve the blocker.", "dependent", risk, 0, "", nil, nil); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("open blocker: %+v %v", result, err)
			}
			if test.want == "carried" {
				if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000000P3%d0", index), "mac-a"), "successor", "Carry the dependency.", OriginHuman, "Resolve the successor."); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("open successor: %+v %v", result, err)
				}
			}

			req := verbReq(root, fmt.Sprintf("01J5X00000000000000000P4%d0", index), "mac-a")
			req.Actor.Human = "Wido"
			result, err := Abandon(req, "blocker", test.spec, goalHumanProof(t, root, req.Now))
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("abandon: %+v %v", result, err)
			}
			tree, err := loadTree(root, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			switch test.want {
			case "carried":
				dependent := tree.Live["dependent"]
				if dependent == nil || dependent.State != StateParked || dependent.Parked == nil || dependent.Parked.Blocker != "successor" || dependent.Parked.Because != "blocked by successor; returns when it is done" || !reflect.DeepEqual(dependent.Blocked, []string{"successor"}) {
					t.Fatalf("carried abandonment did not move the blocker park: %+v", dependent)
				}
			case "waived":
				dependent := tree.Live["dependent"]
				if dependent == nil || dependent.State != StateQueued || dependent.Parked != nil || len(dependent.Blocked) != 0 {
					t.Fatalf("waived abandonment did not lift the blocker park: %+v", dependent)
				}
				if last := dependent.History[len(dependent.History)-1]; last.Verb != "unpark" || !strings.Contains(last.Reason, "abandoned and waived") {
					t.Fatalf("waived abandonment did not record the automatic return: %+v", last)
				}
			case "also":
				if tree.Live["dependent"] != nil || tree.Abandoned["dependent"] == nil || tree.Abandoned["dependent"].Parked != nil {
					t.Fatalf("--also did not abandon the parked dependent cleanly: live=%+v abandoned=%+v", tree.Live["dependent"], tree.Abandoned["dependent"])
				}
			}
		})
	}
}

func TestAbandonWaiverLiftsAParkWhoseMarkerNamesAnAlreadyDoneBlocker(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X00000000000000000Q000")

	if result, err := Open(verbReq(root, "01J5X00000000000000000Q010", "mac-a"), "dependent", "Wait for both blockers.", OriginHuman, "Resume when both are resolved."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open dependent: %+v %v", result, err)
	}
	risk := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "abandon blocker park fixture"}
	person := verbReq(root, "01J5X00000000000000000Q020", "mac-a")
	person.Actor.Human = "Wido"
	if result, err := OpenRisked(person, "done-blocker", "Finish the first blocker.", OriginHuman, "Resolve it.", "dependent", risk, 0, "", nil, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open first blocker: %+v %v", result, err)
	}
	person.Ulid = "01J5X00000000000000000Q030"
	if result, err := OpenRisked(person, "waived-blocker", "Remove the second blocker.", OriginHuman, "Resolve it.", "dependent", risk, 0, "", nil, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open second blocker: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000Q040", "mac-a"), "done-blocker", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim first blocker: %+v %v", result, err)
	}
	person.Ulid = "01J5X00000000000000000Q050"
	done, err := Done(person, "done-blocker", "The first blocker is complete.")
	if err != nil || done.Outcome != OutcomeConfirmed {
		t.Fatalf("finish first blocker: %+v %v", done, err)
	}
	tree, err := loadTree(root, done.Tip)
	if err != nil {
		t.Fatal(err)
	}
	before := tree.Live["dependent"]
	if before == nil || before.State != StateParked || before.Parked == nil || before.Parked.Blocker != "done-blocker" || !reflect.DeepEqual(before.Blocked, []string{"done-blocker", "waived-blocker"}) {
		t.Fatalf("fixture did not leave the park marker on the completed blocker: %+v", before)
	}

	person.Ulid = "01J5X00000000000000000Q060"
	result, err := Abandon(person, "waived-blocker", AbandonSpec{
		Because: "the second blocker is unnecessary",
		Waive:   []string{"dependent=the remaining dependency no longer applies"},
	}, goalHumanProof(t, root, person.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon waived blocker: %+v %v", result, err)
	}
	tree, err = loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	dependent := tree.Live["dependent"]
	if dependent == nil || dependent.State != StateQueued || dependent.Parked != nil || !reflect.DeepEqual(dependent.Blocked, []string{"done-blocker"}) {
		t.Fatalf("waiving the last unfinished blocker did not lift the park: %+v", dependent)
	}
}

func TestAbandonCarryDoesNotMoveAWaivedParkToTheSuccessor(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X00000000000000000Q100")

	if result, err := Open(verbReq(root, "01J5X00000000000000000Q110", "mac-a"), "dependent", "Wait for a blocker.", OriginHuman, "Resume when the dependency is waived."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open dependent: %+v %v", result, err)
	}
	risk := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "abandon blocker park fixture"}
	person := verbReq(root, "01J5X00000000000000000Q120", "mac-a")
	person.Actor.Human = "Wido"
	if result, err := OpenRisked(person, "blocker", "Block the dependent.", OriginHuman, "Resolve it.", "dependent", risk, 0, "", nil, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open blocker: %+v %v", result, err)
	}
	if result, err := Open(verbReq(root, "01J5X00000000000000000Q130", "mac-a"), "successor", "Carry unwaived dependencies.", OriginHuman, "Resolve the successor."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open successor: %+v %v", result, err)
	}

	person.Ulid = "01J5X00000000000000000Q140"
	result, err := Abandon(person, "blocker", AbandonSpec{
		Because: "the blocker moved except where waived",
		Carried: "successor",
		Waive:   []string{"dependent=the dependency no longer applies"},
	}, goalHumanProof(t, root, person.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon carried and waived blocker: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	dependent := tree.Live["dependent"]
	if dependent == nil || dependent.State != StateQueued || dependent.Parked != nil || len(dependent.Blocked) != 0 {
		t.Fatalf("waived dependent kept the carried successor edge or park: %+v", dependent)
	}
	if abandoned := tree.Abandoned["blocker"]; abandoned == nil || abandoned.Abandoned == nil || abandoned.Abandoned.Carried != "successor" {
		t.Fatalf("the abandoned blocker did not retain its carried successor: %+v", abandoned)
	}
}

func TestAbandonRefusalsStartWithAuthorityReasonAndArgumentGrammar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := verbReq(root, "01J5X000000000000000000A00", "mac-a")
	if _, err := Abandon(req, "goal-a", AbandonSpec{Because: "reason"}, nil); err == nil || err.Error() != "abandon is a human act and names its human (--by)" {
		t.Fatalf("authority name must refuse first: %v", err)
	}
	req.Actor.Human = "Wido"
	if _, err := Abandon(req, "goal-a", AbandonSpec{Because: "reason"}, nil); err == nil || err.Error() != "abandon requires freshly observed enrolled-terminal human authority" {
		t.Fatalf("fresh proof must refuse second: %v", err)
	}
	proof := goalHumanProof(t, root, req.Now)
	if _, err := Abandon(req, "goal-a", AbandonSpec{Because: "  "}, proof); err == nil || !strings.Contains(err.Error(), "owes the reader why") {
		t.Fatalf("one-line reason must refuse before reading the tree: %v", err)
	}
	if _, err := Abandon(req, "goal-a", AbandonSpec{Because: "reason", Waive: []string{"dependent"}}, proof); err == nil || err.Error() != "waive names its dependent and its reason: --waive <dependent>=<reason>" {
		t.Fatalf("waive grammar must refuse before reading the tree: %v", err)
	}
}

func TestAbandonCarriedRepointsEveryDependent(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))

	floorReq := verbReq(root, "01J5X000000000000000000A10", "mac-a")
	floorReq.Actor.Human = "Wido"
	if result, err := EngineFloor(floorReq, strings.Repeat("a", 40), goalHumanProof(t, root, floorReq.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("engine-floor: %+v %v", result, err)
	}
	for index, id := range []string{"blocker", "dependent", "dependent-two", "successor"} {
		if result, err := Open(verbReq(root, "01J5X000000000000000000B0"+string(rune('0'+index)), "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	edge := []string{"blocker"}
	for index, dependent := range []string{"dependent", "dependent-two"} {
		if result, err := Edit(verbReq(root, fmt.Sprintf("01J5X000000000000000000B1%d", index), "mac-a"), dependent, EditFields{Blocked: &edge}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("block %s: %+v %v", dependent, result, err)
		}
		approveGoalForTest(t, verbReq(root, fmt.Sprintf("01J5X000000000000000000B4%d", index), "mac-a"), dependent, testBudget())
	}

	abandonReq := verbReq(root, "01J5X000000000000000000B20", "mac-a")
	abandonReq.Actor.Human = "Wido"
	result, err := Abandon(abandonReq, "blocker", AbandonSpec{Because: "superseded"}, goalHumanProof(t, root, abandonReq.Now))
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal dependent is blocked by blocker") {
		t.Fatalf("uncovered dependent must refuse: %+v %v", result, err)
	}

	abandonReq = verbReq(root, "01J5X000000000000000000B30", "mac-a")
	abandonReq.Actor.Human = "Wido"
	result, err = Abandon(abandonReq, "blocker", AbandonSpec{Because: "superseded", Carried: "successor"}, goalHumanProof(t, root, abandonReq.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carried abandon: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["blocker"] != nil || tree.Abandoned["blocker"] == nil || tree.Abandoned["blocker"].Abandoned.Because != "superseded" {
		t.Fatalf("blocker did not move to abandoned: %+v", tree.Abandoned["blocker"])
	}
	for _, dependentID := range []string{"dependent", "dependent-two"} {
		dependent := tree.Live[dependentID]
		if got := dependent.Blocked; !reflect.DeepEqual(got, []string{"successor"}) {
			t.Fatalf("%s blockers = %v, want successor", dependentID, got)
		}
		line := dependent.History[len(dependent.History)-1]
		if line.Verb != "abandon" || !reflect.DeepEqual(line.Targets, []string{"blocker"}) || line.Reason != "blockedBy blocker re-pointed to successor" {
			t.Fatalf("%s did not retain the re-point event: %+v", dependentID, line)
		}
	}
	abandoned := tree.Abandoned["blocker"]
	abandonLine := abandoned.History[abandoned.Abandoned.Revision-1]
	if abandoned.Abandoned.Carried != "successor" || abandonLine.Carried != "successor" {
		t.Fatalf("successor was not retained in the record and event: record=%+v event=%+v", abandoned.Abandoned, abandonLine)
	}
	projection, err := Project(endpointFor(root), false, abandonReq.Now)
	if err != nil {
		t.Fatal(err)
	}
	frontier, err := Next(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(frontier.Blocked, []string{"dependent", "dependent-two"}) || !slices.Contains(frontier.Awaiting, "successor") {
		t.Fatalf("re-pointed frontier = %+v", frontier)
	}
}

func TestAbandonRefusesUncoveredLiveDependents(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X0000000000000000000A0")
	for index, id := range []string{"blocker", "dependent-one", "dependent-two"} {
		if result, err := Open(verbReq(root, []string{"01J5X0000000000000000000A1", "01J5X0000000000000000000A2", "01J5X0000000000000000000A3"}[index], "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	for index, id := range []string{"dependent-one", "dependent-two"} {
		blocked := []string{"blocker"}
		if result, err := Edit(verbReq(root, fmt.Sprintf("01J5X00000000000000000H1%d0", index), "mac-a"), id, EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("block %s: %+v %v", id, result, err)
		}
	}
	req := verbReq(root, "01J5X00000000000000000H200", "mac-a")
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "blocker", AbandonSpec{Because: "superseded"}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeRejected {
		t.Fatalf("uncovered dependents must reject atomically: %+v %v", result, err)
	}
	for _, id := range []string{"dependent-one", "dependent-two"} {
		if !strings.Contains(result.Detail, "goal "+id+" is blocked by blocker") {
			t.Fatalf("refusal omitted %s: %s", id, result.Detail)
		}
	}
	tree, loadErr := loadTree(root, result.Tip)
	if loadErr != nil || tree.Live["blocker"] == nil || tree.Abandoned["blocker"] != nil {
		t.Fatalf("uncovered-dependent refusal changed the ledger: live=%v abandoned=%v err=%v", sortedGoalIds(tree.Live), sortedGoalIds(tree.Abandoned), loadErr)
	}
}

func TestAbandonWaiveRemovesTheEdgeWithARecordedReason(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X0000000000000000000A0")
	for index, id := range []string{"blocker", "dependent", "bystander", "unrelated"} {
		if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000000V0%d0", index), "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	blocked := []string{"blocker", "bystander"}
	if result, err := Edit(verbReq(root, "01J5X00000000000000000V100", "mac-a"), "dependent", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("block dependent: %+v %v", result, err)
	}
	req := verbReq(root, "01J5X00000000000000000V200", "mac-a")
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "blocker", AbandonSpec{Because: "obsolete", Waive: []string{"dependent=the blocker no longer matters"}}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("waived abandon: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	dependent := tree.Live["dependent"]
	if !reflect.DeepEqual(dependent.Blocked, []string{"bystander"}) {
		t.Fatalf("waive removed the wrong blockers: %v", dependent.Blocked)
	}
	line := dependent.History[len(dependent.History)-1]
	if line.Verb != "abandon" || line.Reason != "blocker blocker waived: the blocker no longer matters" {
		t.Fatalf("waive reason was not retained: %+v", line)
	}

	if result, err := Open(verbReq(root, "01J5X00000000000000000V300", "mac-a"), "isolated", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open isolated: %+v %v", result, err)
	}
	badReq := verbReq(root, "01J5X00000000000000000V310", "mac-a")
	badReq.Actor.Human = "Wido"
	if result, err := Abandon(badReq, "isolated", AbandonSpec{Because: "obsolete", Waive: []string{"unrelated=not its dependent"}}, goalHumanProof(t, root, badReq.Now)); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not a live dependent") {
		t.Fatalf("waiving a non-dependent must refuse: %+v %v", result, err)
	}
}

func TestAbandonWaiveRefusesBlankAndDuplicateReasons(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := verbReq(root, "01J5X00000000000000000W000", "mac-a")
	req.Actor.Human = "Wido"
	proof := goalHumanProof(t, root, req.Now)
	tests := []struct {
		name string
		spec AbandonSpec
		want string
	}{
		{"blank", AbandonSpec{Because: "reason", Waive: []string{"dependent="}}, "waive names its dependent and its reason"},
		{"spaces are blank", AbandonSpec{Because: "reason", Waive: []string{"dependent=   "}}, "waive names its dependent and its reason"},
		{"duplicate different", AbandonSpec{Because: "reason", Waive: []string{"dependent=a", "dependent=b"}}, "dependent dependent is waived twice; name it once"},
		{"duplicate equal", AbandonSpec{Because: "reason", Waive: []string{"dependent=a", "dependent=a"}}, "dependent dependent is waived twice; name it once"},
		{"waive and also", AbandonSpec{Because: "reason", Waive: []string{"dependent=a"}, Also: []string{"dependent"}}, "dependent dependent is both waived and abandoned; choose one"},
		{"two waived and also sorted", AbandonSpec{Because: "reason", Waive: []string{"zeta=a", "alpha=b"}, Also: []string{"zeta", "alpha"}}, "dependent alpha is both waived and abandoned; choose one\ndependent zeta is both waived and abandoned; choose one"},
		{"newline", AbandonSpec{Because: "reason", Waive: []string{"dependent=a\nb"}}, "waive names its dependent and its reason"},
		{"also self", AbandonSpec{Because: "reason", Also: []string{"primary"}}, "carried must name a live successor"},
		{"waive self", AbandonSpec{Because: "reason", Waive: []string{"primary=not needed"}}, "carried must name a live successor"},
		{"carried self", AbandonSpec{Because: "reason", Carried: "primary"}, "carried must name a live successor"},
		{"carried also", AbandonSpec{Because: "reason", Carried: "dependent", Also: []string{"dependent"}}, "carried must name a live successor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Abandon(req, "primary", test.spec, proof)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}

func TestAbandonRefusalsAreOrderedInputFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := verbReq(root, "01J5X00000000000000000W100", "mac-a")
	defect := AbandonSpec{Because: " ", Waive: []string{"broken"}}
	if _, err := Abandon(req, "primary", defect, nil); err == nil || err.Error() != "abandon is a human act and names its human (--by)" {
		t.Fatalf("refusal 1 must win: %v", err)
	}
	req.Actor.Human = "Wido"
	if _, err := Abandon(req, "primary", defect, nil); err == nil || err.Error() != "abandon requires freshly observed enrolled-terminal human authority" {
		t.Fatalf("proof half of refusal 1 must win: %v", err)
	}
	proof := goalHumanProof(t, root, req.Now)
	if _, err := Abandon(req, "primary", defect, proof); err == nil || !strings.Contains(err.Error(), "owes the reader why") {
		t.Fatalf("refusal 2 must win: %v", err)
	}
	defect.Because = "reason"
	if _, err := Abandon(req, "primary", defect, proof); err == nil || !strings.Contains(err.Error(), "waive names") {
		t.Fatalf("refusal 3 must win: %v", err)
	}

	_, orderedRoot := oneClone(t)
	seedLedger(t, orderedRoot)
	if result, err := Open(verbReq(orderedRoot, "01J5X00000000000000000W110", "mac-a"), "primary", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open floor-order fixture: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(orderedRoot), false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := goalrevision.Acquire(orderedRoot, "primary", projection.Tree.Live["primary"].Revision, "floor-order-probe")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	jobDir := filepath.Join(orderedRoot, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "ordered-job.json"), []byte(`{"jobId":"ordered-job","goalId":"primary","status":"running"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	orderedReq := verbReq(orderedRoot, "01J5X00000000000000000W120", "mac-a")
	orderedReq.Actor.Human = "Wido"
	_, err = Abandon(orderedReq, "primary", AbandonSpec{Because: "reason"}, goalHumanProof(t, orderedRoot, orderedReq.Now))
	if err == nil || !strings.Contains(err.Error(), "ledger has no record that the fleet runs it") || strings.Contains(err.Error(), "LOCK_BUSY") || strings.Contains(err.Error(), "ordered-job") {
		t.Fatalf("refusal 4 must precede locks and job reads: %v", err)
	}
}

func TestAbandonRefusesWithoutProofOrAReason(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := verbReq(root, "01J5X00000000000000000W200", "mac-a")
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "reason"}, nil); err == nil || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("missing human: %v", err)
	}
	req.Actor.Human = "Wido"
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "reason"}, nil); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("missing proof: %v", err)
	}
	otherRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(otherRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "reason"}, goalHumanProof(t, otherRoot, req.Now)); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("wrong-root proof: %v", err)
	}
	proof := goalHumanProof(t, root, req.Now)
	for _, reason := range []string{"", "  ", "two\nlines", "two\rlines"} {
		if _, err := Abandon(req, "primary", AbandonSpec{Because: reason}, proof); err == nil || !strings.Contains(err.Error(), "owes the reader why") {
			t.Fatalf("reason %q: %v", reason, err)
		}
	}
}

func TestAbandonAlsoCascadesToNamedDependentsOnly(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X0000000000000000000A0")
	for index, id := range []string{"primary", "child", "grandchild", "outsider"} {
		if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000000X0%d0", index), "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	childBlocked := []string{"primary"}
	grandchildBlocked := []string{"child"}
	if result, err := Edit(verbReq(root, "01J5X00000000000000000X100", "mac-a"), "child", EditFields{Blocked: &childBlocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("wire child: %+v %v", result, err)
	}
	if result, err := Edit(verbReq(root, "01J5X00000000000000000X110", "mac-a"), "grandchild", EditFields{Blocked: &grandchildBlocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("wire grandchild: %+v %v", result, err)
	}
	req := verbReq(root, "01J5X00000000000000000X200", "mac-a")
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete", Also: []string{"child"}}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "grandchild") {
		t.Fatalf("unnamed transitive dependent must refuse: %+v %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000X210"
	result, err = Abandon(req, "primary", AbandonSpec{Because: "obsolete", Also: []string{"outsider"}}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != "abandon takes one goal; --also names only its live dependents" {
		t.Fatalf("non-dependent --also must refuse: %+v %v", result, err)
	}
	arguments, err := validateAbandonArguments("primary", AbandonSpec{Because: "obsolete", Also: []string{"outsider"}})
	if err != nil {
		t.Fatal(err)
	}
	tip, err := CaptureTip(endpointFor(root), "missing-also-projection")
	if err != nil {
		t.Fatal(err)
	}
	projectedTree, err := loadTree(root, tip)
	if err != nil {
		t.Fatal(err)
	}
	primaryRevision := projectedTree.Live["primary"].Revision
	_, err = abandonRequest(req, "primary", AbandonSpec{Because: "obsolete", Also: []string{"outsider"}}, arguments, map[string]uint64{"primary": primaryRevision}).Mutate(tip)
	if err == nil || err.Error() != "abandon takes one goal; --also names only its live dependents" || strings.Contains(err.Error(), "revision 0") {
		t.Fatalf("an --also goal absent from the projection did not reach refusal 8: %v", err)
	}
	dependentArguments, err := validateAbandonArguments("primary", AbandonSpec{Because: "obsolete", Also: []string{"child"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = abandonRequest(req, "primary", AbandonSpec{Because: "obsolete", Also: []string{"child"}}, dependentArguments, map[string]uint64{"primary": primaryRevision}).Mutate(tip)
	if err == nil || !strings.Contains(err.Error(), "goal child changed under abandon's lock") || strings.Contains(err.Error(), "revision 0") {
		t.Fatalf("a live dependent absent from the projection did not reach the record-moved refusal without revision zero: %v", err)
	}
	_, err = abandonRequest(req, "primary", AbandonSpec{Because: "obsolete"}, abandonArguments{waive: map[string]string{}, set: map[string]bool{"primary": true}}, map[string]uint64{}).Mutate(tip)
	if err == nil || err.Error() != "goal primary is not live; nothing to abandon" || strings.Contains(err.Error(), "revision 0") {
		t.Fatalf("a primary absent from the projection did not reach the not-live refusal before compare: %v", err)
	}
	req.Ulid = "01J5X00000000000000000X220"
	result, err = Abandon(req, "primary", AbandonSpec{Because: "obsolete", Also: []string{"child", "grandchild"}}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("named cascade: %+v %v", result, err)
	}
	tree, _ := loadTree(root, result.Tip)
	for _, id := range []string{"primary", "child", "grandchild"} {
		if tree.Abandoned[id] == nil {
			t.Fatalf("%s was not abandoned in the cascade", id)
		}
	}
}

func TestAbandonToleratesItsOwnUnfinishedPrerequisites(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X0000000000000000000A0")
	for index, id := range []string{"unfinished", "primary"} {
		if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000000Y0%d0", index), "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	blocked := []string{"unfinished"}
	if result, err := Edit(verbReq(root, "01J5X00000000000000000Y100", "mac-a"), "primary", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("wire prerequisite: %+v %v", result, err)
	}
	req := verbReq(root, "01J5X00000000000000000Y200", "mac-a")
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "primary", AbandonSpec{Because: "not worth waiting"}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("unfinished prerequisite blocked abandon: %+v %v", result, err)
	}
	tree, _ := loadTree(root, result.Tip)
	if tree.Live["unfinished"] == nil || tree.Abandoned["primary"] == nil {
		t.Fatalf("prerequisite or abandonment moved incorrectly: live=%v abandoned=%v", sortedGoalIds(tree.Live), sortedGoalIds(tree.Abandoned))
	}
}

func TestAbandonRefusesWithoutTheFleetFloor(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000Z000", "mac-a"), "primary", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	req := verbReq(root, "01J5X00000000000000000Z100", "mac-a")
	req.Actor.Human = "Wido"
	proof := goalHumanProof(t, root, req.Now)
	req.ConfigureAbandon(func() string { return strings.Repeat("a", 40) }, func(string, string, string) (bool, error) { return true, nil }, func(string, func(string, string) (bool, error), time.Time) ([]string, error) { return nil, nil })
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "ledger has no record that the fleet runs it") {
		t.Fatalf("missing floor: %v", err)
	}
	floorReq := verbReq(root, "01J5X00000000000000000Z110", "mac-a")
	floorReq.Actor.Human = "Wido"
	if result, err := EngineFloor(floorReq, strings.Repeat("b", 40), goalHumanProof(t, root, floorReq.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("engine floor: %+v %v", result, err)
	}
	req.abandon.buildStamp = func() string { return "dev" }
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "cannot be placed against the fleet floor") {
		t.Fatalf("bare dev build: %v", err)
	}
	req.abandon.buildStamp = func() string { return "dev-" + strings.Repeat("a", 40) + "-dirty" }
	req.abandon.isAncestor = func(string, string, string) (bool, error) { return false, nil }
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "(dirty build) is below the fleet floor") {
		t.Fatalf("dirty build below floor: %v", err)
	}
	req.abandon.buildStamp = func() string { return strings.Repeat("a", 40) }
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "is below the fleet floor") {
		t.Fatalf("clean build below floor: %v", err)
	}
	req.abandon.isAncestor = func(string, string, string) (bool, error) { return true, nil }
	req.abandon.registryProblems = func(string, func(string, string) (bool, error), time.Time) ([]string, error) {
		return []string{"checkout /fixture (LiveVerified) runs engine old, below the fleet floor " + strings.Repeat("b", 40) + "; rebuild and re-arm it (metasystem up), close or sweep the stale claim, or record a lower floor only if every other seat runs that"}, nil
	}
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "LiveVerified") {
		t.Fatalf("registry contradiction: %v", err)
	}

	req.abandon.registryProblems = func(string, func(string, string) (bool, error), time.Time) ([]string, error) { return nil, nil }
	jobDir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"jobId": "job-live", "goalId": "primary", "status": "running"})
	if err := os.WriteFile(filepath.Join(jobDir, "job-live.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Abandon(req, "primary", AbandonSpec{Because: "obsolete"}, proof); err == nil || !strings.Contains(err.Error(), "job-live") {
		t.Fatalf("passing floor did not proceed to refusal 5: %v", err)
	}
}

func TestAbandonCompactsTheDepartedPriorityLikeDone(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X0000000000000000000A0")
	for index, id := range []string{"rank-a", "rank-b", "rank-c"} {
		if result, err := Open(verbReq(root, fmt.Sprintf("01J5X00000000000000001A0%d0", index), "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
		priorityReq := verbReq(root, fmt.Sprintf("01J5X00000000000000001A1%d0", index), "mac-a")
		priorityReq.Actor.Human = "Wido"
		if result, err := SetPriority(priorityReq, id, 1, nil, goalHumanProof(t, root, priorityReq.Now)); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("rank %s: %+v %v", id, result, err)
		}
	}
	req := verbReq(root, "01J5X00000000000000001A200", "mac-a")
	req.Actor.Human = "Wido"
	result, err := Abandon(req, "rank-b", AbandonSpec{Because: "obsolete"}, goalHumanProof(t, root, req.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon ranked goal: %+v %v", result, err)
	}
	tree, _ := loadTree(root, result.Tip)
	if tree.Abandoned["rank-b"].Priority != 1 || tree.Abandoned["rank-b"].Sequence != 2 || tree.Live["rank-c"].Sequence != 2 {
		t.Fatalf("historical rank or compaction wrong: abandoned=%d:%d survivor=%d:%d", tree.Abandoned["rank-b"].Priority, tree.Abandoned["rank-b"].Sequence, tree.Live["rank-c"].Priority, tree.Live["rank-c"].Sequence)
	}
	departedLine := tree.Abandoned["rank-b"].History[len(tree.Abandoned["rank-b"].History)-1]
	survivorLine := tree.Live["rank-c"].History[len(tree.Live["rank-c"].History)-1]
	if len(departedLine.Targets) == 0 || departedLine.Targets[0] != "rank-b" || !reflect.DeepEqual(survivorLine.Targets, []string{"rank-b", "rank-c"}) {
		t.Fatalf("ordered compaction targets wrong: departed=%v survivor=%v", departedLine.Targets, survivorLine.Targets)
	}
}

func TestAbandonLocksEveryGoalInTheSetAndRefusesAnyRecordChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		competitor func(root string) (string, error)
		changedID  string
		admitD     bool
	}{
		{
			name: "the unclaimed dependent is claimed",
			competitor: func(root string) (string, error) {
				clear := []string{}
				if result, err := Edit(verbReq(root, "01J5X000000000000000001D35", "mac-b"), "lock-dependent", EditFields{Blocked: &clear}); err != nil || result.Outcome != OutcomeConfirmed {
					return "", fmt.Errorf("clear dependent blocker: %+v %v", result, err)
				}
				result, err := Claim(verbReq(root, "01J5X000000000000000001D40", "mac-b"), "lock-dependent")
				return "lock-dependent", publishMustConfirm("claim dependent", result, err)
			},
			changedID: "lock-dependent",
			admitD:    true,
		},
		{
			name: "the primary claim moves",
			competitor: func(root string) (string, error) {
				release := verbReq(root, "01J5X000000000000000001D50", "mac-b")
				release.Actor.Human = "Wido"
				release.Authority = goalHumanProof(t, root, release.Now)
				if result, err := Release(release, "lock-primary"); err != nil || result.Outcome != OutcomeConfirmed {
					return "", fmt.Errorf("release primary: %+v %v", result, err)
				}
				result, err := Claim(verbReq(root, "01J5X000000000000000001D60", "mac-b"), "lock-primary")
				return "lock-primary", publishMustConfirm("move primary claim", result, err)
			},
			changedID: "lock-primary",
		},
		{
			name: "the primary record is edited without moving its claim",
			competitor: func(root string) (string, error) {
				next := "competitor edit"
				edit := verbReq(root, "01J5X000000000000000001D70", "mac-b")
				edit.Actor.Human = "Wido"
				edit.Authority = goalHumanProof(t, root, edit.Now)
				result, err := Edit(edit, "lock-primary", EditFields{NextStep: &next})
				return "lock-primary", publishMustConfirm("edit primary", result, err)
			},
			changedID: "lock-primary",
		},
		{
			name: "a benign advancement changes neither record",
			competitor: func(root string) (string, error) {
				result, err := Open(verbReq(root, "01J5X000000000000000001D80", "mac-b"), "benign-other", "intent", "main", "next")
				return "", publishMustConfirm("open benign goal", result, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, root, competitor := twoClones(t)
			seedLedger(t, root)
			configureAbandonFloorTest(t, strings.Repeat("a", 40))
			recordAbandonFloorTest(t, root, "01J5X000000000000000001D00")
			risk := RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises tier-three goal admission."}
			for index, id := range []string{"lock-primary", "lock-dependent"} {
				result, err := Open(verbReq(root, []string{"01J5X000000000000000001D10", "01J5X000000000000000001D20"}[index], "mac-a"), id, "intent", "main", "next")
				if err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("open %s: %+v %v", id, result, err)
				}
				if result, err := Edit(verbReq(root, []string{"01J5X000000000000000001D15", "01J5X000000000000000001D25"}[index], "mac-a"), id, EditFields{Risk: &risk}); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("answer %s risk: %+v %v", id, result, err)
				}
			}
			blocked := []string{"lock-primary"}
			if result, err := Edit(verbReq(root, "01J5X000000000000000001D30", "mac-a"), "lock-dependent", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("wire dependent: %+v %v", result, err)
			}
			approveGoalForTest(t, verbReq(root, "01J5X000000000000000001E10", "mac-a"), "lock-primary", testBudget())
			approveGoalForTest(t, verbReq(root, "01J5X000000000000000001E20", "mac-a"), "lock-dependent", testBudget())
			if result, err := Claim(verbReq(root, "01J5X000000000000000001E30", "mac-a"), "lock-primary"); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("claim primary: %+v %v", result, err)
			}
			projection, err := Project(endpointFor(root), false, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			recordRevisions := map[string]uint64{}
			lockRevisions := map[string]uint64{}
			for _, id := range []string{"lock-primary", "lock-dependent"} {
				file := projection.Tree.Live[id]
				recordRevisions[id] = file.Revision
				lockRevisions[id] = file.Revision
				if file.Claimed != nil {
					lockRevisions[id] = file.Claimed.Revision
				}
			}

			injected := false
			beforePush := func(attempt int) error {
				if injected {
					return nil
				}
				injected = true
				for _, id := range []string{"lock-primary", "lock-dependent"} {
					lock, lockErr := goalrevision.Acquire(root, id, lockRevisions[id], "competitor-probe")
					if lockErr == nil {
						_ = lock.Release()
						return fmt.Errorf("concurrent acquire unexpectedly passed for %s", id)
					}
					if !strings.Contains(lockErr.Error(), "LOCK_BUSY") {
						return fmt.Errorf("concurrent acquire for %s did not name LOCK_BUSY: %w", id, lockErr)
					}
				}
				changedID, competitorErr := test.competitor(competitor)
				if competitorErr != nil {
					return competitorErr
				}
				if changedID != test.changedID {
					return fmt.Errorf("competitor changed %q, want %q", changedID, test.changedID)
				}
				if test.admitD {
					competitorProjection, projectErr := Project(endpointFor(competitor), false, time.Now())
					if projectErr != nil {
						return projectErr
					}
					claim := competitorProjection.Tree.Live["lock-dependent"].Claimed
					admissionNow, parseErr := time.Parse(time.RFC3339, claim.At)
					if parseErr != nil {
						return parseErr
					}
					if admissionErr := runGoalRevisionAdmissionCLI(competitor, "lock-dependent", claim.Revision, admissionNow); admissionErr != nil {
						return admissionErr
					}
				}
				return nil
			}
			req := verbReq(root, "01J5X000000000000000001E40", "mac-a")
			req.abandon.beforePush = beforePush
			req.Actor.Human = "Wido"
			result, err := Abandon(req, "lock-primary", AbandonSpec{Because: "superseded", Also: []string{"lock-dependent"}}, goalHumanProof(t, root, req.Now))
			if err != nil {
				t.Fatalf("abandon: %+v %v", result, err)
			}
			if test.changedID == "" {
				if result.Outcome != OutcomeConfirmed {
					t.Fatalf("unchanged records did not survive the compare: %+v", result)
				}
			} else {
				want := fmt.Sprintf("goal %s changed under abandon's lock (revision %d is now ", test.changedID, recordRevisions[test.changedID])
				if result.Outcome != OutcomeRejected || !strings.HasPrefix(result.Detail, want) || !strings.HasSuffix(result.Detail, "); re-read and retry") {
					t.Fatalf("record change did not refuse by goal and revisions: %+v, want prefix %q", result, want)
				}
			}
			for _, id := range []string{"lock-primary", "lock-dependent"} {
				lock, lockErr := goalrevision.Acquire(root, id, lockRevisions[id], "released-probe")
				if lockErr != nil {
					t.Fatalf("abandon did not release %s's lock: %v", id, lockErr)
				}
				_ = lock.Release()
			}
		})
	}
}

func publishMustConfirm(operation string, result PublishResult, err error) error {
	if err != nil || result.Outcome != OutcomeConfirmed {
		return fmt.Errorf("%s: %+v %v", operation, result, err)
	}
	return nil
}

func runGoalRevisionAdmissionCLI(root, id string, revision uint64, now time.Time) error {
	output, err := goalRevisionAdmissionCLI(root, id, revision, now)
	if err != nil {
		return fmt.Errorf("EvaluateGoalRevisionAdmission refused revision %d: %w: %s", revision, err, output)
	}
	return nil
}

func goalRevisionAdmissionCLI(root, id string, revision uint64, now time.Time) (string, error) {
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", err
	}
	command := exec.Command("go", "run", "./cmd/metasystem", "job", "goal-revision-admission",
		"--root", root, "--goal", id, "--revision", fmt.Sprint(revision), "--proposed-cap", "1", "--destructive-reach", "MECHANICAL")
	command.Dir = sourceRoot
	command.Env = append(os.Environ(), "METASYSTEM_GOAL_NOW="+now.UTC().Format(time.RFC3339))
	output, err := command.CombinedOutput()
	return string(output), err
}
