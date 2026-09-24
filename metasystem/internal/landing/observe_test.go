package landing

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type observeFixture struct {
	t    *testing.T
	root string
}

func newObserveFixture(t *testing.T) *observeFixture {
	t.Helper()
	repository := t.TempDir()
	root := filepath.Join(repository, "metasystem")
	if err := os.MkdirAll(filepath.Join(repository, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "development", "metasystem-design.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return newObserveFixtureAt(t, repository, root)
}

func newAdoptedObserveFixture(t *testing.T) *observeFixture {
	t.Helper()
	repository := t.TempDir()
	return newObserveFixtureAt(t, repository, repository)
}

func newObserveFixtureAt(t *testing.T, repository, root string) *observeFixture {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "-C", repository, "init", "-q", "-b", "main")
	command.Env = gittree.ScrubbedEnviron()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	f := &observeFixture{t: t, root: root}
	f.git("config", "user.name", "landing fixture")
	f.git("config", "user.email", "landing@example.invalid")
	f.write(".gitignore", "artifacts/\n")
	f.write("product.txt", "before\n")
	for _, policyFile := range []string{"path-classes.txt", "landing-classes.json"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", policyFile))
		if err != nil {
			t.Fatalf("read repository policy %s: %v", policyFile, err)
		}
		f.writeBytes(filepath.Join("scripts", "agents", policyFile), content)
	}
	f.write("memory/rulings.md", "| R-1 | existing ruling |\n| R-35-m0 | landing class authority |\n| R-54-m1 | tier-1 landing authority |\n")
	f.write("memory/receipts.log", "receipt=existing\n")
	f.write("records/narrator-digest.log", "digest=existing\n")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	if _, err := os.Stat(filepath.Join(repository, ".git")); err != nil {
		t.Fatalf("fixture repository vanished: %v", err)
	}
	f.git("status", "--short")
	return f
}

func (f *observeFixture) git(args ...string) string {
	f.t.Helper()
	command := exec.Command("git", append([]string{"-C", f.root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *observeFixture) write(relative, content string) {
	f.t.Helper()
	path := filepath.Join(f.root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *observeFixture) tree() string {
	f.t.Helper()
	tree, err := (gittree.Workspace{Dir: f.root}).Snapshot("HEAD")
	if err != nil {
		f.t.Fatal(err)
	}
	return tree
}

func (f *observeFixture) writeChainRecord(chain string, record map[string]any) {
	f.t.Helper()
	path := filepath.Join(f.root, "artifacts", "agents", "jobs", chain+".json")
	data, err := json.Marshal(record)
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(filepath.Join("artifacts", "agents", "jobs", chain+".json"), string(append(data, '\n')))
	if _, err := os.Stat(path); err != nil {
		f.t.Fatalf("chain record was not written: %v", err)
	}
}

func (f *observeFixture) writeChainReview(chain string, roundNumber int, implementerJob, reviewedTree string) {
	f.t.Helper()
	round := filepath.Join("artifacts", "agents", chain, "rounds", fmt.Sprint(roundNumber))
	review, err := json.Marshal(map[string]any{
		"diffArtifact": "diff.patch", "implementerJob": implementerJob, "reviewedTree": reviewedTree,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	base, err := (gittree.Workspace{Dir: f.root}).HeadTree()
	if err != nil {
		f.t.Fatal(err)
	}
	patch, err := (gittree.Workspace{Dir: f.root}).Diff(base, reviewedTree)
	if err != nil {
		f.t.Fatal(err)
	}
	f.writeBytes(filepath.Join(round, "diff.patch"), patch)
	f.write(filepath.Join(round, "review.json"), string(append(review, '\n')))
}

func (f *observeFixture) writeBytes(relative string, content []byte) {
	f.t.Helper()
	file := filepath.Join(f.root, relative)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, content, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *observeFixture) writeHeldGoal(id, machine, lineage string) {
	f.writeHeldGoalWithTier(id, machine, lineage, 0)
}

func (f *observeFixture) writeHeldGoalWithTier(id, machine, lineage string, tier uint8) {
	f.t.Helper()
	f.writeBytes(filepath.Join("plans", "goals", id+".md"), goal.RenderFile(&goal.GoalFile{
		Id: id, State: goal.StateClaimed, Tier: tier, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Exercise record carriage.", OpenedAt: "2026-09-03T08:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: "2026-09-03T08:01:00Z", Revision: 1},
		History: []goal.HistoryLine{{
			At: "2026-09-03T08:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001",
			Verb: "claim", Actor: machine + "+" + lineage, Targets: []string{id}, Keep: -1,
		}},
	}))
}

func (f *observeFixture) prepareTierOne(gateWidth string) string {
	f.t.Helper()
	f.writeHeldGoalWithTier("tier-one", "m9", "L1", 1)
	f.git("add", ".")
	f.git("commit", "-qm", "prepare tier-one goal")
	record := map[string]any{
		"jobId": "tier-one-root", "parentJob": nil, "role": "implementer",
		"goalId": "tier-one", "goalRevision": 1, "goalTier": 1,
	}
	if gateWidth != "" {
		record["gateWidth"] = gateWidth
	}
	f.writeChainRecord("tier-one-root", record)
	return "tier-one-root"
}

func (f *observeFixture) tierOneReceipt(command string) (string, string) {
	f.t.Helper()
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		f.t.Fatal(err)
	}
	receipt, err := CreateTestReceipt(f.root, candidate, command, io.Discard, io.Discard)
	if err != nil {
		f.t.Fatal(err)
	}
	if receipt.Tree != candidate {
		f.t.Fatalf("receipt tree %s does not equal candidate %s", receipt.Tree, candidate)
	}
	return candidate, TestReceiptPath(f.root, candidate)
}

func tierOneParams(f *observeFixture, candidate, receipt string) ObserveParams {
	return ObserveParams{
		RepoRoot: f.root, CandidateTree: candidate, DirectFix: "tier-1",
		Goal: "tier-one", Actor: "m9+L1", RootJob: "tier-one-root", TestReceipt: receipt,
	}
}

func TestObserveChainBoundLandingEvaluatesBarA(t *testing.T) {
	change := chainAddition("internal/x.go", "package internal\n")
	patch := chainDiff(change)
	validRecord := chainRoot("impl-chain")
	makeCase := func(t *testing.T) (*repositoryObservationFixture, *observationCase) {
		t.Helper()
		f := newRepositoryObservationFixture(t)
		c := f.chainCase(observeTreeB, change)
		return f, c
	}
	ready := func(f *repositoryObservationFixture, c *observationCase, chain string) {
		f.writeChainReview(chain, 1, chain, chainReviewedTree, patch)
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
	}
	f, c := makeCase(t)
	f.writeChainRecord("impl-chain", validRecord)
	f.writeChainRecord("impl-chain-r2", map[string]any{
		"jobId": "impl-chain-r2", "parentJob": "impl-chain", "role": "implementer",
		"round": 2, "destructiveReach": "DESIGN-BEARING", "status": "completed",
	})
	ready(f, c, "impl-chain")
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
	if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
		t.Fatalf("closed chain classified as %+v", got)
	}
	t.Run("follow-up-id conformance layout", func(t *testing.T) {
		f, c := makeCase(t)
		f.writeChainRecord("follow-chain", chainRoot("follow-chain"))
		f.writeChainRecord("follow-chain-r2", map[string]any{
			"jobId": "follow-chain-r2", "parentJob": "follow-chain", "role": "implementer",
			"round": 2, "destructiveReach": "DESIGN-BEARING", "status": "completed",
		})
		f.writeChainReview("follow-chain", 2, "follow-chain-r2", chainReviewedTree, patch)
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "follow-chain"})
		if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
			t.Fatalf("follow-up-id conformance layout classified as %+v", got)
		}
	})
	t.Run("closed critic selects the certified root-id review", func(t *testing.T) {
		f, c := makeCase(t)
		selected := chainRoot("selected-chain")
		selected["independentCritiqueJobRef"] = "selected-critic"
		f.writeChainRecord("selected-chain", selected)
		f.writeChainRecord("selected-chain-r2", map[string]any{
			"jobId": "selected-chain-r2", "parentJob": "selected-chain", "role": "implementer",
			"round": 2, "destructiveReach": "DESIGN-BEARING", "status": "completed",
		})
		f.writeChainRecord("selected-critic", map[string]any{
			"jobId": "selected-critic", "parentJob": nil, "role": "code-critic", "round": 1,
			"reviews": "selected-chain-r2", "status": "completed",
		})
		f.writeChainReview("selected-chain", 1, "selected-chain", chainReviewedTree, patch)
		f.writeChainReview("selected-chain", 2, "selected-chain-r2", observeTreeC,
			chainDiff(chainAddition("internal/x.go", "package internal // stale\n")))
		result, err := json.Marshal(map[string]any{"reviewedTree": chainReviewedTree})
		if err != nil {
			t.Fatal(err)
		}
		f.write("artifacts/agents/selected-critic/rounds/1/return.json", string(append(result, '\n')))
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "selected-chain"})
		if got.Bar != BarChain || got.Verdict != "pass" {
			t.Fatalf("closed critic's selected conformance output classified as %+v", got)
		}
	})
	t.Run("certified change with register carriage", func(t *testing.T) {
		f := newRepositoryObservationFixture(t)
		receipt := chainReplacement("memory/receipts.log", "receipt=existing\n", "receipt=existing\nreceipt=landing\n")
		f.writeChainRecord("bundle-chain", chainRoot("bundle-chain"))
		c := f.chainCase(observeTreeB, change, receipt)
		f.writeChainReview("bundle-chain", 1, "bundle-chain", chainReviewedTree, patch)
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
		c.wantChainReceiptAppend(1)
		c.wantChainRegister()
		got := c.observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: c.candidate, Chain: "bundle-chain",
			DirectFix: "register-carriage",
		})
		if got.Bar != BarChain || got.Verdict != "pass" ||
			!strings.Contains(got.Provenance, "chain=bundle-chain direct-fix class=register-carriage") ||
			got.VerdictTrailer != "pass bar=a carriage=register-carriage" {
			t.Fatalf("chain plus append-only register carriage classified as %+v", got)
		}
		known := chainAddition("memory/known-issues.md", "protected but not carriage\n")
		c = f.chainCase(observeTreeC, change, receipt, known)
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
		c.wantChainReceiptAppend(1)
		c.wantChainRegister(known.path)
		got = c.observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: c.candidate, Chain: "bundle-chain",
			DirectFix: "register-carriage",
		})
		if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
			t.Fatalf("chain with a new record path classified as %+v", got)
		}
	})
	c = f.chainCase(observeTreeB, change)
	got = c.observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain", RevertOf: strings.Repeat("0", 40),
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "conflicting-declarations" || got.Mode != "refuse" {
		t.Fatalf("chain with direct-fix parameter classified as %+v", got)
	}
	negativeShapes := []struct {
		name         string
		code         string
		mode         string
		refusesAgent bool
		edit         func(map[string]any)
	}{
		{name: "open chain", code: "chain-open", mode: "refuse", refusesAgent: true, edit: func(record map[string]any) { record["chainClosed"] = false }},
		{name: "non-implementer role", code: "chain-not-implementation", mode: "refuse", refusesAgent: true, edit: func(record map[string]any) { record["role"] = "designer" }},
		{name: "non-design-bearing reach", code: "chain-not-design-bearing", mode: "observe", refusesAgent: false, edit: func(record map[string]any) { record["destructiveReach"] = "MECHANICAL" }},
		{name: "parented record", code: "chain-record-malformed", mode: "refuse", refusesAgent: true, edit: func(record map[string]any) { record["parentJob"] = "parent-job" }},
	}
	for _, test := range negativeShapes {
		t.Run(test.name, func(t *testing.T) {
			f, c := makeCase(t)
			record := chainRoot("impl-chain")
			test.edit(record)
			f.writeChainRecord("impl-chain", record)
			got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != test.code || got.Mode != test.mode || got.RefusesAgent != test.refusesAgent {
				t.Fatalf("negative root shape classified as %+v", got)
			}
		})
	}
	t.Run("unreadable record", func(t *testing.T) {
		f, c := makeCase(t)
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "chain-record-unreadable" {
			t.Fatalf("unreadable root record classified as %+v", got)
		}
	})
	t.Run("tampered certified path", func(t *testing.T) {
		f := newRepositoryObservationFixture(t)
		tampered := chainAddition("internal/x.go", "package internal // tampered\n")
		c := f.chainCase(observeTreeB, tampered)
		f.writeChainRecord("impl-chain", validRecord)
		f.writeChainReview("impl-chain", 1, "impl-chain", chainReviewedTree, patch)
		c.bindChain(patch, chainReviewedTree, change)
		c.chain.expected["paths:"+observeBaseTree+":"+c.candidate]--
		got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "chain-output-mismatch" {
			t.Fatalf("closed chain with a tampered certified path classified as %+v", got)
		}
	})
}

