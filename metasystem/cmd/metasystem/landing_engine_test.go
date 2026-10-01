package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// kernelBed is a host whose registered landing lane is a real nested
// checkout (Git toplevel != installation root) with a steward enrollment;
// the kernel verbs run in process, so the executable they hash is this test
// binary, really running.
type kernelBed struct {
	*laneVerbBed
	checkout, installation, installPath string
	advances                            int
}

func newKernelBed(t *testing.T) *kernelBed {
	t.Helper()
	bed := &kernelBed{laneVerbBed: newLaneVerbBed(t)}
	bed.checkout = filepath.Join(filepath.Dir(bed.home), "lane")
	bed.installation = filepath.Join(bed.checkout, "metasystem")
	bed.installPath = filepath.Join(bed.installation, "bin", "metasystem")
	for _, dir := range []string{filepath.Dir(bed.installPath), filepath.Join(bed.installation, "artifacts", "agents", "steward")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "init", "--quiet", "-b", "main", bed.checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registerLane(t, bed.home, bed.checkout, "Wido", laneTestNow)
	return bed
}

// enroll mints the lane installation's enrollment of bytes, written at the
// enrolled install path, built from an older commit.
func (bed *kernelBed) enroll(t *testing.T, enrolled []byte) {
	t.Helper()
	bed.enrollBuiltFrom(t, enrolled, "0123456789abcdef0123456789abcdef01234567")
}

func (bed *kernelBed) enrollBuiltFrom(t *testing.T, enrolled []byte, stamp string) {
	t.Helper()
	if err := testexec.WriteFile(bed.installPath, enrolled, 0o755); err != nil {
		t.Fatal(err)
	}
	id := steward.InstallIdentity{RepoIdentity: bed.installation, Generation: 7, InstallPath: bed.installPath,
		InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(enrolled)), MintedAt: "2026-09-30T18:00:00Z",
		Enrollment: steward.EnrollmentHumanTerminal, EngineBuild: stamp}
	if err := steward.MintIdentity(steward.RepoIdentityPath(bed.installation), id); err != nil {
		t.Fatal(err)
	}
}

func (bed *kernelBed) owners() intentOwners {
	owners := bed.laneVerbBed.owners()
	owners.resolver = stateroot.NewResolver(stateroot.RepositoryTop, os.Executable)
	owners.landing.advance = func(request laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error) {
		bed.advances++
		if request.Checkout != bed.checkout || request.Installation != bed.installation || request.Identity.Running == "" {
			return laneengine.AdvanceOutcome{}, fmt.Errorf("advance request %+v is not the admitted lane", request)
		}
		return laneengine.AdvanceOutcome{Changed: true, Commit: "4b825dc642cb6eb9a060e54bf8d69288fbee4904", PreviousGeneration: 7, Generation: 8}, nil
	}
	return owners
}

func (bed *kernelBed) run(t *testing.T, words ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, bed.owners())
	return code, stdout.String(), stderr.String()
}

