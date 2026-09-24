package landing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type registerChange struct {
	path          string
	before, after *string
}

func registerComparison(f *repositoryObservationFixture, tree string, changes ...registerChange) *observationCase {
	f.t.Helper()
	paths := make([]string, 0, len(changes))
	var diff strings.Builder
	for _, change := range changes {
		paths = append(paths, change.path)
		fmt.Fprintf(&diff, "diff --git a/%s b/%s\n", change.path, change.path)
	}
	c := f.comparison(tree, diff.String(), paths...)
	for _, change := range changes {
		c.declare(change.path, change.before, change.after)
	}
	return c
}

func declareGoalDirectory(c *observationCase, ids ...string) {
	c.fixture.t.Helper()
	entries := make(map[string]gittree.Entry, len(ids))
	for _, id := range ids {
		entries["plans/goals/"+id+".md"] = gittree.Entry{Mode: "100644", OID: observeBaseBlob}
	}
	c.declareEntries(observeBaseTree, []string{"plans/goals/"}, entries)
}

func TestObserveRecordSemantics(t *testing.T) {
	t.Run("existing record appends only under a held goal", func(t *testing.T) {
		f := newRepositoryObservationFixture(t)
		f.base("plans/goals/fx.md", string(observationHeldGoal("fx", "m9", "L1")))
		f.base("records/misc/fx-analysis.md", "base\n")

		c := registerComparison(f, observeTreeB, registerChange{"records/misc/fx-analysis.md", observationText("base\n"), observationText("base\nappend\n")})
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" || got.Verdict != "pass" {
			t.Fatalf("owned existing record append classified as %+v", got)
		}

		c = registerComparison(f, observeTreeC, registerChange{"records/misc/fx-analysis.md", observationText("base\n"), observationText("replacement\n")})
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage-not-append-only" {
			t.Fatalf("existing record replacement classified as %+v", got)
		}

		if err := os.Remove(filepath.Join(f.root, "records/misc/fx-analysis.md")); err != nil {
			t.Fatal(err)
		}
		c = registerComparison(f, observeTreeB, registerChange{"records/misc/fx-analysis.md", observationText("base\n"), nil})
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage-not-append-only" {
			t.Fatalf("existing record deletion classified as %+v", got)
		}

		c = registerComparison(f, observeTreeC, registerChange{"records/misc/fx-analysis.md", observationText("base\n"), observationText("base\nappend\n")})
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
		if got.Code != "record-not-owned" {
			t.Fatalf("ownerless existing record append classified as %+v", got)
		}
	})

	t.Run("goal-bound plans use base claims and longest identifiers", func(t *testing.T) {
		f := newRepositoryObservationFixture(t)
		f.base("plans/goals/fx.md", string(observationHeldGoal("fx", "m9", "L1")))
		f.base("plans/goals/fx-load.md", string(observationHeldGoal("fx-load", "m9", "L1")))
		f.base("plans/fx-design.md", "base\n")
		f.base("plans/fx-load-x.md", "base\n")
		f.base("plans/legacy.md", "legacy\n")

		c := registerComparison(f, observeTreeB, registerChange{"plans/fx-design.md", observationText("base\n"), observationText("modified\n")})
		declareGoalDirectory(c, "fx", "fx-load")
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("owned goal plan classified as %+v", got)
		}
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
		if got.Code != "goal-item-not-held" {
			t.Fatalf("foreign actor classified as %+v", got)
		}
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx-load", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("wrong held goal classified as %+v", got)
		}

		f.write("plans/fx-design.md", "base\n")
		c = registerComparison(f, observeTreeC, registerChange{"plans/fx-load-x.md", observationText("base\n"), observationText("modified\n")})
		declareGoalDirectory(c, "fx", "fx-load")
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("shorter goal identifier won ownership: %+v", got)
		}
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx-load", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("longest goal identifier lost ownership: %+v", got)
		}

		f.write("plans/fx-load-x.md", "base\n")
		c = registerComparison(f, observeTreeB, registerChange{"plans/legacy.md", observationText("legacy\n"), observationText("modified\n")})
		declareGoalDirectory(c, "fx", "fx-load")
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("frozen legacy plan classified as %+v", got)
		}
		manifest := string(f.baseFiles["scripts/agents/path-classes.txt"])
		f.base("scripts/agents/path-classes.txt", manifest+"own:plans/legacy.md fx\n")
		c = registerComparison(f, observeTreeC, registerChange{"plans/legacy.md", observationText("legacy\n"), observationText("owned modification\n")})
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("explicitly owned legacy plan classified as %+v", got)
		}
	})

	t.Run("handoffs belong to their seat after creation", func(t *testing.T) {
		f := newRepositoryObservationFixture(t)
		f.base("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{
			Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
			Free: &goal.FreeRecord{Declared: "2026-09-03T08:00:00Z", Origin: "human", Digest: strings.Repeat("a", 64)},
		})))
		f.base("plans/handoff-m9-x.md", "base\n")
		c := registerComparison(f, observeTreeB, registerChange{"plans/handoff-m9-x.md", observationText("base\n"), observationText("modified\n")})
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("own handoff classified as %+v", got)
		}
		got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Actor: "m1+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("foreign handoff classified as %+v", got)
		}
	})
}

