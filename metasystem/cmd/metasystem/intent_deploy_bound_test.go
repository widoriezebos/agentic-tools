package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

var commandA0, commandA, commandB, commandC = strings.Repeat("0", 40), strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)

type boundCommandBed struct {
	*deployVerbBed
	calls          []string
	fetches, kills int
	active         deploy.Response
	parents        map[string]string
	failBuild      bool
	versioned      func()
}

func newBoundCommandBed(t *testing.T) *boundCommandBed {
	t.Helper()
	b := &boundCommandBed{deployVerbBed: newDeployVerbBed(t, true), parents: map[string]string{commandA0: "", commandA: commandA0, commandB: commandA, commandC: commandA0}}
	b.tip = commandB
	base := b.owners().deploy.git
	b.git = func(root string) deploy.Git {
		owner := base(root)
		owner.FetchMain = func() (string, error) { b.fetches++; return b.tip, nil }
		owner.Contains = func(commit, descendant string) (bool, error) {
			if _, ok := b.parents[commit]; !ok {
				return false, errors.New("unknown commit")
			}
			for at := descendant; at != ""; {
				parent, ok := b.parents[at]
				if !ok {
					return false, errors.New("unknown descendant")
				}
				if at == commit {
					return true, nil
				}
				at = parent
			}
			return false, nil
		}
		return owner
	}
	b.transport = func(command *exec.Cmd) (deploy.Process, error) {
		var request deploy.Request
		if err := json.NewDecoder(command.Stdin).Decode(&request); err != nil {
			return deploy.Process{}, err
		}
		b.calls = append(b.calls, request.Operation+" "+request.Commit)
		return deploy.Process{PID: 999999999, Kill: func() { b.kills++ }, Wait: func() int {
			response := commandArtifact(request.Commit)
			switch request.Operation {
			case "version":
				if b.versioned != nil {
					b.versioned()
				}
				response = b.active
				if response.Outcome == "" {
					response.Outcome = deploy.OutcomeNone
				}
			case "build":
				response.Outcome = "built"
				if b.failBuild {
					response = deploy.Response{Outcome: "failed", Reason: "build failed"}
				}
			case "activate":
				b.active = response
			default:
				t.Errorf("unexpected operation %q", request.Operation)
			}
			if err := json.NewEncoder(command.Stdout).Encode(response); err != nil {
				t.Error(err)
			}
			if response.Outcome == "failed" {
				return 1
			}
			return 0
		}}, nil
	}
	return b
}

func commandArtifact(commit string) deploy.Response {
	return deploy.Response{Outcome: deploy.OutcomeActive, Version: "v-" + commit, Artifact: "art-" + commit, Digest: "d-" + commit}
}

func (b *boundCommandBed) seed(commit string) {
	b.t.Helper()
	b.active = commandArtifact(commit)
	line := deploy.Line{Kind: deploy.KindDeploy, Commit: commit, Outcome: deploy.OutcomeActive, Version: b.active.Version, Artifact: b.active.Artifact, Digest: b.active.Digest, By: "fixture"}
	if err := os.MkdirAll(b.dir(), 0700); err != nil {
		b.t.Fatal(err)
	}
	data, err := json.Marshal(line)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.dir(), "deploys.jsonl"), append(data, '\n'), 0600); err != nil {
		b.t.Fatal(err)
	}
}

func (b *boundCommandBed) wantCalls(want ...string) {
	b.t.Helper()
	if !reflect.DeepEqual(b.calls, want) || b.kills != 0 {
		b.t.Fatalf("calls = %v; want %v; kills %d", b.calls, want, b.kills)
	}
}