func TestObserveDeclaredDirectFixEvaluatesPerClassRule(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	before, after := observationText("before\n"), observationText("after\n")
	params := func(c *observationCase) ObserveParams {
		return ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()}
	}
	makeCase := func(candidate, diff string, paths ...string) *observationCase {
		c := f.comparison(candidate, diff, paths...)
		c.exactRevert("product.txt")
		c.declare("product.txt", after, before)
		c.declareInverseEntries("product.txt", before, after)
		return c
	}
	c := makeCase(observeTreeB, "diff --git a/product.txt b/product.txt\n-after\n+before\n", "product.txt")
	got := c.observe(params(c))
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" {
		t.Fatalf("exact inverse classified as %+v", got)
	}

	c = makeCase(observeTreeC, "diff --git a/product.txt b/product.txt\n-after\n+before\ndiff --git a/extra.txt b/extra.txt\n+not part of the inverse\n", "product.txt", "extra.txt")
	c.declare("extra.txt", nil, observationText("not part of the inverse\n"))
	got = c.observe(params(c))
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" {
		t.Fatalf("expanded inverse classified as %+v", got)
	}
	c = makeCase(observeTreeC, "diff --git a/product.txt b/product.txt\n-after\n+before\ndiff --git a/internal/extra.txt b/internal/extra.txt\n+floor path outside the inverse\n", "product.txt", "internal/extra.txt")
	c.declare("internal/extra.txt", nil, observationText("floor path outside the inverse\n"))
	got = c.observe(params(c))
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
		t.Fatalf("expanded inverse with a floor path classified as %+v", got)
	}

	protected := newRepositoryObservationFixture(t)
	c = protected.comparison(observeTreeB, "diff --git a/internal/authority.txt b/internal/authority.txt\n-protected\n", "internal/authority.txt")
	c.exactRevert("internal/authority.txt")
	c.declare("internal/authority.txt", observationText("protected\n"), nil)
	c.declareInverseEntries("internal/authority.txt", nil, observationText("protected\n"))
	got = c.observe(ObserveParams{RepoRoot: protected.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
		t.Fatalf("floor-changing inverse classified as %+v", got)
	}

	for _, nestedPath := range []string{"product/AGENTS.md", "product/scripts/x"} {
		t.Run("unclassified nested path "+nestedPath, func(t *testing.T) {
			nested := newRepositoryObservationFixture(t)
			c := nested.comparison(observeTreeB, "diff --git a/"+nestedPath+" b/"+nestedPath+"\n-nested instruction\n", nestedPath)
			c.exactRevert(nestedPath)
			c.declare(nestedPath, observationText("nested instruction\n"), nil)
			c.declareInverseEntries(nestedPath, nil, observationText("nested instruction\n"))
			got := c.observe(ObserveParams{RepoRoot: nested.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()})
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" || got.Mode != "refuse" {
				t.Fatalf("nested instruction inverse classified as %+v", got)
			}
		})
	}

	t.Run("committed engine floor", func(t *testing.T) {
		engine := newRepositoryObservationFixture(t)
		c := engine.comparison(observeTreeB, "diff --git a/bin/metasystem b/bin/metasystem\n-committed engine\n", "bin/metasystem")
		c.exactRevert("bin/metasystem")
		c.declare("bin/metasystem", observationText("committed engine\n"), nil)
		c.declareInverseEntries("bin/metasystem", nil, observationText("committed engine\n"))
		got := c.observe(ObserveParams{RepoRoot: engine.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "runtime-path-refused" || got.Mode != "refuse" {
			t.Fatalf("committed engine inverse classified as %+v", got)
		}
	})

	t.Run("record exact revert remains refused", func(t *testing.T) {
		record := newRepositoryObservationFixture(t)
		c := record.comparison(observeTreeB, "diff --git a/memory/receipts.log b/memory/receipts.log\n-receipt=appended\n", "memory/receipts.log")
		c.exactRevert("memory/receipts.log")
		c.declare("memory/receipts.log", observationText("receipt=existing\nreceipt=appended\n"), observationText("receipt=existing\n"))
		c.declareInverseEntries("memory/receipts.log", observationText("receipt=existing\n"), observationText("receipt=existing\nreceipt=appended\n"))
		got := c.observe(ObserveParams{RepoRoot: record.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "exact-revert-record-refused" {
			t.Fatalf("record exact revert widened during path-class cutover: %+v", got)
		}
	})
}

func TestObserveTierOneDirectFixBoundsAndReceipt(t *testing.T) {
	t.Run("lawful area landing with absent gateWidth", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("")
		f.write("docs/constant.txt", "one changed line\n")
		f.git("add", "docs/constant.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "tier-1" {
			t.Fatalf("lawful tier-1 landing classified as %+v", got)
		}
	})

	t.Run("protected floor", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("internal/goal/constant.txt", "protected\n")
		f.git("add", "internal/goal/constant.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Bar != BarRefusal || got.Code != "tier1-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("tier-1 floor landing classified as %+v", got)
		}
	})

	t.Run("foreign tree receipt", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("docs/first.txt", "first\n")
		f.git("add", "docs/first.txt")
		_, receipt := f.tierOneReceipt("true")
		f.write("docs/second.txt", "second\n")
		f.git("add", "docs/second.txt")
		candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
		if err != nil {
			t.Fatal(err)
		}
		foreign, err := os.ReadFile(receipt)
		if err != nil {
			t.Fatal(err)
		}
		expectedPath := TestReceiptPath(f.root, candidate)
		f.writeBytes(filepath.Join("artifacts", "agents", "landing", "receipts", candidate+".json"), foreign)
		got := Observe(tierOneParams(f, candidate, expectedPath))
		if got.Bar != BarRefusal || got.Code != "tier1-receipt-refused" || got.Mode != "refuse" {
			t.Fatalf("foreign-tree receipt classified as %+v", got)
		}
	})

	t.Run("goal raised to tier two after root dispatch", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.writeHeldGoalWithTier("tier-one", "m9", "L1", 2)
		f.git("add", "plans/goals/tier-one.md")
		f.git("commit", "-qm", "raise goal to tier two")
		f.write("docs/constant.txt", "change\n")
		f.git("add", "docs/constant.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Bar != BarRefusal || got.Code != "tier1-goal-tier-2-refused" || got.Mode != "refuse" {
			t.Fatalf("tier-two goal with a tier-one root classified as %+v", got)
		}
	})

	t.Run("forty-one changed lines", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("docs/large.txt", strings.Repeat("changed\n", 41))
		f.git("add", "docs/large.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Bar != BarRefusal || got.Code != "tier1-line-bound-refused" {
			t.Fatalf("forty-one changed lines classified as %+v", got)
		}
	})

	t.Run("four changed files", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		for _, name := range []string{"one", "two", "three", "four"} {
			f.write("docs/"+name+".txt", name+"\n")
		}
		f.git("add", "docs")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Bar != BarRefusal || got.Code != "tier1-file-bound-refused" {
			t.Fatalf("four changed files classified as %+v", got)
		}
	})

	t.Run("missing receipt", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("docs/constant.txt", "change\n")
		f.git("add", "docs/constant.txt")
		candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
		if err != nil {
			t.Fatal(err)
		}
		params := tierOneParams(f, candidate, "")
		got := Observe(params)
		if got.Bar != BarRefusal || got.Code != "tier1-declaration-refused" || got.Mode != "refuse" {
			t.Fatalf("missing tier-1 receipt classified as %+v", got)
		}
	})

	t.Run("missing goal", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("docs/constant.txt", "change\n")
		f.git("add", "docs/constant.txt")
		candidate, receipt := f.tierOneReceipt("true")
		params := tierOneParams(f, candidate, receipt)
		params.Goal = ""
		got := Observe(params)
		if got.Bar != BarRefusal || got.Code != "tier1-declaration-refused" || got.Mode != "refuse" {
			t.Fatalf("missing tier-1 goal classified as %+v", got)
		}
	})

	for name, mutate := range map[string]func(map[string]any){
		"wrong goal":      func(record map[string]any) { record["goalId"] = "other" },
		"wrong tier":      func(record map[string]any) { record["goalTier"] = 2 },
		"unknown width":   func(record map[string]any) { record["gateWidth"] = "wide" },
		"non-root record": func(record map[string]any) { record["parentJob"] = "parent" },
	} {
		t.Run(name+" root", func(t *testing.T) {
			f := newObserveFixture(t)
			f.prepareTierOne("area")
			record := map[string]any{
				"jobId": "tier-one-root", "parentJob": nil, "role": "implementer",
				"goalId": "tier-one", "goalRevision": 1, "goalTier": 1, "gateWidth": "area",
			}
			mutate(record)
			f.writeChainRecord("tier-one-root", record)
			f.write("docs/constant.txt", "change\n")
			f.git("add", "docs/constant.txt")
			candidate, receipt := f.tierOneReceipt("true")
			got := Observe(tierOneParams(f, candidate, receipt))
			wantCode := "tier1-root-refused"
			if name == "wrong goal" {
				wantCode = "goal-binding-mismatch"
			}
			if got.Code != wantCode || got.Mode != "refuse" {
				t.Fatalf("invalid tier-1 root classified as %+v", got)
			}
		})
	}
}

