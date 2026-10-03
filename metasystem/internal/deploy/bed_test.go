package deploy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

var (
	fixtureOnce sync.Once
	fixtureDir  string
	fixturePath string
	fixtureErr  error
)

// fixtureAdapter is testdata/fixtureadapter built once for the package.
func fixtureAdapter(t *testing.T) string {
	t.Helper()
	fixtureOnce.Do(func() {
		if fixtureDir, fixtureErr = os.MkdirTemp("", "deploy-fixture-"); fixtureErr != nil {
			return
		}
		fixturePath = filepath.Join(fixtureDir, "fixtureadapter")
		if out, err := testenv.Go("build", "-o", fixturePath, "./testdata/fixtureadapter").CombinedOutput(); err != nil {
			fixtureErr = fmt.Errorf("build the fixture adapter: %v\n%s", err, out)
		}
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return fixturePath
}

// bed is one project on one computer: its deploy directory, the fixture
// adapter's state, and origin's main as each fetch finds it.
type bed struct {
	t       *testing.T
	base    string
	dir     string
	state   string
	adapter string
	// adapters names, per commit, the adapter its tree holds and its
	// deploy.json calls; any other commit's names the fixture by its path.
	adapters map[string]string

	mu      sync.Mutex
	tips    []string
	fetches int
	clock   time.Time
}

func newBed(t *testing.T) *bed {
	t.Helper()
	base := t.TempDir()
	b := &bed{t: t, base: base, dir: Dir(filepath.Join(base, "home"), "fixture"), state: filepath.Join(base, "adapter"),
		adapter: fixtureAdapter(t), clock: time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)}
	if err := os.MkdirAll(filepath.Join(b.state, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	return b
}

// main sets what the next fetches of origin's main find, one tip per
// fetch; the last one stays.
func (b *bed) main(tips ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tips = tips
}

func (b *bed) fetch() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fetches++
	if len(b.tips) == 0 {
		return "", fmt.Errorf("origin has no main")
	}
	tip := b.tips[0]
	if len(b.tips) > 1 {
		b.tips = b.tips[1:]
	}
	return tip, nil
}

func (b *bed) fetchCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fetches
}

// child starts act ("run" or "rollback") in a runner process of its own
// whose fetches of main find tip.
func (b *bed) child(act, tip string) *exec.Cmd {
	b.t.Helper()
	config, err := json.Marshal(childConfig{Dir: b.dir, Argv: []string{b.adapter, b.state}, Tip: tip, Act: act})
	if err != nil {
		b.t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		b.t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^$")
	child.Env = append(os.Environ(), runnerChildEnv+"="+string(config))
	if err := child.Start(); err != nil {
		b.t.Fatal(err)
	}
	return child
}

// inFlight waits until the child's adapter runs operation, its process
// started, and what is active names commit's artifact (or anything, for
// ""), and returns it.
func (b *bed) inFlight(operation, commit string) Active {
	b.t.Helper()
	var active Active
	testenv.Await(b.t, operation+" in flight", func() bool {
		found, ok, _ := ReadActive(b.dir)
		active = found
		return ok && found.PID > 0 && found.Operation == operation && (commit == "" || strings.Contains(b.file(filepath.Join(b.state, "active.json")), b.artifact(commit)))
	})
	return active
}

// crash ends the child runner; with adapter, its adapter's process group
// too, as a computer that loses its power would.
func (b *bed) crash(child *exec.Cmd, active Active, adapter bool) {
	b.t.Helper()
	_ = syscall.Kill(child.Process.Pid, syscall.SIGKILL)
	_ = child.Wait()
	if adapter {
		_ = syscall.Kill(-active.PID, syscall.SIGKILL)
		if err := testenv.AwaitProcessTargetGone(-active.PID); err != nil {
			b.t.Fatal(err)
		}
	}
}

func (b *bed) now() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clock = b.clock.Add(time.Second)
	return b.clock
}

// addTree makes commit's clean tree with the deploy.json it holds.
func (b *bed) addTree(dir, commit string) error {
	name := b.adapters[commit]
	if name == "" {
		return fakeTree([]string{b.adapter, b.state})(dir, commit)
	}
	if err := fakeTree([]string{"./" + name, b.state})(dir, commit); err != nil {
		return err
	}
	return os.Symlink(b.adapter, filepath.Join(dir, name))
}

// runner is the runner a trigger by `by` starts; started, when not nil,
// hears each adapter process it starts.
func (b *bed) runner(by string, started chan<- Active) *Runner {
	runner := &Runner{Dir: b.dir, Project: "fixture", Installation: ".", By: by, Now: b.now,
		Git: Git{FetchMain: b.fetch, AddTree: b.addTree, RemoveTree: os.RemoveAll}}
	if started != nil {
		runner.Started = func(active Active) { started <- active }
	}
	return runner
}

func (b *bed) run(by string) Report {
	b.t.Helper()
	report, err := b.runner(by, nil).Run()
	if err != nil {
		b.t.Fatalf("run: %v", err)
	}
	return report
}

// deployed runs the runner once on tip and requires it active.
func (b *bed) deployed(tip string) Line {
	b.t.Helper()
	b.main(tip)
	report := b.run("lane")
	if report.Outcome != RunDeployed || len(report.Lines) == 0 || report.Lines[len(report.Lines)-1].Commit != tip {
		b.t.Fatalf("deploying %s = %+v", tip, report)
	}
	return report.Lines[len(report.Lines)-1]
}

// script makes the next calls of operation follow steps, from the first.
func (b *bed) script(operation string, steps ...step) {
	b.t.Helper()
	data, err := json.Marshal(steps)
	if err != nil {
		b.t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(b.state, "script", operation+".count"))
	if err := os.WriteFile(filepath.Join(b.state, "script", operation+".json"), data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// step is one scripted adapter call (testdata/fixtureadapter).
type step struct {
	Exit       int    `json:"exit,omitempty"`
	Apply      bool   `json:"apply,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Raw        string `json:"raw,omitempty"`
	Artifact   string `json:"artifact,omitempty"`
	HoldBefore string `json:"holdBefore,omitempty"`
	HoldAfter  string `json:"holdAfter,omitempty"`
}

// calls are the adapter calls made so far, as "operation commit".
func (b *bed) calls() []string {
	b.t.Helper()
	file, err := os.Open(filepath.Join(b.state, "calls"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		b.t.Fatal(err)
	}
	defer file.Close()
	var calls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}
	return calls
}

func (b *bed) count(call string) int {
	count := 0
	for _, made := range b.calls() {
		if made == call {
			count++
		}
	}
	return count
}

// active is the artifact the fixture adapter holds active, "" for none.
func (b *bed) active() string {
	b.t.Helper()
	var active deployed
	if _, err := readJSON(filepath.Join(b.state, "active.json"), &active); err != nil {
		b.t.Fatal(err)
	}
	return active.Artifact
}

type deployed struct {
	Artifact string `json:"artifact"`
}

func (b *bed) artifact(commit string) string { return filepath.Join(b.state, "artifacts", commit) }

func (b *bed) lines() []Line {
	b.t.Helper()
	lines, err := Lines(b.dir)
	if err != nil {
		b.t.Fatal(err)
	}
	return lines
}

// file is a file's bytes, "" when it does not exist.
func (b *bed) file(path string) string {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		b.t.Fatal(err)
	}
	return string(data)
}

// fifo is a FIFO a scripted call holds on until release.
func (b *bed) fifo(name string) string {
	b.t.Helper()
	path := filepath.Join(b.base, name+".fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		b.t.Fatal(err)
	}
	return path
}

// release lets a call held on fifo go on: it opens the FIFO for writing,
// which waits for the call to open it, and closes it.
func (b *bed) release(fifo string) {
	b.t.Helper()
	writer, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		b.t.Fatal(err)
	}
}

// releaseIfHeld lets go a call still held on fifo, for a test that ends
// before its own release; with no call holding it, it does nothing.
func (b *bed) releaseIfHeld(fifo string) {
	if writer, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
		_ = writer.Close()
	}
}

// awaitStart waits for the adapter call a runner starts and requires it to
// be operation on commit.
func awaitStart(t *testing.T, started <-chan Active, operation, commit string) Active {
	t.Helper()
	for active := range started {
		if active.Operation == operation && active.Commit == commit {
			return active
		}
	}
	t.Fatalf("the run ended before it started %s of %s", operation, commit)
	return Active{}
}

func outcomes(lines []Line) string {
	var words []string
	for _, line := range lines {
		words = append(words, line.Kind+":"+line.Commit+":"+line.Outcome)
	}
	return strings.Join(words, " ")
}
