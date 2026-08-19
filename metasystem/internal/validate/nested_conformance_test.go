package validate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func nestedGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func nestedWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The REAL review stage speaks the mission project's path space in a
// nested checkout: the reviewed tree is the project subtree, the diff
// and changed paths are project-relative, and a boundary declared in
// project-relative terms admits them. A review stage deriving toplevel
// trees would report the change as metasystem/-prefixed, fall outside
// the declared boundary, and fail here.
func TestNestedReviewStageSpeaksProjectSpace(t *testing.T) {
	top := t.TempDir()
	nestedGit(t, top, "init", "-q", "-b", "main")
	nestedWrite(t, top, "outside.txt", "outer\n")
	nestedWrite(t, top, "metasystem/truth/a.txt", "a\n")
	nestedGit(t, top, "add", ".")
	nestedGit(t, top, "commit", "-qm", "first")
	headSHA := nestedGit(t, top, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "delegate")
	nestedGit(t, top, "worktree", "add", "--detach", worktree, "HEAD")
	defer nestedGit(t, top, "worktree", "remove", "--force", worktree)
	nestedWrite(t, worktree, "metasystem/truth/nested-change.txt", "delegate work\n")

	missionRoot := filepath.Join(top, "metasystem")
	nestedWrite(t, missionRoot, "artifacts/agents/j1/rounds/1/return.json",
		`{"diffBoundary": ["metasystem/truth/nested-change.txt"]}`)

	derived, err := projectInstallPrefix(missionRoot)
	if err != nil || derived != "metasystem" {
		t.Fatalf("production prefix derivation must name the project: %q %v", derived, err)
	}
	r := &conformanceRun{
		root: missionRoot, job: "j1", rootJob: "j1", record: map[string]any{},
		workspace: worktree, roundText: "1", installPrefix: derived,
		boundaryBase: headSHA, targetSha: headSHA,
	}
	artifactDir := t.TempDir()
	diffFile := filepath.Join(artifactDir, "diff.patch")
	reviewFile := filepath.Join(artifactDir, "review.json")
	out, errs, code := r.reviewStage(diffFile, reviewFile)
	if code != 0 {
		t.Fatalf("the nested review stage must pass: code=%d out=%v errs=%v", code, out, errs)
	}
	data, err := os.ReadFile(reviewFile)
	if err != nil {
		t.Fatal(err)
	}
	var review map[string]string
	if err := json.Unmarshal(data, &review); err != nil {
		t.Fatal(err)
	}
	project := gittree.Workspace{Dir: filepath.Join(worktree, "metasystem")}
	want, err := project.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if review["reviewedTree"] != want {
		t.Fatalf("the reviewed tree must be the project subtree: got %s want %s", review["reviewedTree"], want)
	}
	patch, err := os.ReadFile(diffFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patch), "truth/nested-change.txt") ||
		strings.Contains(string(patch), "metasystem/truth/") {
		t.Fatalf("the diff must speak project-relative paths:\n%s", patch)
	}
}

// An authorization base that is no named expected-tree point keeps ONE
// generic refusal whatever the cause: a baseline differing from
// committed HEAD does not prove a sealed-dirty admission (a lawful
// mid-turn delegate merge produces the same state), so no sharper
// diagnosis is sound without authenticated admission provenance.
func TestUnnamedBaseKeepsTheGenericRefusal(t *testing.T) {
	root := t.TempDir()
	nestedGit(t, root, "init", "-q", "-b", "main")
	nestedWrite(t, root, "truth/a.txt", "a\n")
	nestedGit(t, root, "add", ".")
	nestedGit(t, root, "commit", "-qm", "first")
	head, err := (gittree.Workspace{Dir: root}).HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"initialBaseline": strings.Repeat("d", 40),
		"turnLog":         []any{},
		"workspaceTaint":  map[string]any{"next": 1, "segment": 0, "entries": []any{}},
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	nestedWrite(t, root, "artifacts/agents/missions/m1/state.json", string(encoded))

	for _, base := range []string{head, strings.Repeat("e", 40)} {
		if _, _, err := missionBaseSequencePoint(root, "m1", base); err == nil ||
			!strings.Contains(err.Error(), "not a named expected-tree sequence point") {
			t.Fatalf("an unnamed base (%s) keeps the generic refusal: %v", base, err)
		}
	}
}

