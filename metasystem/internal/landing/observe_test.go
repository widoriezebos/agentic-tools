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
	for _, policyFile := range []string{"path-classes.txt", "landing-classes.json", "landing-promotion.json"} {
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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
		"goalId": "tier-one", "goalTier": 1,
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
	f := newObserveFixture(t)
	f.write("internal/x.go", "package internal\n")
	candidate := f.tree()
	validRecord := map[string]any{
		"jobId": "impl-chain", "parentJob": nil, "role": "implementer",
		"round": 1, "destructiveReach": "DESIGN-BEARING", "chainClosed": true,
	}
	f.writeChainRecord("impl-chain", validRecord)
	f.writeChainRecord("impl-chain-r2", map[string]any{
		"jobId": "impl-chain-r2", "parentJob": "impl-chain", "role": "implementer",
		"round": 2, "destructiveReach": "DESIGN-BEARING", "status": "completed",
	})
	// This is the real root-id conformance layout: even after a round-two
	// correction, invoking conformance with the root writes review.json in
	// rounds/1 and records the invoked root as implementerJob.
	f.writeChainReview("impl-chain", 1, "impl-chain", candidate)

	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "impl-chain"})
	if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
		t.Fatalf("closed chain classified as %+v", got)
	}
	t.Run("follow-up-id conformance layout", func(t *testing.T) {
		follow := newObserveFixture(t)
		follow.write("internal/x.go", "package internal\n")
		followCandidate := follow.tree()
		followRecord := map[string]any{
			"jobId": "follow-chain", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		}
		follow.writeChainRecord("follow-chain", followRecord)
		follow.writeChainRecord("follow-chain-r2", map[string]any{
			"jobId": "follow-chain-r2", "parentJob": "follow-chain", "role": "implementer", "round": 2,
			"destructiveReach": "DESIGN-BEARING", "status": "completed",
		})
		follow.writeChainReview("follow-chain", 2, "follow-chain-r2", followCandidate)
		got := Observe(ObserveParams{RepoRoot: follow.root, CandidateTree: followCandidate, Chain: "follow-chain"})
		if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
			t.Fatalf("follow-up-id conformance layout classified as %+v", got)
		}
	})
	t.Run("closed critic selects the certified root-id review", func(t *testing.T) {
		selected := newObserveFixture(t)
		selected.write("internal/x.go", "package internal\n")
		selectedTree := selected.tree()
		selected.write("internal/x.go", "package internal // stale\n")
		staleTree := selected.tree()
		selected.writeChainRecord("selected-chain", map[string]any{
			"jobId": "selected-chain", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
			"independentCritiqueJobRef": "selected-critic",
		})
		selected.writeChainRecord("selected-chain-r2", map[string]any{
			"jobId": "selected-chain-r2", "parentJob": "selected-chain", "role": "implementer", "round": 2,
			"destructiveReach": "DESIGN-BEARING", "status": "completed",
		})
		selected.writeChainRecord("selected-critic", map[string]any{
			"jobId": "selected-critic", "parentJob": nil, "role": "code-critic", "round": 1,
			"reviews": "selected-chain-r2", "status": "completed",
		})
		selected.writeChainReview("selected-chain", 1, "selected-chain", selectedTree)
		selected.writeChainReview("selected-chain", 2, "selected-chain-r2", staleTree)
		result, err := json.Marshal(map[string]any{"reviewedTree": selectedTree})
		if err != nil {
			t.Fatal(err)
		}
		selected.write("artifacts/agents/selected-critic/rounds/1/return.json", string(append(result, '\n')))
		got := Observe(ObserveParams{RepoRoot: selected.root, CandidateTree: selectedTree, Chain: "selected-chain"})
		if got.Bar != BarChain || got.Verdict != "pass" {
			t.Fatalf("closed critic's selected conformance output classified as %+v", got)
		}
	})
	t.Run("certified change with register carriage", func(t *testing.T) {
		bundled := newObserveFixture(t)
		bundled.write("internal/x.go", "package internal\n")
		reviewed := bundled.tree()
		bundled.writeChainRecord("bundle-chain", map[string]any{
			"jobId": "bundle-chain", "parentJob": nil, "role": "implementer", "round": 1,
			"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
		})
		bundled.writeChainReview("bundle-chain", 1, "bundle-chain", reviewed)
		bundled.write("memory/receipts.log", "receipt=existing\nreceipt=landing\n")
		got := Observe(ObserveParams{
			RepoRoot: bundled.root, CandidateTree: bundled.tree(), Chain: "bundle-chain",
			DirectFix: "register-carriage",
		})
		if got.Bar != BarChain || got.Verdict != "pass" ||
			!strings.Contains(got.Provenance, "chain=bundle-chain direct-fix class=register-carriage") ||
			got.VerdictTrailer != "pass bar=a carriage=register-carriage" {
			t.Fatalf("chain plus append-only register carriage classified as %+v", got)
		}

		bundled.write("memory/known-issues.md", "protected but not carriage\n")
		got = Observe(ObserveParams{
			RepoRoot: bundled.root, CandidateTree: bundled.tree(), Chain: "bundle-chain",
			DirectFix: "register-carriage",
		})
		if got.Bar != BarChain || got.Verdict != "pass" || got.Code != "closed-chain" {
			t.Fatalf("chain with a new record path classified as %+v", got)
		}
	})
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: candidate, Chain: "impl-chain", RevertOf: strings.Repeat("0", 40),
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "conflicting-declarations" || got.Mode != "refuse" {
		t.Fatalf("chain with direct-fix parameter classified as %+v", got)
	}

	negativeShapes := []struct {
		name string
		code string
		edit func(map[string]any)
	}{
		{name: "open chain", code: "chain-open", edit: func(record map[string]any) { record["chainClosed"] = false }},
		{name: "non-implementer role", code: "chain-not-implementation", edit: func(record map[string]any) { record["role"] = "designer" }},
		{name: "non-design-bearing reach", code: "chain-not-design-bearing", edit: func(record map[string]any) { record["destructiveReach"] = "MECHANICAL" }},
		{name: "parented record", code: "chain-record-malformed", edit: func(record map[string]any) { record["parentJob"] = "parent-job" }},
	}
	for _, test := range negativeShapes {
		t.Run(test.name, func(t *testing.T) {
			record := map[string]any{}
			for key, value := range validRecord {
				record[key] = value
			}
			test.edit(record)
			f.writeChainRecord("impl-chain", record)
			got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "impl-chain"})
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != test.code {
				t.Fatalf("negative root shape classified as %+v", got)
			}
		})
	}
	t.Run("unreadable record", func(t *testing.T) {
		recordPath := filepath.Join(f.root, "artifacts", "agents", "jobs", "impl-chain.json")
		if err := os.Remove(recordPath); err != nil {
			t.Fatal(err)
		}
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "impl-chain"})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "chain-record-unreadable" {
			t.Fatalf("unreadable root record classified as %+v", got)
		}
	})

	t.Run("tampered certified path", func(t *testing.T) {
		f.writeChainRecord("impl-chain", validRecord)
		f.write("internal/x.go", "package internal // tampered\n")
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), Chain: "impl-chain"})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "chain-output-mismatch" {
			t.Fatalf("closed chain with a tampered certified path classified as %+v", got)
		}
	})
}

