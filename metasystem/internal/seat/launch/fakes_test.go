package launch

// The fakes every sequencer test is driven from.
//
// The sequencer opens nothing and waits for nothing: every question it asks
// about this host goes through Host, every subprocess through Runner, every
// presence read through Presence, and every wait through Clock. So these four
// are the whole of the world a test builds, and a test that asserts a command
// is asserting the command the real run would make.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// The paths every test's world is built from. They are names and not
// directories: nothing here touches a filesystem.
const (
	fromRoot    = "/w/agentic-tools"
	destRoot    = "/w/agentic-tools-m1f"
	install     = "metasystem"
	machineName = "m1f"
	originURL   = "https://github.com/widoriezebos/agentic-tools.git"
	humanWord   = "Wido says launch m1f on this host"
	reviewBy    = "2026-10-02"
	launchID    = "01M3BQAVYXE2AT6F0JG9YB64PG"
	headCommit  = "9c1f2ab3d4e5f60718293a4b5c6d7e8f90123456"
)

func fromBinary() string { return filepath.Join(fromRoot, install, "bin", "metasystem") }
func destBinary() string { return filepath.Join(destRoot, install, "bin", "metasystem") }
func destInstall() string {
	return filepath.Join(destRoot, install)
}

// commandKey is how a test names one command: the program's own name and its
// arguments, so an absolute path to an engine reads as "metasystem …".
func commandKey(command Command) string {
	return strings.TrimSpace(filepath.Base(command.Name) + " " + strings.Join(command.Args, " "))
}

type fakeRunner struct {
	ran []Command
	// said answers one command; absent is the empty answer.
	said map[string]string
	// refused is the owner's own words for a command that fails.
	refused map[string]string
}

func newRunner() *fakeRunner {
	return &fakeRunner{
		said: map[string]string{
			"git -C " + destRoot + " rev-parse HEAD":                                                                  headCommit + "\n",
			"git -C " + fromRoot + " config --local --list -z":                                                        "goal.sync-remote\norigin\x00goal.human.wido\nWido <wido@example.invalid>\x00user.name\nfixture\x00",
			"metasystem config get --key evidence.root --conf " + filepath.Join(fromRoot, install, "metasystem.conf"): "/w/evidence/m1u\n",
			"metasystem goal next --root " + destRoot:                                                                 "g1-s44 is claimable here\n",
		},
		refused: map[string]string{
			// An unset key is what git answers with an exit code, which is
			// the precondition the machineName step reads.
			"git -C " + destRoot + " config --get metasystem.goal.machine": "",
		},
	}
}

func (r *fakeRunner) Run(command Command) (string, error) {
	r.ran = append(r.ran, command)
	key := commandKey(command)
	if words, refused := r.refused[key]; refused {
		if words == "" {
			words = "exit status 1"
		}
		return "", fmt.Errorf("%s", words)
	}
	return r.said[key], nil
}

// keys is every command this runner was asked to run, in order.
func (r *fakeRunner) keys() []string {
	keys := make([]string, 0, len(r.ran))
	for _, command := range r.ran {
		keys = append(keys, commandKey(command))
	}
	return keys
}

func (r *fakeRunner) ranCommand(key string) bool {
	for _, held := range r.keys() {
		if held == key {
			return true
		}
	}
	return false
}

type copiedConf struct{ source, destRoot, evidenceRoot string }

type fakeHost struct {
	exists      map[string]bool
	canonical   map[string]string
	present     map[string]bool
	stamp       map[string]string
	enrolled    bool
	supervision bool
	free        uint64
	size        uint64
	copied      []copiedConf
	manifests   []string
	madeDirs    []string
}

func newHost() *fakeHost {
	return &fakeHost{
		exists:    map[string]bool{},
		canonical: map[string]string{"/w/evidence/m1u": "/w/evidence/m1u", "/w/evidence/m1f": "/w/evidence/m1f"},
		present:   map[string]bool{},
		stamp:     map[string]string{destBinary(): headCommit},
		free:      1 << 40,
		size:      1 << 30,
	}
}

func (h *fakeHost) Exists(path string) (bool, error) { return h.exists[path], nil }

func (h *fakeHost) Canonical(path string) (string, error) {
	if resolved, known := h.canonical[path]; known {
		return resolved, nil
	}
	return "", fmt.Errorf("no such path %s", path)
}

func (h *fakeHost) MakeDir(path string) (bool, error) {
	h.madeDirs = append(h.madeDirs, path)
	if h.present[path] {
		return false, nil
	}
	h.present[path] = true
	return true, nil
}

func (h *fakeHost) CopyLocalConf(source, target, evidenceRoot string) error {
	h.copied = append(h.copied, copiedConf{source: source, destRoot: target, evidenceRoot: evidenceRoot})
	return nil
}

func (h *fakeHost) MakeManifest(path string) error {
	h.manifests = append(h.manifests, path)
	return nil
}

func (h *fakeHost) Stamp(binary string) (string, error) { return h.stamp[binary], nil }

func (h *fakeHost) Enrolled(string) bool { return h.enrolled }

func (h *fakeHost) SupervisionAlive(string) bool { return h.supervision }

func (h *fakeHost) Free(string) (uint64, error) { return h.free, nil }

func (h *fakeHost) Size(string) (uint64, error) { return h.size, nil }

type fakePresence struct {
	// appearsAt is the look at which a record for the machine is read; a
	// negative number is a machine that never publishes.
	appearsAt int
	looks     int
	forgotten []string
	problem   error
}

func (p *fakePresence) Look(_, _ string) (bool, error) {
	look := p.looks
	p.looks++
	if p.problem != nil {
		return false, p.problem
	}
	return p.appearsAt >= 0 && look >= p.appearsAt, nil
}

func (p *fakePresence) Forget(namespace string) error {
	p.forgotten = append(p.forgotten, namespace)
	return nil
}

// fakeClock never waits: a tick returns at once, and the clock moves by the
// duration that was asked for, so the presence deadline is counted rather
// than elapsed.
type fakeClock struct {
	now    time.Time
	waited []time.Duration
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) After(wait time.Duration) <-chan time.Time {
	c.waited = append(c.waited, wait)
	c.now = c.now.Add(wait)
	ticked := make(chan time.Time, 1)
	ticked <- c.now
	return ticked
}

// world is one sequencer with its four fakes, and the records it wrote.
type world struct {
	sequencer *Sequencer
	runner    *fakeRunner
	host      *fakeHost
	presence  *fakePresence
	clock     *fakeClock
	written   []Record
}

func newWorld(request Request) *world {
	runner, host := newRunner(), newHost()
	presence, clock := &fakePresence{appearsAt: 0}, newClock()
	built := &world{runner: runner, host: host, presence: presence, clock: clock}
	built.sequencer = &Sequencer{
		Request: request, Installation: install, OriginURL: originURL,
		Host: host, Runner: runner, Presence: presence, Clock: clock,
		Write:     func(record Record) error { built.written = append(built.written, record); return nil },
		Namespace: "refs/metasystem/presence-fetch/" + launchID,
		GitBudget: 60 * time.Second, BuildBudget: 10 * time.Minute,
		PresenceTick: 10 * time.Minute, PresenceTicks: 3,
	}
	return built
}

func request() Request {
	return Request{
		Machine: machineName, From: fromRoot, Destination: destRoot,
		Word: humanWord, ReviewBy: reviewBy,
	}
}

func fresh() Record {
	return Record{SchemaVersion: SchemaVersion, Launch: launchID, Outcome: OutcomeRunning}
}