func TestDeployNowCommit(t *testing.T) {
	t.Parallel()
	t.Run("bound-builds-only-its-commit", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.seed(commandA0)
		if code, result := b.result("deploy", "now", "--commit", commandA); code != 0 || !strings.Contains(result.Summary, "deployed") {
			t.Fatalf("bound deploy = %d, %+v", code, result)
		}
		b.wantCalls("version "+commandA0, "build "+commandA, "activate "+commandA, "version "+commandA)
		if b.fetches != 1 {
			t.Fatalf("fetches = %d; want 1", b.fetches)
		}
	})
	for _, row := range []struct {
		name string
		args []string
	}{{"short-id", []string{"--commit", "abc123"}}, {"empty-id", []string{"--commit", ""}}, {"whitespace-id", []string{"--commit", "   "}}, {"joined-empty-id", []string{"--commit="}}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newBoundCommandBed(t)
			b.seed(commandA0)
			before := idemTreeDigest(t, b.home)
			code, result := b.result(append([]string{"deploy", "now"}, row.args...)...)
			if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "full commit id") {
				t.Fatalf("invalid commit = %d, %+v", code, result)
			}
			b.wantCalls()
			if b.fetches != 0 {
				t.Fatalf("fetches = %d; want 0", b.fetches)
			}
			idemSameTree(t, "invalid commit", before, idemTreeDigest(t, b.home))
		})
	}
	t.Run("help", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		code, text := b.run("deploy", "now", "--help")
		advanced := strings.Index(text, "Advanced")
		if code != 0 || advanced < 0 || !strings.Contains(text[advanced:], "--commit SHA  ") {
			t.Fatalf("advanced commit option row missing: %d\n%s", code, text)
		}
	})
	t.Run("skipped-for-a-person", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.seed(commandC)
		code, result := b.result("deploy", "now", "--commit", commandA)
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "was not deployed: it is not after the active "+deploy.Short(commandC)+"; what is active stays") || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem deploy status --verify" {
			t.Fatalf("skipped = %d, %+v", code, result)
		}
		b.wantCalls("version " + commandC)
	})
	for _, name := range []string{"busy-plain", "busy-bound"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundCommandBed(t)
			b.seed(commandA0)
			held, err := lock.File(filepath.Join(b.dir(), "lock"), 0600, lock.TryExclusive)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			active := deploy.Active{Operation: "build", Commit: commandA, PID: 999999999}
			data, err := json.Marshal(active)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b.dir(), "run.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			words := []string{"deploy", "now"}
			retry := "metasystem deploy now"
			if name == "busy-bound" {
				words = append(words, "--commit", commandA)
				retry += " --commit " + commandA
			}
			code, result := b.result(words...)
			want := "a deploy run is in progress (build of " + deploy.Short(commandA) + ", process 999999999), so nothing was done; when deploy status shows it ended, run " + retry
			if code != 0 || result.Outcome != intentUnchanged || result.Summary != want || result.Next == nil || strings.Join(result.Next.Argv, " ") != retry {
				t.Fatalf("busy = %d, %+v; want %q", code, result, want)
			}
			b.wantCalls()
			if b.fetches != 0 {
				t.Fatalf("busy fetched %d times", b.fetches)
			}
			if name == "busy-bound" {
				if err := held.Release(); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(b.dir(), "run.json")); err != nil {
					t.Fatal(err)
				}
				if code, text := b.run(result.Next.Argv[1:]...); code != 0 || !strings.Contains(text, "deployed") {
					t.Fatalf("retry = %d\n%s", code, text)
				}
				b.wantCalls("version "+commandA0, "build "+commandA, "activate "+commandA, "version "+commandA)
				lines, err := deploy.Lines(b.dir())
				if err != nil || len(lines) != 2 || lines[1].Commit != commandA || lines[1].Outcome != deploy.OutcomeActive {
					t.Fatalf("retry lines = %+v, %v", lines, err)
				}
			}
		})
	}
}

func (b *boundCommandBed) lane(repository bool) string {
	b.t.Helper()
	root := filepath.Join(b.t.TempDir(), "lane")
	dir := root
	if repository {
		dir = filepath.Join(root, ".git")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		b.t.Fatal(err)
	}
	record := lane.Record{Root: root, Install: root, CustodyEpoch: 1, RegisteredBy: "Wido", At: b.clock.Format("2006-01-02T15:04:05Z07:00")}
	data, err := json.Marshal(record)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lane.RecordPath(b.home)), 0700); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(b.home), data, 0600); err != nil {
		b.t.Fatal(err)
	}
	return root
}

func (b *boundCommandBed) pauseLane() {
	b.t.Helper()
	if _, err := lane.SetPause(b.home, "Wido", b.clock); err != nil {
		b.t.Fatal(err)
	}
}