func TestObserveDeclaredDirectFixEvaluatesPerClassRule(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "after\n")
	f.git("add", "product.txt")
	f.git("commit", "-qm", "regression to revert")
	revertOf := f.git("rev-parse", "HEAD")
	f.write("product.txt", "before\n")

	got := Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(),
		DirectFix: "exact-revert", RevertOf: revertOf,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" {
		t.Fatalf("exact inverse classified as %+v", got)
	}

	f.write("extra.txt", "not part of the inverse\n")
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(),
		DirectFix: "exact-revert", RevertOf: revertOf,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" {
		t.Fatalf("expanded inverse classified as %+v", got)
	}
	if err := os.Remove(filepath.Join(f.root, "extra.txt")); err != nil {
		t.Fatal(err)
	}
	f.write("internal/extra.txt", "floor path outside the inverse\n")
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(),
		DirectFix: "exact-revert", RevertOf: revertOf,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
		t.Fatalf("expanded inverse with a floor path classified as %+v", got)
	}

	protected := newObserveFixture(t)
	protected.write("internal/authority.txt", "protected\n")
	protected.git("add", "internal/authority.txt")
	protected.git("commit", "-qm", "protected change")
	protectedCommit := protected.git("rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(protected.root, "internal", "authority.txt")); err != nil {
		t.Fatal(err)
	}
	got = Observe(ObserveParams{
		RepoRoot: protected.root, CandidateTree: protected.tree(),
		DirectFix: "exact-revert", RevertOf: protectedCommit,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
		t.Fatalf("floor-changing inverse classified as %+v", got)
	}

	for _, nestedPath := range []string{"product/AGENTS.md", "product/scripts/x"} {
		t.Run("unclassified nested path "+nestedPath, func(t *testing.T) {
			nested := newObserveFixture(t)
			nested.write(nestedPath, "nested instruction\n")
			nested.git("add", nestedPath)
			nested.git("commit", "-qm", "nested instruction change")
			nestedCommit := nested.git("rev-parse", "HEAD")
			if err := os.Remove(filepath.Join(nested.root, filepath.FromSlash(nestedPath))); err != nil {
				t.Fatal(err)
			}
			got := Observe(ObserveParams{
				RepoRoot: nested.root, CandidateTree: nested.tree(),
				DirectFix: "exact-revert", RevertOf: nestedCommit,
			})
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "path-unclassified" || got.Mode != "refuse" {
				t.Fatalf("nested instruction inverse classified as %+v", got)
			}
		})
	}

	t.Run("committed engine floor", func(t *testing.T) {
		engine := newObserveFixture(t)
		engine.write("bin/metasystem", "committed engine\n")
		engine.git("add", "bin/metasystem")
		engine.git("commit", "-qm", "committed engine change")
		engineCommit := engine.git("rev-parse", "HEAD")
		if err := os.Remove(filepath.Join(engine.root, "bin", "metasystem")); err != nil {
			t.Fatal(err)
		}
		got := Observe(ObserveParams{
			RepoRoot: engine.root, CandidateTree: engine.tree(),
			DirectFix: "exact-revert", RevertOf: engineCommit,
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "runtime-path-refused" || got.Mode != "refuse" {
			t.Fatalf("committed engine inverse classified as %+v", got)
		}
	})

	t.Run("record exact revert remains refused", func(t *testing.T) {
		record := newObserveFixture(t)
		record.write("memory/receipts.log", "receipt=existing\nreceipt=appended\n")
		record.git("add", "memory/receipts.log")
		record.git("commit", "-qm", "append receipt")
		recordCommit := record.git("rev-parse", "HEAD")
		record.write("memory/receipts.log", "receipt=existing\n")
		got := Observe(ObserveParams{
			RepoRoot: record.root, CandidateTree: record.tree(),
			DirectFix: "exact-revert", RevertOf: recordCommit,
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "exact-revert-record-refused" {
			t.Fatalf("record exact revert widened during path-class cutover: %+v", got)
		}
	})

	t.Run("register carriage append-only", func(t *testing.T) {
		for _, register := range []string{"memory/rulings.md", "memory/receipts.log", "records/narrator-digest.log"} {
			t.Run(register, func(t *testing.T) {
				carriage := newObserveFixture(t)
				original, err := os.ReadFile(filepath.Join(carriage.root, register))
				if err != nil {
					t.Fatal(err)
				}
				appended := "appended row\n"
				if register == "memory/rulings.md" {
					appended = "| R-36-m0 | appended ruling |\n"
				}
				carriage.write(register, string(original)+appended)
				got := Observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: carriage.tree(), DirectFix: "register-carriage",
				})
				if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
					t.Fatalf("append-only carriage classified as %+v", got)
				}

				carriage.write(register, "rewritten existing line\n")
				got = Observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: carriage.tree(), DirectFix: "register-carriage",
				})
				if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
					t.Fatalf("rewritten append-only register classified as %+v", got)
				}
			})
		}
	})

	t.Run("rulings carriage requires rows and preserves mode", func(t *testing.T) {
		malformed := newObserveFixture(t)
		original, err := os.ReadFile(filepath.Join(malformed.root, "memory", "rulings.md"))
		if err != nil {
			t.Fatal(err)
		}
		malformed.write("memory/rulings.md", string(original)+"free text\n")
		got := Observe(ObserveParams{
			RepoRoot: malformed.root, CandidateTree: malformed.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
			t.Fatalf("malformed ruling carriage classified as %+v", got)
		}

		modeChanged := newObserveFixture(t)
		modeOriginal, err := os.ReadFile(filepath.Join(modeChanged.root, "memory", "rulings.md"))
		if err != nil {
			t.Fatal(err)
		}
		modeChanged.write("memory/rulings.md", string(modeOriginal)+"| R-36-m0 | appended ruling |\n")
		if err := os.Chmod(filepath.Join(modeChanged.root, "memory", "rulings.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		got = Observe(ObserveParams{
			RepoRoot: modeChanged.root, CandidateTree: modeChanged.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
			t.Fatalf("mode-changing ruling carriage classified as %+v", got)
		}
	})

	t.Run("landing class authority must name an existing ruling row", func(t *testing.T) {
		missing := newObserveFixture(t)
		missing.write("memory/rulings.md", "| R-1 | unrelated ruling |\n")
		missing.git("add", "memory/rulings.md")
		missing.git("commit", "-qm", "remove landing class authority")
		missing.write("memory/receipts.log", "receipt=existing\nreceipt=carried\n")
		got := Observe(ObserveParams{
			RepoRoot: missing.root, CandidateTree: missing.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-policy-unreadable" {
			t.Fatalf("manifest with absent authority row classified as %+v", got)
		}
	})

	t.Run("register carriage exact and constrained glob entries", func(t *testing.T) {
		for _, changedPath := range []string{"records/narrator-digest.log", "plans/handoff-current.md"} {
			t.Run(changedPath, func(t *testing.T) {
				carriage := newObserveFixture(t)
				content := "carried register\n"
				if changedPath == "records/narrator-digest.log" {
					content = "digest=existing\ndigest=carried\n"
				}
				carriage.write(changedPath, content)
				got := Observe(ObserveParams{
					RepoRoot: carriage.root, CandidateTree: carriage.tree(), DirectFix: "register-carriage",
				})
				if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
					t.Fatalf("allowlisted register %s classified as %+v", changedPath, got)
				}
			})
		}
		carriage := newObserveFixture(t)
		carriage.write("plans/handoff-current.txt", "wrong basename shape\n")
		got := Observe(ObserveParams{
			RepoRoot: carriage.root, CandidateTree: carriage.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarDirectFix || got.Code != "register-carriage" {
			t.Fatalf("new plan record classified as %+v", got)
		}

		mixed := newObserveFixture(t)
		mixed.write("adopted.txt", "off-floor miss sorts first\n")
		mixed.write("internal/protected.txt", "floor miss sorts second\n")
		got = Observe(ObserveParams{
			RepoRoot: mixed.root, CandidateTree: mixed.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("mixed carriage misses did not give the floor precedence: %+v", got)
		}
	})

	t.Run("carriage policy files are on the floor", func(t *testing.T) {
		policy := newObserveFixture(t)
		policy.write("scripts/agents/path-classes.txt", "install:memory/ record\n")
		got := Observe(ObserveParams{
			RepoRoot: policy.root, CandidateTree: policy.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("allowlist self-change classified as %+v", got)
		}

		manifest := newObserveFixture(t)
		manifest.write("scripts/agents/landing-classes.json", "{}\n")
		got = Observe(ObserveParams{
			RepoRoot: manifest.root, CandidateTree: manifest.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("manifest self-change classified as %+v", got)
		}

		promotion := newObserveFixture(t)
		promotion.write("scripts/agents/landing-promotion.json", "{}\n")
		got = Observe(ObserveParams{
			RepoRoot: promotion.root, CandidateTree: promotion.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "direct-fix-floor-refused" || got.Mode != "refuse" {
			t.Fatalf("promotion record self-change classified as %+v", got)
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
				"goalId": "tier-one", "goalTier": 1, "gateWidth": "area",
			}
			mutate(record)
			f.writeChainRecord("tier-one-root", record)
			f.write("docs/constant.txt", "change\n")
			f.git("add", "docs/constant.txt")
			candidate, receipt := f.tierOneReceipt("true")
			got := Observe(tierOneParams(f, candidate, receipt))
			if got.Code != "tier1-root-refused" || got.Mode != "refuse" {
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

func TestTierOneClassCutoverAcceptsTheTwoClassLandingBase(t *testing.T) {
	f := newObserveFixture(t)
	f.write("scripts/agents/landing-classes.json", `{
  "schemaVersion": 1,
  "enginePolicyVersion": 1,
  "classes": [
    {"id":"register-carriage","pathRule":"path-class-record","requiredFields":[],"authorizedBy":"R-35-m0"},
    {"id":"exact-revert","pathRule":"tree-shaped-exact-inverse","requiredFields":["revert-of"],"authorizedBy":"R-35-m0"}
  ]
}
`)
	f.git("add", "scripts/agents/landing-classes.json")
	f.git("commit", "-qm", "legacy two-class landing base")
	f.write("memory/receipts.log", "receipt=existing\nreceipt=cutover\n")
	got := Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage",
	})
	if got.Bar != BarDirectFix || got.Code != "register-carriage" {
		t.Fatalf("new evaluator could not land from the previous two-class base: %+v", got)
	}
}

func TestSliceOneRetainsHandoffCarriage(t *testing.T) {
	fixture := newObserveFixture(t)
	fixture.write("plans/handoff-fixture-1.md", "# fixture handoff\n")
	got := Observe(ObserveParams{
		RepoRoot: fixture.root, CandidateTree: fixture.tree(), DirectFix: "register-carriage",
	})
	if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "register-carriage" {
		t.Fatalf("new handoff lost register carriage during the manifest transition: %+v", got)
	}
}

func TestObserveManifestIsBehavior(t *testing.T) {
	fixture := newObserveFixture(t)
	fixture.write("scripts/agents/path-classes.txt", "install:memory/ record\n")
	got := Observe(ObserveParams{
		RepoRoot: fixture.root, CandidateTree: fixture.tree(), DirectFix: "register-carriage",
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
			f := newObserveFixture(t)
			f.write(fixture.path, "changed\n")
			got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage"})
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
			f := newObserveFixture(t)
			f.write(leg.path, "changed\n")
			candidate := f.tree()
			f.writeChainRecord("class-chain", map[string]any{
				"jobId": "class-chain", "parentJob": nil, "role": "implementer", "round": 1,
				"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
			})
			f.writeChainReview("class-chain", 1, "class-chain", candidate)
			got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Chain: "class-chain"})
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
			f := newObserveFixture(t)
			f.write(leg.path, "after\n")
			f.git("add", leg.path)
			f.git("commit", "-qm", "change to revert")
			revertOf := f.git("rev-parse", "HEAD")
			if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(leg.path))); err != nil {
				t.Fatal(err)
			}
			got := Observe(ObserveParams{
				RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "exact-revert", RevertOf: revertOf,
			})
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

func TestObserveRecordSemantics(t *testing.T) {
	t.Run("existing record appends only under a held goal", func(t *testing.T) {
		f := newObserveFixture(t)
		f.writeHeldGoal("fx", "m9", "L1")
		f.write("records/misc/fx-analysis.md", "base\n")
		f.git("add", "plans/goals/fx.md", "records/misc/fx-analysis.md")
		f.git("commit", "-qm", "owned record base")

		f.write("records/misc/fx-analysis.md", "base\nappend\n")
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" || got.Verdict != "pass" {
			t.Fatalf("owned existing record append classified as %+v", got)
		}

		f.write("records/misc/fx-analysis.md", "replacement\n")
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage-not-append-only" {
			t.Fatalf("existing record replacement classified as %+v", got)
		}

		if err := os.Remove(filepath.Join(f.root, "records/misc/fx-analysis.md")); err != nil {
			t.Fatal(err)
		}
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage-not-append-only" {
			t.Fatalf("existing record deletion classified as %+v", got)
		}

		f.write("records/misc/fx-analysis.md", "base\nappend\n")
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage"})
		if got.Code != "record-not-owned" {
			t.Fatalf("ownerless existing record append classified as %+v", got)
		}
	})

	t.Run("goal-bound plans use base claims and longest identifiers", func(t *testing.T) {
		f := newObserveFixture(t)
		f.writeHeldGoal("fx", "m9", "L1")
		f.writeHeldGoal("fx-load", "m9", "L1")
		f.write("plans/fx-design.md", "base\n")
		f.write("plans/fx-load-x.md", "base\n")
		f.write("plans/legacy.md", "legacy\n")
		f.git("add", "plans")
		f.git("commit", "-qm", "plan ownership base")

		f.write("plans/fx-design.md", "modified\n")
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("owned goal plan classified as %+v", got)
		}
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
		if got.Code != "goal-item-not-held" {
			t.Fatalf("foreign actor classified as %+v", got)
		}
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx-load", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("wrong held goal classified as %+v", got)
		}

		f.write("plans/fx-design.md", "base\n")
		f.write("plans/fx-load-x.md", "modified\n")
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("shorter goal identifier won ownership: %+v", got)
		}
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx-load", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("longest goal identifier lost ownership: %+v", got)
		}

		f.write("plans/fx-load-x.md", "base\n")
		f.write("plans/legacy.md", "modified\n")
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("frozen legacy plan classified as %+v", got)
		}
		manifest, err := os.ReadFile(filepath.Join(f.root, "scripts/agents/path-classes.txt"))
		if err != nil {
			t.Fatal(err)
		}
		f.write("scripts/agents/path-classes.txt", string(manifest)+"own:plans/legacy.md fx\n")
		f.git("add", "scripts/agents/path-classes.txt")
		f.git("commit", "-qm", "own legacy plan")
		f.write("plans/legacy.md", "owned modification\n")
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("explicitly owned legacy plan classified as %+v", got)
		}
	})

	t.Run("handoffs belong to their seat after creation", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("plans/handoff-m9-x.md", "base\n")
		f.git("add", "plans/handoff-m9-x.md")
		f.git("commit", "-qm", "handoff base")
		f.write("plans/handoff-m9-x.md", "modified\n")
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Actor: "m9+L1"})
		if got.Code != "register-carriage" {
			t.Fatalf("own handoff classified as %+v", got)
		}
		got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Actor: "m1+L1"})
		if got.Code != "record-not-owned" {
			t.Fatalf("foreign handoff classified as %+v", got)
		}
	})
}

