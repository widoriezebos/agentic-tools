package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// publications is how many ledger transactions the bed's repository took.
func (b *intentBed) publications() int { return b.repo.publications }

func (b *intentBed) expectNoEffect(before int, args []string, code int, result intentResult) {
	b.t.Helper()
	if code == 0 || result.Outcome != intentRefused || b.publications() != before {
		b.t.Fatalf("%v = exit %d %+v with %d publications (was %d); want a refusal with no effect", args, code, result, b.publications(), before)
	}
	if !strings.Contains(strings.ToLower(result.Summary), "nothing was") {
		b.t.Fatalf("%v refusal does not say nothing happened: %q", args, result.Summary)
	}
}

func unprovable(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
	return humanauthority.Proof{}, errors.New("no enrolled terminal among this process's ancestors")
}

func TestIntentPlanningIntakeAndEdit(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	before := bed.publications()
	missingRisk := []string{"open", "new-goal", "--intent", "Make it so.", "--next", "Measure it."}
	code, result := bed.runJSON(bed.owners(), missingRisk...)
	bed.expectNoEffect(before, missingRisk, code, result)
	if !slices.Contains(result.Data.(map[string]any)["missing"].([]any), "--risk") {
		t.Fatalf("open refusal does not name the missing risk answers: %+v", result)
	}

	intentFile := filepath.Join(t.TempDir(), "intent.txt")
	if err := os.WriteFile(intentFile, []byte("Make it so.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A seat opens only the defect blocking the goal it holds; the owner's
	// intake law refuses anything else.
	improvement := []string{"open", "new-goal", "--intent", "Make it so.", "--next", "Measure it.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "local tooling only"}
	code, result = bed.runJSON(bed.owners(), improvement...)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "--blocks") || bed.publications() != before {
		t.Fatalf("a seat's unblocking open = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "open", "new-goal", "--intent-file", intentFile, "--next", "Measure it.", "--blocks", bedGoal,
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "local tooling only", "--label", "speed")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("open = %d %+v", code, result)
	}
	opened := bed.goalFile("new-goal")
	if opened.Intent != "Make it so." || opened.NextStep != "Measure it." || opened.State != goal.StateQueued || !slices.Contains(opened.Labels, "speed") || opened.Risk == nil {
		t.Fatalf("opened goal = %+v", opened)
	}
	if blocked := bed.goalFile(bedGoal); !slices.Contains(blocked.Blocked, "new-goal") {
		t.Fatalf("the held goal does not wait for the opened blocker: %+v", blocked)
	}

	before = bed.publications()
	code, result = bed.runJSON(bed.owners(), "edit", "new-goal", "--next-append", "Then halve it.")
	if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before+1 {
		t.Fatalf("edit --next-append = %d %+v", code, result)
	}
	edited := bed.goalFile("new-goal")
	if edited.NextStep != "Measure it. Then halve it." || edited.Intent != "Make it so." || !slices.Contains(edited.Labels, "speed") {
		t.Fatalf("edit changed more than the appended next step: %+v", edited)
	}

	code, result = bed.runJSON(bed.owners(), "edit", "new-goal", "--intent", "Make it faster.")
	if code != 0 || bed.goalFile("new-goal").NextStep != "Measure it. Then halve it." || bed.goalFile("new-goal").Intent != "Make it faster." {
		t.Fatalf("edit --intent = %d %+v; goal %+v", code, result, bed.goalFile("new-goal"))
	}
}

func TestIntentPlanningArgumentConflicts(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	queued := queuedIntentGoal("queued-goal", 1)
	bed.addGoal(queued)
	before := bed.publications()
	for _, args := range [][]string{
		{"edit", bedGoal, "--next", "a", "--next-append", "b"},
		{"edit", bedGoal},
		{"edit", bedGoal, "--owner", "Wido"},
		{"edit", bedGoal, "--obligation", "DRAFT", "--next", "a"},
		{"claim", bedGoal, "--reason", "because"},
		{"claim", bedGoal, "--take-over"},
		{"pin", bedGoal, "m1e", "--clear"},
		{"prioritize", bedGoal, "4"},
		{"approve", "queued-goal", "--budget", "1d/10/720m/1/3", "--elapsed-limit", "1d", "--fixture-human-authority"},
		{"budget", "queued-goal", "--elapsed-limit", "1d", "--attempt-limit", "10"},
		{"goals", "--ready", "--tiers"},
		{"grant", "--tiers", "1", "--acts", "resume", "--until", "2026-09-05"},
		{"notes", bedGoal, "--add", "x"},
		{"notes", bedGoal, "--close", "r1", "--fixed", "abc", "--moved", "other"},
		{"resolve", bedGoal, "--review", "critic", "--finding", "F", "--test", "T", "--artifact", "a"},
		{"red", "close", "tr-1", "--goal", bedGoal, "--reason", "x"},
		{"abandon", bedGoal, "--reason", "x", "--successor", bedGoal},
		{"open", "other", "--intent", "a", "--intent-file", "b", "--next", "n"},
	} {
		code, result := bed.runJSON(bed.owners(), args...)
		bed.expectNoEffect(before, args, code, result)
		if code != 2 {
			t.Fatalf("%v is a mistake in the command line, exit %d want 2: %+v", args, code, result)
		}
	}
	if file := bed.goalFile("queued-goal"); file.Approved != nil {
		t.Fatalf("a refused approve recorded an approval: %+v", file)
	}

	// The five long limits are one box, taken the same way as the compact one.
	norm := tierBox(t, bed.root(), 1)
	code, result := bed.runJSON(bed.owners(), "approve", "queued-goal", "--fixture-human-authority", "--lineage", "m1",
		"--elapsed-limit", norm.ElapsedLimit, "--attempt-limit", fmt.Sprint(norm.AttemptLimit), "--reserved-job-minutes-limit", fmt.Sprint(norm.ReservedJobMinutesLimit),
		"--active-job-limit", fmt.Sprint(norm.ActiveJobLimit), "--review-round-limit", fmt.Sprint(norm.ReviewRoundLimit))
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("approve with the long limits = %d %+v", code, result)
	}
	if box := goalBoxString(bed.goalFile("queued-goal")); box != goalbudget.FormatBox(norm) {
		t.Fatalf("approved box = %s", box)
	}
}

func TestIntentPlanningClaimAndRelease(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	held := bed.goalFile(bedGoal)
	if held.State != goal.StateClaimed || held.Claimed == nil {
		t.Fatalf("fixture goal is not claimed: %+v", held)
	}
	before := bed.publications()
	code, result := bed.runJSON(bed.owners(), "claim")
	if code != 0 || result.Outcome != intentUnchanged || bed.publications() != before || len(result.Targets) != 1 || result.Targets[0].ID != bedGoal {
		t.Fatalf("claim without a goal while one is held = %d %+v", code, result)
	}

	other := *held
	other.Id = "second-held"
	bed.addGoal(&other)
	code, result = bed.runJSON(bed.owners(), "release", "--reason", "done for today")
	bed.expectNoEffect(before, []string{"release"}, code, result)
	if candidates := result.Data.(map[string]any)["candidates"].([]any); len(candidates) != 2 {
		t.Fatalf("ambiguous release names %v", candidates)
	}

	bed.lineage = ""
	code, result = bed.runJSON(bed.owners(), "release", "--reason", "done for today")
	bed.expectNoEffect(before, []string{"release"}, code, result)

	single := newIntentBed(t, false, nil)
	single.lineage = "m1"
	unreasoned := single.publications()
	code, result = single.runJSON(single.owners(), "release")
	single.expectNoEffect(unreasoned, []string{"release"}, code, result)
	code, result = single.runJSON(single.owners(), "release", "--reason", "handing it back")
	released := single.goalFile(bedGoal)
	if code != 0 || result.Outcome != intentConfirmed || released.State == goal.StateClaimed {
		t.Fatalf("release of the one held goal = %d %+v; goal %+v", code, result, released)
	}
	if last := released.History[len(released.History)-1]; last.Verb != "release" || last.Reason != "handing it back" {
		t.Fatalf("the release reason is not on the goal's history: %+v", last)
	}
}

func TestIntentPlanningHumanOnlyActs(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	agent := bed.owners()
	agent.prove = unprovable
	before := bed.publications()
	for _, args := range [][]string{
		{"pin", bedGoal, "m1e"},
		{"prioritize", bedGoal, "1"},
		{"unapprove", bedGoal, "--reason", "not yet"},
		{"claim", bedGoal, "--take-over", "--reason", "the holder is gone"},
		{"grant", "--tiers", "1", "--acts", "approve", "--until", "2026-09-05"},
		{"red", "close", "tr-1", "--reason", "fixed"},
	} {
		code, result := bed.runJSON(agent, args...)
		bed.expectNoEffect(before, args, code, result)
		if !strings.Contains(result.Summary, "person's act") {
			t.Fatalf("%v refusal does not name the person's act: %+v", args, result)
		}
	}
	// A typed name is never proof: with no enrolled person proven, --by
	// changes nothing.
	for _, args := range [][]string{
		{"pin", bedGoal, "m1e", "--by", "Wido"},
		{"claim", bedGoal, "--take-over", "--reason", "the holder is gone", "--by", "Wido", "--lineage", "m1"},
		{"red", "close", "tr-1", "--reason", "fixed", "--by", "Wido"},
	} {
		code, result := bed.runJSON(agent, args...)
		bed.expectNoEffect(before, args, code, result)
	}
	// A proven terminal does not lend its authority to a different name.
	code, result := bed.runJSON(bed.owners(), "pin", bedGoal, "m1e", "--by", "Mallory")
	bed.expectNoEffect(before, []string{"pin", "--by", "Mallory"}, code, result)
	if !strings.Contains(result.Summary, "not the person enrolled") {
		t.Fatalf("a mismatched --by is not named: %+v", result)
	}
	// A holder's own act takes no person's name.
	code, result = bed.runJSON(agent, "ready", bedGoal, "--by", "Wido")
	bed.expectNoEffect(before, []string{"ready"}, code, result)

	// At the enrolled terminal the name is filled from the proof.
	code, result = bed.runJSON(bed.owners(), "prioritize", bedGoal, "1")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).Priority != 1 {
		t.Fatalf("prioritize at the enrolled terminal = %d %+v", code, result)
	}
	// A proven, matching --by pins; the observed proof reaches the owner.
	code, result = bed.runJSON(bed.owners(), "pin", bedGoal, "mac-cli", "--by", "Wido")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).Pinned != "mac-cli" {
		t.Fatalf("pin at the enrolled terminal = %d %+v", code, result)
	}
}

