package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func recertGuardRepo(t *testing.T) string {
	t.Helper()
	// The repository's top level comes back resolved (/private/var on macOS),
	// so the fixture root is resolved the same way before paths are compared.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func guardFilesystemRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

type guardTopLevelFacts struct {
	t        *testing.T
	root     string
	expected []string
	next     int
}

func (f *guardTopLevelFacts) answer(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	if f.next >= len(f.expected) {
		f.t.Fatalf("unexpected top-level request from %q: %q", request.Dir, request.Args)
	}
	wantDir := f.expected[f.next]
	f.next++
	wantArgs := append(append([]string{"-C", wantDir}, rawPins...), "rev-parse", "--show-toplevel")
	if request.Dir != wantDir || !reflect.DeepEqual(request.Args, wantArgs) || request.Stdin != nil ||
		request.Operation != "git rev-parse --show-toplevel" || request.Timeout.Limit <= 0 {
		f.t.Fatalf("top-level request %d = %+v, want directory %q and argv %q", f.next, request, wantDir, wantArgs)
	}
	if !reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) {
		f.t.Fatalf("top-level request %d has an unscrubbed environment", f.next)
	}
	if wantDir != f.root {
		return gittree.RawResult{ExitCode: 128, Stderr: []byte("fatal: not a git repository\n")}
	}
	return gittree.RawResult{Stdout: []byte(f.root + "\n")}
}

func (f *guardTopLevelFacts) consumed() {
	f.t.Helper()
	if f.next != len(f.expected) {
		f.t.Fatalf("top-level requests consumed %d of %d", f.next, len(f.expected))
	}
}

func TestArtifactRegularNoFollowRefusesLinksAndNonFiles(t *testing.T) {
	root := guardFilesystemRoot(t)
	outside := t.TempDir()
	facts := &guardTopLevelFacts{t: t, root: root}
	for range 10 {
		facts.expected = append(facts.expected, root)
	}
	facts.expected = append(facts.expected, outside)
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "real", "review.md"), []byte("review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	absolute, err := artifactRegularNoFollowWithRaw(root, "artifacts/real/review.md", facts.answer)
	if err != nil || absolute != filepath.Join(root, "artifacts", "real", "review.md") {
		t.Fatalf("regular artifact = %q, %v", absolute, err)
	}
	if err := os.Symlink(filepath.Join(root, "artifacts", "real"), filepath.Join(root, "artifacts", "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactRegularNoFollowWithRaw(root, "artifacts/linked/review.md", facts.answer); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked directory = %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "artifacts", "real", "review.md"), filepath.Join(root, "artifacts", "real", "alias.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactRegularNoFollowWithRaw(root, "artifacts/real/alias.md", facts.answer); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked file = %v", err)
	}
	if _, err := artifactRegularNoFollowWithRaw(root, "artifacts/real", facts.answer); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory as artifact = %v", err)
	}
	if _, err := artifactRegularNoFollowWithRaw(root, "artifacts/real/absent.md", facts.answer); err == nil {
		t.Fatal("absent artifact")
	}
	for _, relative := range []string{"", "/etc/passwd", "../outside", "artifacts//double", "a\x00b"} {
		if _, err := artifactRegularNoFollowWithRaw(root, relative, facts.answer); err == nil || !strings.Contains(err.Error(), "not canonical") {
			t.Fatalf("%q = %v", relative, err)
		}
	}
	if _, err := artifactRegularNoFollowWithRaw(outside, "artifacts/real/review.md", facts.answer); err == nil {
		t.Fatal("a root outside any repository resolved an artifact")
	}
	facts.consumed()
}

func TestRefuseSymlinkedDirectoryWalksEveryComponent(t *testing.T) {
	t.Parallel()
	root := guardFilesystemRoot(t)
	outside := t.TempDir()
	facts := &guardTopLevelFacts{t: t, root: root, expected: []string{root, root, root, root, outside}}
	if err := refuseSymlinkedDirectoryWithRaw(root, filepath.Join(root, "artifacts", "agents", "not-yet-created"), facts.answer); err != nil {
		t.Fatalf("absent directory refused: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if err := refuseSymlinkedDirectoryWithRaw(root, filepath.Join(root, "artifacts", "agents"), facts.answer); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked parent = %v", err)
	}
	if err := refuseSymlinkedDirectoryWithRaw(root, filepath.Join(root, "..", "sibling"), facts.answer); err == nil || !strings.Contains(err.Error(), "escapes the repository") {
		t.Fatalf("escaping directory = %v", err)
	}
	if err := refuseSymlinkedDirectoryWithRaw(root, root, facts.answer); err != nil {
		t.Fatalf("the root itself refused: %v", err)
	}
	if err := refuseSymlinkedDirectoryWithRaw(outside, "x", facts.answer); err == nil {
		t.Fatal("a root outside any repository passed")
	}
	facts.consumed()
}

