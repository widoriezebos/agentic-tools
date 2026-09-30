package landpath

// The test harness of the landing path: every test builds its own fake Git
// and its own owners. Git is scripted, never emulated: a test states the
// answers the landing needs (the staged tree, the branch, the push result)
// and the harness records every call, so a test asserts the order of the
// landing's effects. Nothing here sleeps, reads a clock, or shares state
// between tests.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// gitAnswer answers one git call; ok false passes it to the next answer.
type gitAnswer func(call GitCall) (GitResult, bool)

// fakeGit is one test's scripted Git.
type fakeGit struct {
	t       *testing.T
	calls   []GitCall
	answers []gitAnswer
	// state the default answers read and the commit answer advances.
	head, tree, branch, prefix, toplevel, machine string
	originHead                                    string
	message                                       string
	commits                                       int
	stagedEmpty                                   bool
}

func newFakeGit(t *testing.T, root string) *fakeGit {
	return &fakeGit{t: t, head: "h0", tree: "t1", branch: "main", toplevel: root, machine: "m1", originHead: "h0"}
}

func ok(stdout string) GitResult { return GitResult{Stdout: []byte(stdout)} }

func failed(code int, stderr string) GitResult { return GitResult{Code: code, Stderr: []byte(stderr)} }

// on adds an answer for calls whose arguments start with prefix; answers
// added later win.
func (g *fakeGit) on(prefix string, answer func(call GitCall) GitResult) {
	words := strings.Fields(prefix)
	g.answers = append([]gitAnswer{func(call GitCall) (GitResult, bool) {
		if len(call.Args) < len(words) {
			return GitResult{}, false
		}
		for i, word := range words {
			if call.Args[i] != word {
				return GitResult{}, false
			}
		}
		return answer(call), true
	}}, g.answers...)
}

// run is the Git owner.
func (g *fakeGit) run(call GitCall) GitResult {
	g.calls = append(g.calls, call)
	for _, answer := range g.answers {
		if result, handled := answer(call); handled {
			return result
		}
	}
	return g.defaults(call)
}

func (g *fakeGit) defaults(call GitCall) GitResult {
	args := strings.Join(call.Args, " ")
	switch {
	case args == "rev-parse --show-prefix":
		return ok(g.prefix + "\n")
	case args == "rev-parse --show-toplevel":
		return ok(g.toplevel + "\n")
	case args == "write-tree":
		return ok(g.tree + "\n")
	case args == "rev-parse --git-path index":
		// No index file: the landing holds nothing to give back.
		return ok(filepath.Join(g.toplevel, "absent-index") + "\n")
	case args == "config --get metasystem.goal.machine":
		return ok(g.machine + "\n")
	case args == "rev-parse --verify --quiet HEAD", args == "rev-parse HEAD", args == "rev-parse HEAD^{commit}":
		return ok(g.head + "\n")
	case args == "rev-parse HEAD^{tree}":
		return ok(g.tree + "\n")
	case args == "rev-parse --short HEAD":
		return ok(g.head + "\n")
	case args == "symbolic-ref --quiet --short HEAD", args == "symbolic-ref --short HEAD":
		if g.branch == "" {
			return failed(1, "")
		}
		return ok(g.branch + "\n")
	case strings.HasPrefix(args, "rev-parse refs/remotes/origin/"):
		return ok(g.originHead + "\n")
	case args == "diff --cached --quiet --":
		if g.stagedEmpty {
			return ok("")
		}
		return failed(1, "")
	case strings.HasPrefix(args, "commit "):
		return g.commit(call)
	case args == "log -1 --format=%B":
		return ok(g.message + "\n")
	case strings.HasPrefix(args, "log -1 --format=%B "):
		return ok(g.message + "\n")
	case strings.HasPrefix(args, "diff "), strings.HasPrefix(args, "ls-files "), strings.HasPrefix(args, "add "),
		strings.HasPrefix(args, "fetch "), strings.HasPrefix(args, "push "), strings.HasPrefix(args, "show "),
		strings.HasPrefix(args, "reset "), strings.HasPrefix(args, "update-ref "), args == "remote":
		return ok("")
	}
	g.t.Fatalf("unscripted git call: %s", args)
	return GitResult{}
}

