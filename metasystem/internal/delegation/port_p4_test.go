package delegation_test

// Ported scenarios of dispatch-fixtures.sh cluster b (lines 2442-3300 at
// 700e8d506): the lifecycle orchestration of follow-up admission, the reap's
// reconciliation, recollection, loss and cap verdicts, the record
// callbacks, the mirror retry and the chain build cache. Owner decisions
// keep their own tests; these prove what the lifecycle does with them.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// newCensusBed is a stubbed-Git bed whose supervision census is fresh, so a
// follow-up passes its entry gates and reaches the chain's admission.
func newCensusBed(t *testing.T) *bed {
	t.Helper()
	b := newBed(t)
	b.writeFile("metasystem.conf", "metasystem.runtimes=fake\nevidence.root="+t.TempDir()+"\n")
	b.writeFile("bin/metasystem", "engine bytes\n")
	b.armSupervision()
	return b
}

func (b *bed) exists(relative string) bool {
	_, err := os.Stat(filepath.Join(b.root, relative))
	return err == nil
}

// L2477, L3284: a follow-up whose newest round is pending (its dispatcher
// still launching) or cancelled is refused toward a fresh dispatch, and
// nothing of a round 2 is published.
func TestP4FollowUpRefusesAPendingOrCancelledNewestRound(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"pending", "cancelled"} {
		b := newCensusBed(t)
		job := status + "-chain"
		b.writeRecord(job, map[string]any{"status": status, "role": "design-critic", "round": 1, "dispatchMode": "fresh",
			"launchMode": "shared-checkout", "workspaceRoot": b.root, "destructiveReach": "MECHANICAL", "sessionId": "s-1"})
		message := b.writeFile("follow.md", "follow up\n")
		result := b.run("follow-up", "--job", job, "--message", message)
		requireExit(t, result, 1, b.stderr.String())
		if !strings.Contains(b.stderr.String(), "follow-up refused: a follow-up does not continue the newest round") {
			t.Fatalf("%s: stderr %q", status, b.stderr.String())
		}
		if b.exists("artifacts/agents/jobs/"+job+"-r2.json") || b.exists("artifacts/agents/"+job+"/rounds/2") || len(b.calls("adapter.Launch")) != 0 {
			t.Fatalf("%s: a refused follow-up published a round: %v", status, b.doubles.Log.Calls())
		}
		if b.record(job)["status"] != status {
			t.Fatalf("%s: the refused follow-up moved its parent: %v", status, b.record(job))
		}
	}
}

// L3073-3105: a capped implementer chain whose worktree is gone has nothing
// to continue.
func TestP4FollowUpAfterACapRefusesWhenTheWorktreeIsGone(t *testing.T) {
	t.Parallel()
	b := newCensusBed(t)
	gone := filepath.Join(b.root, "artifacts", "agents", "worktrees", "capped-gone")
	b.writeRecord("capped-gone", map[string]any{"status": "timeout", "error": "budget-cap", "role": "implementer", "round": 1,
		"dispatchMode": "fresh", "launchMode": "worktree", "workspaceRoot": gone, "destructiveReach": "MECHANICAL", "sessionId": "s-1"})
	message := b.writeFile("follow.md", "follow up\n")
	result := b.run("follow-up", "--job", "capped-gone", "--message", message)
	requireExit(t, result, 1, b.stderr.String())
	want := fmt.Sprintf("follow-up refused: the chain's worktree %s is gone, so nothing is left to continue\nuse a fresh dispatch", gone)
	if !strings.Contains(b.stderr.String(), want) {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if b.exists("artifacts/agents/jobs/capped-gone-r2.json") || len(b.doubles.Git.Calls) != 0 {
		t.Fatalf("the refusal published a round or reached Git: %v", b.doubles.Git.Calls)
	}
}

// L3296-3297: a parent without a resumable session points to the
// fresh-context embed fallback.
func TestP4FollowUpWithoutASessionPointsToTheEmbedFallback(t *testing.T) {
	t.Parallel()
	b := newCensusBed(t)
	b.writeRecord("default-role", map[string]any{"status": "completed", "role": "verifier", "round": 1, "sessionId": nil,
		"dispatchMode": "fresh", "launchMode": "shared-checkout", "workspaceRoot": b.root, "destructiveReach": "MECHANICAL"})
	message := b.writeFile("follow.md", "follow up\n")
	result := b.run("follow-up", "--job", "default-role", "--message", message)
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "follow-up has no resumable session id; use the fresh-context embed fallback") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
		t.Fatalf("outcome %v", outcome)
	}
	if b.exists("artifacts/agents/jobs/default-role-r2.json") {
		t.Fatal("the refusal published a child record")
	}
}