func TestObservationRequiresAGoalFromANonHumanActor(t *testing.T) {
	nonFree := newRepositoryObservationFixture(t)
	c := registerComparison(nonFree, observeTreeB, registerChange{"records/misc/fixture.md", nil, observationText("fixture\n")})
	c.declare("plans/goals/backlog.md", nil, nil)
	params := ObserveParams{RepoRoot: nonFree.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Actor: "m9+lineage"}
	if got := c.observe(params); got.Code != "goal-binding-missing" || got.Verdict != "would-refuse" {
		t.Fatalf("goal-less agent landing on an ordinary ledger classified as %+v", got)
	}

	free := newRepositoryObservationFixture(t)
	free.base("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
		Free: &goal.FreeRecord{Declared: "2026-09-03T08:00:00Z", Origin: "human", Digest: strings.Repeat("a", 64)},
	})))
	freeCase := registerComparison(free, observeTreeB, registerChange{"records/misc/fixture.md", nil, observationText("fixture\n")})
	params.RepoRoot, params.CandidateTree = free.root, freeCase.candidate
	if got := freeCase.observe(params); got.Code != "register-carriage" || !strings.Contains(got.Provenance, " goal-free") {
		t.Fatalf("goal-less agent landing on a Goal-free ledger classified as %+v", got)
	}

	params.Actor = "m9+human"
	params.RepoRoot, params.CandidateTree = nonFree.root, c.candidate
	if got := c.observe(params); got.Code != "goal-binding-missing" || got.Verdict != "would-refuse" {
		t.Fatalf("human observation did not record the would-refuse verdict: %+v", got)
	}
}