func TestObserveUnclassifiedDetailFromBase(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "changed\n")
	candidate := f.tree()
	manifest, err := os.ReadFile(filepath.Join(f.root, "scripts/agents/path-classes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f.write("scripts/agents/path-classes.txt", string(manifest)+"install:product.txt record\n")
	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: candidate, DirectFix: "register-carriage"})
	if got.Code != "path-unclassified" || len(got.Unclassified) != 1 || got.Unclassified[0] != "product.txt" || !strings.Contains(got.Refusal, "path product.txt has no class") {
		t.Fatalf("candidate manifest reclassified its own landing: %+v", got)
	}
}

func TestObserveFloorPrecedesGoalOwnershipValidation(t *testing.T) {
	f := newObserveFixture(t)
	f.writeHeldGoal("fx", "m9", "L1")
	f.write("records/misc/base.md", "base\n")
	f.git("add", "plans/goals/fx.md", "records/misc/base.md")
	f.git("commit", "-qm", "foreign-goal base")

	f.write("internal/x.go", "package internal\n")
	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
	if got.Code != "direct-fix-floor-refused" {
		t.Fatalf("goal ownership ran before the behavior floor: %+v", got)
	}
	if err := os.Remove(filepath.Join(f.root, "internal/x.go")); err != nil {
		t.Fatal(err)
	}
	f.write("records/misc/base.md", "base\nappend\n")
	got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", Goal: "fx", Actor: "m1+L2"})
	if got.Code != "goal-item-not-held" {
		t.Fatalf("record-only foreign goal classified as %+v", got)
	}
}

