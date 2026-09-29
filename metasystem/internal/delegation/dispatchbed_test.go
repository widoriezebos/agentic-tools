package delegation_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// moduleRoot is the metasystem module whose shipped assets a dispatch bed
// installs.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// installAsset copies one shipped file of the module into the bed.
func (b *bed) installAsset(relative string) {
	b.t.Helper()
	content, err := os.ReadFile(filepath.Join(moduleRoot(b.t), relative))
	if err != nil {
		b.t.Fatalf("install %s: %v", relative, err)
	}
	info, _ := os.Stat(filepath.Join(moduleRoot(b.t), relative))
	path := filepath.Join(b.root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, content, info.Mode().Perm()); err != nil {
		b.t.Fatal(err)
	}
}

// dispatchBedConfig is the fake-runtime configuration a dispatch bed
// installs; each test may append keys.
const dispatchBedConfig = `metasystem.runtimes=fake
role.default.runtime=fake
role.default.model.fake=fake-model
role.verifier.runtime=fake
role.investigator.runtime=fake
dispatch.cap-min=30
dispatch.cap-max=240
dispatch.max-inline-input-kb=256
dispatch.permissions.implementer=workspace
watch.cap-min=240
watch.interval-sec=60
`

// newDispatchBed is a bed a whole dispatch can run through: the shipped
// roles, packets and presets, a fake runtime with a current capability
// snapshot, an armed supervision state with a fresh census and a live
// watcher attestation, and an adapter double that starts a real, inert,
// detached process as the supervisor and publishes the session handshake
// when the dispatcher waits for it.
//
// It is an integration bed whose real Git is the claim: the lifecycle
// composes owners (brief authority over the delegate base tree, record
// build over the workspace head, envelope expansion over a worktree's git
// dir, the critic read subject, the follow-up rebase plan) that read Git
// inside internal/dispatch, where no lifecycle stub reaches. Tests over it
// carry "Integration" in their names; every other lifecycle test stubs Git.
func newDispatchBed(t *testing.T) *bed {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the dispatch integration bed needs git")
	}
	b := newBed(t)
	b.life = nil
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), []byte(dispatchBedConfig+"evidence.root="+filepath.Join(b.root, "evidence")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var packets struct {
		Roles map[string]struct {
			Sources []struct {
				Path string `json:"path"`
			} `json:"sources"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(protocol.RolePackets(), &packets); err != nil {
		t.Fatalf("role packets: %v", err)
	}
	for _, role := range packets.Roles {
		for _, source := range role.Sources {
			if !protocol.IsReference(source.Path) && !exists(filepath.Join(b.root, source.Path)) {
				b.installAsset(source.Path)
			}
		}
	}
	b.writeFile("bin/metasystem", "engine bytes\n")
	if _, err := adapter.WriteFakeCapabilitySnapshot(filepath.Join(b.root, "artifacts", "agents", "capabilities"), "current", 0, 20); err != nil {
		t.Fatal(err)
	}
	b.doubles.Adapter.ConfigIdentityFunc = func(string) (string, error) {
		return `{"cliVersion":"fake-1","configHash":"fake-config-v1","configKeyHashes":{},"runtime":"fake"}`, nil
	}
	b.doubles.Adapter.OutputStreamFunc = func(_ string, roundDir string) (string, error) {
		return filepath.Join(roundDir, "stream.jsonl"), nil
	}
	b.doubles.Adapter.LaunchFunc = b.launchInertSupervisor
	b.doubles.Clock.OnSleep = func(time.Time) { b.publishHandshakes() }
	b.gitInit()
	b.armSupervision()
	real, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: b.root, Host: b.doubles.Host})
	if err != nil {
		t.Fatal(err)
	}
	ports := b.doubles.Ports()
	ports.Git = real.Git
	ports.Goal = bedGoal{Goal: b.doubles.Goal, root: b.root}
	life, err := delegation.New(delegation.Config{Root: b.root, RepoScope: b.root, Engine: "/engine/metasystem"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	b.life = life
	return b
}

// git runs git in the bed's repository.
func (b *bed) git(args ...string) string {
	b.t.Helper()
	command := exec.Command("git", append([]string{"-C", b.root, "-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// gitInit commits the installed bed as the repository's first commit; the
// runtime state under artifacts/ stays untracked.
func (b *bed) gitInit() {
	b.t.Helper()
	b.git("init", "-q", "-b", "main")
	b.writeFile(".gitignore", "artifacts/\nevidence/\n")
	b.git("add", "-A")
	b.git("commit", "-qm", "bed baseline")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// armSupervision writes a generation-1 arming record attested by a fresh,
// fingerprint-matched census and a live watcher heartbeat.
func (b *bed) armSupervision() {
	b.t.Helper()
	supervision := filepath.Join(b.root, "artifacts", "agents", "supervision")
	now := b.doubles.Clock.Now().Unix()
	heartbeat := b.writeFile("artifacts/agents/supervision/watcher.heartbeat.json",
		fmt.Sprintf(`{"pid":4242,"pidStartedAt":7,"instanceTag":"watcher","loadedCapMin":900,"observedAtEpoch":%d}`, now))
	state := fmt.Sprintf(`{"generation":1,"intervalSec":60,"components":{"watcher":{"pid":4242,"pidStartedAt":7,"instanceTag":"watcher","heartbeat":%q}}}`, heartbeat)
	b.writeFile("artifacts/agents/supervision/state.json", state)
	sum := sha256.Sum256([]byte(state))
	fingerprint, err := census.Fingerprint(b.root, b.root)
	if err != nil {
		b.t.Fatalf("census fingerprint: %v", err)
	}
	verdict := map[string]any{
		"schemaVersion": 2, "writer": "watch-background-jobs.sh", "verdict": "SUCCESS",
		"completedAtEpoch": now, "intervalSec": 60, "fingerprint": fingerprint,
		"counts": map[string]any{}, "inventory": []any{}, "diagnostics": []any{}, "errors": []any{},
		"generation": 1, "stateDigest": hex.EncodeToString(sum[:]),
	}
	encoded, _ := json.Marshal(verdict)
	if err := os.WriteFile(filepath.Join(supervision, "last-census.json"), encoded, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// launchInertSupervisor starts `sleep` in its own session as the adapter's
// supervisor: a real pid with a real start identity for the ownership
// proof, which the bed's process double reports live under the launch tag.
func (b *bed) launchInertSupervisor(request delegation.AdapterLaunch) (int64, error) {
	command := exec.Command("sleep", "120")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := int64(command.Process.Pid)
	b.t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	b.doubles.Process.Tags[pid] = request.InstanceTag
	b.launches = append(b.launches, request)
	return pid, nil
}

// publishHandshakes is the fake runtime's session signal: every launched
// round whose start gate is open and whose record still waits in pending
// moves to running with a session and a job log.
func (b *bed) publishHandshakes() {
	for _, launch := range b.launches {
		if !exists(launch.StartGate) || b.noHandshake[launch.Job] {
			continue
		}
		record, err := dispatch.ReadRecordObject(b.recordPath(launch.Job))
		if err != nil || record["status"] != "pending" {
			continue
		}
		patch := filepath.Join(b.t.TempDir(), "handshake.json")
		if err := os.WriteFile(patch, []byte(`{"sessionId":"fake-session-`+launch.Job+`"}`), 0o600); err != nil {
			b.t.Fatal(err)
		}
		if _, err := dispatch.RecordCAS(b.root, launch.Job, "pending", "running", patch); err != nil {
			b.t.Errorf("fake handshake for %s: %v", launch.Job, err)
		}
		b.writeFile("artifacts/agents/jobs/"+launch.Job+".log", "session established\n")
	}
}

// dispatchEnv is the delegate boundary's standing with a fresh claim
// capability for mode.
func (b *bed) dispatchEnv(mode dispatch.DispatchMode) delegation.Env {
	b.t.Helper()
	capability, err := dispatch.MintDelegateClaimCapability(b.root, mode)
	if err != nil {
		b.t.Fatal(err)
	}
	return delegation.Env{RecordOutcome: true, DelegateInternal: true, ClaimCapability: capability, FixtureHazard: "MECHANICAL"}
}

// brief writes a brief with the given working mode and body lines.
func (b *bed) brief(name, mode string, lines ...string) string {
	return b.writeFile(name, "Working Mode: "+mode+"\n\n"+strings.Join(lines, "\n")+"\n")
}

// bedGoal reads the ledger identity through the goal owner's own Git, the
// bed's claim; goal bindings stay the double's.
type bedGoal struct {
	*fake.Goal
	root string
}

func (g bedGoal) LedgerIdentity() string { return goal.ExistingLedgerIdentity(g.root) }
