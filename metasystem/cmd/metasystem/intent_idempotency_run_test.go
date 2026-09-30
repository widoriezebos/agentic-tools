package main

// Idempotency rows of the run and administration objects (R-129-ui, unit
// U-idem): system, machine, ui, settings, session and the top-level status.
// Each stateful action's witness runs it twice, through the router where the
// process bed's seams stand in for the terminal and the arm sequence, or
// through its owner where the router would need a real process, and asserts
// the repeat is success that wrote nothing.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	seatlaunch "github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

func init() {
	registerIdempotency("session wait", idemCreation, "each registration records one new wait with its own id and deadline; --end of a wait already over succeeds and changes nothing", nil)
	registerIdempotency("system status", idemRead, "reads the checkout's process families and fence, or the steward's view; changes nothing", nil)
	registerIdempotency("system check", idemRead, "the health preview; it writes no observation record (TestIntentProcessAndAnswerTargets)", nil)
	registerIdempotency("machine list", idemRead, "reads every machine's presence and each checkout of this computer as system status reads it; --refresh fetches a newer copy of that derived state, which a repeat fetches again with the same effect", nil)
	registerIdempotency("ui status", idemRead, "reads the interface record; removing a dead server's stale record is repair of derived state", nil)
	registerIdempotency("settings show", idemRead, "reads settings", nil)
	registerIdempotency("settings keys", idemRead, "reads settings", nil)
	registerIdempotency("settings check", idemRead, "validates settings and the testing contract; runs nothing", nil)
	registerIdempotency("session status", idemRead, "reads one Stop report", nil)
	registerIdempotency("status", idemRead, "reads the checkout overview or one goal's work", nil)

	registerIdempotency("system restart", idemCreation,
		"an explicit request for fresh processes: it stops every helper and job and starts new ones, so a second restart is a second cycle, not a repeat; a start of what already runs is system start", nil)
	registerIdempotency("ui restart", idemCreation,
		"an explicit request for a fresh interface process running the executable on disk; a second restart replaces the server again, so it is not a repeat; a start of what already runs is ui start", nil)
	registerIdempotency("session start", idemCreation,
		"each call renews this session's checkout lease (a heartbeat: renewedAt and revision advance) and re-verifies its helpers; the renewal is the act, and the lease's liveness depends on it; the announcement, main id and running helpers are reused, never duplicated", nil)

	registerIdempotency("system start", idemStateful, "MetaSystem already runs for this checkout: success, no fence generation", witnessSystemStartRepeat)
	registerIdempotency("system stop", idemStateful, "already stopped with nothing running: success, no fence generation", witnessSystemStopRepeat)
	registerIdempotency("system setup", idemStateful, "the checkout's hooks and commit fence already run the engine: success, no settings or hook written", witnessSystemSetupRepeat)
	registerIdempotency("system enroll", idemStateful, "the same person at the same terminal is already enrolled: success, no enrollment generation, no fleet publication", witnessSystemEnrollRepeat)
	registerIdempotency("machine stop", idemStateful, "every machine already stopped with nothing of MetaSystem's running: success, no fence generation, no launch touched", witnessMachineStopRepeat)
	registerIdempotency("machine start", idemStateful, "a machine already launched and supervised from here: success, no launch record", witnessMachineStartRepeat)
	registerIdempotency("ui start", idemStateful, "the interface already runs at the address asked for: success, nothing launched", witnessUIStartRepeat)
	registerIdempotency("ui stop", idemStateful, "the interface is not running: success, nothing signalled", witnessUIStopRepeat)
	registerIdempotency("settings coordinator", idemStateful, "--declare of a declared or --withdraw of an undeclared checkout: success, nothing touched", witnessCoordinatorRepeat)
	registerIdempotency("session stop", idemStateful, "the same person's unspent authorization for this session holds: success, no second authorization", witnessSessionStopRepeat)
	registerIdempotency("session handoff", idemStateful, "--cancel of a cancelled handoff: success, nothing written; --note is a new handoff and --status/--verify are reads", witnessSessionHandoffCancelRepeat)
	registerIdempotency("session isolate", idemStateful, "a named isolated worktree already exists: success, nothing created or armed; an unnamed isolate mints a new one", witnessSessionIsolateRepeat)
}