func runningTestBinary(t *testing.T) []byte {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The design's K-e witness: a kernel verb hashes its own running
// executable and refuses when it is not the lane's enrolled engine: a
// hand-rebuilt engine or any other executable never acts for the kernel,
// and the refusal is two plain lines naming the enrolled engine. The
// enrolled bytes themselves are admitted and reach the verb.
func TestKernelRefusesUnenrolledExecutable(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.enroll(t, []byte("the lane's enrolled engine, not this process"))
	code, stdout, stderr := bed.run(t, "landing", "engine", "advance")
	if code == 0 || bed.advances != 0 {
		t.Fatalf("an unenrolled executable ran the kernel verb: exit %d, advances %d\n%s%s", code, bed.advances, stdout, stderr)
	}
	text := stdout + stderr
	for _, want := range []string{"isn't the landing lane's enrolled engine", "→ " + bed.installPath + " landing engine advance"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, laneengine.CodeNotEnrolled) || strings.Contains(text, "sha256:") {
		t.Errorf("default refusal text shows internals:\n%s", text)
	}
	_, jsonOut, jsonErr := bed.run(t, "landing", "engine", "advance", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(jsonOut+jsonErr), &result); err != nil || result.Outcome != intentRefused ||
		!strings.Contains(strings.Join(result.Details, "\n"), laneengine.CodeNotEnrolled) {
		t.Fatalf("--json refusal = %+v %v\n%s%s", result, err, jsonOut, jsonErr)
	}

	bed.enroll(t, runningTestBinary(t))
	code, stdout, stderr = bed.run(t, "landing", "engine", "advance")
	if code != 0 || bed.advances != 1 {
		t.Fatalf("the enrolled executable was refused: exit %d, advances %d\n%s%s", code, bed.advances, stdout, stderr)
	}
}

// The call contract: every landing verb whose action is a kernel action
// checks the engine before it does anything. A later unit's kernel verb
// that forgot the check fails here.
func TestEveryLaneKernelVerbChecksItsEngineFirst(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.enroll(t, []byte("the lane's enrolled engine, not this process"))
	checked := 0
	for _, command := range landingIntentCommands() {
		if !laneKernelActions[command.action] {
			continue
		}
		checked++
		words := []string{command.object, command.action}
		if command.action == "engine" {
			words = append(words, "advance")
		}
		_, stdout, stderr := bed.run(t, append(words, "--json")...)
		var result intentResult
		if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil || result.Outcome != intentRefused ||
			!strings.Contains(strings.Join(result.Details, "\n"), laneengine.CodeNotEnrolled) {
			t.Errorf("%s ran without its engine check: %+v %v\n%s%s", strings.Join(words, " "), result, err, stdout, stderr)
		}
	}
	if checked == 0 || bed.advances != 0 {
		t.Fatalf("checked %d kernel verbs, advances %d; want at least landing engine and none reached", checked, bed.advances)
	}
}

// landMain gives the lane a bare file:// origin whose main is the
// checkout's one commit, and returns that landed commit.
func (bed *kernelBed) landMain(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(filepath.Dir(bed.checkout), "origin.git")
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(filepath.Dir(bed.checkout), "init", "--quiet", "--bare", "-b", "main", origin)
	if err := os.WriteFile(filepath.Join(bed.checkout, ".gitignore"), []byte("metasystem/bin/\nmetasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(bed.checkout, "add", "-A")
	git(bed.checkout, "commit", "--quiet", "-m", "landed")
	git(bed.checkout, "remote", "add", "origin", "file://"+origin)
	git(bed.checkout, "push", "--quiet", "origin", "main")
	return git(bed.checkout, "rev-parse", "HEAD")
}

func init() {
	registerIdempotency("landing engine", idemStateful,
		"the lane's engine already is landed main's build with its enrolled bytes in place: success, nothing built, installed or re-armed", witnessLandingEngineAdvanceRepeat)
}

// witnessLandingEngineAdvanceRepeat runs landing engine advance twice with
// the production advance on a real lane whose enrolled engine (this test
// binary) is landed main's build: both runs are success, and the second
// leaves the lane checkout and the host home as they were.
func witnessLandingEngineAdvanceRepeat(t *testing.T) {
	bed := newKernelBed(t)
	main := bed.landMain(t)
	bed.enrollBuiltFrom(t, runningTestBinary(t), main)
	owners := bed.owners()
	owners.landing.advance = nil
	run := func() (int, string, string) {
		command, _ := findIntentAction("landing", "engine")
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, []string{"advance"}, &stdout, &stderr, bed.cwd, owners)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := run(); code != 0 || !strings.Contains(stdout, "already landed main's build") {
		t.Fatalf("first advance = %d %q %q", code, stdout, stderr)
	}
	before, home := idemTreeDigest(t, bed.checkout), idemTreeDigest(t, bed.home)
	if code, stdout, stderr := run(); code != 0 || !strings.Contains(stdout, "already landed main's build") {
		t.Fatalf("repeated advance = %d %q %q", code, stdout, stderr)
	}
	idemSameTree(t, "a repeated landing engine advance (checkout)", before, idemTreeDigest(t, bed.checkout))
	idemSameTree(t, "a repeated landing engine advance (home)", home, idemTreeDigest(t, bed.home))
}

// The kernel verb's layout goldens join G1b through the group hook.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, landingEngineLayoutCases)
	return true
}()

func landingEngineLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "landing-engine-advance", args: []string{"landing", "engine", "advance"}, bed: landingEngineLayoutBed(false)},
		{name: "landing-engine-refusal", args: []string{"landing", "engine", "advance"}, bed: landingEngineLayoutBed(true)},
	}
}