func (b *boundCommandBed) laneSkip(reason string) {
	b.t.Helper()
	code, result := b.result("deploy", "now", "--commit", commandA, "--by", "the landing lane")
	retry := "metasystem deploy now --commit " + commandA
	want := "not deployed automatically: " + reason
	if code != 1 || result.Outcome != intentRefused || result.Summary != want || result.Next == nil || strings.Join(result.Next.Argv, " ") != retry {
		b.t.Fatalf("lane skip = %d, %+v; want %q", code, result, want)
	}
}

func TestDeployNowCommitByTheLane(t *testing.T) {
	t.Parallel()
	t.Run("failure-holds-the-lane-only", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.lane(true)
		b.failBuild = true
		code, result := b.result("deploy", "now", "--commit", commandA, "--by", "the landing lane")
		if code != 1 || result.Outcome != intentFailed {
			t.Fatalf("failed build = %d, %+v", code, result)
		}
		b.wantCalls("version "+commandA, "build "+commandA)
		lines, err := deploy.Lines(b.dir())
		if err != nil || len(lines) != 1 || lines[0].Outcome != deploy.OutcomeBuildFailed {
			t.Fatalf("failed lines = %+v, %v", lines, err)
		}
		before := b.record()
		for range 2 {
			b.calls = nil
			b.laneSkip("its earlier attempt ended build-failed")
			b.wantCalls()
			if b.record() != before {
				t.Fatal("a skipped retry changed the record")
			}
		}
		b.failBuild = false
		b.calls = nil
		code, result = b.result("deploy", "now", "--commit", commandA)
		if code != 0 || !strings.Contains(result.Summary, "deployed") {
			t.Fatalf("person retry = %d, %+v", code, result)
		}
		b.wantCalls("version "+commandA, "build "+commandA, "activate "+commandA, "version "+commandA)
		lines, err = deploy.Lines(b.dir())
		if err != nil || len(lines) != 2 || lines[1].Commit != commandA || lines[1].By != "Wido" || lines[1].Outcome != deploy.OutcomeActive {
			t.Fatalf("person retry lines = %+v, %v", lines, err)
		}
		if code, text := b.run("deploy", "pause", "--reason", "maintenance"); code != 0 {
			t.Fatalf("pause = %d: %s", code, text)
		}
		b.calls = nil
		code, result = b.result("deploy", "now", "--commit", commandA)
		if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "paused") {
			t.Fatalf("paused person = %d, %+v", code, result)
		}
		b.wantCalls()
		if err := os.Remove(filepath.Join(b.dir(), "pause.json")); err != nil {
			t.Fatal(err)
		}
		b.seed(commandB)
		code, result = b.result("deploy", "now", "--commit", commandA)
		if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already covered by "+deploy.Short(commandB)) {
			t.Fatalf("covered person = %d, %+v", code, result)
		}
		b.wantCalls("version " + commandB)
	})
	t.Run("lane-paused", func(t *testing.T) {
		t.Parallel()
		for _, registered := range []bool{true, false} {
			name := "unregistered"
			if registered {
				name = "registered"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				b := newBoundCommandBed(t)
				reason := "the lane is stopped: it is not registered here"
				if registered {
					b.lane(true)
					b.pauseLane()
					reason = "the lane is stopped"
				}
				b.laneSkip(reason)
				b.wantCalls()
			})
		}
	})
	t.Run("lane-stops-during-version", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.lane(true)
		b.versioned = b.pauseLane
		b.laneSkip("the lane is stopped")
		b.wantCalls("version " + commandA)
		if b.record() != "" {
			t.Fatal("lane stop wrote an attempt")
		}
	})
	t.Run("person-ignores-the-lane", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.lane(true)
		b.pauseLane()
		b.seed(commandA0)
		code, result := b.result("deploy", "now", "--commit", commandA)
		if code != 0 || !strings.Contains(result.Summary, "deployed") {
			t.Fatalf("person deploy = %d, %+v", code, result)
		}
		b.wantCalls("version "+commandA0, "build "+commandA, "activate "+commandA, "version "+commandA)
	})
	t.Run("helm-held", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		root := b.lane(true)
		if _, err := helm.Write(root, helm.Record{By: "Wido", At: b.clock.Format("2006-01-02T15:04:05Z07:00")}); err != nil {
			t.Fatal(err)
		}
		b.laneSkip("the lane is stopped")
		b.wantCalls()
	})
	t.Run("helm-unreadable", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		b.lane(false)
		b.laneSkip("the lane is stopped")
		b.wantCalls()
	})
}
