package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func newPartnerActBed(t *testing.T) *partnerBed {
	t.Helper()
	b := newPartnerBed(t)
	if code, result := b.runJSON(b.owners(), "partner", "start", "--for", "1w"); code != 0 {
		t.Fatalf("start: %d %+v", code, result)
	}
	file := b.goalFile(bedGoal)
	makeQueued(file)
	b.addGoal(file)
	b.lineage, b.holder.OwnerLineage = "project-partner", "project-partner"
	return b
}

// These are the real admission and goal owners. Only their process ancestry,
// ledger transport, and clock are supplied by the isolated fixture.
func partnerActOwners(b *partnerBed, impact string, notice *bytes.Buffer) intentOwners {
	o := b.owners()
	a := &attorneyAdmitter{owners: attorneyAdmitOwners{
		grants: o.attorney.entries, holder: o.attorney.holder,
		machine: func(string) (string, error) { return o.dependencies.machine(b.root()) },
		classify: func(string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassMain, MainId: b.holder.MainId}, nil
		},
		fixture: func(string) bool { return false }, verb: func() string { return "goal edit " + bedGoal },
		impact: func() string { return impact }, stderr: notice,
		lockShared: func(string) error { return nil },
	}}
	o.attorney.admit = a.admit
	o.prove = func(root string, pid int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		grant, admitted := a.admit(root, pid, now)
		if !admitted {
			return humanauthority.Proof{}, fmt.Errorf("the grant did not admit this session")
		}
		return humanauthority.HelmProof(root, grant, now)
	}
	o.dependencies.proveHuman = func(root string, pid int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return o.prove(root, pid, reader, "", "", now)
	}
	return o
}

func TestPartnerActNeedsImpactAndIsRecordedThroughTheGrant(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	var notice bytes.Buffer
	owners := partnerActOwners(b, "", &notice)
	now, _ := b.commandNow(b.root())
	if _, admitted := owners.attorney.admit(b.root(), 80, now); admitted {
		t.Fatal("a partner grant answered without an impact")
	}
	impact := "Changes the next step; undo: restore Run it."
	owners = partnerActOwners(b, impact, &notice)
	code, result := b.runJSON(owners, "goal", "edit", bedGoal, "--next", "Review it.", "--impact", impact)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("edit: %d %+v", code, result)
	}
	file := b.goalFile(bedGoal)
	last := file.History[len(file.History)-1]
	grant := b.rootRecord().PowerOfAttorney[0].ID
	if file.NextStep != "Review it." || last.Actor != "human:Wido" || last.Through != grant || file.History[0].Through != "" {
		t.Fatalf("history: %+v", file)
	}
	if !strings.HasPrefix(notice.String(), "IMPACT: "+impact+"\nPOWER OF ATTORNEY") {
		t.Fatalf("first printed line: %q", notice.String())
	}
	logBytes, _ := os.ReadFile(attorneyLogPath(b.root()))
	log := string(logBytes)
	if !strings.Contains(log, "answered grant="+grant) || !strings.Contains(log, fmt.Sprintf("impact=%q", impact)) || !strings.Contains(log, "refused grant="+grant) {
		t.Fatalf("grant log: %s", log)
	}
	code, shown, stderr := b.run(owners, "goal", "show", bedGoal, "--history")
	if code != 0 || !strings.Contains(shown, "Wido, through the partner") {
		t.Fatalf("show: %d %s %s", code, shown, stderr)
	}
	t.Logf("goal show --history:\n%s", shown)
	code, result = b.runJSON(owners, "goal", "pause", bedGoal, "--reason", "Review first.", "--impact", impact)
	file = b.goalFile(bedGoal)
	last = file.History[len(file.History)-1]
	if code != 0 || last.Actor != "human:Wido" || last.Through != grant {
		t.Fatalf("pause lost the grant: %d %+v history=%+v", code, result, last)
	}
}

