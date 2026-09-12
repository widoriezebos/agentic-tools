package validate

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestArtifactRegularNoFollowRefusesLinksAndNonFiles(t *testing.T) {
	root := recertGuardRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "real", "review.md"), []byte("review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	absolute, err := artifactRegularNoFollow(root, "artifacts/real/review.md")
	if err != nil || absolute != filepath.Join(root, "artifacts", "real", "review.md") {
		t.Fatalf("regular artifact = %q, %v", absolute, err)
	}
	if err := os.Symlink(filepath.Join(root, "artifacts", "real"), filepath.Join(root, "artifacts", "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactRegularNoFollow(root, "artifacts/linked/review.md"); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked directory = %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "artifacts", "real", "review.md"), filepath.Join(root, "artifacts", "real", "alias.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactRegularNoFollow(root, "artifacts/real/alias.md"); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked file = %v", err)
	}
	if _, err := artifactRegularNoFollow(root, "artifacts/real"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory as artifact = %v", err)
	}
	if _, err := artifactRegularNoFollow(root, "artifacts/real/absent.md"); err == nil {
		t.Fatal("absent artifact")
	}
	for _, relative := range []string{"", "/etc/passwd", "../outside", "artifacts//double", "a\x00b"} {
		if _, err := artifactRegularNoFollow(root, relative); err == nil || !strings.Contains(err.Error(), "not canonical") {
			t.Fatalf("%q = %v", relative, err)
		}
	}
	if _, err := artifactRegularNoFollow(t.TempDir(), "artifacts/real/review.md"); err == nil {
		t.Fatal("a root outside any repository resolved an artifact")
	}
}

func TestRefuseSymlinkedDirectoryWalksEveryComponent(t *testing.T) {
	root := recertGuardRepo(t)
	if err := refuseSymlinkedDirectory(root, filepath.Join(root, "artifacts", "agents", "not-yet-created")); err != nil {
		t.Fatalf("absent directory refused: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if err := refuseSymlinkedDirectory(root, filepath.Join(root, "artifacts", "agents")); err == nil || !strings.Contains(err.Error(), "contains symlink") {
		t.Fatalf("linked parent = %v", err)
	}
	if err := refuseSymlinkedDirectory(root, filepath.Join(root, "..", "sibling")); err == nil || !strings.Contains(err.Error(), "escapes the repository") {
		t.Fatalf("escaping directory = %v", err)
	}
	if err := refuseSymlinkedDirectory(root, root); err != nil {
		t.Fatalf("the root itself refused: %v", err)
	}
	if err := refuseSymlinkedDirectory(t.TempDir(), "x"); err == nil {
		t.Fatal("a root outside any repository passed")
	}
}

func TestTerminalMemberPicksTheHighestRoundOfAClosedChain(t *testing.T) {
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
