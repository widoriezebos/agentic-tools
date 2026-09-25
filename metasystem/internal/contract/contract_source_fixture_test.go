package contract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	fixtureGateSHA      = "1111111111111111111111111111111111111111"
	fixtureCandidateSHA = "2222222222222222222222222222222222222222"
	fixtureGateScript   = "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\n'\n"
	fixtureAuditScript  = "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\nmetric=audit=1\\n'\n"
)

type sourceCall struct {
	kind, dir string
	args      []string
	out       string
}

type contractFixtureSource struct {
	t            *testing.T
	path, repo   string
	repository   func(string) (string, error)
	bytes        map[string]string // one gateRef:path table owns show and checkout bytes
	calls        []sourceCall
	next         int
	worktrees    []string
	removed      []string
	failCheckout bool
}

func newContractSource(t *testing.T, repositoryCalls int, audit bool) *contractFixtureSource {
	t.Helper()
	path, repository := newContractFiles(t, repositoryCalls)
	repo := filepath.Dir(filepath.Dir(resolvePath(path)))
	script := fixtureGateScript
	if audit {
		script = fixtureAuditScript
	}
	f := &contractFixtureSource{
		t: t, path: path, repo: repo, repository: repository,
		bytes: map[string]string{
			fixtureGateSHA + ":scripts/gate.sh":     script,
			fixtureGateSHA + ":truth/reference.txt": "certified truth\n",
		},
	}
	t.Cleanup(func() {
		if f.next != len(f.calls) {
			t.Errorf("raw source consumed %d of %d expected calls", f.next, len(f.calls))
		}
		if !reflect.DeepEqual(f.removed, f.worktrees) {
			t.Errorf("worktree removals %q, created %q", f.removed, f.worktrees)
		}
		seen := map[string]bool{}
		for _, worktree := range f.worktrees {
			if seen[worktree] {
				t.Errorf("gate and guard used the same worktree: %s", worktree)
			}
			seen[worktree] = true
			if _, err := os.Stat(filepath.Dir(worktree)); !os.IsNotExist(err) {
				t.Errorf("scratch directory still exists: %s (%v)", filepath.Dir(worktree), err)
			}
		}
	})
	return f
}

func (f *contractFixtureSource) expect(args []string, out string) {
	f.calls = append(f.calls, sourceCall{kind: "read", dir: f.repo, args: args, out: out})
}

func (f *contractFixtureSource) tree() {
	f.expect([]string{"ls-tree", "-r", "--name-only", "-z", fixtureGateSHA},
		"docs/project-rules.md\x00scripts/agents/arm-supervision.sh\x00scripts/gate.sh\x00truth/reference.txt\x00")
}

func (f *contractFixtureSource) ref() {
	f.expect([]string{"rev-parse", "instruments^{commit}"}, fixtureGateSHA+"\n")
}

func (f *contractFixtureSource) materialize() {
	f.tree()
	f.tree()
	f.calls = append(f.calls, sourceCall{kind: "add"}, sourceCall{kind: "checkout"}, sourceCall{kind: "remove"})
}

func (f *contractFixtureSource) seal(failCheckout bool) (string, error) {
	f.failCheckout = failCheckout
	f.ref()
	f.tree()
	f.tree()
	f.expect([]string{"branch", "--show-current"}, "main\n")
	for _, path := range []string{"scripts/gate.sh", "truth/reference.txt"} {
		f.expect([]string{"show", fixtureGateSHA + ":" + path}, f.bytes[fixtureGateSHA+":"+path])
	}
	f.ref()
	f.expect([]string{"branch", "--show-current"}, "main\n")
	f.expect([]string{"rev-parse", "main^{commit}"}, fixtureCandidateSHA+"\n")
	f.materialize()
	return contractSealWithSource(f.path, f.repository, f.source())
}

func (f *contractFixtureSource) verify() {
	f.ref()
	f.tree()
	f.tree()
	for _, path := range []string{"scripts/gate.sh", "truth/reference.txt"} {
		f.expect([]string{"show", fixtureGateSHA + ":" + path}, f.bytes[fixtureGateSHA+":"+path])
	}
}

func (f *contractFixtureSource) preflight(fetchFailure bool) (string, string, error) {
	f.verify()
	if fetchFailure {
		f.calls = append(f.calls, sourceCall{kind: "fetch"})
	}
	return contractPreflightWithSource(f.path, "", f.repository, f.source())
}

func (f *contractFixtureSource) measure(previous map[string]string) (*MeasureResult, error) {
	f.ref()
	f.expect([]string{"rev-parse", "main^{commit}"}, fixtureCandidateSHA+"\n")
	f.materialize()
	f.materialize()
	return contractMeasureWithSource(f.path, previous, f.repository, f.source())
}

