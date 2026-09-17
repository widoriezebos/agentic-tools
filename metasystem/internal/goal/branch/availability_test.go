package branch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitRefusesWithoutPushProtocol(t *testing.T) {
	prior := pushProtocolAvailable
	pushProtocolAvailable = false
	t.Cleanup(func() { pushProtocolAvailable = prior })
	_, err := CommitStaged(CommitRequest{})
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != UnavailableCode {
		t.Fatalf("unavailable error = %v", err)
	}
}

func TestCommitHasSingleAmendDecision(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s: %v", strings.Join(args, " "), output, err)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(path, body string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "--", path)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "fixture")
	run("config", "user.email", "fixture@example.invalid")
	write("base.txt", "base")
	run("commit", "-qm", "base")
	base := run("rev-parse", "HEAD")
	request := CommitRequest{Repo: root, Remote: root, EndpointTip: base, GoalID: "goal-a", CheckClaim: func() error { return nil }}
	write("metasystem/code.go", "one")
	request.Kind, request.Unit, request.OpID = Unit, "u1", "amend-decision-unit"
	old, err := CommitStaged(request)
	if err != nil {
		t.Fatal(err)
	}
	write("metasystem/plans/later.md", "later")
	request.Kind, request.Unit, request.OpID = Plan, "", "amend-decision-plan"
	if _, err := CommitStaged(request); err != nil {
		t.Fatal(err)
	}
	write("metasystem/code.go", "two")
	request.Kind, request.Unit, request.OpID, request.Amend = Unit, "u1", "amend-decision-replace", true
	tip, err := CommitStaged(request)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := ValidateRange(root, base, tip, "goal-a")
	if err != nil || len(commits) != 2 || commits[0].ID == old || commits[0].Kind != Unit || commits[1].Kind != Plan {
		t.Fatalf("amend decision commits=%+v old=%s err=%v", commits, old, err)
	}
}