func TestObserveRegisterCarriagePerClassRules(t *testing.T) {
	t.Run("register carriage append-only", func(t *testing.T) {
		for _, register := range []string{"memory/rulings.md", "memory/receipts.log", "records/narrator-digest.log"} {
			t.Run(register, func(t *testing.T) {
				carriage := newRepositoryObservationFixture(t)
				original := string(carriage.baseFiles[register])
				appended := "appended row\n"
				if register == "memory/rulings.md" {
					appended = "| R-36-m0 | appended ruling |\n"
				}
				c := registerComparison(carriage, observeTreeB, registerChange{register, &original, observationText(original + appended)})
				got := c.observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
				})
				if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
					t.Fatalf("append-only carriage classified as %+v", got)
				}

				c = registerComparison(carriage, observeTreeC, registerChange{register, &original, observationText("rewritten existing line\n")})
				got = c.observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
				})
				if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
					t.Fatalf("rewritten append-only register classified as %+v", got)
				}
			})
		}
	})

	t.Run("rulings carriage requires rows and preserves mode", func(t *testing.T) {
		malformed := newRepositoryObservationFixture(t)
		original := string(malformed.baseFiles["memory/rulings.md"])
		c := registerComparison(malformed, observeTreeB, registerChange{"memory/rulings.md", &original, observationText(original + "free text\n")})
		got := c.observe(ObserveParams{
			RepoRoot: malformed.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
			t.Fatalf("malformed ruling carriage classified as %+v", got)
		}

		modeChanged := newRepositoryObservationFixture(t)
		modeOriginal := string(modeChanged.baseFiles["memory/rulings.md"])
		c = registerComparison(modeChanged, observeTreeB, registerChange{"memory/rulings.md", &modeOriginal, observationText(modeOriginal + "| R-36-m0 | appended ruling |\n")})
		c.declareEntries(c.candidate, []string{"memory/rulings.md"}, map[string]gittree.Entry{
			"memory/rulings.md": {Mode: "100755", OID: observeNextBlob},
		})
		if err := os.Chmod(filepath.Join(modeChanged.root, "memory", "rulings.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		got = c.observe(ObserveParams{
			RepoRoot: modeChanged.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
			t.Fatalf("mode-changing ruling carriage classified as %+v", got)
		}
	})

	t.Run("landing class authority must name an existing ruling row", func(t *testing.T) {
		missing := newRepositoryObservationFixture(t)
		missing.base("memory/rulings.md", "| R-1 | unrelated ruling |\n")
		c := registerComparison(missing, observeTreeB, registerChange{"memory/receipts.log", observationText("receipt=existing\n"), observationText("receipt=existing\nreceipt=carried\n")})
		got := c.observe(ObserveParams{
			RepoRoot: missing.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-policy-unreadable" {
			t.Fatalf("manifest with absent authority row classified as %+v", got)
		}
	})

	t.Run("register carriage exact and constrained glob entries", func(t *testing.T) {
		for _, changedPath := range []string{"records/narrator-digest.log", "plans/handoff-current.md"} {
			t.Run(changedPath, func(t *testing.T) {
				carriage := newRepositoryObservationFixture(t)
				content := "carried register\n"
				var before *string
				if changedPath == "records/narrator-digest.log" {
					before = observationText("digest=existing\n")
					content = "digest=existing\ndigest=carried\n"
				}
				c := registerComparison(carriage, observeTreeB, registerChange{changedPath, before, &content})
				if changedPath == "plans/handoff-current.md" {
					c.declareEntries(observeBaseTree, []string{"plans/goals/"}, nil)
				}
				got := c.observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
				})
				if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
					t.Fatalf("allowlisted register %s classified as %+v", changedPath, got)
				}
			})
		}
		carriage := newRepositoryObservationFixture(t)
		c := registerComparison(carriage, observeTreeB, registerChange{"plans/handoff-current.txt", nil, observationText("wrong basename shape\n")})
		got := c.observe(ObserveParams{
			RepoRoot: carriage.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarDirectFix || got.Code != "register-carriage" {
			t.Fatalf("new plan record classified as %+v", got)
		}

		mixed := newRepositoryObservationFixture(t)
		c = registerComparison(mixed, observeTreeB,
			registerChange{"adopted.txt", nil, observationText("off-floor miss sorts first\n")},
			registerChange{"internal/protected.txt", nil, observationText("floor miss sorts second\n")})
		got = c.observe(ObserveParams{
			RepoRoot: mixed.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("mixed carriage misses did not give the floor precedence: %+v", got)
		}
	})

	t.Run("carriage policy files are on the floor", func(t *testing.T) {
		policy := newRepositoryObservationFixture(t)
		before := string(policy.baseFiles["scripts/agents/path-classes.txt"])
		c := registerComparison(policy, observeTreeB, registerChange{"scripts/agents/path-classes.txt", &before, observationText("install:memory/ record\n")})
		got := c.observe(ObserveParams{
			RepoRoot: policy.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("allowlist self-change classified as %+v", got)
		}

		manifest := newRepositoryObservationFixture(t)
		before = string(manifest.baseFiles["scripts/agents/landing-classes.json"])
		c = registerComparison(manifest, observeTreeB, registerChange{"scripts/agents/landing-classes.json", &before, observationText("{}\n")})
		got = c.observe(ObserveParams{
			RepoRoot: manifest.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("manifest self-change classified as %+v", got)
		}
	})
}
