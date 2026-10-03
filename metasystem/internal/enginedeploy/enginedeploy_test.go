package enginedeploy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const (
	oldCommit = "1111111111111111111111111111111111111111"
	newCommit = "2222222222222222222222222222222222222222"
)

// engine is a test's engine: a small shell program carrying the stamp
// record of commit. Run with help, it marks that it ran beside itself and
// exits with exit.
func engine(commit string, exit int) []byte {
	return []byte("#!/bin/sh\n# " + enginebuild.StampRecord(commit) + "\n[ \"$1\" = help ] || exit 9\n: > \"$0.started\"\nexit " + strconv.Itoa(exit) + "\n")
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// world is one home directory whose builds write the engine next names.
type world struct {
	t       *testing.T
	engines Engines
	next    []byte
	fail    error
	builds  int
}

func newWorld(t *testing.T) *world {
	w := &world{t: t}
	w.engines = Engines{Home: t.TempDir(), Build: func(out string) error {
		w.builds++
		if w.fail != nil {
			return w.fail
		}
		return testexec.WriteFile(out, w.next, 0o755)
	}}
	return w
}

func (w *world) serve(operation string, request deploy.Request) (int, deploy.Response) {
	w.t.Helper()
	request.Schema, request.Operation = 1, operation
	body, err := json.Marshal(request)
	if err != nil {
		w.t.Fatal(err)
	}
	return w.serveRaw(operation, body)
}

func (w *world) serveRaw(operation string, body []byte) (int, deploy.Response) {
	w.t.Helper()
	var out bytes.Buffer
	code := w.engines.Serve(operation, bytes.NewReader(body), &out)
	var response deploy.Response
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			w.t.Fatalf("response %q: %v", out.String(), err)
		}
	}
	return code, response
}

// build builds commit's engine from data and requires it built.
func (w *world) build(commit string, data []byte) deploy.Response {
	w.t.Helper()
	w.next = data
	code, built := w.serve("build", deploy.Request{Commit: commit})
	if code != ExitDone || built.Outcome != "built" {
		w.t.Fatalf("build %s = %d %+v", commit, code, built)
	}
	return built
}

func (w *world) activate(artifact string) {
	w.t.Helper()
	if code, active := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: artifact}); code != ExitDone || active.Outcome != "active" {
		w.t.Fatalf("activate %s = %d %+v", artifact, code, active)
	}
}

func (w *world) version() deploy.Response {
	w.t.Helper()
	code, active := w.serve("version", deploy.Request{})
	if code != ExitDone {
		w.t.Fatalf("version = %d %+v", code, active)
	}
	return active
}