func TestPartnerEditWithoutImpactLeavesTheGoalUnchanged(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	before := goal.RenderFile(b.goalFile(bedGoal))
	publications := b.repo.publications
	for _, extra := range [][]string{nil, {"--impact", "  "}, {"--lineage", "ordinary-agent"}} {
		var notice bytes.Buffer
		code, result := b.runJSON(partnerActOwners(b, "", &notice), append([]string{"goal", "edit", bedGoal, "--next", "Changed."}, extra...)...)
		if code == 0 || result.Summary != "goal edit is the person's act through the partner; --impact is missing; nothing was done" || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), `--impact WHY; undo: HOW`) {
			t.Fatalf("missing impact: %d %+v", code, result)
		}
	}
	if !bytes.Equal(before, goal.RenderFile(b.goalFile(bedGoal))) || b.repo.publications != publications {
		t.Fatal("a refused edit reached the owner or changed the goal")
	}
}

func TestPartnerEditInAnotherCheckoutLeavesTheGoalUnchanged(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{"absent", "corrupt", "unreadable"} {
		t.Run(declaration, func(t *testing.T) {
			b := newPartnerActBed(t)
			other := newProcessBed(t)
			if err := os.MkdirAll(filepath.Dir(brain.Path(other.root())), 0o700); err != nil {
				t.Fatal(err)
			}
			if declaration == "corrupt" {
				if err := os.WriteFile(brain.Path(other.root()), []byte("not JSON"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if declaration == "unreadable" {
				if err := os.Mkdir(brain.Path(other.root()), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			owners := other.owners()
			owners.dependencies.ownerLineage = b.owners().dependencies.ownerLineage
			owners.attorney = b.owners().attorney
			before := goal.RenderFile(other.goalFile(bedGoal))
			publications := other.repo.publications
			for _, extra := range [][]string{nil, {"--impact", "Change it; undo: restore it."}} {
				args := append([]string{"goal", "edit", bedGoal, "--next", "Changed.", "--repo", other.root()}, extra...)
				code, result := b.runJSON(owners, args...)
				if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "the partner acts only from its own checkout in this step") || !strings.Contains(result.Summary, other.root()) || !strings.HasSuffix(result.Summary, "; nothing was done") {
					t.Fatalf("edit in another checkout: %d %+v", code, result)
				}
				if !bytes.Equal(before, goal.RenderFile(other.goalFile(bedGoal))) || other.repo.publications != publications {
					t.Fatal("a refused edit reached the owner or changed the goal")
				}
			}
			for _, args := range [][]string{{"goal", "show", bedGoal, "--repo", other.root()}, {"system", "status", "--repo", other.root(), "--installation", other.root()}} {
				if code, result := b.runJSON(owners, args...); code != 0 {
					t.Fatalf("a read in another checkout was refused: %d %+v", code, result)
				}
			}
		})
	}
}

func TestPartnerEditInAnotherDeclaredCheckoutNeverReadsItsGrant(t *testing.T) {
	t.Parallel()
	b, other := newPartnerActBed(t), newPartnerActBed(t)
	owners := other.owners()
	owners.dependencies.ownerLineage = b.owners().dependencies.ownerLineage
	owners.attorney = b.owners().attorney
	reads := 0
	owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) {
		reads++
		return other.rootRecord().PowerOfAttorney, nil
	}
	before := goal.RenderFile(other.goalFile(bedGoal))
	publications := other.repo.publications
	for _, extra := range [][]string{nil, {"--impact", "Change it; undo: restore it."}} {
		args := append([]string{"goal", "edit", bedGoal, "--next", "Changed.", "--repo", other.root()}, extra...)
		code, result := b.runJSON(owners, args...)
		want := "goal edit is the person's act through the partner; the partner acts only from its own checkout in this step (requested checkout: " + other.root() + "); nothing was done"
		if code == 0 || result.Outcome != intentRefused || result.Summary != want || reads != 0 {
			t.Fatalf("edit in another declared checkout: %d %+v grant reads=%d", code, result, reads)
		}
		if !bytes.Equal(before, goal.RenderFile(other.goalFile(bedGoal))) || other.repo.publications != publications {
			t.Fatal("a refused edit reached the owner or changed the goal")
		}
	}
	if code, result := b.runJSON(owners, "goal", "show", bedGoal, "--repo", other.root()); code != 0 || reads != 0 {
		t.Fatalf("a read in another declared checkout was refused or read a grant: %d %+v grant reads=%d", code, result, reads)
	}
}

func TestPartnerWorkStatusInAnotherCheckoutNeedsNoGrant(t *testing.T) {
	t.Parallel()
	b, other := newPartnerBed(t), newIntentBed(t, false, makeQueued)
	b.lineage = "project-partner"
	owners := other.owners()
	owners.dependencies.ownerLineage = b.owners().dependencies.ownerLineage
	owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) {
		return nil, nil
	}
	if code, result := b.runJSON(owners, "work", "status", "--repo", other.root()); code != 0 {
		t.Fatalf("status in another checkout: %d %+v", code, result)
	}
	code, result := b.runJSON(owners, "goal", "edit", bedGoal, "--next", "Changed.", "--repo", other.root(), "--impact", "Change it; undo: restore it.")
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "the partner acts only from its own checkout in this step") {
		t.Fatalf("write in another checkout: %d %+v", code, result)
	}
}

func TestPartnerEditAfterRevokeLeavesTheGoalUnchanged(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	grant := b.rootRecord().PowerOfAttorney[0].ID
	b.lineage = ""
	if code, result := b.runJSON(b.owners(), "grant", "revoke", grant); code != 0 {
		t.Fatalf("revoke: %d %+v", code, result)
	}
	b.lineage = "project-partner"
	before := goal.RenderFile(b.goalFile(bedGoal))
	publications := b.repo.publications
	var notice bytes.Buffer
	code, result := b.runJSON(partnerActOwners(b, "Change it; undo: restore it.", &notice), "goal", "edit", bedGoal, "--next", "Changed.", "--impact", "Change it; undo: restore it.")
	if code == 0 || !strings.HasPrefix(result.Summary, "goal edit is the person's act through the partner; the grant does not admit this session (revoked ") || !strings.HasSuffix(result.Summary, " by Wido); nothing was done") || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem partner start" {
		t.Fatalf("revoked edit: %d %+v", code, result)
	}
	if !bytes.Equal(before, goal.RenderFile(b.goalFile(bedGoal))) || b.repo.publications != publications {
		t.Fatal("an edit after revoke reached the owner or changed the goal")
	}
	for _, read := range [][]string{{"goal", "show", bedGoal}, {"goal", "budget", bedGoal}} {
		if code, result := b.runJSON(b.owners(), read...); code != 0 {
			t.Fatalf("a read after revoke was refused: %d %+v", code, result)
		}
	}
	logBytes, _ := os.ReadFile(attorneyLogPath(b.root()))
	log := string(logBytes)
	if !strings.Contains(log, "refused grant="+grant) || !strings.Contains(log, "revoked") {
		t.Fatalf("refusal log: %s", log)
	}
	t.Log(result.Summary)
}