func (f *contractFixtureSource) nextCall(kind, dir string, args []string) sourceCall {
	f.t.Helper()
	if f.next >= len(f.calls) {
		f.t.Fatalf("unexpected raw %s at %s: %q", kind, dir, args)
	}
	want := f.calls[f.next]
	f.next++
	if want.kind != kind && !(want.kind == "read" && kind == "output") {
		f.t.Fatalf("raw call %d: got %s %q, want %s %q", f.next, kind, args, want.kind, want.args)
	}
	if want.kind == "read" && (dir != want.dir || !reflect.DeepEqual(args, want.args)) {
		f.t.Fatalf("raw read %d: got %s %q, want %s %q", f.next, dir, args, want.dir, want.args)
	}
	return want
}

func (f *contractFixtureSource) source() *Source {
	return &Source{
		Repository: f.repository,
		Output: func(dir string, args ...string) (string, error) {
			if len(args) == 0 {
				f.t.Fatal("empty raw output call")
			}
			switch args[0] {
			case "worktree":
				f.nextCall("add", dir, args)
				if dir != f.repo || len(args) != 6 || !reflect.DeepEqual(args[:4], []string{"worktree", "add", "--detach", "--quiet"}) || args[5] != fixtureCandidateSHA {
					f.t.Fatalf("unexpected worktree add: %s %q", dir, args)
				}
				worktree := args[4]
				f.checkRegistry(worktree)
				f.worktrees = append(f.worktrees, worktree)
				for path, content := range map[string]string{
					"scripts/gate.sh":     "#!/usr/bin/env bash\necho candidate-version\n",
					"truth/reference.txt": "candidate truth\n",
					"docs/candidate.txt":  "outside restored paths\n",
				} {
					writeFileMode(f.t, filepath.Join(worktree, path), content, 0o755)
				}
				return "", nil
			case "checkout":
				f.nextCall("checkout", dir, args)
				if len(f.worktrees) == 0 || dir != f.worktrees[len(f.worktrees)-1] ||
					!reflect.DeepEqual(args, []string{"checkout", "--quiet", fixtureGateSHA, "--", "scripts/gate.sh", "truth/reference.txt"}) {
					f.t.Fatalf("unexpected restored paths: %s %q", dir, args)
				}
				f.checkRegistry(dir)
				if f.failCheckout {
					return "", errors.New("injected checkout failure")
				}
				old, err := os.ReadFile(filepath.Join(dir, "scripts/gate.sh"))
				if err != nil || string(old) != "#!/usr/bin/env bash\necho candidate-version\n" {
					f.t.Fatalf("candidate version absent before restore: %q %v", old, err)
				}
				for _, path := range args[4:] {
					want := f.bytes[fixtureGateSHA+":"+path]
					writeFileMode(f.t, filepath.Join(dir, path), want, 0o755)
					got, err := os.ReadFile(filepath.Join(dir, path))
					if err != nil || string(got) != want {
						f.t.Fatalf("restored %s differs from show bytes: %q %v", path, got, err)
					}
				}
				outside, err := os.ReadFile(filepath.Join(dir, "docs/candidate.txt"))
				if err != nil || string(outside) != "outside restored paths\n" {
					f.t.Fatalf("candidate file outside restored paths changed: %q %v", outside, err)
				}
				return "", nil
			default:
				return f.nextCall("output", dir, args).out, nil
			}
		},
		Try: func(dir string, args ...string) (string, int) {
			f.nextCall("remove", dir, args)
			if dir != f.repo || len(f.worktrees) == 0 || !reflect.DeepEqual(args, []string{"worktree", "remove", "--force", f.worktrees[len(f.worktrees)-1]}) {
				f.t.Fatalf("unexpected worktree removal: %s %q", dir, args)
			}
			f.removed = append(f.removed, args[3])
			return "", 0
		},
		Fetch: func(dir string) (string, error) {
			f.nextCall("fetch", dir, nil)
			if dir != f.repo {
				f.t.Fatalf("fetch in %s, want %s", dir, f.repo)
			}
			return "fixture origin unavailable", errors.New("origin unavailable")
		},
	}
}

func (f *contractFixtureSource) checkRegistry(worktree string) {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.repo, "artifacts", "agents", "measure-worktrees.jsonl"))
	if err != nil {
		f.t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row struct {
			Path    string `json:"path"`
			SHA     string `json:"sha"`
			GateRef string `json:"gateRef"`
		}
		if json.Unmarshal([]byte(line), &row) == nil && row.Path == worktree && row.SHA == fixtureCandidateSHA && row.GateRef == fixtureGateSHA {
			return
		}
	}
	f.t.Fatalf("registry lacks worktree %s at candidate %s and gate %s: %s", worktree, fixtureCandidateSHA, fixtureGateSHA, data)
}