// One commit cannot carry reviewed project changes and unreviewed
// sibling changes: the delegate's worktree is writable everywhere, so
// the review stage fences the WHOLE repository — any change outside the
// mission project refuses by name before boundaries are even consulted.
func TestReviewRefusesSiblingChanges(t *testing.T) {
	top := t.TempDir()
	nestedGit(t, top, "init", "-q", "-b", "main")
	nestedWrite(t, top, "outside.txt", "outer\n")
	nestedWrite(t, top, "metasystem/truth/a.txt", "a\n")
	nestedGit(t, top, "add", ".")
	nestedGit(t, top, "commit", "-qm", "first")
	headSHA := nestedGit(t, top, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "delegate")
	nestedGit(t, top, "worktree", "add", "--detach", worktree, "HEAD")
	defer nestedGit(t, top, "worktree", "remove", "--force", worktree)
	nestedWrite(t, worktree, "metasystem/truth/nested-change.txt", "reviewed work\n")
	nestedWrite(t, worktree, "sibling-note.txt", "smuggled beside the project\n")

	missionRoot := filepath.Join(top, "metasystem")
	nestedWrite(t, missionRoot, "artifacts/agents/j1/rounds/1/return.json",
		`{"diffBoundary": ["metasystem/truth/nested-change.txt"]}`)
	derived, err := projectInstallPrefix(missionRoot)
	if err != nil {
		t.Fatal(err)
	}
	r := &conformanceRun{
		root: missionRoot, job: "j1", rootJob: "j1", record: map[string]any{},
		workspace: worktree, roundText: "1", installPrefix: derived,
		boundaryBase: headSHA, targetSha: headSHA,
	}
	artifactDir := t.TempDir()
	_, errs, code := r.reviewStage(
		filepath.Join(artifactDir, "diff.patch"), filepath.Join(artifactDir, "review.json"))
	if code == 0 {
		t.Fatal("a sibling change must refuse the review stage")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "outside the mission project") ||
		!strings.Contains(joined, "sibling-note.txt") {
		t.Fatalf("the refusal must name the fence and the path: %s", joined)
	}
}

// The sibling fence guards the MERGE stage too: review may have passed
// on a clean worktree, but the branch's final commit is what integrates,
// and a sibling change committed after review would otherwise ride it.
func TestMergeRefusesCommittedSiblingChanges(t *testing.T) {
	top := t.TempDir()
	nestedGit(t, top, "init", "-q", "-b", "main")
	nestedWrite(t, top, "outside.txt", "outer\n")
	nestedWrite(t, top, "metasystem/truth/a.txt", "a\n")
	nestedGit(t, top, "add", ".")
	nestedGit(t, top, "commit", "-qm", "first")
	headSHA := nestedGit(t, top, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "delegate")
	nestedGit(t, top, "worktree", "add", "--detach", worktree, "HEAD")
	defer nestedGit(t, top, "worktree", "remove", "--force", worktree)
	nestedWrite(t, worktree, "metasystem/truth/nested-change.txt", "reviewed work\n")
	nestedWrite(t, worktree, "sibling-note.txt", "committed beside the project\n")
	nestedGit(t, worktree, "add", ".")
	nestedGit(t, worktree, "commit", "-qm", "delegate commit carrying a sibling change")

	missionRoot := filepath.Join(top, "metasystem")
	derived, err := projectInstallPrefix(missionRoot)
	if err != nil {
		t.Fatal(err)
	}
	r := &conformanceRun{
		root: missionRoot, job: "j1", rootJob: "j1", record: map[string]any{},
		workspace: worktree, roundText: "1", installPrefix: derived,
		boundaryBase: headSHA, targetSha: headSHA,
	}
	_, errs, code := r.mergeStage(filepath.Join(t.TempDir(), "record.json"))
	if code == 0 {
		t.Fatal("a committed sibling change must refuse the merge stage")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "outside the mission project") ||
		!strings.Contains(joined, "sibling-note.txt") {
		t.Fatalf("the merge refusal must name the fence and the path: %s", joined)
	}
}