func TestPartnerOtherActorSelectionsNeedImpact(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	for _, act := range [][]string{
		{"goal", "budget", bedGoal, "norm"}, {"goal", "pause", bedGoal, "--reason", "Wait."},
		{"system", "restart"}, {"ui", "restart"}, {"settings", "set", "steward.tick-sec", "300"},
	} {
		var notice bytes.Buffer
		if code, result := b.runJSON(partnerActOwners(b, "", &notice), act...); code == 0 || !strings.Contains(result.Summary, "; --impact is missing; nothing was done") {
			t.Fatalf("%v: %d %+v", act, code, result)
		}
	}
}

func TestPartnerNotesReadsSurviveRevocationAndWritesStayGuarded(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	file := b.goalFile(bedGoal)
	file.ReadItems = []goal.ReadItem{{ID: "review-1", Read: "review", State: goal.ReadItemOpen, AddedAt: file.OpenedAt, Text: "Keep this note."}}
	b.addGoal(file)
	path := filepath.Join(b.root(), "notes.txt")
	if err := os.WriteFile(path, []byte("Another note.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	impact := "Changes notes; undo: restore the notes."
	for _, revoked := range []bool{false, true} {
		if revoked {
			b.lineage = ""
			if code, result := b.runJSON(b.owners(), "grant", "revoke", b.rootRecord().PowerOfAttorney[0].ID); code != 0 {
				t.Fatalf("revoke: %d %+v", code, result)
			}
			b.lineage = "project-partner"
		}
		before, publications := goal.RenderFile(b.goalFile(bedGoal)), b.repo.publications
		for _, extra := range [][]string{nil, {"--impact", impact}} {
			var notice bytes.Buffer
			owners := partnerActOwners(b, impact, &notice)
			code, shown, stderr := b.run(owners, append([]string{"goal", "notes", bedGoal}, extra...)...)
			if code != 0 || !strings.Contains(shown, "review-1 [open] review: Keep this note.") || notice.Len() != 0 {
				t.Fatalf("read (revoked=%v): %d %s %s notice=%s", revoked, code, shown, stderr, &notice)
			}
			if !revoked && len(extra) != 0 {
				continue
			}
			for _, write := range [][]string{{"--add", "Another note.", "--read", "review"}, {"--add-file", path, "--read", "review"}, {"--close", "review-1", "--accepted", "No longer needed."}} {
				args := append(append([]string{"goal", "notes", bedGoal}, write...), extra...)
				code, result := b.runJSON(owners, args...)
				reason := "--impact is missing"
				if revoked {
					reason = "the grant does not admit this session (revoked "
				}
				if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "goal notes is the person's act through the partner; "+reason) {
					t.Fatalf("write %v (revoked=%v): %d %+v", write, revoked, code, result)
				}
			}
		}
		if !bytes.Equal(before, goal.RenderFile(b.goalFile(bedGoal))) || b.repo.publications != publications {
			t.Fatal("reading or a refused write changed the goal or published the ledger")
		}
	}
}

func TestPartnerWorkActionsAreRefusedBeforeTheirOwners(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	impact := "Drives work; undo: restore the work."
	for _, args := range [][]string{
		{"work", "build", bedGoal}, {"work", "build", "run:another-seat"}, {"work", "build", "--plan", "plan.json"},
		{"work", "revise", bedGoal}, {"work", "revise", "run:another-seat"}, {"work", "revise", "j2:review"},
		{"work", "review", bedGoal}, {"work", "review", "run:another-seat"}, {"work", "review", "j2:job"},
		{"work", "review", "--commit", "abc123", "--goal", bedGoal}, {"work", "review", "--changes"},
		{"work", "review", "--patch", "patch.diff"}, {"work", "review", "--check-only", "j2:job", "--stage", "review"},
		{"work", "land", bedGoal}, {"work", "land", "j2:job"}, {"work", "land", bedGoal, "--queue-only"},
		{"work", "land", "--message", "message.txt", "--staged"}, {"work", "land", bedGoal, "--using-exception", "exception"},
		{"work", "finish", "j2:job"}, {"work", "wait", "run:another-seat"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			for _, extra := range [][]string{nil, {"--impact", impact}} {
				command := mustIntentArgvCommand(t, args)
				reached := false
				command.run = func(*intentInvocation) int { reached = true; return 0 }
				var stdout, stderr, notice bytes.Buffer
				owners := partnerActOwners(b, impact, &notice)
				owners.work.git = func(string, ...string) ([]byte, error) { return nil, nil }
				code := runIntentIn(command, append(append(intentArgvRest(args), extra...), "--json"), &stdout, &stderr, b.root(), owners)
				var result intentResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if code != 1 || reached || result.Outcome != intentRefused || result.Summary != "the project partner never drives a build; a seat does; nothing was done" {
					t.Fatalf("%v: %d %+v owner=%v stderr=%s", extra, code, result, reached, &stderr)
				}
			}
		})
	}
}