// idemTreeDigest is every regular file under root with its content digest;
// lock files, which a repeat may open without recording anything, are left
// out.
func idemTreeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		name := entry.Name()
		if !entry.Type().IsRegular() || strings.HasSuffix(name, ".flock") || strings.HasSuffix(name, ".lock") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		files[path] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func idemSameTree(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for path, digest := range after {
		if before[path] != digest {
			t.Errorf("%s wrote %s", what, path)
		}
	}
	for path := range before {
		if _, kept := after[path]; !kept {
			t.Errorf("%s removed %s", what, path)
		}
	}
}

func witnessSystemStartRepeat(t *testing.T) {
	b := newProcessBed(t)
	b.helpersRun = true
	if code, result := b.runJSON(b.owners(), "system", "start"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first start = %d %+v", code, result)
	}
	fence := b.fence()
	before := idemTreeDigest(t, b.root())
	code, result := b.runJSON(b.owners(), "system", "start")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "MetaSystem already runs for this checkout (pid 4242 since "+fence.ChangedAt+")") {
		t.Fatalf("repeated start = %d %+v", code, result)
	}
	if after := b.fence(); after.Generation != fence.Generation || after.ChangedAt != fence.ChangedAt {
		t.Fatalf("a repeated start moved the fence %+v to %+v", fence, after)
	}
	idemSameTree(t, "a repeated start", before, idemTreeDigest(t, b.root()))
}

func witnessSystemStopRepeat(t *testing.T) {
	b := newProcessBed(t)
	if code, result := b.runJSON(b.owners(), "system", "stop"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first stop = %d %+v", code, result)
	}
	fence := b.fence()
	before := idemTreeDigest(t, b.root())
	code, result := b.runJSON(b.owners(), "system", "stop")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already stopped") {
		t.Fatalf("repeated stop = %d %+v", code, result)
	}
	if after := b.fence(); after.Generation != fence.Generation {
		t.Fatalf("a repeated stop moved the fence %+v to %+v", fence, after)
	}
	idemSameTree(t, "a repeated stop", before, idemTreeDigest(t, b.root()))
	// Anything that restarts the checkout makes the next stop a real one.
	if code, result := b.runJSON(b.owners(), "system", "start"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("start after stop = %d %+v", code, result)
	}
	if code, result := b.runJSON(b.owners(), "system", "stop"); code != 0 || result.Outcome != intentConfirmed || b.fence().Generation != fence.Generation+2 {
		t.Fatalf("stop after start = %d %+v; fence %+v", code, result, b.fence())
	}
}

func witnessSystemEnrollRepeat(t *testing.T) {
	b := newProcessBed(t)
	// The terminal owner reads this process as a person's shell on one
	// terminal; the fleet publication proves the same terminal.
	reader := goalSyncTerminalReader(t, b.root(), "tty-idempotency")
	b.facts.reader = &reader
	owners := b.owners()
	owners.processes.enroll = func(root string, _ int64, _ humanauthority.Reader, by string, now time.Time) (humanauthority.Enrollment, error) {
		return humanauthority.Enroll(root, reader.exact.Pid, reader, by, now)
	}
	code, result := b.runJSON(owners, "system", "enroll", "--name", "Wido")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first enrollment = %d %+v", code, result)
	}
	publications := b.repo.publications
	before := idemTreeDigest(t, b.root())
	code, result = b.runJSON(owners, "system", "enroll", "--name", "Wido")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already enrolled for Wido") {
		t.Fatalf("repeated enrollment = %d %+v", code, result)
	}
	if b.repo.publications != publications {
		t.Fatalf("a repeated enrollment published %d times", b.repo.publications-publications)
	}
	idemSameTree(t, "a repeated enrollment", before, idemTreeDigest(t, b.root()))
	// Another name at this terminal is a change: a new generation.
	if code, result := b.runJSON(owners, "system", "enroll", "--name", "Ada"); code != 0 || result.Outcome == intentUnchanged {
		t.Fatalf("enrollment under another name = %d %+v", code, result)
	}
}