func TestRefuseSymlinkedDirectoryNativeTopLevelAdapter(t *testing.T) {
	t.Parallel()
	root := recertGuardRepo(t)
	if err := refuseSymlinkedDirectory(root, root); err != nil {
		t.Fatalf("native repository top level refused: %v", err)
	}
}

func TestTerminalMemberPicksTheHighestRoundOfAClosedChain(t *testing.T) {
	t.Parallel()
	records := map[string]map[string]any{
		"root":  {"round": float64(1), "status": "completed"},
		"r2":    {"round": float64(2), "status": "failed", "parentJob": "root"},
		"r3":    {"round": float64(3), "status": "completed", "parentJob": "r2"},
		"other": {"round": float64(9), "status": "completed"},
		"loop":  {"round": float64(1), "status": "completed", "parentJob": "loop"},
	}
	id, record, round, err := terminalMember(records, "root")
	if err != nil || id != "r3" || round != 3 || record["status"] != "completed" {
		t.Fatalf("terminal member = %q round %d %+v, %v", id, round, record, err)
	}
	if _, _, _, err := terminalMember(records, "absent"); err == nil || !strings.Contains(err.Error(), "has no members") {
		t.Fatalf("absent chain = %v", err)
	}
	records["r4"] = map[string]any{"round": float64(4), "status": "running", "parentJob": "r3"}
	if _, _, _, err := terminalMember(records, "root"); err == nil || !strings.Contains(err.Error(), "r4 is active") {
		t.Fatalf("active member = %v", err)
	}
	records["r4"] = map[string]any{"round": "four", "status": "completed", "parentJob": "r3"}
	if _, _, _, err := terminalMember(records, "root"); err == nil || !strings.Contains(err.Error(), "r4 has malformed round") {
		t.Fatalf("malformed round = %v", err)
	}
	records["r4"] = map[string]any{"round": float64(0), "status": "completed", "parentJob": "r3"}
	if _, _, _, err := terminalMember(records, "root"); err == nil || !strings.Contains(err.Error(), "malformed round") {
		t.Fatalf("zero round = %v", err)
	}
}

func TestCommandForWidthFollowsTheContractAndTheGateWidth(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if _, _, _, err := commandForWidth(root, map[string]any{}, "go test ./..."); err == nil {
		t.Fatal("an absent configuration resolved a width")
	}
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	width, command, schema, err := commandForWidth(root, map[string]any{}, "go test ./...")
	if err != nil || width != "area" || command != "go test ./..." || schema != 1 {
		t.Fatalf("area width = %q %q %d %v", width, command, schema, err)
	}
	if _, _, _, err := commandForWidth(root, map[string]any{}, "   "); err == nil || !strings.Contains(err.Error(), "requires an explicit") {
		t.Fatalf("area width without a command = %v", err)
	}
	width, command, schema, err = commandForWidth(root, map[string]any{"gateWidth": "full"}, "")
	if err != nil || width != "full" || command != FullBatteryCommand || schema != 1 {
		t.Fatalf("full width = %q %q %d %v", width, command, schema, err)
	}
	if _, _, _, err := commandForWidth(root, map[string]any{"gateWidth": "full"}, "go test ./..."); err == nil || !strings.Contains(err.Error(), "byte-equal") {
		t.Fatalf("full width with another command = %v", err)
	}
	for _, malformed := range []any{"wide", 3} {
		if _, _, _, err := commandForWidth(root, map[string]any{"gateWidth": malformed}, ""); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("gate width %v = %v", malformed, err)
		}
	}
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\ntesting.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	width, command, schema, err = commandForWidth(root, map[string]any{"gateWidth": "full"}, "")
	if err != nil || width != "full" || command != "" || schema != 2 {
		t.Fatalf("migrated width = %q %q %d %v", width, command, schema, err)
	}
	if _, _, _, err := commandForWidth(root, map[string]any{}, "go test ./..."); err == nil || !strings.Contains(err.Error(), "schema-2 testing evidence") {
		t.Fatalf("migrated width with a command = %v", err)
	}
}

func TestActiveOperationReadsPseudorefsAndSequencerState(t *testing.T) {
	root := recertGuardRepo(t)
	workspace := gittree.Workspace{Dir: root}
	if err := activeOperation(workspace); err != nil {
		t.Fatalf("a quiet repository reads as active: %v", err)
	}
	gitDir, err := workspace.GitPath("MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitDir, []byte(strings.Repeat("a", 40)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := activeOperation(workspace); err == nil || !strings.Contains(err.Error(), "MERGE_HEAD") {
		t.Fatalf("merge in flight = %v", err)
	}
	if err := os.Remove(gitDir); err != nil {
		t.Fatal(err)
	}
	sequencer, err := workspace.GitPath("rebase-merge")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sequencer, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := activeOperation(workspace); err == nil || !strings.Contains(err.Error(), "rebase-merge") {
		t.Fatalf("rebase in flight = %v", err)
	}
}
