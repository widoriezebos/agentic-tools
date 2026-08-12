package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// conformanceFixture builds the controller checkout and implementer
// worktree the shell fixtures used: a controller repository with the
// instruction-bearing path list, a conf, and an impl job record whose
// workspace is a branch worktree with one declared change.
type conformanceFixture struct {
	t          *testing.T
	controller string
	worktree   string
	baseSha    string
}

func (f *conformanceFixture) git(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *conformanceFixture) writeJSON(relative string, value any) {
	f.t.Helper()
	path := filepath.Join(f.controller, relative)
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(value, "", "  ")
	os.WriteFile(path, append(data, '\n'), 0o644)
}

func newConformanceFixture(t *testing.T) *conformanceFixture {
	t.Helper()
	root := t.TempDir()
	f := &conformanceFixture{t: t,
		controller: filepath.Join(root, "controller"),
		worktree:   filepath.Join(root, "worktree")}
	os.MkdirAll(filepath.Join(f.controller, "scripts", "agents"), 0o755)
	os.MkdirAll(filepath.Join(f.controller, "docs"), 0o755)
	source, _ := os.ReadFile("../../scripts/agents/instruction-bearing-paths.txt")
	os.WriteFile(filepath.Join(f.controller, "scripts", "agents", "instruction-bearing-paths.txt"), source, 0o644)
	os.WriteFile(filepath.Join(f.controller, ".gitignore"), []byte("artifacts/\nlocal.conf\n"), 0o644)
	os.WriteFile(filepath.Join(f.controller, "source.txt"), []byte("base\n"), 0o644)
	os.WriteFile(filepath.Join(f.controller, "docs", "note.md"), []byte("base\n"), 0o644)
	os.WriteFile(filepath.Join(f.controller, "metasystem.conf"),
		[]byte("metasystem.version=1\nrole.code-critic.runtime=fake\n"), 0o644)
	f.git(f.controller, "init", "-q")
	f.git(f.controller, "add", ".")
	f.git(f.controller, "-c", "user.name=m", "-c", "user.email=m@x", "commit", "-qm", "base")
	f.git(f.controller, "worktree", "add", "-q", "-b", "fixture", f.worktree, "HEAD")
	f.baseSha = f.git(f.worktree, "rev-parse", "HEAD")
	return f
}

func (f *conformanceFixture) writeImplementer(waiverClass string, boundary ...string) {
	record := map[string]any{
		"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil,
		"workspaceRoot": f.worktree, "baseSha": f.baseSha,
		"status": "completed", "effectiveModel": "shared-model",
	}
	if waiverClass != "" {
		record["critiqueWaived"] = map[string]any{"class": waiverClass}
	}
	f.writeJSON("artifacts/agents/jobs/impl.json", record)
	f.writeJSON("artifacts/agents/impl/rounds/1/return.json", map[string]any{
		"jobId": "impl", "round": 1, "diffBoundary": boundary,
	})
	brief := filepath.Join(f.controller, "artifacts", "agents", "impl", "brief.md")
	os.WriteFile(brief, []byte("Working Mode: implement\nMission Stream: fixture-stream\n"), 0o644)
}

func (f *conformanceFixture) writeFollowUp() {
	f.writeJSON("artifacts/agents/jobs/impl-r2.json", map[string]any{
		"jobId": "impl-r2", "role": "implementer", "round": 2, "parentJob": "impl",
		"status": "completed", "effectiveModel": "shared-model",
	})
	prompt := filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "2")
	os.MkdirAll(prompt, 0o755)
	os.WriteFile(filepath.Join(prompt, "prompt.md"), []byte("Implementer follow-up enumerates F-9.\n"), 0o644)
}

func (f *conformanceFixture) writeCritic(tree, materialID, exhaustion, model string) {
	var items []any
	switch exhaustion {
	case "one":
		items = []any{map[string]any{"round": 1, "openFindingIds": []any{"F-9"}, "successorJobId": "impl-r2"}}
	case "two":
		items = []any{
			map[string]any{"round": 1, "openFindingIds": []any{"F-9"}, "successorJobId": "impl-r2"},
			map[string]any{"round": 4, "openFindingIds": []any{"F-11"}, "successorJobId": "impl-r2"},
		}
	case "wrong-party":
		items = []any{map[string]any{"round": 1, "openFindingIds": []any{"F-9"}, "successorJobId": "critic"}}
	}
	f.writeJSON("artifacts/agents/jobs/critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "impl", "status": "completed", "effectiveModel": model,
		"chainClosed": true, "critiqueExhaustions": items,
	})
	findings := []any{}
	if materialID != "" {
		findings = []any{map[string]any{"id": materialID, "material": true}}
	}
	f.writeJSON("artifacts/agents/critic/rounds/1/return.json", map[string]any{
		"jobId": "critic", "round": 1, "reviewedTree": tree,
		"findings": findings, "verdictMaterialCount": len(findings),
	})
}