// landingEngineLayoutBed is the running lane's bed whose engine is admitted
// and already landed main's build, or refused as not the enrolled one.
func landingEngineLayoutBed(unenrolled bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := landingLayoutBed(landingLayoutRunning)(t)
		bed.owners.landing.engine = func(checkout, installation string, retry []string) (laneengine.Identity, error) {
			if unenrolled {
				return laneengine.Identity{}, &laneengine.Refusal{Code: laneengine.CodeNotEnrolled,
					Message: "this metasystem isn't the landing lane's enrolled engine, so nothing was done",
					Argv:    append([]string{filepath.Join(installation, "bin", "metasystem")}, retry...),
					Detail:  "running sha256:" + strings.Repeat("a", 64) + "; enrolled engine number 7 is sha256:" + strings.Repeat("b", 64)}
			}
			return laneengine.Identity{Installation: installation, Running: "sha256:" + strings.Repeat("b", 64)}, nil
		}
		bed.owners.landing.advance = func(laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error) {
			return laneengine.AdvanceOutcome{Commit: "4b825dc642cb6eb9a060e54bf8d69288fbee4904", PreviousGeneration: 7, Generation: 7}, nil
		}
		return bed
	}
}

// syncWriter is an output a test reads while the verb still writes it; seen
// closes once the output holds want.
type syncWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	want string
	seen chan struct{}
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if w.seen != nil && strings.Contains(w.buf.String(), w.want) {
		close(w.seen)
		w.seen = nil
	}
	return n, err
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// N-3: a landing stop that finds the host flock held (an engine advance
// re-arming) says so in one plain line before it waits, then stops the lane
// once the flock is free.
func TestLandingStopSaysWhyItWaitsForTheHostLock(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	held, err := lock.File(lane.LockPath(bed.home), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	command, _ := findIntentAction("landing", "stop")
	seen := make(chan struct{})
	var stdout syncWriter
	stderr := syncWriter{want: "the lane's engine is being changed; the stop takes effect when that finishes", seen: seen}
	done := make(chan int, 1)
	go func() {
		done <- runIntentIn(command, []string{"--by", "Wido"}, &stdout, &stderr, bed.cwd, bed.owners())
	}()
	select {
	case code := <-done:
		t.Fatalf("stop finished (%d) while the host lock was held, or waited silently: %q %q", code, stdout.String(), stderr.String())
	case <-seen:
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("stop after the lock was freed = %d %q", code, stderr.String())
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatal("the lane is not paused after the stop")
	}
}

// An advance the lane gate refuses (K-a) names the lane's own fix, never
// the engine-enrollment fix: a lane record an older engine wrote asks a
// person to register the lane again.
func TestLandingEngineRefusalNamesTheLaneFix(t *testing.T) {
	t.Parallel()
	refusal := &laneengine.Refusal{Code: lane.CodeRecordIncomplete,
		Message: "this computer's landing lane record for /lane names no installation or custody epoch (an older engine wrote it), so nothing was done",
		Argv:    []string{"metasystem", "landing", "set", "/lane"}, Detail: "a person registers the lane again: metasystem landing set /lane"}
	result := laneEngineResult(refusal, laneTargets("/lane"))
	if result.Outcome != intentRefused || result.nextReason != "a person registers the lane again" {
		t.Fatalf("lane refusal renders %+v; want the lane's fix", result)
	}
}