// failed requires a failed answer whose reason says want.
func failed(t *testing.T, code int, response deploy.Response, want string) {
	t.Helper()
	if code != ExitFailed || response.Outcome != "failed" || !strings.Contains(response.Reason, want) {
		t.Fatalf("answer = %d %+v, want failed saying %q", code, response, want)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	found, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range found {
		names = append(names, entry.Name())
	}
	return names
}

func TestBuildPlacesTheStartedEngineAndLeavesThePointer(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	data := engine(newCommit, 0)
	built := w.build(newCommit, data)
	if built.Version != newCommit || built.Artifact != w.engines.Engine(newCommit) || built.Digest != digestOf(data) {
		t.Fatalf("built = %+v", built)
	}
	if _, err := os.Stat(built.Artifact + ".started"); err != nil {
		t.Fatalf("the engine was not run with help before the build answered: %v", err)
	}
	if names := entries(t, filepath.Join(w.engines.Home, "engines")); len(names) != 1 || names[0] != newCommit {
		t.Fatalf("engines = %q, want only the commit's directory", names)
	}
	if active := w.version(); active.Outcome != "none" {
		t.Fatalf("version after a build = %+v, want none: a build never changes what is active", active)
	}
}

func TestBuildReusesAnEngineDirectoryWithTheSameChecksum(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	data := engine(newCommit, 0)
	first := w.build(newCommit, data)
	before, err := os.Stat(first.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	again := w.build(newCommit, data)
	after, err := os.Stat(again.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if again != first || !os.SameFile(before, after) || w.builds != 2 {
		t.Fatalf("second build = %+v (same file %v), want the first %+v reused", again, os.SameFile(before, after), first)
	}
	if names := entries(t, filepath.Join(w.engines.Home, "engines")); len(names) != 1 {
		t.Fatalf("engines = %q, want the staging directory gone", names)
	}
}

func TestBuildRefusesAnEngineDirectoryWithAnotherChecksum(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	first := w.build(newCommit, engine(newCommit, 0))
	w.next = append(engine(newCommit, 0), "# rebuilt\n"...)
	code, response := w.serve("build", deploy.Request{Commit: newCommit})
	failed(t, code, response, "never rewritten")
	if data, err := os.ReadFile(first.Artifact); err != nil || digestOf(data) != first.Digest {
		t.Fatalf("the engine directory was rewritten: %v", err)
	}
	// A directory of the commit whose engine is gone is not filled again.
	dir := filepath.Dir(w.engines.Engine(oldCommit))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "link"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w.next = engine(oldCommit, 0)
	code, response = w.serve("build", deploy.Request{Commit: oldCommit})
	failed(t, code, response, "its engine can't be read")
	if names := entries(t, dir); len(names) != 1 {
		t.Fatalf("%s = %q, want it left as it was", dir, names)
	}
}

func TestBuildRefusesAnEngineWhoseStampIsNotTheCommit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	old := w.build(oldCommit, engine(oldCommit, 0))
	w.activate(old.Artifact)
	for name, data := range map[string][]byte{"another commit": engine(oldCommit, 0), "no stamp": []byte("#!/bin/sh\nexit 0\n")} {
		w.next = data
		code, response := w.serve("build", deploy.Request{Commit: newCommit})
		failed(t, code, response, "not its commit")
		if _, err := os.Lstat(filepath.Dir(w.engines.Engine(newCommit))); !os.IsNotExist(err) {
			t.Fatalf("%s: the refused engine was placed: %v", name, err)
		}
	}
	if active := w.version(); active.Artifact != old.Artifact {
		t.Fatalf("active = %+v, want the old engine still", active)
	}
	if names := entries(t, filepath.Join(w.engines.Home, "engines")); len(names) != 1 {
		t.Fatalf("engines = %q, want no staging directory left", names)
	}
}

func TestBuildRefusesAnEngineThatDoesNotStart(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.next = engine(newCommit, 3)
	code, response := w.serve("build", deploy.Request{Commit: newCommit})
	failed(t, code, response, "does not start")
	if names := entries(t, filepath.Join(w.engines.Home, "engines")); len(names) != 0 {
		t.Fatalf("engines = %q, want nothing placed", names)
	}
}

func TestBuildRefusesAFailedBuildAndANameThatIsNoCommit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.fail = errors.New("compile error")
	code, response := w.serve("build", deploy.Request{Commit: newCommit})
	failed(t, code, response, "compile error")
	code, response = w.serve("build", deploy.Request{Commit: "../" + newCommit[3:]})
	failed(t, code, response, "not a full commit name")
	if w.builds != 1 {
		t.Fatalf("builds = %d, want none for a name that is no commit", w.builds)
	}
}

func TestBuildClearsTheStagingOfEndedBuilds(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	dir := filepath.Join(w.engines.Home, "engines")
	// No process has this pid on Linux or macOS; this test's process is alive.
	ended := oldCommit + stagingMark + "2147483646"
	running := oldCommit + stagingMark + strconv.Itoa(os.Getpid())
	for _, name := range []string{ended, running} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.build(newCommit, engine(newCommit, 0))
	if names := entries(t, dir); strings.Join(names, " ") != running+" "+newCommit {
		t.Fatalf("engines = %q, want the ended build's staging removed and the running one's kept", names)
	}
}

func TestActivateClearsTheNewNamesOfEndedActivations(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	built := w.build(newCommit, engine(newCommit, 0))
	bin := filepath.Dir(w.engines.Pointer())
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// No process has this pid on Linux or macOS; the go command that runs
	// this test is alive.
	ended, running := linkMark+"2147483646", linkMark+strconv.Itoa(os.Getppid())
	for _, name := range []string{ended, running} {
		if err := os.Symlink(built.Artifact, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	w.activate(built.Artifact)
	if names := entries(t, bin); strings.Join(names, " ") != running+" metasystem" {
		t.Fatalf("bin = %q, want the ended activation's name removed and the running one's kept", names)
	}
}

// A reader resolving the pointer in a loop while it is swapped back and
// forth always finds a complete old or new engine, never no engine.
func TestActivateSwapsThePointerUnderAConcurrentReader(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	oldData, newData := engine(oldCommit, 0), engine(newCommit, 0)
	old, fresh := w.build(oldCommit, oldData), w.build(newCommit, newData)
	w.activate(old.Artifact)
	var reads atomic.Int64
	stop := make(chan struct{})
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			target, err := os.Readlink(w.engines.Pointer())
			if err != nil || target != old.Artifact && target != fresh.Artifact {
				t.Errorf("the pointer names %q, %v; want the old or the new engine", target, err)
				return
			}
			data, err := os.ReadFile(w.engines.Pointer())
			if err != nil || !bytes.Equal(data, oldData) && !bytes.Equal(data, newData) {
				t.Errorf("read through the pointer: %q, %v; want a complete old or new engine", data, err)
				return
			}
			reads.Add(1)
		}
	}()
	for swaps := 0; swaps < 200 || reads.Load() < 200; swaps++ {
		if t.Failed() {
			break
		}
		artifact := fresh.Artifact
		if swaps%2 == 1 {
			artifact = old.Artifact
		}
		code, response := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: artifact})
		if code != ExitDone {
			t.Errorf("swap %d = %d %+v", swaps, code, response)
			break
		}
	}
	close(stop)
	group.Wait()
	if names := entries(t, filepath.Join(w.engines.Home, "bin")); len(names) != 1 {
		t.Fatalf("bin = %q, want only the pointer", names)
	}
}