func TestObserveTierOneRefusesForbiddenDiffShapes(t *testing.T) {
	t.Run("binary", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.writeBytes("docs/binary.dat", []byte{'a', 0, 'b'})
		f.git("add", "docs/binary.dat")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Code != "tier1-diff-shape-refused" || got.Mode != "refuse" {
			t.Fatalf("binary change classified as %+v", got)
		}
	})

	t.Run("rename", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("docs/old.txt", "unchanged content\n")
		f.prepareTierOne("area")
		f.git("mv", "docs/old.txt", "docs/new.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Code != "tier1-diff-shape-refused" || got.Mode != "refuse" {
			t.Fatalf("rename classified as %+v", got)
		}
	})

	t.Run("mode only", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("docs/mode.txt", "same content\n")
		f.prepareTierOne("area")
		if err := os.Chmod(filepath.Join(f.root, "docs", "mode.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
		f.git("add", "docs/mode.txt")
		candidate, receipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, receipt))
		if got.Code != "tier1-diff-shape-refused" || got.Mode != "refuse" {
			t.Fatalf("mode-only change classified as %+v", got)
		}
	})
}

func TestObserveTierOneFullGateRequiresExactCommand(t *testing.T) {
	f := newObserveFixture(t)
	for _, script := range []string{"go-gate.sh", "dispatch-fixtures.sh", "goal-cli-fixtures.sh"} {
		f.write("scripts/agents/"+script, "#!/usr/bin/env bash\nexit 0\n")
		if err := os.Chmod(filepath.Join(f.root, "scripts", "agents", script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.prepareTierOne("full")
	f.write("docs/constant.txt", "change\n")
	f.git("add", "docs/constant.txt")
	candidate, receipt := f.tierOneReceipt("true")
	got := Observe(tierOneParams(f, candidate, receipt))
	if got.Code != "tier1-full-gate-refused" || got.Mode != "refuse" {
		t.Fatalf("area command under a full gate classified as %+v", got)
	}

	candidate, receipt = f.tierOneReceipt(fullBatteryCommand)
	got = Observe(tierOneParams(f, candidate, receipt))
	if got.Bar != BarDirectFix || got.Code != "tier-1" || got.Verdict != "pass" {
		t.Fatalf("exact full-battery receipt classified as %+v", got)
	}
}

func TestSTR4R1FullWidthChainRequiresFullBatteryReceipt(t *testing.T) {
	run := func(command string) Observation {
		f := newObserveFixture(t)
		for _, script := range []string{"go-gate.sh", "dispatch-fixtures.sh", "goal-cli-fixtures.sh"} {
			f.write("scripts/agents/"+script, "#!/usr/bin/env bash\nexit 0\n")
			if err := os.Chmod(filepath.Join(f.root, "scripts", "agents", script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		f.git("add", "scripts/agents")
		f.git("commit", "-qm", "full battery fixture")
		f.write("internal/x.go", "package internal\n")
		f.git("add", "internal/x.go")
		candidate, receipt := f.tierOneReceipt(command)
		f.writeChainRecord("full-chain", map[string]any{
			"jobId": "full-chain", "parentJob": nil, "role": "implementer", "round": 1,
			"goalTier": 2, "gateWidth": "full", "destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		})
		f.writeChainReview("full-chain", 1, "full-chain", candidate)
		return Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "full-chain", TestReceipt: receipt})
	}
	got := run("true")
	if got.Code != "chain-full-gate-refused" || got.Verdict != "would-refuse" {
		t.Fatalf("area receipt under full-width tier-two chain classified as %+v", got)
	}
	got = run(fullBatteryCommand)
	if got.Bar != BarChain || got.Verdict != "pass" {
		t.Fatalf("full-battery receipt under full-width tier-two chain classified as %+v", got)
	}
}

func TestTierOneClassCutoverAcceptsTheTwoClassLandingBase(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	f.base("scripts/agents/landing-classes.json", `{
  "schemaVersion": 1,
  "enginePolicyVersion": 1,
  "classes": [
    {"id":"register-carriage","pathRule":"path-class-record","requiredFields":[],"authorizedBy":"R-35-m0"},
    {"id":"exact-revert","pathRule":"tree-shaped-exact-inverse","requiredFields":["revert-of"],"authorizedBy":"R-35-m0"}
  ]
}
`)
	c := f.comparison(observeTreeB, "diff --git a/memory/receipts.log b/memory/receipts.log\n+receipt=cutover\n", "memory/receipts.log")
	c.declare("memory/receipts.log", observationText("receipt=existing\n"), observationText("receipt=existing\nreceipt=cutover\n"))
	got := c.observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
	})
	if got.Bar != BarDirectFix || got.Code != "register-carriage" {
		t.Fatalf("new evaluator could not land from the previous two-class base: %+v", got)
	}
}

func TestSliceOneRetainsHandoffCarriage(t *testing.T) {
	fixture := newRepositoryObservationFixture(t)
	c := fixture.comparison(observeTreeB, "diff --git a/plans/handoff-fixture-1.md b/plans/handoff-fixture-1.md\n+# fixture handoff\n", "plans/handoff-fixture-1.md")
	c.declare("plans/handoff-fixture-1.md", nil, observationText("# fixture handoff\n"))
	got := c.observe(ObserveParams{
		RepoRoot: fixture.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
	})
	if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
		t.Fatalf("new handoff lost register carriage during the manifest transition: %+v", got)
	}
}

func TestObserveManifestIsBehavior(t *testing.T) {
	fixture := newRepositoryObservationFixture(t)
	before := string(fixture.baseFiles["scripts/agents/path-classes.txt"])
	c := fixture.comparison(observeTreeB, "diff --git a/scripts/agents/path-classes.txt b/scripts/agents/path-classes.txt\n+install:memory/ record\n", "scripts/agents/path-classes.txt")
	c.declare("scripts/agents/path-classes.txt", &before, observationText("install:memory/ record\n"))
	got := c.observe(ObserveParams{
		RepoRoot: fixture.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" {
		t.Fatalf("path class manifest change escaped the behavior floor: %+v", got)
	}
}

func TestObserveClassifiesEachPathClass(t *testing.T) {
	for name, fixture := range map[string]struct {
		path       string
		wantCode   string
		wantPass   bool
		wantDetail bool
	}{
		"behavior":     {path: "internal/x.go", wantCode: "direct-fix-floor-refused"},
		"record":       {path: "plans/handoff-fixture-1.md", wantCode: "register-carriage", wantPass: true},
		"ledger":       {path: "plans/goals/x.md", wantCode: "ledger-path-not-goal-verb"},
		"runtime":      {path: "bin/metasystem", wantCode: "runtime-path-refused"},
		"unclassified": {path: "product.txt", wantCode: "path-unclassified", wantDetail: true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRepositoryObservationFixture(t)
			c := f.comparison(observeTreeB, "diff --git a/"+fixture.path+" b/"+fixture.path+"\n+changed\n", fixture.path)
			var before *string
			if fixture.path == "product.txt" {
				before = observationText("before\n")
			}
			c.declare(fixture.path, before, observationText("changed\n"))
			got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
			if got.Code != fixture.wantCode || (got.Verdict == "pass") != fixture.wantPass {
				t.Fatalf("%s path classified as %+v", name, got)
			}
			if fixture.wantDetail && (len(got.Unclassified) != 1 || got.Unclassified[0] != "product.txt" || got.Refusal != "path product.txt has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for product.txt or its directory to scripts/agents/path-classes.txt") {
				t.Fatalf("unclassified detail was not preserved from the base manifest: %+v", got)
			}
		})
	}
}

func TestObserveChainRefusesLedgerRuntimeAndUnclassifiedPaths(t *testing.T) {
	for name, leg := range map[string]struct {
		path string
		code string
	}{
		"ledger":       {path: "plans/goals/x.md", code: "ledger-path-not-goal-verb"},
		"runtime":      {path: "bin/metasystem", code: "runtime-path-refused"},
		"unclassified": {path: "product.txt", code: "path-unclassified"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRepositoryObservationFixture(t)
			change := chainAddition(leg.path, "changed\n")
			if leg.path == "product.txt" {
				change = chainReplacement(leg.path, "before\n", "changed\n")
			}
			c := f.chainCase(observeTreeB, change)
			f.writeChainRecord("class-chain", chainRoot("class-chain"))
			patch := chainDiff(change)
			f.writeChainReview("class-chain", 1, "class-chain", chainReviewedTree, patch)
			c.bindChain(patch, chainReviewedTree, change)
			c.wantChainPolicy(c.changed...)
			got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "class-chain"})
			if got.Code != leg.code || got.Verdict != "would-refuse" {
				t.Fatalf("certified %s path classified as %+v", name, got)
			}
		})
	}
}

