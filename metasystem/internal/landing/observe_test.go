package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type observeFixture struct {
	t    *testing.T
	root string
}

func newObserveFixture(t *testing.T) *observeFixture {
	t.Helper()
	root := t.TempDir()
	f := &observeFixture{t: t, root: root}
	f.git("init", "-q", "-b", "main")
	f.git("config", "user.name", "landing fixture")
	f.git("config", "user.email", "landing@example.invalid")
	f.write(".gitignore", "artifacts/\n")
	f.write("product.txt", "before\n")
	for _, policyFile := range []string{"register-carriage-paths.txt", "landing-classes.json"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", policyFile))
		if err != nil {
			t.Fatalf("read repository policy %s: %v", policyFile, err)
		}
		f.writeBytes(filepath.Join("scripts", "agents", policyFile), content)
	}
	f.write("memory/rulings.md", "R-1 | existing ruling\n")
	f.write("memory/receipts.log", "receipt=existing\n")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
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

func TestObserveChainBoundLandingEvaluatesBarA(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "chain change\n")
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
		follow.write("product.txt", "chain change\n")
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
		selected.write("product.txt", "selected chain change\n")
		selectedTree := selected.tree()
		selected.write("product.txt", "stale different change\n")
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
		bundled.write("product.txt", "certified change\n")
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
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-path-refused" {
			t.Fatalf("chain with a non-allowlisted protected path classified as %+v", got)
		}
	})
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: candidate, Chain: "impl-chain", RevertOf: strings.Repeat("0", 40),
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "unexpected-revert-commit" {
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
		f.write("product.txt", "tampered after review\n")
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
	if got.Bar != BarDirectFix || got.Verdict != "pass" || got.Code != "exact-revert" {
		t.Fatalf("exact inverse classified as %+v", got)
	}

	f.write("extra.txt", "not part of the inverse\n")
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(),
		DirectFix: "exact-revert", RevertOf: revertOf,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "not-exact-revert" {
		t.Fatalf("expanded inverse classified as %+v", got)
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
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "not-exact-revert" {
		t.Fatalf("floor-changing inverse classified as %+v", got)
	}

	nested := newObserveFixture(t)
	nested.write("product/AGENTS.md", "nested instruction\n")
	nested.git("add", "product/AGENTS.md")
	nested.git("commit", "-qm", "nested instruction change")
	nestedCommit := nested.git("rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(nested.root, "product", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	got = Observe(ObserveParams{
		RepoRoot: nested.root, CandidateTree: nested.tree(),
		DirectFix: "exact-revert", RevertOf: nestedCommit,
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "not-exact-revert" {
		t.Fatalf("nested instruction inverse classified as %+v", got)
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
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "not-exact-revert" {
			t.Fatalf("committed engine inverse classified as %+v", got)
		}
	})

	t.Run("register carriage append-only", func(t *testing.T) {
		for _, register := range []string{"memory/rulings.md", "memory/receipts.log"} {
			t.Run(register, func(t *testing.T) {
				carriage := newObserveFixture(t)
				original, err := os.ReadFile(filepath.Join(carriage.root, register))
				if err != nil {
					t.Fatal(err)
				}
				carriage.write(register, string(original)+"appended row\n")
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

	t.Run("register carriage exact and constrained glob entries", func(t *testing.T) {
		for _, changedPath := range []string{"records/narrator-digest.log", "plans/handoff-current.md"} {
			t.Run(changedPath, func(t *testing.T) {
				carriage := newObserveFixture(t)
				carriage.write(changedPath, "carried register\n")
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
		if got.Bar != BarRefusal || got.Code != "register-carriage-path-refused" {
			t.Fatalf("non-matching handoff path classified as %+v", got)
		}
	})

	t.Run("carriage policy files are on the floor", func(t *testing.T) {
		policy := newObserveFixture(t)
		policy.write("scripts/agents/register-carriage-paths.txt", "memory/rulings.md\n")
		got := Observe(ObserveParams{
			RepoRoot: policy.root, CandidateTree: policy.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-path-refused" {
			t.Fatalf("allowlist self-change classified as %+v", got)
		}

		manifest := newObserveFixture(t)
		manifest.write("scripts/agents/landing-classes.json", "{}\n")
		got = Observe(ObserveParams{
			RepoRoot: manifest.root, CandidateTree: manifest.tree(), DirectFix: "register-carriage",
		})
		if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "register-carriage-path-refused" {
			t.Fatalf("manifest self-change classified as %+v", got)
		}
	})
}

func TestObserveUndeclaredLandingRecordsWouldRefuse(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "undeclared change\n")
	got := Observe(ObserveParams{RepoRoot: f.root, CandidateTree: f.tree()})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "missing-declaration" {
		t.Fatalf("undeclared landing classified as %+v", got)
	}
	if got.VerdictTrailer != "would-refuse code=missing-declaration" {
		t.Fatalf("undeclared landing has non-durable verdict value %q", got.VerdictTrailer)
	}
	got = Observe(ObserveParams{
		RepoRoot: f.root, CandidateTree: f.tree(), RevertOf: strings.Repeat("0", 40),
	})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" || got.Code != "revert-without-direct-fix" {
		t.Fatalf("orphaned revert parameter classified as %+v", got)
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
		`--trailer "Landing-Provenance: $landing_provenance"`,
		`--trailer "Landing-Provenance-Verdict: $landing_verdict"`,
	} {
		if !strings.Contains(string(commitWrapper), required) {
			t.Fatalf("commit chokepoint lost %q", required)
		}
	}
	for _, required := range []string{"--chain", "--direct-fix", "--revert-of"} {
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
