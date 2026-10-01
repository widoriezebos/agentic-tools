package batchowner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// K4 with the metadata producers moved before begin (design r10, Astra r9
// note): an ordinary batch of a goal member and a change member, composed
// by the agent as plain replays in a nested lane checkout, is begun into a
// canonical series that held admits as the lane's — ownership and revision
// trailers read from the ledger at B — and whose provenance trailers let
// landed-trailer recovery finalize both members once it is on main.
func TestOrdinaryGoalLandingAndRecovery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checkout := filepath.Join(dir, "lane")
	install := filepath.Join(checkout, "metasystem")
	origin := filepath.Join(dir, "origin.git")
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", checkout, "-c", "user.name=Lane Agent", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-01T09:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T09:00:00Z")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(checkout, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		t.Helper()
		git("add", "-A")
		git("commit", "-q", "-m", message)
		return git("rev-parse", "HEAD")
	}
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init origin: %v %s", err, out)
	}
	if out, err := exec.Command("git", "init", "-q", "-b", "main", checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	const actor = "m1e+" + lane.ClaimLineage
	const goalID = "ordinary"
	write(".gitignore", "/artifacts/\n")
	write("metasystem/metasystem.conf", "metasystem.template=true\n")
	write("metasystem/app/base.txt", "base\n")
	// The goal is held by the lane on main: its forward handover landed.
	history := []goal.HistoryLine{
		{At: "2026-09-30T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m1e-00000001", Verb: "edit", Actor: actor, Targets: []string{goalID}, Keep: -1},
		{At: "2026-09-30T08:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m1e-00000002", Verb: "edit", Actor: actor, Targets: []string{goalID}, Keep: -1},
		{At: "2026-09-30T08:02:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m1e-00000003", Verb: "claim", Actor: actor, Targets: []string{goalID}, Keep: -1},
	}
	ledger := goal.RenderFile(&goal.GoalFile{Id: goalID, State: goal.StateClaimed, Intent: "Fixture goal.", Origin: goal.OriginMain,
		NextStep: "Land it.", OpenedAt: "2026-09-30T08:00:00Z", Revision: 3,
		Claimed: &goal.ClaimRecord{Machine: "m1e", Lineage: lane.ClaimLineage, At: history[2].At, Revision: 3, AccountingRevision: 3}, History: history})
	if _, problems := goal.ParseFile(ledger); len(problems) != 0 {
		t.Fatalf("invalid goal: %v", problems)
	}
	write("metasystem/plans/goals/"+goalID+".md", string(ledger))
	base := commit("base with the goal held by the lane")
	git("remote", "add", "origin", origin)
	git("push", "-q", "origin", "main")
	git("fetch", "-q", "origin")
	baseTree := git("rev-parse", "HEAD^{tree}")

	// The goal's reviewed build on its branch, and a seat's change.
	git("checkout", "-q", "-b", "goal/"+goalID, base)
	write("metasystem/app/feature.txt", "feature\n")
	build := commit("build one\n\nCo-Authored-By: Builder <builder@example.invalid>")
	digest, err := goalbranch.UnitDigest(checkout, build)
	if err != nil {
		t.Fatal(err)
	}
	git("checkout", "-q", "--detach", base)
	write("metasystem/app/change.txt", "a seat's change\n")
	change := commit("record: a seat's change\n\nMachine: m1e+human")
	git("checkout", "-q", "main")

	goalMember := batch.BindBranchMember(batch.Unit{GoalID: goalID, Chain: build, SeatRoot: "/seat", State: batch.UnitJoined, Approver: "Wido",
		AuthorName: "Wido Riezebos", AuthorEmail: "wido@example.invalid",
		Claim: batch.Claim{Machine: "m1e", Lineage: "seat", Epoch: 1, Revision: 2, AccountingRevision: 2}},
		batch.BranchMember{GoalID: goalID, Tip: build, Last: true, Builds: []batch.BranchBuild{{Units: []string{"u1"}, Commit: build, Digest: digest,
			CoAuthors: []string{"Builder <builder@example.invalid>"}}}})
	changeMember := batch.NewChangeUnit(batch.ChangeMember{Commit: change, Parent: base, AskedBy: "m1e+human", Subject: "record: a seat's change"},
		"/seat", "m1e", "human", []string{"metasystem/app/change.txt"}, nil)
	changeMember.State = batch.UnitJoined
	store := batch.NewStore(checkout, nil)
	const id = "01j5x00000000000000000rd01"
	record := batch.Record{Schema: 1, BatchID: id, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen, Units: []batch.Unit{goalMember, changeMember}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}

	// The agent composes plain replays on B.
	git("checkout", "-q", "-B", "lane/"+id, base)
	git("cherry-pick", "--allow-empty", build)
	git("cherry-pick", "--allow-empty", change)
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")

	at := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	opening, err := batch.PlanOpening(checkout, record, batch.BeginRequest{BatchID: id, Members: []string{goalID, changeMember.GoalID},
		Base: base, Head: head, LedgerRoot: install, Actor: actor, At: at, OpID: "01j5x00000000000000000op01"})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, changed, err := batch.RecordOpening(store, id, opening, at); err != nil || !changed {
		t.Fatalf("record the opening: changed=%v %v", changed, err)
	}
	if _, changed, err := batch.RecordOpening(store, id, opening, at); err != nil || changed {
		t.Fatalf("the same opening again: changed=%v %v; want unchanged", changed, err)
	}
	loaded, err := store.Load(id)
	if err != nil || loaded.State != batch.StateSealed || len(loaded.Openings) != 1 || loaded.Openings[0].Candidate != opening.Candidate {
		t.Fatalf("recorded batch = state %s openings %d, %v; want sealed with the one opening", loaded.State, len(loaded.Openings), err)
	}
	goalMessage := git("show", "-s", "--format=%B", opening.Series[0].Commit) + "\n"
	for _, trailer := range []string{"Goal-Unit: ordinary/u1", "Goal-Digest: " + digest, "Goal-Source: " + build, "Goal-Last: ordinary",
		"Co-Authored-By: Builder <builder@example.invalid>", "Machine: " + actor, "Goal-Item: ordinary", "Goal-Revision: 3", "Landed-By: " + actor} {
		if !strings.Contains(goalMessage, trailer+"\n") {
			t.Errorf("the goal's canonical commit lacks %q:\n%s", trailer, goalMessage)
		}
	}
	if author := git("show", "-s", "--format=%an <%ae>", opening.Series[0].Commit); author != "Wido Riezebos <wido@example.invalid>" {
		t.Errorf("the goal's landing is authored by %s; want its approver", author)
	}
	if changeMessage := git("show", "-s", "--format=%B", opening.Series[1].Commit); !strings.Contains(changeMessage, batch.LandingChangeTrailer+": "+changeMember.GoalID) ||
		!strings.Contains(changeMessage, "Machine: m1e+human") {
		t.Errorf("the change's canonical commit lacks its trailers:\n%s", changeMessage)
	}

	verdict, err := landing.Held(install, base, opening.Candidate, "origin", "refs/heads/main")
	if err != nil || verdict.Outcome != "ok" || verdict.Commits != 2 {
		t.Fatalf("held on the canonical series = %+v, %v; want ok over 2 commits", verdict, err)
	}

	// Publication is K-c's: here main simply moves to the candidate.
	git("push", "-q", "origin", opening.Candidate+":refs/heads/main")
	git("fetch", "-q", "origin")
	if err := store.Update(id, func(current *batch.Record) error {
		current.Landing = &batch.LandingProgress{Base: baseTree, PushComplete: true, PushedTip: opening.Candidate, CandidateTip: opening.Candidate}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	finalized := map[string]string{}
	seams := RecoverySeams(checkout, batch.ModuleRoot(checkout), "", store, id, at, GitOutput, &BatchOwnerCalls, nil)
	seams.SweepGoalBranch = func(batch.Unit, string) error { return nil }
	seams.Finalize = func(unit batch.Unit, landed string) error { finalized[unit.GoalID] = landed; return nil }
	seams.Cleanup = func() error { return nil }
	seams.Release = func(batch.Unit, *diskstore.ReleaseSet) {}
	if err := batch.RecoverPushedSeries(store, id, actor, at, seams); err != nil {
		t.Fatalf("recover the landed series: %v", err)
	}
	recovered, err := store.Load(id)
	if err != nil || recovered.State != batch.StateLanded {
		t.Fatalf("recovered batch = %s, %v; want landed", recovered.State, err)
	}
	landed := map[string]string{}
	for _, unit := range recovered.Units {
		landed[unit.GoalID] = unit.LandedCommit
	}
	if landed[goalID] != opening.Series[0].Commit || landed[changeMember.GoalID] != opening.Series[1].Commit || finalized[goalID] != opening.Series[0].Commit {
		t.Fatalf("recovery found %v (finalized %v); want %s and %s", landed, finalized, opening.Series[0].Commit, opening.Series[1].Commit)
	}
}