// commit records a commit whose message is the -F file and the stamped
// trailers, as git would record it.
func (g *fakeGit) commit(call GitCall) GitResult {
	var trailers []string
	message := ""
	for i := 1; i < len(call.Args); i++ {
		switch call.Args[i] {
		case "--trailer":
			trailers = append(trailers, call.Args[i+1])
			i++
		case "-F":
			data, err := os.ReadFile(call.Args[i+1])
			if err != nil {
				g.t.Fatalf("commit message: %v", err)
			}
			message = strings.TrimRight(string(data), "\n")
			i++
		}
	}
	g.commits++
	g.head = fmt.Sprintf("c%d", g.commits)
	g.message = message + "\n\n" + strings.Join(trailers, "\n")
	return ok(fmt.Sprintf("[%s %s] %s\n", g.branch, g.head, strings.SplitN(message, "\n", 2)[0]))
}

// called reports the calls whose arguments start with prefix.
func (g *fakeGit) called(prefix string) []GitCall {
	var matched []GitCall
	for _, call := range g.calls {
		if strings.HasPrefix(strings.Join(call.Args, " "), prefix) {
			matched = append(matched, call)
		}
	}
	return matched
}

// ownerLog records the owner calls a test asserts.
type ownerLog struct {
	calls []string
}

func (l *ownerLog) add(format string, args ...any) {
	l.calls = append(l.calls, fmt.Sprintf(format, args...))
}