func witnessMachineStartRepeat(t *testing.T) {
	root := t.TempDir()
	clone := filepath.Join(t.TempDir(), "agentic-tools-m1f")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	ended := "2026-09-25T12:00:00Z"
	record := seatlaunch.Record{SchemaVersion: seatlaunch.SchemaVersion, Launch: "01K5ZZZZZZZZZZZZZZZZZZZZZZ", Machine: "m1f", Destination: clone,
		StartedAt: "2026-09-25T11:00:00Z", EndedAt: &ended, Outcome: seatlaunch.OutcomeDone}
	if err := seatlaunch.Save(root, record); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, map[string]any) {
		t.Helper()
		var stdout bytes.Buffer
		code := runSeatLaunch(append([]string{"--from", root, "--json"}, args...), &stdout, t.Output())
		var printed map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &printed); err != nil {
			t.Fatalf("%v printed no record: %v %q", args, err, stdout.String())
		}
		return code, printed
	}
	before := idemTreeDigest(t, root)
	for _, args := range [][]string{{"--machine", "m1f"}, {"--machine", "m1f"}, {"--resume", record.Launch}} {
		code, printed := run(args...)
		if code != 0 || printed["alreadyLaunched"] != true || printed["launch"] != record.Launch {
			t.Fatalf("%v = %d %v", args, code, printed)
		}
	}
	idemSameTree(t, "a repeated machine start", before, idemTreeDigest(t, root))
	if launched, already, err := seatlaunch.Launched(root, seatlaunch.Request{Machine: "m1f", Destination: filepath.Join(t.TempDir(), "elsewhere")}); err != nil || already {
		t.Fatalf("another destination is a change, not a repeat: %+v %v %v", launched, already, err)
	}
}

// idemUIProber answers for one exact process: alive, and dead for any other.
type idemUIProber struct{ exact identity.Exact }

func (p idemUIProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == p.exact.Pid {
		return p.exact, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

// idemUIChild is a spawned interface server that is ready at once.
type idemUIChild struct{ address string }

func (c idemUIChild) ReadyLine(time.Duration) (string, error) { return "ready " + c.address, nil }
func (c idemUIChild) Pid() int                                { return 4343 }
func (c idemUIChild) Kill() error                             { return nil }
func (c idemUIChild) Release() error                          { return nil }

func idemUIBed(t *testing.T) (lifecycle.Roots, identity.Exact) {
	t.Helper()
	root := t.TempDir()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe this process: %v %v", state, err)
	}
	return lifecycle.Roots{Checkout: root, Installation: root, StateRoot: root}, exact
}

func witnessUIStartRepeat(t *testing.T) {
	roots, exact := idemUIBed(t)
	const listen = "127.0.0.1:8765"
	spawns := 0
	effects := uiLifecycleEffects{
		prober:     idemUIProber{exact: exact},
		executable: func() (string, error) { return "/fake/metasystem", nil },
		spawn: func(spec lifecycle.LaunchSpec) (lifecycle.Child, error) {
			spawns++
			// The server records itself as it binds; this one is the test
			// process, which the prober reports alive.
			process, err := identity.EncodeRef(exact.Ref())
			if err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(lifecycle.Record{SchemaVersion: 1, Process: process, Address: listen, Checkout: roots.Checkout,
				Installation: roots.Installation, StartedAt: "2026-09-25T12:00:00Z", EngineBuild: "fixture"})
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(lifecycle.Dir(roots.StateRoot), "server.json"), encoded, 0o644); err != nil {
				return nil, err
			}
			return idemUIChild{address: listen}, nil
		},
	}
	if first := uiLifecycleRunWith("start", roots, listen, 0, effects); first.Result.Code != 0 || first.Unchanged || spawns != 1 {
		t.Fatalf("first start = %+v, spawns %d", first, spawns)
	}
	before := idemTreeDigest(t, roots.StateRoot)
	second := uiLifecycleRunWith("start", roots, listen, 0, effects)
	if second.Result.Code != 0 || !second.Unchanged || spawns != 1 || !strings.Contains(strings.Join(second.Result.Lines, "\n"), "already runs at http://"+listen) {
		t.Fatalf("repeated start = %+v, spawns %d", second, spawns)
	}
	idemSameTree(t, "a repeated interface start", before, idemTreeDigest(t, roots.StateRoot))
	// Another address is a change: the start runs (and its server refuses).
	if other := uiLifecycleRunWith("start", roots, "127.0.0.1:9999", 0, effects); other.Unchanged || spawns != 2 {
		t.Fatalf("start at another address = %+v, spawns %d", other, spawns)
	}
}