func TestPartnerWorkStopStillNeedsImpactAndReachesItsOwner(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	impact := "Stops work; undo: restart the work."
	for _, extra := range [][]string{nil, {"--impact", impact}} {
		command := mustIntentArgvCommand(t, []string{"work", "stop"})
		reached := false
		command.run = func(*intentInvocation) int { reached = true; return 0 }
		var stdout, stderr, notice bytes.Buffer
		code := runIntentIn(command, append([]string{"j2:job", "--json"}, extra...), &stdout, &stderr, b.root(), partnerActOwners(b, impact, &notice))
		if len(extra) == 0 && (code != 1 || reached || !strings.Contains(stdout.String(), "work stop is the person's act through the partner; --impact is missing; nothing was done")) || len(extra) != 0 && (code != 0 || !reached) {
			t.Fatalf("stop %v: %d %s %s owner=%v", extra, code, &stdout, &stderr, reached)
		}
	}
}

func TestPartnerUnitWaitNeverAdvancesTheRun(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	code, built, _ := b.work(append([]string{"work", "build", b.id, "main", "--brief", b.brief("brief.md", "Build it.\n"), "--lines", "5"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("held build: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	b.releaseHeldBuild(built)
	path := filepath.Join(b.unitRoot, run, "run.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	launched := len(b.starter.launched())
	b.lineage = "project-partner"
	for _, target := range []string{"run:" + run, b.id} {
		for _, extra := range [][]string{nil, {"--impact", "Continue it; undo: restore it."}} {
			code, result, _ := b.work(append([]string{"work", "wait", target}, extra...)...)
			after, err := os.ReadFile(path)
			if code != 1 || result.Outcome != intentRefused || result.Summary != "the project partner never drives a build; a seat does; nothing was done" || err != nil || !bytes.Equal(before, after) || len(b.starter.launched()) != launched {
				t.Fatalf("partner wait %s: %d %+v read=%v launches=%v", target, code, result, err, b.starter.launched())
			}
		}
	}
}

func TestPartnerIsolateTargetsTheOwnerCheckout(t *testing.T) {
	b, other := newPartnerActBed(t), newPartnerActBed(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "project-partner")
	for _, target := range []string{other.root(), b.root()} {
		var stdout, stderr, notice bytes.Buffer
		reached := false
		// Each checkout keeps its own clock and ledger fixtures.
		owners := map[string]intentOwners{b.root(): partnerActOwners(b, "Isolate it; undo: remove the worktree.", &notice), other.root(): other.owners()}[target]
		code := runPassthroughIn(mustIntentArgvCommand(t, []string{"session", "isolate"}), func([]string, io.Writer, io.Writer) int { reached = true; return 0 }, []string{"--root", target, "--impact", "Isolate it; undo: remove the worktree."}, &stdout, &stderr, b.root(), owners)
		if target == other.root() && (code != 1 || reached || !strings.Contains(stdout.String()+stderr.String(), "the partner acts only from its own checkout in this step")) || target == b.root() && (code != 0 || !reached) {
			t.Fatalf("isolate %s: %d %s %s owner=%v", target, code, &stdout, &stderr, reached)
		}
	}
}

func TestPartnerEmptyCheckoutOptionsNeverReachTheOwner(t *testing.T) {
	b := newPartnerActBed(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "project-partner")
	for option, verb := range map[string]string{"root": "session isolate", "installation": "system restart", "repo": "session isolate"} {
		command := mustIntentCommand(t, verb)
		code, reached := 0, false
		command.run = func(*intentInvocation) int { reached = true; return 0 }
		var stdout, stderr bytes.Buffer
		args := []string{"--" + option, "", "--impact", "Isolate it; undo: remove the worktree."}
		if command.passthrough != nil {
			code = runPassthroughIn(command, func([]string, io.Writer, io.Writer) int { reached = true; return 0 }, args, &stdout, &stderr, b.root(), b.owners())
		} else {
			code = runIntentIn(command, args, &stdout, &stderr, b.root(), b.owners())
		}
		if code == 0 || reached || !strings.Contains(stdout.String()+stderr.String(), "an empty --"+option+" names no checkout; nothing was done") {
			t.Fatalf("empty --%s: %d %s %s owner=%v", option, code, &stdout, &stderr, reached)
		}
	}
}

func TestPartnerSyncPublishUsesTheGrantsPersonWithoutBy(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	impact := "Publish reviewed edits; undo: restore the goal."
	var notice bytes.Buffer
	owners := partnerActOwners(b, impact, &notice)
	calls := defaultIntentOwnerCalls()
	calls.goalReconcile = func(deps syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
		if by, given, _ := takeIntentFlag(args, "by", true); !given || by != "Wido" {
			t.Errorf("reconcile received --by %q, given=%v; want the grant's person", by, given)
		}
		if !deps.partnerGrantAdmitted {
			t.Fatal("reconcile did not receive the partner's grant admission")
		}
		now, err := b.commandNow(b.root())
		if err != nil {
			t.Fatal(err)
		}
		proof, err := deps.proveHuman(b.root(), deps.authorityFacts.caller.Pid, nil, now)
		if err != nil {
			t.Fatalf("reconcile's person proof: %v", err)
		}
		if proof.Helm == nil || proof.Helm.By != "Wido" || proof.Helm.Grant != b.rootRecord().PowerOfAttorney[0].ID {
			t.Fatalf("reconcile's proof did not carry the grant's person and id: %+v", proof)
		}
		deps.stdout, deps.stderr = stdout, stderr
		return deps.publish(goal.PublishResult{Outcome: goal.OutcomeConfirmed}, nil)
	}
	owners.delivery = &intentDeliveryOwners{calls: calls}
	path := filepath.Join(b.root(), "plans", "goals", bedGoal+".md")
	edited := bytes.ReplaceAll(goal.RenderFile(b.goalFile(bedGoal)), []byte("Run it."), []byte("Publish the reviewed edit."))
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	code, result := b.runJSON(owners, "goal", "sync", "--publish", "--goal", bedGoal, "--impact", impact)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("publish: %d %+v", code, result)
	}
}

func TestPartnerSyncPublishRefusesTheLaterProofError(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	impact := "Publish the edits; undo: restore the goal."
	var notice bytes.Buffer
	owners := partnerActOwners(b, impact, &notice)
	proofCalls := 0
	owners.dependencies.proveHuman = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		proofCalls++
		return humanauthority.Proof{}, fmt.Errorf("the grant expired before publication")
	}
	path := filepath.Join(b.root(), "plans", "goals", bedGoal+".md")
	edited := bytes.ReplaceAll(goal.RenderFile(b.goalFile(bedGoal)), []byte("Run it."), []byte("Publish the reviewed edit."))
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	before, publications := goal.RenderFile(b.goalFile(bedGoal)), b.repo.publications
	code, result := b.runJSON(owners, "goal", "sync", "--publish", "--goal", bedGoal, "--by", "Wido", "--impact", impact)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "the grant expired before publication") || proofCalls != 1 || !strings.Contains(notice.String(), "POWER OF ATTORNEY") {
		t.Fatalf("publish: %d %+v proof calls=%d notice=%s", code, result, proofCalls, &notice)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(edited, after) || !bytes.Equal(before, goal.RenderFile(b.goalFile(bedGoal))) || b.repo.publications != publications {
		t.Fatalf("a failed proof changed the ledger, published, or changed the reviewed edits: %v", err)
	}
}

func TestPartnerSessionWaitAfterRevokeRegisters(t *testing.T) {
	b := newPartnerActBed(t)
	b.lineage = ""
	if code, result := b.runJSON(b.owners(), "grant", "revoke", b.rootRecord().PowerOfAttorney[0].ID); code != 0 {
		t.Fatalf("revoke: %d %+v", code, result)
	}
	b.lineage = "project-partner"
	waitRegisterCommandFixture(t, b.root())
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "project-partner")
	command, _ := findIntentCommand("session wait")
	owners := b.owners()
	owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) { t.Fatal("waiting read a grant"); return nil, nil }
	var stdout, stderr bytes.Buffer
	args := []string{"--question", "May I stop?", "--timeout", "1h"}
	if code := runPassthroughIn(command, command.owner, args, &stdout, &stderr, b.root(), owners); code != 0 || len(registeredRows(t, b.root())) != 1 {
		t.Fatalf("own wait after revoke: %d %s %s", code, &stdout, &stderr)
	}
}

func TestPartnerObservationFormsNeedNoGrant(t *testing.T) {
	b := newPartnerBed(t)
	b.lineage = "project-partner"
	t.Setenv("METASYSTEM_OWNER_LINEAGE", b.lineage)
	owners := b.owners()
	owners.attorney.entries = func(string) ([]goal.PowerOfAttorneyEntry, error) {
		t.Fatal("observation read a grant")
		return nil, nil
	}
	baseline := mustIntentArgvCommand(t, []string{"test", "baseline"})
	if inv := (&intentInvocation{command: baseline, owners: owners, input: intentInput{values: map[string][]string{"check": {"false"}}}}); !inv.partnerMutation() {
		t.Fatal("baseline --check=false bypassed actor selection")
	}
	waitID := strings.Repeat("a", 32)
	row := metarun.Waiter{SchemaVersion: 2, WaitID: waitID, Nonce: strings.Repeat("b", 32), State: "pending", Kind: "job", TargetID: "job-1", OwnerDigest: "owner", Selector: metarun.WaitSelector{Kind: "job", TargetID: "job-1"}}
	body, _ := json.Marshal(row)
	if err := os.MkdirAll(metarun.WaitersDir(b.root()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metarun.WaiterPath(b.root(), row.Kind, row.TargetID, row.OwnerDigest), body, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"goal", "show", bedGoal}, {"goal", "list"}, {"work", "status", bedGoal}, {"goal", "notes", bedGoal}, {"goal", "budget", bedGoal},
		{"settings", "coordinator"}, {"test", "baseline", "--check"},
		{"session", "start"}, {"session", "stop"}, {"session", "status"}, {"session", "wait"}, {"session", "handoff"},
		{"work", "wait", "--path", b.root(), "--until", "present"}, {"work", "wait", "--list"},
		{"work", "wait", "j1:launch"}, {"work", "wait", "j2:job"}, {"work", "wait", "wait:" + waitID},
		{"work", "wait", bedGoal, "--for", "landing"}, {"work", "wait", bedGoal, "--for", "human-act"}, {"question", "wait", "channel:question"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			command := mustIntentArgvCommand(t, args)
			reached := false
			command.run = func(inv *intentInvocation) int {
				reached = true
				if _, problem := inv.partnerActor(""); problem != nil {
					t.Fatalf("observation's owner was guarded: %+v", problem)
				}
				return 0
			}
			var stdout, stderr bytes.Buffer
			code := 0
			if command.passthrough != nil {
				code = runPassthroughIn(command, func([]string, io.Writer, io.Writer) int { reached = true; return 0 }, intentArgvRest(args), &stdout, &stderr, b.root(), owners)
			} else {
				code = runIntentIn(command, intentArgvRest(args), &stdout, &stderr, b.root(), owners)
			}
			if code != 0 || !reached {
				t.Fatalf("observation: %d %s %s owner=%v", code, &stdout, &stderr, reached)
			}
		})
	}
}