func TestIntentPlanningAttorneyGrant(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	code, result := bed.runJSON(bed.owners(), "grant", "--tiers", "1", "--acts", "approve,budget,resume-parked", "--until", "2026-09-05")
	if code != 0 || result.Outcome != intentConfirmed || len(result.Targets) != 1 || result.Targets[0].Kind != "grant" {
		t.Fatalf("grant = %d %+v", code, result)
	}
	acts := result.Data.(map[string]any)["acts"].([]any)
	if !slices.Equal([]string{acts[0].(string), acts[1].(string), acts[2].(string)}, []string{"approve", "set-budget", "unpark"}) {
		t.Fatalf("grant acts map to %v", acts)
	}
	entry := result.Targets[0].ID
	code, result = bed.runJSON(bed.owners(), "grant", "--tiers", "1", "--acts", "approve", "--until", "2026-12-31")
	if code == 0 || result.Outcome != intentRefused {
		t.Fatalf("a grant beyond seven days = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "revoke", entry)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("revoke %s = %d %+v", entry, code, result)
	}
}

// writeCriticChain records one critic chain with an open severe finding and a
// follow-up round, as the review owner leaves it.
func writeCriticChain(t *testing.T, root string) {
	t.Helper()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	finding := map[string]any{
		"findingId": "S-1", "critic": "critic", "rigorClass": "severe",
		"factsDigest": strings.Repeat("a", 64), "facts": map[string]any{"local": true},
		"artifact": "metasystem/in.go", "title": "severe finding", "status": "open",
		"resolution": "", "decisionOpid": "", "evidence": "direct proof",
		"evidenceDigest": strings.Repeat("b", 64), "multiplicity": 1,
	}
	writeTemp(t, jobs, "critic.json", map[string]any{
		"jobId": "critic", "role": "design-critic", "round": 1, "parentJob": nil,
		"status": "completed", "goalId": bedGoal, "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3, "demotions": []any{},
		"findingRegister": []any{finding},
	})
	writeTemp(t, jobs, "critic-r2.json", map[string]any{"jobId": "critic-r2", "role": "design-critic", "round": 2, "parentJob": "critic", "status": "completed"})
}

func TestIntentPlanningDecidePartial(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	writeCriticChain(t, bed.root())
	before := bed.publications()
	code, result := bed.runJSON(bed.owners(), "decide", bedGoal, "--finding", "S-1", "--review", "missing-job", "--reason", "bounded exposure")
	bed.expectNoEffect(before, []string{"decide"}, code, result)

	// The accepted-risk register cannot be written, so only the goal act lands.
	counselor := filepath.Join(bed.root(), "records", "counselor")
	if err := os.MkdirAll(filepath.Dir(counselor), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(counselor, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result = bed.runJSON(bed.owners(), "decide", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure")
	if code == 0 || result.Outcome != intentPartial || bed.publications() != before+1 {
		t.Fatalf("decide with an unwritable register = %d %+v (publications %d, was %d)", code, result, bed.publications(), before)
	}
	if data := result.Data.(map[string]any); data["chain"] != "critic" {
		t.Fatalf("decide did not derive the chain root from the round job: %+v", data)
	}
	if risks := bed.goalFile(bedGoal).AcceptedRisks; len(risks) != 1 || risks[0].Finding != "S-1" || risks[0].By != "Wido" {
		t.Fatalf("the goal's accepted risk = %+v", risks)
	}
}

func TestIntentPlanningReadyTiersAndNotes(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	code, result := bed.runJSON(bed.owners(), "goals", "--ready")
	if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "continue your claimed goal: "+bedGoal) {
		t.Fatalf("goals --ready = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "goals", "--tiers")
	if code != 0 || result.Outcome != intentConfirmed || result.Data.(map[string]any)["recorded"] == nil {
		t.Fatalf("goals --tiers = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "notes", bedGoal, "--read", "r2", "--add", "help wraps at 80 columns", "--add", "one more")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("notes --add = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.owners(), "notes", bedGoal)
	if items := result.Data.(map[string]any)["items"].([]any); code != 0 || len(items) != 2 {
		t.Fatalf("notes = %d %+v", code, result)
	}
}

// intentPlanningDisposition is where each goal-family verb in this group is
// reached from the public surface.
var intentPlanningDisposition = map[string]string{
	"open": "open", "edit": "edit", "set-next": "edit", "set-obligation": "edit", "claim": "claim", "steal": "claim",
	"release": "release", "land-ready": "ready", "accept-risk": "decide", "set-pin": "pin", "set-priority": "prioritize",
	"reopen": "reopen", "abandon": "abandon", "carry": "abandon", "block": "block", "unblock": "unblock",
	"unapprove": "unapprove", "grant": "grant", "revoke": "revoke", "split": "split", "set-arc": "group", "detach": "ungroup",
	"discharge-review-obligation": "resolve", "read-items": "notes", "recover": "recover", "trunk-red": "red",
	"next": "goals", "tier-probe": "goals", "set-budget": "budget",
}

func TestIntentPlanningDescriptorCoverage(t *testing.T) {
	t.Parallel()
	var goalVerbs []string
	for _, registered := range families() {
		if registered.name == "goal" {
			for _, entry := range registered.verbs {
				goalVerbs = append(goalVerbs, entry.name)
			}
		}
	}
	for verb, public := range intentPlanningDisposition {
		if !slices.Contains(goalVerbs, verb) {
			t.Fatalf("goal %s is no longer registered", verb)
		}
		command, ok := findIntentCommand(public)
		if !ok {
			t.Fatalf("goal %s is reached through %s, which is not a public command", verb, public)
		}
		if command.run == nil || command.summary == "" || len(command.usage) == 0 || len(command.examples) == 0 {
			t.Fatalf("%s descriptor is incomplete", public)
		}
	}
	expect := map[string][]string{
		"approve":    {"elapsed-limit", "attempt-limit", "reserved-job-minutes-limit", "active-job-limit", "review-round-limit"},
		"budget":     {"elapsed-limit", "attempt-limit", "reserved-job-minutes-limit", "active-job-limit", "review-round-limit"},
		"goals":      {"ready", "tiers", "machine"},
		"open":       {"risk", "basis", "tier", "reason", "origin", "blocked-by", "blocks", "label", "intent-file", "next-file"},
		"edit":       {"next-append", "evidence", "unlabel", "obligation", "owner", "recurrence", "platform", "toolchain-identity", "surface-digest", "max-active-jobs", "timing-envelope-sec", "effect", "value-judgment", "reversibility", "severe-harm", "unfamiliar-approach", "test-discrimination", "correlated-assumption-risk", "authority-scope-change", "destructive-reach", "approved-ref"},
		"claim":      {"take-over", "reason", "arc", "budget"},
		"abandon":    {"successor", "waive", "also"},
		"resolve":    {"test", "implementation-chain", "artifact", "result", "critic"},
		"prioritize": {"sequence"},
		"grant":      {"tiers", "acts", "until"},
		"notes":      {"add", "add-file", "read", "close", "fixed", "moved", "accepted"},
		"red":        {"goal", "branch", "to", "reason"},
		"recover":    {"session"},
	}
	for name, flags := range expect {
		command, _ := findIntentCommand(name)
		var help bytes.Buffer
		writeIntentCommandHelp(&help, command)
		for _, flag := range flags {
			definition, ok := command.lookupFlag(flag)
			// Compatibility-only binding options stay parseable and are
			// absent from help.
			if !ok || definition.hidden == strings.Contains(help.String(), "--"+flag+" ") {
				t.Fatalf("%s does not take --%s, or its help does not match its visibility", name, flag)
			}
		}
	}
	// The owner has retired claiming in the same act as opening; open does
	// not advertise it.
	open, _ := findIntentCommand("open")
	for _, retired := range []string{"claim", "budget", "elapsed-limit", "attempt-limit", "reserved-job-minutes-limit", "active-job-limit", "review-round-limit"} {
		if _, ok := open.lookupFlag(retired); ok {
			t.Fatalf("open still advertises --%s", retired)
		}
	}
	for _, command := range intentPlanningCommands() {
		for _, definition := range command.flags {
			if definition.usage == "" {
				t.Fatalf("%s --%s has no help", command.name, definition.name)
			}
		}
		var stdout, stderr bytes.Buffer
		failing := intentOwners{}
		if code := runIntentIn(command, []string{"--help"}, &stdout, &stderr, t.TempDir(), failing); code != 0 || !strings.Contains(stdout.String(), command.usage[0]) {
			t.Fatalf("%s --help = %d %q %q", command.name, code, stdout.String(), stderr.String())
		}
	}
}

// blockProofRecords makes the authority proof records unwritable, so an act
// lands and its proof record fails afterwards.
func blockProofRecords(t *testing.T, root string) {
	t.Helper()
	proofs := filepath.Join(root, "artifacts", "agents", "authority", "proofs")
	if err := os.MkdirAll(filepath.Dir(proofs), 0o755); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(proofs); err == nil && info.IsDir() {
		if err := os.Chmod(proofs, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(proofs, 0o755) })
		return
	}
	// The regular-file obstruction goes through the executable owner too,
	// so every write at this path is under the fork lock.
	if err := testexec.WriteFile(proofs, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectPartial(t *testing.T, bed *intentBed, before int, args []string, code int, result intentResult) {
	t.Helper()
	if code == 0 || result.Outcome != intentPartial || bed.publications() != before+1 {
		t.Fatalf("%v with an unwritable proof record = %d %+v (publications %d, was %d)", args, code, result, bed.publications(), before)
	}
	data := result.Data.(map[string]any)
	owner := data["owner"].(map[string]any)
	if owner["outcome"] != string(goal.OutcomeConfirmed) || owner["tip"] == "" || len(data["incomplete"].([]any)) == 0 || !strings.Contains(result.Summary, "landed") {
		t.Fatalf("%v partial result does not keep its publication: %+v", args, result)
	}
}

func TestIntentPlanningPartialAfterPublication(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	code, result := bed.runJSON(bed.owners(), "grant", "--tiers", "1", "--acts", "approve", "--until", "2026-09-05")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("grant = %d %+v", code, result)
	}
	entry := result.Targets[0].ID
	blockProofRecords(t, bed.root())
	for _, args := range [][]string{
		{"grant", "--tiers", "1", "--acts", "budget", "--until", "2026-09-05"},
		{"revoke", entry},
		{"unapprove", bedGoal, "--reason", "the design changes first"},
	} {
		before := bed.publications()
		code, result := bed.runJSON(bed.owners(), args...)
		expectPartial(t, bed, before, args, code, result)
	}
}

func TestIntentPlanningResolveByOwningSession(t *testing.T) {
	t.Parallel()
	obligation := func(file *goal.GoalFile) {
		file.ReviewObligations = append(file.ReviewObligations, goal.ReviewObligation{
			Finding: "F-1", Chain: "critic", Artifact: "metasystem/in.go", Test: "pending", State: "open"})
	}
	foreign := newIntentBed(t, false, obligation)
	writeCriticChain(t, foreign.root())
	foreign.lineage = "m9"
	before := foreign.publications()
	code, result := foreign.runJSON(foreign.owners(), "resolve", bedGoal, "--review", "critic-r2", "--finding", "F-1", "--test", "TestIntentReady")
	if code == 0 || result.Outcome != intentRefused || foreign.publications() != before || !strings.Contains(result.Summary, "owning pair") {
		t.Fatalf("a foreign session's resolve = %d %+v", code, result)
	}

	owning := newIntentBed(t, false, obligation)
	writeCriticChain(t, owning.root())
	owning.lineage = "m1"
	code, result = owning.runJSON(owning.owners(), "resolve", bedGoal, "--review", "critic-r2", "--finding", "F-1", "--test", "TestIntentReady")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the owning session's resolve = %d %+v", code, result)
	}
	resolved := owning.goalFile(bedGoal)
	if got := resolved.ReviewObligations[len(resolved.ReviewObligations)-1]; got.State != "discharged" || got.Test != "TestIntentReady" {
		t.Fatalf("the obligation was not discharged: %+v", got)
	}
	if last := resolved.History[len(resolved.History)-1]; last.Verb != "discharge-review-obligation" || strings.HasPrefix(last.Actor, "human:") {
		t.Fatalf("the owning session's discharge is attributed to a person: %+v", last)
	}
}

// TestIntentPlanningNotesAuthority drives the public notes add and close on a
// goal another pair holds: a proven person's add and close reach the owner
// with their proof; a typed name without proof and another agent change
// nothing; the owning agent records its own note.
func TestIntentPlanningNotesAuthority(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	notes := func(file *goal.GoalFile) []goal.ReadItem { return file.ReadItems }
	before := bed.publications()

	agent := bed.owners()
	agent.prove = unprovable
	for _, args := range [][]string{
		{"notes", bedGoal, "--read", "r2", "--add", "named only", "--by", "Wido"},
		{"notes", bedGoal, "--read", "r2", "--add", "named only", "--by", "Wido", "--lineage", "m9"},
	} {
		code, result := bed.runJSON(agent, args...)
		bed.expectNoEffect(before, args, code, result)
	}
	bed.lineage = "m9"
	code, result := bed.runJSON(bed.owners(), "notes", bedGoal, "--read", "r2", "--add", "not mine")
	if code == 0 || result.Outcome != intentRefused || bed.publications() != before || !strings.Contains(result.Summary, "human act") {
		t.Fatalf("another agent's note = %d %+v", code, result)
	}

	bed.lineage = ""
	code, result = bed.runJSON(bed.owners(), "notes", bedGoal, "--read", "r2", "--add", "a person's note")
	if code != 0 || result.Outcome != intentConfirmed || len(notes(bed.goalFile(bedGoal))) != 1 {
		t.Fatalf("a proven person's note on another pair's claim = %d %+v", code, result)
	}
	item := notes(bed.goalFile(bedGoal))[0]
	code, result = bed.runJSON(bed.owners(), "notes", bedGoal, "--close", item.ID, "--accepted", "not a defect")
	if code != 0 || result.Outcome != intentConfirmed || notes(bed.goalFile(bedGoal))[0].State == goal.ReadItemOpen {
		t.Fatalf("a proven person's close on another pair's claim = %d %+v", code, result)
	}

	bed.lineage = "m1"
	code, result = bed.runJSON(bed.owners(), "notes", bedGoal, "--read", "r3", "--add", "the owner's own note")
	if code != 0 || result.Outcome != intentConfirmed || len(notes(bed.goalFile(bedGoal))) != 2 {
		t.Fatalf("the owning agent's note = %d %+v", code, result)
	}
}