func (f *conformanceFixture) commitWorktree() {
	f.git(f.worktree, "add", ".")
	f.git(f.worktree, "-c", "user.name=m", "-c", "user.email=m@x", "commit", "-qm", "change")
}

func (f *conformanceFixture) reviewedTree() string {
	data, err := os.ReadFile(filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "review.json"))
	if err != nil {
		f.t.Fatalf("review.json missing: %v", err)
	}
	var review map[string]string
	json.Unmarshal(data, &review)
	return review["reviewedTree"]
}

func appendFile(t *testing.T, path, text string) {
	t.Helper()
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	handle.WriteString(text)
	handle.Close()
}

func expectConformance(t *testing.T, f *conformanceFixture, stage string, wantCode int, wantText string) (out, errs []string) {
	t.Helper()
	out, errs, code := Conformance(f.controller, stage, "impl")
	if code != wantCode {
		t.Fatalf("%s stage exit %d, want %d\nout: %v\nerr: %v", stage, code, wantCode, out, errs)
	}
	joined := strings.Join(append(append([]string{}, out...), errs...), "\n")
	if wantText != "" && !strings.Contains(joined, wantText) {
		t.Fatalf("%s stage output lacks %q:\n%s", stage, wantText, joined)
	}
	return out, errs
}

func TestConformanceReviewAndCritiqueMerge(t *testing.T) {
	f := newConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "changed\n")
	f.writeImplementer("", "source.txt")

	out, _ := expectConformance(t, f, "review", 0, "reviewedTree=")
	if !strings.HasPrefix(out[1], "diffArtifact=") {
		t.Fatalf("review did not report the diff artifact: %v", out)
	}
	diffPath := filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "diff.patch")
	if data, err := os.ReadFile(diffPath); err != nil || !strings.Contains(string(data), "source.txt") {
		t.Fatalf("diff.patch wrong: %v %s", err, data)
	}

	// An undeclared change refuses and names the path.
	appendFile(t, filepath.Join(f.worktree, "extra.txt"), "undeclared\n")
	_, errs := expectConformance(t, f, "review", 1, "some implementation round must declare every changed path")
	if !strings.Contains(strings.Join(errs, "\n"), "'extra.txt'") {
		t.Fatalf("undeclared path not named: %v", errs)
	}
	os.Remove(filepath.Join(f.worktree, "extra.txt"))

	// Trusted plans/ state and the control plane refuse in review.
	os.MkdirAll(filepath.Join(f.worktree, "plans"), 0o755)
	os.WriteFile(filepath.Join(f.worktree, "plans", "note.md"), []byte("delegate plan\n"), 0o644)
	expectConformance(t, f, "review", 1, "trusted plans/ state changed: plans/note.md")
	os.RemoveAll(filepath.Join(f.worktree, "plans"))
	os.MkdirAll(filepath.Join(f.worktree, "artifacts", "agents"), 0o755)
	os.WriteFile(filepath.Join(f.worktree, "artifacts", "agents", "tamper"), []byte("tamper\n"), 0o644)
	expectConformance(t, f, "review", 1, "agent control plane contains delegate-created files")
	os.RemoveAll(filepath.Join(f.worktree, "artifacts"))

	expectConformance(t, f, "review", 0, "")
	reviewedTree := f.reviewedTree()
	f.commitWorktree()

	// Merge without any critic chain names the dispatch remedy.
	_, errs = expectConformance(t, f, "merge", 1, "reviews field names implementer job 'impl'; dispatch that role with --reviews impl")
	if strings.Contains(strings.Join(errs, "\n"), "role.code-critic.runtime") {
		t.Fatalf("configured critic role reported as unconfigured: %v", errs)
	}

	f.writeFollowUp()
	f.writeCritic(strings.Repeat("0", 40), "", "", "critic-model")
	expectConformance(t, f, "merge", 1, "is stale; the implementer branch final committed tree is")

	f.writeCritic(reviewedTree, "F-7", "", "critic-model")
	expectConformance(t, f, "merge", 1, "still has material findings despite any dispositions: F-7")

	f.writeCritic(reviewedTree, "F-9", "one", "critic-model")
	expectConformance(t, f, "merge", 1, "critique exhausted at round 1 with open material findings: F-9")

	f.writeCritic(reviewedTree, "F-9", "wrong-party", "critic-model")
	expectConformance(t, f, "merge", 1, "is not an implementer follow-up in the reviewed implementation chain")

	f.writeCritic(reviewedTree, "F-9", "two", "critic-model")
	expectConformance(t, f, "merge", 1, "waiting on the human is the only remedy")

	f.writeCritic(reviewedTree, "", "", "critic-model")
	expectConformance(t, f, "merge", 0, "merge critique accepted with model independence")

	f.writeCritic(reviewedTree, "", "", "shared-model")
	_, errs = expectConformance(t, f, "merge", 1, "independence refused")
	joined := strings.Join(errs, "\n")
	for _, remedy := range []string{"dispatch a critic on a different model", "declare independence=session-only"} {
		if !strings.Contains(joined, remedy) {
			t.Fatalf("same-model refusal lacks remedy %q: %s", remedy, joined)
		}
	}
	appendFile(t, filepath.Join(f.controller, "metasystem.conf"), "independence=session-only\n")
	expectConformance(t, f, "merge", 0, "independence=session-only recorded in gate evidence")
}