// deadCreator is an exact creator breadcrumb that names no live process:
// this process's pid with a start that is not its own, so the kernel read
// proves the recorded incarnation gone.
func deadCreator(t *testing.T) map[string]any {
	t.Helper()
	exact, state, err := identity.KernelProber{}.ReadStart(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read this process's start: %v %v", state, err)
	}
	ref := exact.Ref()
	fields := map[string]any{"pid": ref.Pid, "pidStartedAt": ref.StartedAtSec - 1, "recordedAt": time.Now().UTC().Format(time.RFC3339)}
	if ref.StartTicks != 0 || ref.BootID != "" {
		fields["pidStartTicks"], fields["bootId"] = ref.StartTicks-100, ref.BootID
	} else {
		fields["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro - 1_000_000
	}
	return fields
}

// L2480-2554: a pending, identityless reservation (no supervisor was ever
// published) is left alone inside its handshake window; once the window
// ends, the reap routes it through nonce-wide reconciliation, whose census
// (injected: no process carries the tag) and dead creator conclude it
// creator-abandoned, and it never claims a process loss or a group death.
func TestP4ReapReconcilesAnIdentitylessReservationOnlyAfterItsWindow(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	real, err := b.doubles.Process.ClaimProcesses()
	if err != nil {
		t.Fatal(err)
	}
	b.doubles.Process.ClaimProcessesFunc = func() (delegation.ClaimProcesses, error) {
		return delegation.ClaimProcesses{Reader: real.Reader, Scanner: noTaggedProcesses{}, Verifier: real.Verifier}, nil
	}
	now := b.doubles.Clock.Now()
	b.writeRecord("launch-window", map[string]any{
		"status": "pending", "phase": "handshake", "role": "design-critic", "round": 1,
		"fingerprintVersion": 2, "fingerprint": "launch-window-fingerprint",
		"instanceTag":     fmt.Sprintf("p4-launch-window-%d-%d", os.Getpid(), time.Now().UnixNano()),
		"startedAt":       now.Format(time.RFC3339),
		"createdAt":       time.Now().UTC().Format(time.RFC3339),
		"creatorLiveness": deadCreator(t),
		"pid":             nil, "pgid": nil, "sessionId": nil,
		"sessionEstablishedTimeoutSec": 60, "capMin": 120,
	})
	requireExit(t, b.run("reap", "--job", "launch-window"), 0, b.stderr.String())
	if record := b.record("launch-window"); record["status"] != "pending" || record["reconciliation"] != nil {
		t.Fatalf("a reservation was reaped or reconciled inside its handshake window: %v", record)
	}
	record := b.record("launch-window")
	record["startedAt"] = "2000-01-01T00:00:00Z"
	encoded, _ := json.Marshal(record)
	b.writeFile("artifacts/agents/jobs/launch-window.json", string(encoded)+"\n")
	requireExit(t, b.run("reap", "--job", "launch-window"), 0, b.stderr.String())
	record = b.record("launch-window")
	evidence, _ := record["reconciliation"].(map[string]any)
	if evidence == nil || evidence["outcome"] != "CREATOR-ABANDONED" {
		t.Fatalf("an out-of-window reservation was not concluded by reconciliation: %v", record)
	}
	if record["status"] != "failed" || record["error"] != "creator-abandoned" || record["phase"] != "reconciliation" {
		t.Fatalf("creator abandonment did not conclude the record: %v", record)
	}
	if record["error"] == "process-lost" || record["groupDeathProvenAt"] != nil || len(b.doubles.Process.Signals) != 0 {
		t.Fatalf("an identityless reservation claimed a process loss or group death: %v signals %v", record, b.doubles.Process.Signals)
	}
}

// L2556-2588: the cancel-pending-setup-husk ordering law. A cancelled
// reservation refuses the setup that would have outrun the stop; the same
// setup against an uncancelled reservation completes, so the refusal is the
// cancel's.
func TestP4CancelledHuskRefusesItsSetup(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	for _, job := range []string{"cancel-husk", "setup-control"} {
		b.writeRecord(job, map[string]any{"status": "pending-setup", "createdAt": b.doubles.Clock.Now().Format(time.RFC3339)})
	}
	requireExit(t, b.run("cancel", "--job", "cancel-husk"), 0, b.stderr.String())
	if record := b.record("cancel-husk"); record["status"] != "cancelled" || record["phase"] != "cancelled" {
		t.Fatalf("the cancel did not stick on the husk: %v", record)
	}
	for _, job := range []string{"cancel-husk", "setup-control"} {
		source := b.writeFile(job+"-setup.json", fmt.Sprintf(`{"jobId":%q,"status":"pending","instanceTag":"tag-%s"}`, job, job))
		result := b.run("__record-setup", "--job", job, "--source", source)
		if job == "setup-control" {
			requireExit(t, result, 0, b.stderr.String())
			continue
		}
		if result.ExitCode == 0 {
			t.Fatal("a cancelled husk completed setup: the stop was outrun")
		}
	}
	if status := b.record("cancel-husk")["status"]; status != "cancelled" {
		t.Fatalf("the refused setup moved the husk to %v", status)
	}
	if b.exists("artifacts/agents/hb/cancel-husk") || b.exists("artifacts/agents/jobs/cancel-husk.log") || len(b.calls("adapter.")) != 0 {
		t.Fatalf("the cancelled husk shows launch traces: %v", b.doubles.Log.Calls())
	}
}

// L2589-2600: a pending round whose supervisor died is concluded by the reap
// even inside its handshake window; a live supervisor defers it.
func TestP4ReapConcludesADeadPendingSupervisor(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	now := b.doubles.Clock.Now()
	for _, job := range []string{"pending-loss", "pending-live"} {
		b.writeRecord(job, map[string]any{"status": "pending", "phase": "handshake", "pid": 8101, "pgid": 8101,
			"instanceTag": "tag-" + job, "startedAt": now.Format(time.RFC3339), "handshakeDeadline": now.Unix() + 60,
			"sessionEstablishedTimeoutSec": 60, "capMin": 60})
	}
	requireExit(t, b.run("reap", "--job", "pending-loss"), 0, b.stderr.String())
	if record := b.record("pending-loss"); record["status"] != "failed" || record["error"] != "process-lost" || record["groupDeathProvenAt"] == nil {
		t.Fatalf("a dead pending supervisor was not concluded: %v", record)
	}
	b.doubles.Process.Tags[8101] = "tag-pending-live"
	requireExit(t, b.run("reap", "--job", "pending-live"), 0, b.stderr.String())
	if record := b.record("pending-live"); record["status"] != "pending" {
		t.Fatalf("a live pending supervisor was reaped: %v", record)
	}
}

const p4DeliveredReturn = `{
  "schemaVersion":3,"jobId":"recollect-loss","round":1,"runtime":"fake",
  "sessionId":"fake-session","model":{"requested":"fake-model","effective":"fake-model"},
  "claimed":{"model":null,"sessionId":null},"mode":"design","reviewedCommit":"abc1234",
  "evidence":[{"command":"fixture delivery","observed":"canonical return recorded","level":"ran"}],
  "gaps":[],"findings":[],"rigor":[],"verdictMaterialCount":0
}`

// L2602-2621: a critic whose supervisor died after its valid return landed
// delivered its work: the reap concludes it completed with recollection
// provenance. A malformed return is still a process loss.
func TestP4ReapRecollectsAReturnDeliveredBeforeTheLoss(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		returned, status string
	}{{p4DeliveredReturn, "completed"}, {`{`, "failed"}} {
		b := newBed(t)
		b.writeRecord("recollect-loss", map[string]any{"status": "running", "role": "design-critic", "round": 1,
			"pid": 8201, "pgid": 8201, "sessionId": "fake-session", "requestedModel": "fake-model", "effectiveModel": "fake-model",
			"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60})
		b.writeFile("artifacts/agents/recollect-loss/rounds/1/return.json", tc.returned)
		b.writeFile("artifacts/agents/recollect-loss/rounds/1/usage.json", `{"cost":{"usd":0.5}}`)
		b.doubles.Adapter.ResultPatchFunc = func(output, failure, phase, usage string) error {
			if failure != "null" || phase != "supervision" || usage == "" {
				t.Errorf("result patch failure=%s phase=%s usage=%q", failure, phase, usage)
			}
			return os.WriteFile(output, []byte(`{"error":null,"phase":"supervision","usage":{"cost":{"usd":0.5}}}`), 0o600)
		}
		requireExit(t, b.run("reap", "--job", "recollect-loss"), 0, b.stderr.String())
		record := b.record("recollect-loss")
		if record["status"] != tc.status {
			t.Fatalf("return %q: record %v", tc.returned, record)
		}
		if tc.status == "completed" {
			if record["recollectedAt"] == nil || record["recollectedFrom"] != "process-lost" || record["error"] != nil {
				t.Fatalf("the recollected record lost its provenance: %v", record)
			}
			if event := b.doubles.Events.Emitted; len(event) != 1 || event[0].Fields["reason"] != "recollected" {
				t.Fatalf("events %v", event)
			}
		} else if record["error"] != "process-lost" || record["recollectedAt"] != nil {
			t.Fatalf("a malformed return was recollected: %v", record)
		}
	}
}

// L2641-2647: a terminal record keeps its first writer. A lost compare exits
// 3 naming what it found; a transition out of a terminal status is illegal.
func TestP4TerminalRecordKeepsItsFirstWriter(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("interrupted", map[string]any{"status": "completed", "error": nil})
	patch := b.writeFile("terminal-race.json", `{"error":"loser"}`)
	result := b.run("__record-cas", "--job", "interrupted", "--expect", "running", "--status", "failed", "--patch", patch)
	requireExit(t, result, 3, b.stderr.String())
	if string(result.Stdout) != "observed=completed\n" {
		t.Fatalf("stdout %q", result.Stdout)
	}
	result = b.run("__record-cas", "--job", "interrupted", "--expect", "completed", "--status", "failed", "--patch", patch)
	if result.ExitCode == 0 || !strings.Contains(b.stderr.String(), "illegal job transition") {
		t.Fatalf("exit %d stderr %q", result.ExitCode, b.stderr.String())
	}
	if record := b.record("interrupted"); record["status"] != "completed" || record["error"] != nil {
		t.Fatalf("the first terminal writer lost: %v", record)
	}
	requireExit(t, b.run("status", "--job", "interrupted"), 0, b.stderr.String())
}

// L2649-2662: the handshake callback fails a round whose runtime reports a
// wider envelope than requested (permissions_mismatch:network) and admits a
// narrower one.
func TestP4HandshakeRecordsAWiderEffectiveEnvelope(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	requested := func(network string) map[string]any {
		return map[string]any{"requested": map[string]any{"readRoots": []any{"."}, "writeRoots": []any{}, "network": network,
			"approvals": "deny", "tools": "read-only"}, "effective": nil, "enforcementSnapshot": "snap.json"}
	}
	b.writeRecord("effective-wider", map[string]any{"status": "pending", "permissions": requested("deny")})
	b.writeRecord("effective-narrower", map[string]any{"status": "pending", "permissions": requested("allow")})
	wider := b.writeFile("wider.json", `{"readRoots":["."],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}`)
	narrower := b.writeFile("narrower.json", `{"readRoots":["."],"writeRoots":[],"network":"deny","approvals":"deny","tools":"read-only"}`)
	result := b.run("__handshake", "--job", "effective-wider", "--session", "s-1", "--model", "fake-model", "--effective", wider, "--signal", "true")
	requireExit(t, result, 1, b.stderr.String())
	if record := b.record("effective-wider"); record["status"] != "failed" || record["error"] != "permissions_mismatch:network" || record["phase"] != "handshake" {
		t.Fatalf("wider envelope record %v", record)
	}
	result = b.run("__handshake", "--job", "effective-narrower", "--session", "s-2", "--model", "fake-model", "--effective", narrower, "--signal", "true")
	requireExit(t, result, 0, b.stderr.String())
	if record := b.record("effective-narrower"); record["status"] != "running" || record["sessionId"] != "s-2" {
		t.Fatalf("narrower envelope record %v", record)
	}
	if calls := b.calls("lease.Authorize"); len(calls) != 4 || !strings.Contains(calls[0], "mode=adapter-writer job=effective-wider") ||
		!strings.Contains(calls[1], "mode=record-writer job=effective-wider") {
		t.Fatalf("the handshake ran outside its adapter-writer authority: %v", calls)
	}
}

// L2938-2951: a running round whose supervisor died with an orphaned child
// still holding its owned group: the reap TERMs the group, records the loss
// and the group's death.
func TestP4ReapTermsTheOrphanedGroupOfALostSupervisor(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("process-loss", map[string]any{"status": "running", "pid": 8301, "pgid": 8301, "sessionId": "s-1",
		"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60, "role": "design-critic"})
	b.doubles.Process.Groups[8301] = true
	b.doubles.Process.Owned[8301] = true
	requireExit(t, b.run("reap", "--job", "process-loss"), 0, b.stderr.String())
	record := b.record("process-loss")
	if record["status"] != "failed" || record["error"] != "process-lost" || record["groupDeathProvenAt"] == nil {
		t.Fatalf("record %v", record)
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"8301:15"}) {
		t.Fatalf("the orphaned group was not TERMed: %v", b.doubles.Process.Signals)
	}
}

// L2953-2981: the explicit cap deadline owns budget expiration even while
// startedAt and capMin say the budget remains; the whole owned group is
// wound down and its death recorded.
func TestP4ReapHonorsTheExplicitCapDeadline(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("timed", map[string]any{"status": "running", "pid": 8401, "pgid": 8401, "sessionId": "s-1",
		"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60, "capDeadline": "2000-01-01T00:01:00Z"})
	b.doubles.Process.Tags[8401] = "tag-timed"
	b.doubles.Process.Groups[8401] = true
	b.doubles.Process.Owned[8401] = true
	requireExit(t, b.run("reap", "--job", "timed"), 0, b.stderr.String())
	record := b.record("timed")
	if record["status"] != "timeout" || record["error"] != "budget-cap" || record["groupDeathProvenAt"] == nil {
		t.Fatalf("record %v", record)
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"8401:15"}) {
		t.Fatalf("signals %v", b.doubles.Process.Signals)
	}
}

func sha256Of(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// L3155-3181: a scripted first mirror failure leaves the terminal record
// unmirrored; the next reap mirrors it, a further reap changes nothing, and
// the manifest the record stamps binds every mirrored file.
func TestP4ReapRetriesAFailedMirrorIdempotently(t *testing.T) {
	t.Parallel()
	b := newCensusBed(t)
	b.writeRecord("mirror-retry", map[string]any{"status": "completed", "round": 1, "role": "design-critic", "error": nil,
		"capabilitySnapshot": "artifacts/agents/capabilities/fake-current.json"})
	b.writeFile("artifacts/agents/capabilities/fake-current.json", `{"runtime":"fake"}`)
	b.writeFile("artifacts/agents/mirror-retry/rounds/1/return.json", `{"ok":true}`)
	b.writeFile("artifacts/agents/mirror-retry/.mirror-fail-once", "")
	requireExit(t, b.run("reap", "--job", "mirror-retry"), 0, b.stderr.String())
	if !b.exists("artifacts/agents/mirror-retry/.mirror-failed") {
		t.Fatal("the scripted first mirror failure did not occur")
	}
	if _, has := b.record("mirror-retry")["mirror"]; has {
		t.Fatalf("a failed mirror stamped the record: %v", b.record("mirror-retry"))
	}
	requireExit(t, b.run("reap", "--job", "mirror-retry"), 0, b.stderr.String())
	mirror, _ := b.record("mirror-retry")["mirror"].(map[string]any)
	if mirror == nil || mirror["manifest"] == nil || mirror["path"] == nil {
		t.Fatalf("the retried reap did not mirror: %v (stderr %q)", b.record("mirror-retry"), b.stderr.String())
	}
	requireExit(t, b.run("reap", "--job", "mirror-retry"), 0, b.stderr.String())
	again, _ := b.record("mirror-retry")["mirror"].(map[string]any)
	if again["manifest"] != mirror["manifest"] || b.record("mirror-retry")["status"] != "completed" {
		t.Fatalf("an idempotent retry changed the terminal state or durable content: %v then %v", mirror, again)
	}
	home, _ := mirror["path"].(string)
	if sha256Of(t, filepath.Join(home, "manifest.json")) != mirror["manifest"] {
		t.Fatal("the manifest digest does not match the record's stamp")
	}
	var manifest struct {
		Files map[string]struct {
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	content, _ := os.ReadFile(filepath.Join(home, "manifest.json"))
	if err := json.Unmarshal(content, &manifest); err != nil || len(manifest.Files) == 0 {
		t.Fatalf("manifest %s: %v", content, err)
	}
	for relative, entry := range manifest.Files {
		if sha256Of(t, filepath.Join(home, relative)) != entry.SHA256 {
			t.Fatalf("mirrored file digest mismatch: %s", relative)
		}
	}
}

// Nothing but the steward's trimmer removes a build cache (disk-lifetimes
// A7, 3.3): every round of every chain builds in the one machine delegate
// cache, so a reap, a post-wait reap, a reap sweep and a chain close leave
// whatever a round wrote beside its worktree's git dir, and the worktree.
func TestP4ReapNeverRemovesABuildCache(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	worktree := filepath.Join(b.root, "artifacts", "agents", "worktrees", "cache-chain")
	gitDir := filepath.Join(b.root, ".git", "worktrees", "cache-chain")
	sentinel := filepath.Join(gitDir, "metasystem-build-cache", "go-cache", "warm-cache-sentinel")
	b.writeFile(strings.TrimPrefix(sentinel, b.root+"/"), "created-by-round-1\n")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	b.doubles.Git.Responses[worktree+"|rev-parse --absolute-git-dir"] = fake.GitResponse{Stdout: gitDir + "\n"}
	b.writeRecord("cache-chain", map[string]any{"status": "completed", "round": 1, "workspaceRoot": worktree})
	b.writeRecord("cache-chain-r2", map[string]any{"status": "completed", "round": 2, "parentJob": "cache-chain", "workspaceRoot": worktree})

	requireExit(t, b.run("__reap-held", "--job", "cache-chain-r2", "--purpose", "post-wait"), 0, b.stderr.String())
	requireExit(t, b.run("reap", "--job", "cache-chain"), 0, b.stderr.String())
	requireExit(t, b.run("reap"), 0, b.stderr.String())
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "created-by-round-1\n" {
		t.Fatalf("a reap of a terminal chain removed a build cache: %q %v", data, err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("the reap removed the worktree: %v", err)
	}
}

// noTaggedProcesses is a complete census in which no process carries the tag.
type noTaggedProcesses struct{}

func (noTaggedProcesses) ScanTag(string, time.Time) census.TaggedProcessCensus {
	return census.TaggedProcessCensus{}
}