func TestPartnerWaitAndCheckAreGuardedBeforeTheirOwners(t *testing.T) {
	t.Parallel()
	b := newPartnerActBed(t)
	for _, revoked := range []bool{false, true} {
		if revoked {
			b.lineage = ""
			if code, result := b.runJSON(b.owners(), "grant", "revoke", b.rootRecord().PowerOfAttorney[0].ID); code != 0 {
				t.Fatalf("revoke: %d %+v", code, result)
			}
			b.lineage = "project-partner"
		}
		for _, args := range [][]string{{"app", "check"}, {"work", "wait", bedGoal}, {"work", "wait", "wait:" + strings.Repeat("f", 32)}, {"test", "wait", "proof:p"}, {"future", "wait"}, {"future", "check"}} {
			command, _, known := resolveIntentArgv(args)
			if !known {
				command = mustIntentArgvCommand(t, []string{"app", "check"})
				command.object, command.action, command.name = args[0], args[1], strings.Join(args, " ")
			}
			path := filepath.Join(b.root(), "plans", "designs", bedGoal+".md")
			if command.name == "app check" {
				path = filepath.Join(b.root(), "artifacts", "agents", "app", "standing.json")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			reached := false
			command.run = func(inv *intentInvocation) int {
				reached = true
				if err := os.WriteFile(path, []byte("verdict or design"), 0o600); err != nil {
					t.Fatal(err)
				}
				return 0
			}
			extra, reason := []string{"--json"}, "--impact is missing"
			if revoked {
				extra = append(extra, "--impact", "Change it; undo: restore it.")
				reason = "the grant does not admit this session (revoked "
			}
			var stdout, stderr, notice bytes.Buffer
			owners := partnerActOwners(b, "Change it; undo: restore it.", &notice)
			owners.work.git = func(string, ...string) ([]byte, error) { return nil, nil }
			code := runIntentIn(command, append(intentArgvRest(args), extra...), &stdout, &stderr, b.root(), owners)
			if code != 1 || reached || !strings.Contains(stdout.String(), reason) {
				t.Fatalf("%v revoked=%v: %d %s %s", args, revoked, code, &stdout, &stderr)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("refused owner wrote a verdict or design: %v", err)
			}
		}
	}
}