func TestObserveExactRevertRefusesByClass(t *testing.T) {
	for name, leg := range map[string]struct {
		path string
		code string
	}{
		"behavior":     {path: "internal/x.go", code: "direct-fix-floor-refused"},
		"record":       {path: "records/misc/x.md", code: "exact-revert-record-refused"},
		"ledger":       {path: "plans/goals/x.md", code: "ledger-path-not-goal-verb"},
		"runtime":      {path: "bin/metasystem", code: "runtime-path-refused"},
		"unclassified": {path: "product.txt", code: "path-unclassified"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRepositoryObservationFixture(t)
			c := f.comparison(observeTreeB, "diff --git a/"+leg.path+" b/"+leg.path+"\n-after\n", leg.path)
			c.exactRevert(leg.path)
			c.declare(leg.path, observationText("after\n"), nil)
			c.declareInverseEntries(leg.path, nil, observationText("after\n"))
			got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "exact-revert", RevertOf: c.revertCommit()})
			if got.Code != leg.code || got.Verdict != "would-refuse" {
				t.Fatalf("exact revert of %s path classified as %+v", name, got)
			}
		})
	}
}

func TestObserveAdoptedApplicationPathsAreOutside(t *testing.T) {
	t.Run("exact revert passes", func(t *testing.T) {
		f := newAdoptedObserveFixture(t)
		f.write("product.txt", "after\n")
		f.git("add", "product.txt")
		f.git("commit", "-qm", "application change to revert")
		revertOf := f.git("rev-parse", "HEAD")
		f.write("product.txt", "before\n")

		got := Observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "exact-revert", RevertOf: revertOf,
		})
		if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "exact-revert" {
			t.Fatalf("exact inverse of adopted application path classified as %+v", got)
		}
	})

	t.Run("certified chain passes", func(t *testing.T) {
		f := newAdoptedObserveFixture(t)
		f.write("product.txt", "chain change\n")
		candidate := f.tree()
		f.writeChainRecord("application-chain", map[string]any{
			"jobId": "application-chain", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		})
		f.writeChainReview("application-chain", 1, "application-chain", candidate)

		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "application-chain"})
		if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
			t.Fatalf("certified adopted application path classified as %+v", got)
		}
	})

	t.Run("register carriage refuses", func(t *testing.T) {
		f := newAdoptedObserveFixture(t)
		f.write("product.txt", "carriage change\n")

		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage"})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-path-refused" {
			t.Fatalf("register carriage of adopted application path classified as %+v", got)
		}
	})
}