// A merge WAIVER in the nested layout still protects the project's own
// plans/: the waiver diff runs over the whole worktree in repository
// space, and its paths must convert to project space before the
// protected-path policy — without the conversion, the project's
// plans/note.md hides behind its repository prefix and a non-mission
// waiver ships a protected change.
func TestNestedWaiverProtectsProjectPlans(t *testing.T) {
	top := t.TempDir()
	nestedGit(t, top, "init", "-q", "-b", "main")
	nestedWrite(t, top, "metasystem/truth/a.txt", "a\n")
	nestedWrite(t, top, "metasystem/scripts/agents/instruction-bearing-paths.txt", "AGENTS.md\n")
	nestedGit(t, top, "add", ".")
	nestedGit(t, top, "commit", "-qm", "first")
	headSHA := nestedGit(t, top, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "delegate")
	nestedGit(t, top, "worktree", "add", "--detach", worktree, "HEAD")
	defer nestedGit(t, top, "worktree", "remove", "--force", worktree)
	nestedWrite(t, worktree, "metasystem/plans/note.md", "a waived plan change\n")
	nestedGit(t, worktree, "add", ".")
	nestedGit(t, worktree, "commit", "-qm", "waived change touching protected plans")

	missionRoot := filepath.Join(top, "metasystem")
	derived, err := projectInstallPrefix(missionRoot)
	if err != nil {
		t.Fatal(err)
	}
	r := &conformanceRun{
		root: missionRoot, job: "j1", rootJob: "j1",
		record:    map[string]any{"critiqueWaived": map[string]any{"class": "prose-under-30"}},
		workspace: worktree, roundText: "1", installPrefix: derived,
		boundaryBase: headSHA, targetSha: headSHA,
	}
	_, errs, code := r.mergeStage(filepath.Join(t.TempDir(), "record.json"))
	if code == 0 {
		t.Fatal("a waiver touching the project's plans/ must refuse")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "trusted plans/ state changed") {
		t.Fatalf("the refusal must name the protected location: %s", joined)
	}
}

// Declarations speak the implementer's own dialect — repository-relative
// from the worktree top — and MUST carry the project prefix in a nested
// layout: a prefix-less declaration refuses by name instead of silently
// matching nothing or aliasing onto a different project path.
func TestNestedDeclarationDialectIsMandatory(t *testing.T) {
	top := t.TempDir()
	nestedGit(t, top, "init", "-q", "-b", "main")
	nestedWrite(t, top, "metasystem/truth/a.txt", "a\n")
	nestedGit(t, top, "add", ".")
	nestedGit(t, top, "commit", "-qm", "first")
	headSHA := nestedGit(t, top, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "delegate")
	nestedGit(t, top, "worktree", "add", "--detach", worktree, "HEAD")
	defer nestedGit(t, top, "worktree", "remove", "--force", worktree)
	nestedWrite(t, worktree, "metasystem/truth/nested-change.txt", "delegate work\n")

	missionRoot := filepath.Join(top, "metasystem")
	nestedWrite(t, missionRoot, "artifacts/agents/j1/rounds/1/return.json",
		`{"diffBoundary": ["truth/nested-change.txt"]}`)
	derived, err := projectInstallPrefix(missionRoot)
	if err != nil {
		t.Fatal(err)
	}
	r := &conformanceRun{
		root: missionRoot, job: "j1", rootJob: "j1", record: map[string]any{},
		workspace: worktree, roundText: "1", installPrefix: derived,
		boundaryBase: headSHA, targetSha: headSHA,
	}
	artifactDir := t.TempDir()
	_, errs, code := r.reviewStage(
		filepath.Join(artifactDir, "diff.patch"), filepath.Join(artifactDir, "review.json"))
	if code == 0 {
		t.Fatal("a prefix-less declaration must refuse in the nested layout")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "does not name a path inside the project") {
		t.Fatalf("the refusal must name the dialect: %s", joined)
	}
}