func TestActivateRepeatedPointsAtTheSameEngineAndVersionReportsIt(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	built := w.build(newCommit, engine(newCommit, 0))
	// The second activation spells the engine another way: the pointer
	// still names the path its build answered.
	for _, artifact := range []string{built.Artifact, filepath.Dir(built.Artifact) + "/./metasystem"} {
		code, active := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: artifact})
		if code != ExitDone || active != (deploy.Response{Outcome: "active", Version: newCommit}) {
			t.Fatalf("activate = %d %+v", code, active)
		}
	}
	want := deploy.Response{Outcome: "active", Version: newCommit, Artifact: built.Artifact, Digest: built.Digest}
	if active := w.version(); active != want {
		t.Fatalf("version = %+v, want %+v", active, want)
	}
	// The pointer is a name of the engine's own link, which outlives it.
	pointer, err := os.Lstat(w.engines.Pointer())
	if err != nil {
		t.Fatal(err)
	}
	if own, err := os.Lstat(filepath.Join(filepath.Dir(built.Artifact), "link")); err != nil || !os.SameFile(pointer, own) {
		t.Fatalf("the pointer is not a name of the engine's link: %v", err)
	}
	if names := entries(t, filepath.Join(w.engines.Home, "bin")); len(names) != 1 {
		t.Fatalf("bin = %q, want only the pointer after a repeated activation", names)
	}
}

func TestActivateRefusesAnEngineWhoseLinkNamesAnotherEngine(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	old := w.build(oldCommit, engine(oldCommit, 0))
	fresh := w.build(newCommit, engine(newCommit, 0))
	if err := os.Symlink(old.Artifact, filepath.Join(filepath.Dir(fresh.Artifact), "link")); err != nil {
		t.Fatal(err)
	}
	code, response := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: fresh.Artifact})
	failed(t, code, response, "not its engine")
	if active := w.version(); active.Outcome != "none" {
		t.Fatalf("version = %+v, want nothing activated", active)
	}
}

func TestActivateRefusesWhatIsNoEngineOfTheEnginesDirectory(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	outside := filepath.Join(w.engines.Home, newCommit, "metasystem")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(outside, engine(newCommit, 0), 0o755); err != nil {
		t.Fatal(err)
	}
	code, response := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: outside})
	failed(t, code, response, "is not an engine of")
	code, response = w.serve("activate", deploy.Request{Commit: newCommit, Artifact: w.engines.Engine(newCommit)})
	failed(t, code, response, "can't be activated")
	if active := w.version(); active.Outcome != "none" {
		t.Fatalf("version = %+v, want nothing activated", active)
	}
}