func TestObserveUnclassifiedDetailFromBase(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\n+changed\n", "product.txt")
	c.declare("product.txt", observationText("before\n"), observationText("changed\n"))
	candidate := c.candidate
	manifest, err := os.ReadFile(filepath.Join(f.root, "scripts/agents/path-classes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f.write("scripts/agents/path-classes.txt", string(manifest)+"install:product.txt record\n")
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, DirectFix: "register-carriage"})
	if got.Code != "path-unclassified" || len(got.Unclassified) != 1 || got.Unclassified[0] != "product.txt" || !strings.Contains(got.Refusal, "path product.txt has no class") {
		t.Fatalf("candidate manifest reclassified its own landing: %+v", got)
	}
}

func TestObserveFloorPrecedesGoalOwnershipValidation(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	f.base("plans/goals/fx.md", string(observationHeldGoal("fx", "m9", "L1")))
	f.base("records/misc/base.md", "base\n")

	c := f.comparison(observeTreeB, "diff --git a/internal/x.go b/internal/x.go\n+package internal\n", "internal/x.go")
	c.declare("internal/x.go", nil, observationText("package internal\n"))
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
	if got.Code != "direct-fix-floor-refused" {
		t.Fatalf("goal ownership ran before the behavior floor: %+v", got)
	}
	if err := os.Remove(filepath.Join(f.root, "internal/x.go")); err != nil {
		t.Fatal(err)
	}
	c = f.comparison(observeTreeC, "diff --git a/records/misc/base.md b/records/misc/base.md\n+append\n", "records/misc/base.md")
	c.declare("records/misc/base.md", observationText("base\n"), observationText("base\nappend\n"))
	got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
	if got.Code != "goal-item-not-held" {
		t.Fatalf("record-only foreign goal classified as %+v", got)
	}
}