func witnessUIStopRepeat(t *testing.T) {
	roots, exact := idemUIBed(t)
	effects := uiLifecycleEffects{prober: idemUIProber{exact: exact}, executable: func() (string, error) { return "", errors.New("no launch") },
		spawn: func(lifecycle.LaunchSpec) (lifecycle.Child, error) {
			return nil, errors.New("a stop launched the interface")
		}}
	if first := uiLifecycleRunWith("stop", roots, "", 0, effects); first.Result.Code != 0 || !first.Unchanged {
		t.Fatalf("first stop = %+v", first)
	}
	before := idemTreeDigest(t, roots.StateRoot)
	if second := uiLifecycleRunWith("stop", roots, "", 0, effects); second.Result.Code != 0 || !second.Unchanged {
		t.Fatalf("repeated stop = %+v", second)
	}
	idemSameTree(t, "a repeated interface stop", before, idemTreeDigest(t, roots.StateRoot))
}

func witnessCoordinatorRepeat(t *testing.T) {
	root := t.TempDir()
	for run := 1; run <= 2; run++ {
		before := idemTreeDigest(t, root)
		var stdout, stderr bytes.Buffer
		// The caller is no person: a repeat whose effect holds needs none.
		code := brainWithdraw(ownercall.Process{}, &stdout, &stderr, root, "Wido", false)
		var printed map[string]any
		if code != 0 || json.Unmarshal(stdout.Bytes(), &printed) != nil || printed["unchanged"] != true {
			t.Fatalf("withdraw %d = %d %q %q", run, code, stdout.String(), stderr.String())
		}
		idemSameTree(t, "a repeated withdrawal", before, idemTreeDigest(t, root))
	}
}

func witnessSessionStopRepeat(t *testing.T) {
	root, _ := sessionStopFileBed(t)
	human, leaseRecord := sessionStopLiveRef(t)
	installSessionStopLease(t, root, leaseRecord)
	installSessionStopAnnouncement(t, root, "session-human", "main-1", human)
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	proof := sessionStopCommandProof(t, root, now)
	store := &goal.Store{Root: root, Now: func() time.Time { return now.Add(time.Minute) }}
	marker := func(by string) goal.SessionStop {
		return goal.SessionStop{SchemaVersion: 3, SessionId: "session-human", HolderMainId: "main-1", ClaimEpoch: 7, By: by,
			WrittenAt: now.Format(time.RFC3339), ExpiresAt: now.Add(sessionStopLifetime).Format(time.RFC3339)}
	}
	first, err := store.WriteSessionStop(marker("Wido"), proof)
	if err != nil {
		t.Fatal(err)
	}
	before := idemTreeDigest(t, root)
	second, err := store.WriteSessionStop(marker("Wido"), proof)
	var already goal.AlreadyHolds
	if !errors.As(err, &already) || second.AuthorizationId != first.AuthorizationId || !strings.Contains(already.Reason, "already authorized once for session-human") {
		t.Fatalf("repeated authorization = %+v %v", second, err)
	}
	idemSameTree(t, "a repeated session stop", before, idemTreeDigest(t, root))
	// Another person's authorization is a change.
	if third, err := store.WriteSessionStop(marker("Ada"), proof); err != nil || third.AuthorizationId == first.AuthorizationId {
		t.Fatalf("another person's authorization = %+v %v", third, err)
	}
}