func TestObserveClassPrecedenceIsSetWide(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "unclassified\n")
	f.write("bin/metasystem", "runtime\n")
	f.write("plans/goals/new.md", "ledger\n")
	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage"})
	if got.Code != "ledger-path-not-goal-verb" {
		t.Fatalf("ledger did not precede runtime and unclassified paths: %+v", got)
	}

	f.write("internal/x.go", "package internal\n")
	got = Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage"})
	if got.Code != "direct-fix-floor-refused" {
		t.Fatalf("behavior did not precede every other class: %+v", got)
	}
}

func TestObserveRegisterCarriageRefusesStagedNarratorDigestRewrite(t *testing.T) {
	f := newObserveFixture(t)
	f.write("records/narrator-digest.log", "digest=rewritten\n")

	got := Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage",
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-not-append-only" {
		t.Fatalf("staged narrator digest rewrite classified as %+v", got)
	}
}

func TestObserveUndeclaredLandingRecordsWouldRefuse(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "undeclared change\n")
	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree()})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "missing-declaration" || got.Mode != "refuse" {
		t.Fatalf("undeclared landing classified as %+v", got)
	}
	if got.VerdictTrailer != "would-refuse code=missing-declaration" {
		t.Fatalf("undeclared landing has non-durable verdict value %q", got.VerdictTrailer)
	}
	for name, params := range map[string]ObserveParams{
		"orphaned revert parameter": {
			RepoRoot: f.root, CandidateTree: f.tree(), RevertOf: strings.Repeat("0", 40),
		},
		"revert parameter on register carriage": {
			RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage", RevertOf: strings.Repeat("0", 40),
		},
		"exact revert missing its commit": {
			RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "exact-revert",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := Observe(params)
			if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "conflicting-declarations" || got.Mode != "refuse" {
				t.Fatalf("partial classification parameter classified as %+v", got)
			}
		})
	}
}

