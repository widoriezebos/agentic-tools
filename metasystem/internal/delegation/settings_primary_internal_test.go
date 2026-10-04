package delegation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

func TestACriticFromAGoalWorktreeIsAuthorizedByThePrimary(t *testing.T) {
	t.Parallel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := toolPrimaryGit{primary: filepath.Join(base, "primary"), linked: filepath.Join(base, "goal")}
	primary, linked := filepath.Join(git.primary, "tools", "install"), filepath.Join(git.linked, "tools", "install")
	for _, dir := range []string{filepath.Join(git.primary, ".git"), primary, linked} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(git.linked, ".git"), "gitdir: synthetic-linked-worktree\n")
	write(filepath.Join(primary, "metasystem.conf"), "metasystem.runtimes=fake\nrole.default.runtime=fake\nrole.verifier.runtime=fake\nrole.verifier.model.fake=base-model\ndispatch.cap-min=21\ndispatch.cap-max=240\nwatch.cap-min=220\nwatch.interval-sec=60\n")
	write(filepath.Join(primary, "metasystem.conf.local"), "role.verifier.model.fake=primary-family\nruntime.fake.model-alias.primary-family=primary-model\ndispatch.cap-min=37\nwatch.cap-min=230\n")
	write(filepath.Join(linked, "metasystem.conf"), "watch.cap-min=7\nwatch.cap-min=8\nrole.verifier.model.fake=worktree-model\ndispatch.cap-min=9\n")
	var stderr bytes.Buffer
	life := &Lifecycle{root: linked, repoScope: git.linked, lookupEnv: func(string) (string, bool) { return "", false }, ports: Ports{Git: git, Lease: &stubLease{}, Goal: stubGoal{}, Guard: ownerGuard{}, Clock: &stubClock{now: time.Unix(1800000000, 0)}}}
	s := life.newSession(context.Background(), Request{Stderr: &stderr})
	if got, err := s.configGet("watch.cap-min", "1"); err != nil || got != "230" {
		t.Fatalf("primary local setting = %q, %v", got, err)
	}
	life.ports.Git = nil
	capFile := filepath.Join(base, "cap.json")
	if err := s.resolveNonmissionCap("verifier", "fake", "primary-model", "", "", capFile); err != nil || fieldOr(capFile, "capMin") != "37" {
		t.Fatalf("primary local cap = %s, %v; %s", fieldOr(capFile, "capMin"), err, stderr.String())
	}
	brief := filepath.Join(base, "brief.md")
	write(brief, "Working Mode: verify\n")
	life.ports.Git = git
	result := life.Run(context.Background(), Request{Env: Env{DelegateInternal: true}, Stderr: &stderr}, []string{"dispatch", "--role", "verifier", "--brief", brief, "--model", "explicit-model", "--destructive-reach", "MECHANICAL"})
	if result.ExitCode != 1 || !strings.Contains(stderr.String(), "the roster gives fake:primary-model") {
		t.Fatalf("dispatch must resolve the primary roster and its alias before refusing the override: exit %d, %s", result.ExitCode, stderr.String())
	}
	if err := os.MkdirAll(filepath.Join(linked, "skills", "code-critique"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(linked, "skills", "code-critique", "SKILL.md"), "Review the implementation against its brief.\n")
	if err := os.MkdirAll(s.recordLocks, 0o755); err != nil {
		t.Fatal(err)
	}
	s.env.DelegateInternal = true
	s.l.ports.Process = ownerProcess{root: linked}
	claim := claimRequest{opID: "primary-critic", session: "claude:critic", dispatchMode: "fresh", adapterVerb: "dispatch", runtime: "claude", model: "critic-model", role: "code-critic", reviews: "commit:" + strings.Repeat("a", 40), launchMode: "shared-checkout", permissionDigest: strings.Repeat("b", 64), inputHash: strings.Repeat("c", 64), destructiveReach: "DESIGN-BEARING"}
	for index, row := range []struct {
		primary, goal string
		claim, packet int
	}{{"critic-model", "other-model", 0, 0}, {"other-model", "critic-model", 1, 9}, {"critic-model", "other-model", 0, 9}} {
		primaryLimit, goalLimit := "64", "1"
		if index == 2 {
			primaryLimit, goalLimit = goalLimit, primaryLimit
		}
		write(filepath.Join(primary, "metasystem.conf.local"), "watch.cap-min=230\nruntime.claude.maximal-models="+row.primary+"\ndispatch.max-inline-input-kb="+primaryLimit+"\n")
		write(filepath.Join(primary, "metasystem.conf"), "runtime.claude.maximal-models="+row.goal+"\n")
		write(filepath.Join(linked, "metasystem.conf"), "runtime.claude.maximal-models="+row.goal+"\ndispatch.max-inline-input-kb="+goalLimit+"\n")
		claim.opID, claim.session = claim.opID+"x", claim.session+"x"
		s.env.ClaimCapability, err = dispatch.MintDelegateClaimCapability(linked, dispatch.DispatchModeFresh)
		if err != nil {
			t.Fatal(err)
		}
		stderr.Reset()
		s.stdout.Reset()
		if result, code := s.claimLaunch(claim, false); code != row.claim || (code == 0 && (!strings.Contains(result, `"outcome":"WON"`) || fieldOr(s.recordPath(claim.opID), "role") != "code-critic")) || (code == 1 && !strings.Contains(stderr.String(), "no executable maximal-effort mapping")) {
			t.Fatalf("row %d claim: exit %d, %s, %s", index, code, result, stderr.String())
		}
		_, err := s.composePacket(composeRequest{role: "code-critic", brief: brief, job: "primary-critic", runtime: "claude", model: "critic-model", round: 1, destructiveReach: "DESIGN-BEARING", toolPolicy: "read-only", capMin: "30", roundDir: filepath.Join(s.agents, "primary-critic", "rounds", "1")})
		if ExitCode(err) != row.packet || (index == 1 && !strings.Contains(s.stdout.String(), "REFUSED-HAZARD-CONFIGURATION")) || (index == 2 && !strings.Contains(s.stdout.String(), "REFUSED-INLINE-INPUT-LIMIT")) {
			t.Fatalf("row %d packet: %v, %s, %s", index, err, s.stdout.String(), stderr.String())
		}
		s.cleanupCompositionTemporaries()
	}
	life.root, life.ports.Git = primary, nil
	self := life.newSession(context.Background(), Request{})
	if got, err := self.configGet("watch.cap-min", "1"); err != nil || got != "230" {
		t.Fatalf("a primary checkout must read its own local setting: %q, %v", got, err)
	}
}