func TestActivateLeavesAPointerThatIsNoSymbolicLink(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	built := w.build(newCommit, engine(newCommit, 0))
	if err := os.MkdirAll(filepath.Dir(w.engines.Pointer()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.engines.Pointer(), []byte("a person's file"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, response := w.serve("activate", deploy.Request{Commit: newCommit, Artifact: built.Artifact})
	failed(t, code, response, "is not the symbolic link")
	code, response = w.serve("version", deploy.Request{})
	failed(t, code, response, "can't be read as a symbolic link")
	if data, _ := os.ReadFile(w.engines.Pointer()); string(data) != "a person's file" {
		t.Fatalf("pointer = %q, want it left as it was", data)
	}
}

func TestVersionResolvesARelativePointerAndFailsOnAMissingEngine(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	built := w.build(newCommit, engine(newCommit, 0))
	if err := os.MkdirAll(filepath.Dir(w.engines.Pointer()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "engines", newCommit, "metasystem"), w.engines.Pointer()); err != nil {
		t.Fatal(err)
	}
	if active := w.version(); active.Artifact != built.Artifact || active.Digest != built.Digest {
		t.Fatalf("version = %+v, want the built engine", active)
	}
	if err := os.RemoveAll(filepath.Dir(built.Artifact)); err != nil {
		t.Fatal(err)
	}
	code, response := w.serve("version", deploy.Request{})
	failed(t, code, response, "which can't be read")
}

func TestRollbackActivatesThePreviousEngine(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	old := w.build(oldCommit, engine(oldCommit, 0))
	w.activate(w.build(newCommit, engine(newCommit, 0)).Artifact)
	previous := &deploy.Deployed{Commit: oldCommit, Version: old.Version, Artifact: old.Artifact, Digest: old.Digest}
	code, back := w.serve("rollback", deploy.Request{Commit: oldCommit, Previous: previous})
	if code != ExitDone || back != (deploy.Response{Outcome: "active", Version: oldCommit}) {
		t.Fatalf("rollback = %d %+v", code, back)
	}
	if active := w.version(); active.Artifact != old.Artifact || active.Digest != old.Digest {
		t.Fatalf("version after the rollback = %+v, want the old engine", active)
	}
}

func TestRollbackRefusesAPreviousEngineThatIsGoneOrChanged(t *testing.T) {
	t.Parallel()
	for _, refusal := range []struct {
		name   string
		damage func(t *testing.T, old deploy.Response) *deploy.Deployed
		want   string
	}{
		{"no previous", func(*testing.T, deploy.Response) *deploy.Deployed { return nil }, "nothing to roll back to"},
		{"gone", func(t *testing.T, old deploy.Response) *deploy.Deployed {
			if err := os.RemoveAll(filepath.Dir(old.Artifact)); err != nil {
				t.Fatal(err)
			}
			return &deploy.Deployed{Commit: oldCommit, Artifact: old.Artifact, Digest: old.Digest}
		}, "can't be activated"},
		{"another checksum", func(_ *testing.T, old deploy.Response) *deploy.Deployed {
			return &deploy.Deployed{Commit: oldCommit, Artifact: old.Artifact, Digest: digestOf(nil)}
		}, "not the " + digestOf(nil)},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			old := w.build(oldCommit, engine(oldCommit, 0))
			fresh := w.build(newCommit, engine(newCommit, 0))
			w.activate(fresh.Artifact)
			code, response := w.serve("rollback", deploy.Request{Commit: oldCommit, Previous: refusal.damage(t, old)})
			failed(t, code, response, refusal.want)
			if active := w.version(); active.Artifact != fresh.Artifact {
				t.Fatalf("active = %+v, want the current engine kept", active)
			}
		})
	}
}

func TestServeAnswersTheContractsExitMeanings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	code, response := w.serve("promote", deploy.Request{})
	if code != ExitUnsupported || response.Outcome != "failed" || !strings.Contains(response.Reason, `does not support the operation "promote"`) {
		t.Fatalf("an unknown operation = %d %+v, want exit 64 and a failed answer", code, response)
	}
	code, response = w.serveRaw("version", []byte("{"))
	failed(t, code, response, "can't be read")
	code, response = w.serveRaw("version", []byte(`{"schema":2,"operation":"version"}`))
	failed(t, code, response, "not schema 1")
	code, response = w.serveRaw("build", []byte(`{"schema":1,"operation":"version"}`))
	failed(t, code, response, `not schema 1 for "build"`)
	if w.builds != 0 {
		t.Fatalf("builds = %d, want none for a refused request", w.builds)
	}
}