func TestObserveClassPrecedenceIsSetWide(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\ndiff --git a/bin/metasystem b/bin/metasystem\ndiff --git a/plans/goals/new.md b/plans/goals/new.md\n", "product.txt", "bin/metasystem", "plans/goals/new.md")
	c.declare("product.txt", observationText("before\n"), observationText("unclassified\n"))
	c.declare("bin/metasystem", nil, observationText("runtime\n"))
	c.declare("plans/goals/new.md", nil, observationText("ledger\n"))
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
	if got.Code != "ledger-path-not-goal-verb" {
		t.Fatalf("ledger did not precede runtime and unclassified paths: %+v", got)
	}

	c = f.comparison(observeTreeC, "diff --git a/product.txt b/product.txt\ndiff --git a/bin/metasystem b/bin/metasystem\ndiff --git a/plans/goals/new.md b/plans/goals/new.md\ndiff --git a/internal/x.go b/internal/x.go\n", "product.txt", "bin/metasystem", "plans/goals/new.md", "internal/x.go")
	c.declare("product.txt", observationText("before\n"), observationText("unclassified\n"))
	c.declare("bin/metasystem", nil, observationText("runtime\n"))
	c.declare("plans/goals/new.md", nil, observationText("ledger\n"))
	c.declare("internal/x.go", nil, observationText("package internal\n"))
	got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
	if got.Code != "direct-fix-floor-refused" {
		t.Fatalf("behavior did not precede every other class: %+v", got)
	}
}

func TestObserveRegisterCarriageRefusesStagedNarratorDigestRewrite(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	c := f.comparison(observeTreeB, "diff --git a/records/narrator-digest.log b/records/narrator-digest.log\n+digest=rewritten\n", "records/narrator-digest.log")
	c.declare("records/narrator-digest.log", observationText("digest=existing\n"), observationText("digest=rewritten\n"))

	got := c.observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage",
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
		t.Fatalf("staged narrator digest rewrite classified as %+v", got)
	}
}

func TestObserveUndeclaredLandingRecordsWouldRefuse(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	f.base("product.txt", "before\n")
	c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\n+undeclared change\n", "product.txt")
	c.declare("product.txt", observationText("before\n"), observationText("undeclared change\n"))
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "missing-declaration" || got.Mode != "refuse" {
		t.Fatalf("undeclared landing classified as %+v", got)
	}
	if got.VerdictTrailer != "would-refuse code=missing-declaration" {
		t.Fatalf("undeclared landing has non-durable verdict value %q", got.VerdictTrailer)
	}
	for name, params := range map[string]ObserveParams{
		"orphaned revert parameter": {
			RepoRoot: f.root, CandidateTree: c.candidate, RevertOf: strings.Repeat("0", 40),
		},
		"revert parameter on register carriage": {
			RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", RevertOf: strings.Repeat("0", 40),
		},
		"exact revert missing its commit": {
			RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "exact-revert",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := c.observe(params)
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "conflicting-declarations" || got.Mode != "refuse" {
				t.Fatalf("partial classification parameter classified as %+v", got)
			}
		})
	}
}