func witnessSessionHandoffCancelRepeat(t *testing.T) {
	root := t.TempDir()
	const nonce = "6600000000000009"
	if err := steward.MintIntent(root, steward.Intent{Nonce: nonce, Reason: "seatHandoff",
		Handoff: &steward.HandoffBinding{Session: "session-a", RecordedAt: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)}}); err != nil {
		t.Fatal(err)
	}
	if err := steward.CancelIntent(root, nonce, "cancelled by the seat"); err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		before := idemTreeDigest(t, root)
		err := steward.CancelHandoff(root, nonce, steward.HandoffCanceller{})
		var already *steward.HandoffAlreadyCancelled
		if !errors.As(err, &already) || already.Nonce != nonce {
			t.Fatalf("cancel %d of a cancelled handoff = %v", run, err)
		}
		idemSameTree(t, "a repeated cancellation", before, idemTreeDigest(t, root))
	}
	// A nonce that was never cancelled is still no live handoff.
	var refusal *steward.HandoffRefusal
	if err := steward.CancelHandoff(root, "6600000000000008", steward.HandoffCanceller{}); !errors.As(err, &refusal) || refusal.Code != "HANDOFF_NOT_LIVE" {
		t.Fatalf("unknown nonce = %v", err)
	}
}

func witnessSessionIsolateRepeat(t *testing.T) {
	parent := t.TempDir()
	checkout := filepath.Join(parent, "checkout")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	var added, armed, isolated int
	worktrees := "worktree " + checkout + "\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/main\n"
	options := seatlaunch.SecondSessionOptions{
		HarnessRoot: checkout, Name: "s1", Pid: 4242,
		Git: func(args ...string) (string, error) {
			switch {
			case len(args) >= 3 && args[2] == "rev-parse":
				return checkout + "\n", nil
			case len(args) >= 4 && args[2] == "worktree" && args[3] == "add":
				added++
				destination := args[len(args)-2]
				worktrees += "\nworktree " + destination + "\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/" + args[len(args)-3] + "\n"
				return "", os.MkdirAll(destination, 0o755)
			case len(args) >= 4 && args[2] == "worktree" && args[3] == "list":
				return worktrees, nil
			}
			return "", errors.New("unexpected git " + strings.Join(args, " "))
		},
		StartedAt: func(int64) (int64, error) { return 1700000000, nil },
		Token:     func() (string, error) { return "abcd", nil },
		Now:       func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) },
		Isolate: func(_, destination, _, _ string) (string, error) {
			isolated++
			return destination, nil
		},
		ArmSupervision: func(string, []string) error { armed++; return nil },
	}
	first, err := seatlaunch.SecondSession(options)
	if err != nil || added != 1 || armed != 1 || isolated != 1 {
		t.Fatalf("first isolation = %q %v (added %d, armed %d, isolated %d)", first, err, added, armed, isolated)
	}
	before := idemTreeDigest(t, parent)
	second, err := seatlaunch.SecondSession(options)
	var already *seatlaunch.SecondSessionIsolated
	if !errors.As(err, &already) || second != first || already.Path != first || added != 1 || armed != 1 || isolated != 1 {
		t.Fatalf("repeated isolation = %q %v (added %d, armed %d, isolated %d)", second, err, added, armed, isolated)
	}
	idemSameTree(t, "a repeated isolation", before, idemTreeDigest(t, parent))
}