func TestConformanceMissingCriticConfiguration(t *testing.T) {
	f := newConformanceFixture(t)
	os.WriteFile(filepath.Join(f.controller, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "changed\n")
	f.writeImplementer("", "source.txt")
	f.commitWorktree()
	expectConformance(t, f, "merge", 1,
		"the code-critic role is unconfigured; set the exact key role.code-critic.runtime")
}

func TestConformanceWaivers(t *testing.T) {
	cases := []struct {
		name     string
		change   func(f *conformanceFixture)
		boundary string
		code     int
		want     string
	}{
		{"script path", func(f *conformanceFixture) {
			os.MkdirAll(filepath.Join(f.worktree, "scripts"), 0o755)
			os.WriteFile(filepath.Join(f.worktree, "scripts", "tool.sh"), []byte("changed\n"), 0o644)
		}, "scripts/tool.sh", 1, "prose-under-30 includes non-Markdown paths"},
		{"small prose", func(f *conformanceFixture) {
			for i := 0; i < 10; i++ {
				appendFile(f.t, filepath.Join(f.worktree, "docs", "note.md"), fmt.Sprintf("line %d\n", i))
			}
		}, "docs/note.md", 0, "critique waiver accepted and counted: class=prose-under-30 stream='fixture-stream' count=1 changedLines=10"},
		{"large prose", func(f *conformanceFixture) {
			for i := 0; i < 40; i++ {
				appendFile(f.t, filepath.Join(f.worktree, "docs", "note.md"), fmt.Sprintf("line %d\n", i))
			}
		}, "docs/note.md", 1, "the maximum is 30 additions plus deletions"},
		{"plan path", func(f *conformanceFixture) {
			os.MkdirAll(filepath.Join(f.worktree, "plans"), 0o755)
			os.WriteFile(filepath.Join(f.worktree, "plans", "note.md"), []byte("delegate plan\n"), 0o644)
		}, "plans/note.md", 1, "trusted plans/ state changed: plans/note.md"},
		{"instruction path", func(f *conformanceFixture) {
			os.WriteFile(filepath.Join(f.worktree, "AGENTS.md"), []byte("changed\n"), 0o644)
		}, "AGENTS.md", 1, "instruction-bearing paths that are never waivable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newConformanceFixture(t)
			tc.change(f)
			f.writeImplementer("prose-under-30", tc.boundary)
			f.commitWorktree()
			expectConformance(t, f, "merge", tc.code, tc.want)
		})
	}
	t.Run("wrong class", func(t *testing.T) {
		f := newConformanceFixture(t)
		appendFile(t, filepath.Join(f.worktree, "docs", "note.md"), "small\n")
		f.writeImplementer("prose-under-100", "docs/note.md")
		f.commitWorktree()
		expectConformance(t, f, "merge", 1,
			"unsupported critique waiver class 'prose-under-100'; the only class is prose-under-30")
	})
}

func TestConformanceFactRefusals(t *testing.T) {
	f := newConformanceFixture(t)
	if _, errs, code := Conformance(f.controller, "review", "absent"); code != 1 ||
		errs[0] != "conformance failure: unknown job: absent" {
		t.Fatalf("unknown job wrong: %v %d", errs, code)
	}
	f.writeImplementer("", "source.txt")
	// A non-implementer record refuses with the fact-resolution wrapper.
	f.writeJSON("artifacts/agents/jobs/impl.json", map[string]any{
		"jobId": "impl", "role": "design-critic",
	})
	_, errs, code := Conformance(f.controller, "review", "impl")
	if code != 1 || errs[0] != "conformance review is only defined for implementer records" ||
		errs[1] != "conformance failure: could not resolve implementer job facts" {
		t.Fatalf("non-implementer wrong: %v", errs)
	}
	// A round directory without a return refuses.
	f.writeImplementer("", "source.txt")
	os.Remove(filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "return.json"))
	if _, errs, code := Conformance(f.controller, "review", "impl"); code != 1 ||
		errs[0] != "conformance failure: implementer round return is missing" {
		t.Fatalf("missing return wrong: %v %d", errs, code)
	}
}