func TestObservePromotionRecordIsStrictAndAbsentMeansObserve(t *testing.T) {
	t.Parallel()
	observeMissingDeclaration := func(f *repositoryObservationFixture, content string) Observation {
		f.t.Helper()
		f.base("product.txt", "before\n")
		c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\n+"+content, "product.txt")
		c.declare("product.txt", observationText("before\n"), &content)
		return c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate})
	}

	absent := newRepositoryObservationFixture(t)
	if _, err := os.Stat(filepath.Join(absent.root, "scripts", "agents", "landing-promotion.json")); !os.IsNotExist(err) {
		t.Fatalf("deleted promotion record exists or cannot be inspected: %v", err)
	}
	absentObservation := observeMissingDeclaration(absent, "undeclared without promotion record\n")
	if absentObservation.Mode != "refuse" || !absentObservation.RefusesAgent || absentObservation.Code != "missing-declaration" {
		t.Fatalf("missing promotion record changed refusing behavior: %+v", absentObservation)
	}

	ignored := newRepositoryObservationFixture(t)
	ignored.write("scripts/agents/landing-promotion.json", "not policy\n")
	ignoredObservation := observeMissingDeclaration(ignored, "undeclared with ignored legacy file\n")
	if ignoredObservation.Mode != absentObservation.Mode || ignoredObservation.Code != absentObservation.Code ||
		ignoredObservation.RefusesAgent != absentObservation.RefusesAgent || ignoredObservation.VerdictTrailer != absentObservation.VerdictTrailer {
		t.Fatalf("legacy promotion filename changed behavior: absent=%+v present=%+v", absentObservation, ignoredObservation)
	}
}

func TestObserveVerdictSurvivesLanding(t *testing.T) {
	commitWrapper, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "commit.sh"))
	if err != nil {
		t.Fatal(err)
	}
	landDriver, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "land.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`landing observe --root "$root" --tree "$landing_tree"`,
		`--root-job "$landing_root_job"`,
		`--test-receipt "$landing_test_receipt"`,
		`--trailer "Landing-Provenance: $landing_provenance"`,
		`--trailer "Landing-Provenance-Verdict: $landing_verdict"`,
	} {
		if !strings.Contains(string(commitWrapper), required) {
			t.Fatalf("commit chokepoint lost %q", required)
		}
	}
	for _, required := range []string{"--chain", "--direct-fix", "--revert-of", "--root-job", "--tests", "landing test-receipt"} {
		if !strings.Contains(string(landDriver), required) {
			t.Fatalf("landing driver does not carry %s", required)
		}
	}

	f := newObserveFixture(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	command := exec.Command("git", "init", "--bare", "-q", remote)
	command.Env = gittree.ScrubbedEnviron()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("initialize remote: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(remote, "HEAD")); err != nil {
		t.Fatalf("fixture remote vanished: %v", err)
	}
	f.git("remote", "add", "origin", remote)
	f.write("product.txt", "landing\n")
	f.git("add", "product.txt")
	tree := f.git("write-tree")
	observation := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: tree})
	f.git("commit", "-q", "-m", "observed landing",
		"--trailer", "Landing-Provenance: "+observation.Provenance,
		"--trailer", "Landing-Provenance-Verdict: "+observation.VerdictTrailer)
	f.git("push", "-q", "origin", "main")

	messageCommand := exec.Command("git", "--git-dir="+remote, "log", "-1", "--format=%B", "refs/heads/main")
	messageCommand.Env = gittree.ScrubbedEnviron()
	message, err := messageCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("read landed message: %v\n%s", err, message)
	}
	for _, want := range []string{
		"Landing-Provenance: " + observation.Provenance,
		"Landing-Provenance-Verdict: would-refuse code=missing-declaration",
	} {
		if !strings.Contains(string(message), want) {
			t.Fatalf("landed commit lost %q:\n%s", want, message)
		}
	}
}

func TestObserveTierOneBoundsIgnoreTheReceiptLedger(t *testing.T) {
	const receipt = "receipt=existing\n1|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=tier-one|note=fixture\n"
	t.Run("three files and the receipt line", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		for _, name := range []string{"one", "two", "three"} {
			f.write("docs/"+name+".txt", name+"\n")
		}
		f.write("memory/receipts.log", receipt)
		f.git("add", "docs", "memory/receipts.log")
		candidate, testReceipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, testReceipt))
		if got.Verdict != "pass" || got.Bar != BarDirectFix {
			t.Fatalf("three files with their receipt line classified as %+v", got)
		}
	})
	t.Run("forty lines and the receipt line", func(t *testing.T) {
		f := newObserveFixture(t)
		f.prepareTierOne("area")
		f.write("docs/large.txt", strings.Repeat("changed\n", 40))
		f.write("memory/receipts.log", receipt)
		f.git("add", "docs", "memory/receipts.log")
		candidate, testReceipt := f.tierOneReceipt("true")
		got := Observe(tierOneParams(f, candidate, testReceipt))
		if got.Verdict != "pass" || got.Bar != BarDirectFix {
			t.Fatalf("forty lines with their receipt line classified as %+v", got)
		}
	})
}

func TestObserveChainCarriesAnAppendedReceiptLedger(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	change := chainAddition("internal/x.go", "package internal\n")
	patch := chainDiff(change)
	f.writeChainRecord("impl-chain", chainRoot("impl-chain"))
	f.writeChainRecord("impl-chain-r2", map[string]any{
		"jobId": "impl-chain-r2", "parentJob": "impl-chain", "role": "implementer",
		"round": 2, "destructiveReach": "DESIGN-BEARING", "status": "completed",
	})
	f.writeChainReview("impl-chain", 1, "impl-chain", chainReviewedTree, patch)
	receipt := chainReplacement("memory/receipts.log", "receipt=existing\n",
		"receipt=existing\n1|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=fx|note=fixture\n")
	c := f.chainCase(observeTreeB, change, receipt)
	c.bindChain(patch, chainReviewedTree, change)
	c.wantChainPolicy(c.changed...)
	c.wantChainReceiptAppend(1)
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
	if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
		t.Fatalf("chain landing with its appended receipt line classified as %+v", got)
	}
	rewritten := chainReplacement("memory/receipts.log", "receipt=existing\n", "rewritten\n")
	c = f.chainCase(observeTreeC, change, rewritten)
	c.bindChain(patch, chainReviewedTree, change)
	c.wantChainPolicy(c.changed...)
	c.wantChainReceiptAppend(1)
	got = c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "impl-chain"})
	if got.Code != "chain-has-uncarried-paths" {
		t.Fatalf("chain landing that rewrites the receipt ledger classified as %+v", got)
	}
}