func TestASelectedInstallationSuppliesAnArmedWorktreesDispatchSettings(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	git := toolPrimaryGit{primary: filepath.Join(base, "primary"), linked: filepath.Join(base, "goal")}
	root := filepath.Join(git.linked, "tools", "install")
	selected := filepath.Join(base, "selected")
	state := filepath.Join(root, "artifacts", "agents", "supervision")
	for _, dir := range []string{state, selected} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(git.linked, ".git"):                "gitdir: synthetic-linked-worktree\n",
		filepath.Join(state, "state.json"):               "{}\n",
		filepath.Join(root, "metasystem.conf"):           "metasystem.runtimes=fake\nrole.verifier.runtime=fake\nrole.verifier.model.fake=worktree-model\n",
		filepath.Join(selected, "metasystem.conf"):       "metasystem.runtimes=fake\nrole.verifier.runtime=fake\nrole.verifier.model.fake=selected-base\n",
		filepath.Join(selected, "metasystem.conf.local"): "role.verifier.model.fake=selected-model\n",
		filepath.Join(base, "brief.md"):                  "Working Mode: verify\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	life := &Lifecycle{root: root, repoScope: git.linked, lookupEnv: func(string) (string, bool) { return "", false }, ports: Ports{Git: git, Lease: &stubLease{}, Goal: stubGoal{}, Guard: ownerGuard{}}}
	var stderr bytes.Buffer
	result := life.Run(context.Background(), Request{Env: Env{DelegateInternal: true}, Stderr: &stderr,
		ConfigEnv: []string{dispatch.SelectedInstallationEnv + "=" + selected}},
		[]string{"dispatch", "--role", "verifier", "--brief", filepath.Join(base, "brief.md"), "--model", "explicit-model", "--destructive-reach", "MECHANICAL"})
	if result.ExitCode != 1 || !strings.Contains(stderr.String(), "the roster gives fake:selected-model") {
		t.Fatalf("dispatch must resolve the selected local roster before refusing the override: exit %d, %s", result.ExitCode, stderr.String())
	}
}
