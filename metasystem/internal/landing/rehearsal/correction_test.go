package rehearsal

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestRehearsalFixtureProofUsesRealContractWithoutSourceChecks(t *testing.T) {
	t.Parallel()
	original, err := os.ReadFile("../../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	module := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		path = filepath.Join(module, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("testing.json", string(original))
	write("go.mod", "module example.invalid/rehearsal\n\ngo 1.27\n")
	for _, name := range []string{"value.go", "value_test.go"} {
		data, err := os.ReadFile("../../../internal/landing/rehearsal/fixtures/green/" + name)
		if err != nil {
			t.Fatal(err)
		}
		write("internal/rehearsalgreen/"+name, string(data))
	}
	write("internal/unrelated/broken.go", "invalid Go outside the fixture\n")
	s := &Scratch{Module: module, ctx: context.Background(), Env: os.Environ()}
	if err := s.fixtureProofContract(); err != nil {
		t.Fatal(err)
	}
	before, err := testpolicy.Load("../../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	after, err := testpolicy.Load(filepath.Join(module, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Always, after.Always) || !reflect.DeepEqual(before.Surfaces, after.Surfaces) || len(before.Groups) != len(after.Groups) {
		t.Fatal("scratch override changed real canaries, surfaces or group inventory")
	}
	for i, group := range after.Groups {
		if group.ID != "fast-static-build" {
			// Raw test selectors retain JSON whitespace when loaded.
			var selector bytes.Buffer
			if len(group.Tests) > 0 {
				if err := json.Compact(&selector, group.Tests); err != nil {
					t.Fatal(err)
				}
				group.Tests = selector.Bytes()
			}
			if !reflect.DeepEqual(before.Groups[i], group) {
				t.Fatalf("scratch changed real group %s", group.ID)
			}
			continue
		}
		if len(group.Requires) != 0 {
			t.Fatal("fixture static check still runs repository audits as prerequisites")
		}
		if output, err := s.Run(module, group.Argv[0], group.Argv[1:]...); err != nil {
			t.Fatalf("fixture-only static command ran unrelated source checks: %s: %v", output, err)
		}
		write("internal/rehearsalgreen/value.go", "package rehearsalgreen\nfunc Value() int { return 7 }\n")
		if _, err := s.Run(module, group.Argv[0], group.Argv[1:]...); err == nil {
			t.Fatal("fixture static command did not run its test")
		}
	}
	data, _ := os.ReadFile("../../../testing.json")
	if string(data) != string(original) {
		t.Fatal("source testing contract changed")
	}
}

func TestRehearsalReusesOnlyMatchingExecutableBuild(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"tree", "commit", "wrong", "unreadable", "not-executable", "missing"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			module, target := t.TempDir(), filepath.Join(t.TempDir(), "engine")
			s := &Scratch{Record: Record{Candidate: "candidate", Tree: "tree", Engine: target}}
			if kind != "missing" {
				if err := os.MkdirAll(filepath.Join(module, "bin"), 0755); err != nil {
					t.Fatal(err)
				}
				stamp := map[string]string{"tree": "tree", "commit": "candidate", "wrong": "other", "not-executable": "tree"}[kind]
				mode := os.FileMode(0755)
				if kind == "not-executable" {
					mode = 0644
				}
				if err := testexec.WriteFile(filepath.Join(module, "bin/metasystem"), []byte(enginebuild.StampRecord(stamp)), mode); err != nil {
					t.Fatal(err)
				}
			}
			reused, err := s.reuseEngine(module)
			want := kind == "tree" || kind == "commit"
			if err != nil || reused != want {
				t.Fatalf("reuse %s: %v: %v", kind, reused, err)
			}
			if want {
				if err := s.CheckIdentity(); err != nil || len(s.Steps) != 1 || s.Steps[0].Exit != 0 {
					t.Fatalf("copied build lost identity or time: %+v: %v", s.Steps, err)
				}
			} else if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("nonmatching build was copied: %v", err)
			}
		})
	}
}

// Real Git resolves the commit recorded by an older build to its actual tree.
func TestRehearsalReusesEarlierCommitOnlyForTheSameRealTree(t *testing.T) {
	t.Parallel()
	module := t.TempDir()
	s := &Scratch{Module: module, ctx: context.Background(), Env: append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")}
	git := func(args ...string) string {
		t.Helper()
		data, err := s.Git(module, args...)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	git("init", "-b", "main")
	git("config", "core.hooksPath", "/dev/null")
	git("config", "user.name", "rehearsal")
	git("config", "user.email", "rehearsal@example.invalid")
	git("commit", "--allow-empty", "-m", "first build")
	stamp := git("rev-parse", "HEAD")
	git("commit", "--allow-empty", "-m", "same source tree")
	s.Candidate, s.Tree = git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
	s.Engine = filepath.Join(t.TempDir(), "engine")
	if err := os.MkdirAll(filepath.Join(module, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(module, "bin/metasystem"), []byte(enginebuild.StampRecord(stamp)), 0755); err != nil {
		t.Fatal(err)
	}
	if reused, err := s.reuseEngine(module); err != nil || !reused {
		t.Fatalf("equivalent real tree was rebuilt: %v: %v", reused, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.ctx = ctx
	if err := s.CheckIdentity(); err != nil {
		t.Fatal(err)
	}
	s.ctx = context.Background()
	if err := os.WriteFile(filepath.Join(module, "value"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "value")
	git("commit", "-m", "different source tree")
	s.Candidate, s.Tree = git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
	s.Engine = filepath.Join(t.TempDir(), "engine")
	if reused, err := s.reuseEngine(module); err != nil || reused {
		t.Fatalf("different real tree reused the stale build: %v: %v", reused, err)
	}
}

func TestRehearsalBreakdownRetainsFailedStageAndPrintsSlowest(t *testing.T) {
	t.Parallel()
	s := &Scratch{stage: "gate", stageStart: time.Unix(100, 0), Record: Record{Breakdown: Breakdown{Setup: 1, Bootstrap: 2, HandIn: 3, Proof: 4, Push: 5, Total: 2015}}}
	s.beginStage("", time.Unix(102, 0))
	name, duration := s.Breakdown.Slowest()
	if name != "gate" || duration < 2*time.Second || s.stage != "" {
		t.Fatalf("failed stage lost its time: %+v; %s %s", s.Breakdown, name, duration)
	}
	data, err := json.Marshal(s.Record)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"setup", "bootstrap", "hand-in", "gate", "proof", "push", "total"} {
		if !strings.Contains(string(data), `"`+field+`":`) {
			t.Fatalf("record lost %s: %s", field, data)
		}
	}
}

func TestRehearsalExpiredRunCannotPublishPass(t *testing.T) {
	t.Parallel()
	module, path := t.TempDir(), recordScratch(t)
	engine := filepath.Join(path, "engine")
	if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("candidate")), 0644); err != nil {
		t.Fatal(err)
	}
	s := &Scratch{Record: Record{Candidate: "candidate", Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path))}
	if err := s.Finish(module, time.Unix(0, 0), false, nil); err == nil || !strings.Contains(err.Error(), "budget") || s.Outcome != "failed" {
		t.Fatalf("expired rehearsal issued a pass: %+v: %v", s.Record, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expired rehearsal removed its evidence: %v", err)
	}
}