func TestObservePromotionRecordIsStrictAndAbsentMeansObserve(t *testing.T) {
	t.Run("promoted conflicting declarations refuse", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("product.txt", "conflicting declaration\n")
		got := Observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: f.tree(), Chain: "implementation-chain",
			DirectFix: "exact-revert",
		})
		if got.Mode != "refuse" || got.Code != "conflicting-declarations" || got.VerdictTrailer != "would-refuse code=conflicting-declarations" {
			t.Fatalf("promoted conflict classified as %+v", got)
		}
	})

	t.Run("absent record observes everything", func(t *testing.T) {
		f := newObserveFixture(t)
		if err := os.Remove(filepath.Join(f.root, promotionRecordPath)); err != nil {
			t.Fatal(err)
		}
		f.git("add", "-u", promotionRecordPath)
		f.git("commit", "-qm", "remove promotion policy")
		f.write("product.txt", "undeclared without promotion\n")
		got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree()})
		if got.Mode != "observe" || got.Code != "missing-declaration" {
			t.Fatalf("absent promotion record classified as %+v", got)
		}
		got = Observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: f.tree(), Chain: "implementation-chain",
			DirectFix: "exact-revert",
		})
		if got.Mode != "observe" || got.Code != "conflicting-declarations" {
			t.Fatalf("absent promotion record promoted a conflict: %+v", got)
		}
	})

	malformed := map[string]string{
		"unparseable":    "{\n",
		"unknown field":  `{"schemaVersion":1,"refuseCodes":[],"extra":true}` + "\n",
		"unknown code":   `{"schemaVersion":1,"refuseCodes":["not-a-verdict"]}` + "\n",
		"duplicate code": `{"schemaVersion":1,"refuseCodes":["missing-declaration","missing-declaration"]}` + "\n",
		"missing codes":  `{"schemaVersion":1}` + "\n",
		"wrong schema":   `{"schemaVersion":2,"refuseCodes":[]}` + "\n",
		"trailing JSON":  `{"schemaVersion":1,"refuseCodes":[]} {}` + "\n",
	}
	for name, content := range malformed {
		t.Run(name, func(t *testing.T) {
			f := newObserveFixture(t)
			f.write(promotionRecordPath, content)
			f.git("add", promotionRecordPath)
			f.git("commit", "-qm", "install malformed promotion policy")
			f.write("product.txt", "otherwise lawful classification\n")
			got := Observe(ObserveParams{
				RepoRoot: f.root, CandidateTree: f.tree(), DirectFix: "register-carriage",
			})
			if got.Mode != "refuse" || got.Code != "promotion-record-malformed" || got.VerdictTrailer != "would-refuse code=promotion-record-malformed" {
				t.Fatalf("malformed promotion record classified as %+v", got)
			}
		})
	}

	t.Run("unreadable landing base reports its own cause", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("product.txt", "candidate before base loss\n")
		candidate := f.tree()
		f.git("update-ref", "-d", "refs/heads/main")
		got := Observe(ObserveParams{
			RepoRoot: f.root, CandidateTree: candidate, DirectFix: "register-carriage",
		})
		if got.Mode != "refuse" || got.Code != "promotion-base-unreadable" || got.VerdictTrailer != "would-refuse code=promotion-base-unreadable" {
			t.Fatalf("unreadable landing base classified as %+v", got)
		}
	})
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
