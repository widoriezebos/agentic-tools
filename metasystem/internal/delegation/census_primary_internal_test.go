package delegation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// censusPrimaryBed is a primary checkout whose installation (metasystem/)
// the running system censuses, and a linked worktree of it whose own
// installation has never had a census, the shape of a goal worktree.
type censusPrimaryBed struct {
	primary, primaryInstall string
	linked, linkedInstall   string
	now                     time.Time
}

func newCensusPrimaryBed(t *testing.T) censusPrimaryBed {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := censusPrimaryBed{
		primary: filepath.Join(base, "primary"),
		linked:  filepath.Join(base, "primary-some-goal"),
		now:     time.Unix(1_800_000_000, 0),
	}
	b.primaryInstall = filepath.Join(b.primary, "metasystem")
	b.linkedInstall = filepath.Join(b.linked, "metasystem")
	git := func(dir string, args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.MkdirAll(filepath.Join(b.primaryInstall, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"bin/metasystem": "engine\n",
		".gitignore":     "artifacts/\n",
	} {
		if err := os.WriteFile(filepath.Join(b.primaryInstall, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(b.primary, "init", "-q", "-b", "main")
	git(b.primary, "add", "-A")
	git(b.primary, "commit", "-qm", "bed baseline")
	git(b.primary, "worktree", "add", "-q", "-b", "goal/some-goal", b.linked)
	return b
}

// arm writes a fresh, fingerprint-matched census for the primary's
// installation, completed ageSec seconds before the bed's now.
func (b censusPrimaryBed) arm(t *testing.T, ageSec int64) {
	t.Helper()
	supervision := filepath.Join(b.primaryInstall, "artifacts", "agents", "supervision")
	if err := os.MkdirAll(supervision, 0o755); err != nil {
		t.Fatal(err)
	}
	state := []byte(`{"generation":1,"intervalSec":60}`)
	if err := os.WriteFile(filepath.Join(supervision, "state.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(state)
	fingerprint, err := census.Fingerprint(b.primaryInstall, b.primary)
	if err != nil {
		t.Fatalf("census fingerprint: %v", err)
	}
	encoded, _ := json.Marshal(map[string]any{
		"schemaVersion": 2, "writer": census.VerdictWriter, "verdict": "SUCCESS",
		"completedAtEpoch": b.now.Unix() - ageSec, "intervalSec": 60, "fingerprint": fingerprint,
		"counts": map[string]any{}, "inventory": []any{}, "diagnostics": []any{}, "errors": []any{},
		"generation": 1, "stateDigest": hex.EncodeToString(sum[:]),
	})
	if err := os.WriteFile(filepath.Join(supervision, "last-census.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}

// session is a delegate session rooted at the linked worktree's
// installation, the root `work review` hands the critic launcher.
func (b censusPrimaryBed) session(stderr *bytes.Buffer) *session {
	return &session{
		l:         &Lifecycle{ports: Ports{Git: ownerGit{}, Clock: &stubClock{now: b.now}}},
		ctx:       context.Background(),
		stderr:    stderr,
		root:      b.linkedInstall,
		repoScope: b.linked,
		agents:    filepath.Join(b.linkedInstall, "artifacts", "agents"),
	}
}

// A goal worktree never has a census of its own: the system runs from its
// primary checkout. The gate reads the primary's census, and still refuses
// when that census is absent or stale, naming the primary checkout.
func TestCensusGateLinkedWorktreeReadsPrimaryCensus(t *testing.T) {
	t.Parallel()
	b := newCensusPrimaryBed(t)

	var absent bytes.Buffer
	err := b.session(&absent).requireFreshCensus()
	var exit *Exit
	if !errors.As(err, &exit) || exit.Code != 1 {
		t.Fatalf("absent primary census did not refuse with exit 1: %v\n%s", err, absent.String())
	}
	want := "dispatch refused: census verdict is absent; run metasystem system start --repo " + b.primary + "\n"
	if !strings.Contains(absent.String(), want) {
		t.Fatalf("absent refusal = %q, want it to name the primary checkout: %q", absent.String(), want)
	}

	b.arm(t, 7)
	var fresh bytes.Buffer
	if err := b.session(&fresh).requireFreshCensus(); err != nil {
		t.Fatalf("a fresh primary census refused the linked worktree: %v\n%s", err, fresh.String())
	}

	b.arm(t, 600)
	var stale bytes.Buffer
	err = b.session(&stale).requireFreshCensus()
	if !errors.As(err, &exit) || exit.Code != 1 {
		t.Fatalf("stale primary census did not refuse with exit 1: %v\n%s", err, stale.String())
	}
	if !strings.Contains(stale.String(), "health check is 600s old") || !strings.Contains(stale.String(), "--repo "+b.primary+"\n") {
		t.Fatalf("stale refusal = %q, want the age and the primary checkout", stale.String())
	}
}

// The watcher that bounds a job's cap is the primary checkout's too: a
// goal worktree's launch reads the ceiling its primary's watcher attests.
func TestJobCapLinkedWorktreeReadsPrimaryWatcherCeiling(t *testing.T) {
	t.Parallel()
	b := newCensusPrimaryBed(t)
	conf := "dispatch.cap-min=120\ndispatch.cap-max=900\ncap.min.implementer.fake.fake-model=500\n"
	if err := os.WriteFile(filepath.Join(b.linkedInstall, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	supervision := filepath.Join(b.primaryInstall, "artifacts", "agents", "supervision")
	if err := os.MkdirAll(supervision, 0o755); err != nil {
		t.Fatal(err)
	}
	heartbeat := filepath.Join(supervision, "watcher.heartbeat.json")
	for path, body := range map[string]string{
		heartbeat:                                fmt.Sprintf(`{"pid":4242,"pidStartedAt":7,"instanceTag":"w","loadedCapMin":330,"observedAtEpoch":%d}`, b.now.Unix()),
		filepath.Join(supervision, "state.json"): fmt.Sprintf(`{"intervalSec":60,"components":{"watcher":{"pid":4242,"pidStartedAt":7,"instanceTag":"w","heartbeat":%q}}}`, heartbeat),
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stderr bytes.Buffer
	s := b.session(&stderr)
	output := filepath.Join(t.TempDir(), "cap.json")
	if err := s.authorizeJobCap("job-b", "implementer", "fake", "fake-model", "", "", "300", "dispatch", output); err != nil {
		t.Fatalf("a cap below the primary watcher's ceiling refused the linked worktree: %v %s", err, stderr.String())
	}
	stderr.Reset()
	err := s.authorizeJobCap("job-a", "implementer", "fake", "fake-model", "", "", "", "dispatch", output)
	if ExitCode(err) != 1 || !strings.Contains(stderr.String(), "its 500m cap is not below the watcher's 330m ceiling") {
		t.Fatalf("exit %d stderr %q", ExitCode(err), stderr.String())
	}
}