func (l *ownerLog) has(prefix string) bool {
	for _, call := range l.calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

func (l *ownerLog) count(prefix string) int {
	count := 0
	for _, call := range l.calls {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}
	return count
}

// passingObservation is an evaluator's pass for a reviewed chain.
func passingObservation() landing.Observation {
	return landing.Observation{Mode: "observe", Code: "reviewed-chain", Provenance: "chain=j1 change=abc",
		VerdictTrailer: "pass bar=area", GoalRevision: 7}
}

// bed is one test's landing: a root with a message file, its fake Git, and
// owners that succeed unless the test replaces one.
type bed struct {
	t        *testing.T
	root     string
	git      *fakeGit
	log      *ownerLog
	owners   Owners
	observed landing.Observation
	epoch    *int64
	// messagePath replaces the default message file when set.
	messagePath string
	stdout      bytes.Buffer
	stderr      bytes.Buffer
}

func newBed(t *testing.T) *bed {
	t.Helper()
	root := t.TempDir()
	b := &bed{t: t, root: root, git: newFakeGit(t, root), log: &ownerLog{}, observed: passingObservation()}
	if err := os.WriteFile(filepath.Join(root, "message"), []byte("land the change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.owners = Owners{
		Git:        b.git.run,
		CallerPID:  100,
		Getpid:     func() int64 { return 100 },
		LiveEngine: func() (string, error) { return filepath.Join(root, "message"), nil },
		Now:        func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) },
		Environ:    func() []string { return []string{"PATH=/usr/bin:/bin"} },
		RequireHolder: func(_ string, _ int64, epoch *int64) (*int64, error) {
			b.log.add("require-holder epoch=%v", epoch != nil)
			return b.epoch, nil
		},
		WithHeld: func(_ string, _ int64, epoch *int64, fn func() error) error {
			b.log.add("with-held")
			return fn()
		},
		BrainFence: func(string, string) (string, error) { b.log.add("brain-fence"); return "", nil },
		ConfValue: func(_ string, key string) string {
			if key == "testing.contract" {
				return "testing.json"
			}
			return ""
		},
		StartedAt:  func(int64) (int64, error) { return 1700000000, nil },
		TokenNonce: func() (string, error) { return strings.Repeat("ab", 16), nil },
		WriteToken: func(path string, token WrapperToken) error {
			b.log.add("token pid=%d start=%d", token.WrapperPid, token.WrapperPidStartedAt)
			return nil
		},
		RemoveFile:   func(path string) error { b.log.add("remove %s", filepath.Base(path)); return nil },
		ReadFile:     os.ReadFile,
		FileReadable: func(path string) bool { _, err := os.Stat(path); return err == nil },
		FileExists:   func(path string) bool { _, err := os.Stat(path); return err == nil },
		Verify: func(request VerifyRequest, stdout, stderr io.Writer) int {
			b.log.add("verify tree=%s goal=%s", request.Tree, request.Goal)
			return 0
		},
		SelectLanding: func(paths []string, _ string) ([]string, error) { return paths, nil },
		Live: func() Judge {
			return Judge{
				Observe: func(request ObserveRequest) (landing.Observation, int) {
					b.log.add("observe judge=%s goal=%s chain=%s", request.Judge, request.Goal, request.Chain)
					return b.observed, 0
				},
				Workspace:     func(_, tree string) (string, error) { return "w-" + tree, nil },
				VerifyCarried: func(string, string, string) ([]byte, int) { return []byte(`{"delivery":{"sufficient":true}}`), 0 },
				Digest:        "",
			}
		},
		BuildBaseJudge: func(string, string, io.Writer) (Judge, func(), error) {
			return Judge{}, func() {}, fmt.Errorf("no base judge in this bed")
		},
		Held: func(_, base, commit, _, _ string, stdout, _ io.Writer) int {
			b.log.add("held base=%s", base)
			fmt.Fprintln(stdout, "held: ok 1 commit(s) above base")
			return 0
		},
		WeightAdd:     func(string, string, string, string, []byte, io.Writer, io.Writer) int { b.log.add("weight"); return 0 },
		SyncTransport: func(_, branch string, _, _ io.Writer) int { b.log.add("transport %s", branch); return 0 },
		Drift: func(_ string, requireEmpty bool, _, _ io.Writer) int {
			b.log.add("drift empty=%t", requireEmpty)
			return 0
		},
		Advance: func(_, upstream string, _, _ io.Writer) int { b.log.add("advance %s", upstream); return 0 },
		ReceiptLine: func(_, tree, goal, _ string) (ReceiptDecision, error) {
			b.log.add("receipt-line goal=%s", goal)
			return ReceiptDecision{Encoded: `{"outcome":"appended"}`}, nil
		},
		TestReceipt:  func(string, string, string, io.Writer, io.Writer) int { return 0 },
		JobGateWidth: func(string, string) string { return "area" },
		Park: func(request ParkRequest) (string, error) {
			b.log.add("park reason=%s", request.Reason)
			return "state=parked\nreason=" + request.Reason, nil
		},
		OutputSpill: func(_, verb, _ string, _ []byte) (string, error) { return "output retained: " + verb + ".log", nil },
		BootClock:   func() (BootSample, error) { return BootSample{ID: "boot", Nanos: 1}, nil },
		NotifyGoal:  func(_, goal, publication string, _ *BootSample) { b.log.add("notify %s %s", goal, publication) },
	}
	return b
}

func (b *bed) messageFile() string {
	if b.messagePath != "" {
		return b.messagePath
	}
	return filepath.Join(b.root, "message")
}

func (b *bed) commit(request CommitRequest) int {
	b.t.Helper()
	if request.Root == "" {
		request.Root = b.root
	}
	if request.MessageFile == "" {
		request.MessageFile = b.messageFile()
	}
	return Commit(b.owners, request, &b.stdout, &b.stderr)
}

func (b *bed) land(request LandRequest) int {
	b.t.Helper()
	if request.Root == "" {
		request.Root = b.root
	}
	if request.MessageFile == "" {
		request.MessageFile = b.messageFile()
	}
	return Land(b.owners, request, &b.stdout, &b.stderr)
}

// expect fails unless the status is want and stderr contains every text.
func (b *bed) expect(status, want int, texts ...string) {
	b.t.Helper()
	if status != want {
		b.t.Fatalf("status %d, want %d\nstdout:\n%s\nstderr:\n%s", status, want, b.stdout.String(), b.stderr.String())
	}
	for _, text := range texts {
		if !strings.Contains(b.stderr.String()+b.stdout.String(), text) {
			b.t.Fatalf("output lacks %q\nstdout:\n%s\nstderr:\n%s", text, b.stdout.String(), b.stderr.String())
		}
	}
}