func TestObservationBindsTheChainRootsGoalAndRevision(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	f.baseHeldGoalAtRevision("ship-widget", "m9", "L1", 3)
	change := chainAddition("internal/x.go", "package internal\n")
	patch := chainDiff(change)
	record := chainRoot("goal-chain")
	record["goalId"], record["goalRevision"] = "ship-widget", 3
	f.writeChainRecord("goal-chain", record)
	f.writeChainReview("goal-chain", 1, "goal-chain", chainReviewedTree, patch)
	ready := func(candidate string) *observationCase {
		c := f.chainCase(candidate, change)
		c.bindChain(patch, chainReviewedTree, change)
		c.wantChainPolicy(c.changed...)
		c.wantChainGoal("ship-widget")
		return c
	}
	c := ready(observeTreeB)
	params := ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Chain: "goal-chain", Goal: "ship-widget", Actor: "m9+L1"}
	got := c.observe(params)
	if got.Code != "closed-chain" || got.GoalRevision != 3 {
		t.Fatalf("matching goal-bound chain classified as %+v", got)
	}
	c = f.chainCase(observeTreeB, change)
	missing := params
	missing.Goal = ""
	if got := c.observe(missing); got.Code != "goal-binding-missing" || got.Verdict != "would-refuse" {
		t.Fatalf("goal-bound chain without --goal classified as %+v", got)
	}
	c = f.chainCase(observeTreeB, change)
	mismatch := params
	mismatch.Goal = "ship-gadget"
	if got := c.observe(mismatch); got.Code != "goal-binding-mismatch" || got.Verdict != "would-refuse" {
		t.Fatalf("goal-bound chain under another goal classified as %+v", got)
	}
	f.baseHeldGoalAtRevision("ship-widget", "m9", "L1", 4)
	c = ready(observeTreeC)
	params.CandidateTree = c.candidate
	if got := c.observe(params); got.Code != "goal-revision-moved" || got.Verdict != "would-refuse" {
		t.Fatalf("chain at an old claim revision classified as %+v", got)
	}
	delete(record, "goalRevision")
	f.writeChainRecord("goal-chain", record)
	c = f.chainCase(observeTreeC, change)
	if got := c.observe(params); got.Code != "chain-record-malformed" || got.Verdict != "would-refuse" {
		t.Fatalf("goal-bound chain without a positive revision classified as %+v", got)
	}
	goalFreeRoot := map[string]any{}
	for key, value := range record {
		goalFreeRoot[key] = value
	}
	goalFreeRoot["goalId"] = nil
	f.writeChainRecord("goal-chain", goalFreeRoot)
	c = ready(observeTreeC)
	if got := c.observe(params); got.Code != "closed-chain" || got.GoalRevision != 4 {
		t.Fatalf("goal-free root with a currently held --goal classified as %+v", got)
	}
	carriage := newRepositoryObservationFixture(t)
	carriage.baseHeldGoalAtRevision("ship-widget", "m9", "L1", 4)
	recordChange := chainAddition("records/misc/fixture.md", "fixture\n")
	recordCase := carriage.chainCase(observeTreeB, recordChange)
	recordCase.wantChain("head", 1)
	recordCase.wantChain("paths:"+observeBaseTree+":"+recordCase.candidate, 1)
	recordCase.wantChainRegister(recordChange.path)
	recordCase.wantChainGoal("ship-widget")
	got = recordCase.observe(ObserveParams{
		RepoRoot: carriage.root, CandidateTree: recordCase.candidate,
		DirectFix: "register-carriage", Goal: "ship-widget", Actor: "m9+L1",
	})
	if got.Code != "register-carriage" || got.GoalRevision != 4 {
		t.Fatalf("register carriage did not bind the base claim revision: %+v", got)
	}
	tierOne := newRepositoryObservationFixture(t)
	tierChange := chainAddition("docs/constant.txt", "change\n")
	tierCase := tierOne.chainCase(observeTreeB, tierChange)
	tierOne.writeChainRecord("tier-one-root", map[string]any{
		"jobId": "tier-one-root", "parentJob": nil, "role": "implementer",
		"goalId": "other-goal", "goalRevision": 1, "goalTier": 1, "gateWidth": "area",
	})
	if got := tierCase.observe(ObserveParams{
		RepoRoot: tierOne.root, CandidateTree: tierCase.candidate, DirectFix: "tier-1",
		Goal: "tier-one", Actor: "m9+L1", RootJob: "tier-one-root", TestReceipt: "unused",
	}); got.Code != "goal-binding-mismatch" || got.Mode != "refuse" {
		t.Fatalf("tier-one root under another goal classified as %+v", got)
	}
}

func TestObservationRequiresAGoalFromANonHumanActorForGoalBoundChain(t *testing.T) {
	free := newRepositoryObservationFixture(t)
	free.base("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
		Free: &goal.FreeRecord{Declared: "2026-09-03T08:00:00Z", Origin: "human", Digest: strings.Repeat("a", 64)},
	})))
	change := chainAddition("internal/x.go", "package internal\n")
	c := free.chainCase(observeTreeB, change)
	record := chainRoot("goal-chain")
	record["goalId"], record["goalRevision"] = "ship-widget", 3
	free.writeChainRecord("goal-chain", record)
	free.writeChainReview("goal-chain", 1, "goal-chain", chainReviewedTree, chainDiff(change))
	if got := c.observe(ObserveParams{RepoRoot: free.root, CandidateTree: c.candidate, Chain: "goal-chain", Actor: "m9+lineage"}); got.Code != "goal-binding-missing" {
		t.Fatalf("Goal-free ledger overrode a goal-bound chain root: %+v", got)
	}
}

func (f *observeFixture) writeHeldGoalAtRevision(id, machine, lineage string, revision uint64) {
	f.t.Helper()
	history := make([]goal.HistoryLine, revision)
	for index := range history {
		at := fmt.Sprintf("2026-09-03T08:%02d:00Z", index)
		history[index] = goal.HistoryLine{
			At: at, Opid: fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FAW-%s-%08x", machine, index+1),
			Verb: "edit", Actor: machine + "+" + lineage, Targets: []string{id}, Keep: -1,
		}
	}
	history[revision-1].Verb = "claim"
	f.writeBytes(filepath.Join("plans", "goals", id+".md"), goal.RenderFile(&goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Exercise landing binding.", OpenedAt: "2026-09-03T08:00:00Z", Revision: revision,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: history[revision-1].At, Revision: revision, AccountingRevision: revision},
		History: history,
	}))
}
