package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

func automaticWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func automaticLane(b *deployVerbBed, commits ...string) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Join(b.root, ".git"), 0700); err != nil {
		b.t.Fatal(err)
	}
	record, err := json.Marshal(lane.Record{Root: b.root, Install: b.root, CustodyEpoch: 1, RegisteredBy: "Wido", At: b.clock.Format("2006-01-02T15:04:05Z07:00")})
	if err != nil {
		b.t.Fatal(err)
	}
	automaticWrite(b.t, lane.RecordPath(b.home), record)
	var data []byte
	for _, commit := range commits {
		line, err := json.Marshal(plain.Pushed{Commit: commit})
		if err != nil {
			b.t.Fatal(err)
		}
		data = append(data, append(line, '\n')...)
	}
	automaticWrite(b.t, filepath.Join(plain.Dir(b.root), "pushes.jsonl"), data)
}

func automaticAppend(b *deployVerbBed, commit, outcome, reason string) {
	b.t.Helper()
	if err := deploy.AppendAutomatic(b.dir(), deploy.Automatic{Commit: commit, Outcome: outcome, Reason: reason, At: b.clock}); err != nil {
		b.t.Fatal(err)
	}
}

func automaticStatus(b *deployVerbBed, code int, commit, outcome, reason string) {
	b.t.Helper()
	got, text := b.run("deploy", "status")
	jsonCode, result := b.result("deploy", "status")
	data, ok := result.Data.(map[string]any)
	if !ok || jsonCode != code {
		b.t.Fatalf("status data = %d, %+v", jsonCode, result)
	}
	if commit == "" {
		if got != code || strings.Contains(text, "not deployed automatically: ") || data["automatic"] != nil {
			b.t.Fatalf("covered status = %d, %s, %+v", got, text, data)
		}
		return
	}
	want := "not deployed automatically: " + reason
	retry := "metasystem deploy now --commit " + commit
	line, ok := data["automatic"].(map[string]any)
	if got != code || !strings.Contains(text, want) || !strings.Contains(text, "→ "+retry) || result.Summary != want && !strings.HasPrefix(result.Summary, want+"\n") || result.Next == nil || strings.Join(result.Next.Argv, " ") != retry || !ok || line["commit"] != commit || line["outcome"] != outcome {
		b.t.Fatalf("status = %d, %s, %+v; want %q and outcome %s", got, text, data, want, outcome)
	}
}

// A launched line is lost only when nothing runs its commit: while the
// lane's run builds that commit, status says so and owes no warning.
func TestDeployStatusAutomaticLaunchedWhileRunning(t *testing.T) {
	t.Parallel()
	for _, running := range []bool{true, false} {
		b := newBoundCommandBed(t)
		automaticLane(b.deployVerbBed, commandA)
		b.seed(commandA0)
		automaticAppend(b.deployVerbBed, commandA, deploy.AutomaticLaunched, "")
		if !running {
			automaticStatus(b.deployVerbBed, 0, commandA, deploy.AutomaticLaunched, "its run has reported no outcome")
			continue
		}
		born, ok := identity.ProcessBirth(int64(os.Getpid()))
		if !ok {
			t.Fatal("this process has no birth time")
		}
		active, err := json.Marshal(deploy.Active{Runner: os.Getpid(), RunnerBorn: born, Kind: "deploy", Operation: "build", Commit: commandA, PID: os.Getpid(), Born: born, StartedAt: b.clock, Log: filepath.Join(b.dir(), "build.log")})
		if err != nil {
			t.Fatal(err)
		}
		automaticWrite(t, filepath.Join(b.dir(), "run.json"), active)
		if _, text := b.run("deploy", "status"); !strings.Contains(text, "running: build of "+deploy.Short(commandA)) {
			t.Fatalf("status shows no run of A:\n%s", text)
		}
		automaticStatus(b.deployVerbBed, 0, "", "", "")
	}
}

func TestDeployNowByTheLaneRecordsItsOutcome(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"deployed", "current", "skipped", "failed", "busy", "paused", "stopped", "error", "no-outcome", "person-writes-nothing", "seat-writes-nothing", "plain-lane-writes-nothing"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundCommandBed(t)
			automaticLane(b.deployVerbBed, commandA)
			b.seed(commandA0)
			reason := ""
			switch name {
			case "current":
				b.seed(commandA)
			case "skipped":
				b.seed(commandC)
				reason = "it is not after the active " + deploy.Short(commandC)
			case "failed":
				b.failBuild = true
			case "busy":
				held, err := lock.File(filepath.Join(b.dir(), "lock"), 0600, lock.TryExclusive)
				if err != nil {
					t.Fatal(err)
				}
				defer held.Release()
				reason = "its run ended busy"
			case "paused":
				if code, text := b.run("deploy", "pause"); code != 0 {
					t.Fatalf("pause = %d, %s", code, text)
				}
				reason = "its run ended paused"
			case "stopped":
				base := b.transport
				b.transport = func(cmd *exec.Cmd) (deploy.Process, error) {
					process, err := base(cmd)
					if len(b.calls) == 2 {
						automaticWrite(t, filepath.Join(b.dir(), "pause.json"), []byte(`{"by":"Wido","reason":"maintenance"}`))
					}
					return process, err
				}
			case "error":
				base := b.git
				b.git = func(root string) deploy.Git {
					owner := base(root)
					owner.FetchMain = func() (string, error) { return "", errors.New("fetch unavailable") }
					return owner
				}
				reason = "its run ended error: origin's main can't be fetched: fetch unavailable"
			case "no-outcome":
				automaticAppend(b.deployVerbBed, commandA, deploy.AutomaticLaunched, "")
				automaticStatus(b.deployVerbBed, 0, commandA, deploy.AutomaticLaunched, "its run has reported no outcome")
				return
			}
			args := []string{"deploy", "now", "--commit", commandA, "--by", laneDeployer}
			if name == "person-writes-nothing" {
				args = args[:4]
			}
			if name == "seat-writes-nothing" {
				args[5] = "the seat"
			}
			if name == "plain-lane-writes-nothing" {
				args = []string{"deploy", "now", "--by", laneDeployer}
			}
			b.run(args...)
			lines, err := deploy.AutomaticLines(b.dir())
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(name, "writes-nothing") {
				if len(lines) != 0 {
					t.Fatalf("non-bound-lane run wrote %+v", lines)
				}
				if _, err := os.Stat(filepath.Join(b.dir(), "automatic.jsonl")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("outcome record exists: %v", err)
				}
				return
			}
			if len(lines) != 1 || lines[0].Commit != commandA || lines[0].Outcome != name || lines[0].At.IsZero() {
				t.Fatalf("outcomes = %+v; want one %s for %s", lines, name, commandA)
			}
			if name == "failed" || name == "stopped" {
				if !strings.HasPrefix(lines[0].Reason, "build-failed: ") && name == "failed" {
					t.Fatalf("failure reason = %q", lines[0].Reason)
				}
				reason = lines[0].Reason
			}
			if name == "deployed" {
				b.wantCalls("version "+commandA0, "build "+commandA, "activate "+commandA, "version "+commandA)
				automaticStatus(b.deployVerbBed, 0, "", "", "")
			} else if name == "current" {
				automaticStatus(b.deployVerbBed, 0, "", "", "")
			} else {
				automaticStatus(b.deployVerbBed, 0, commandA, name, reason)
			}
		})
	}
	t.Run("newest-push", func(t *testing.T) {
		t.Parallel()
		b := newBoundCommandBed(t)
		automaticLane(b.deployVerbBed, commandA, commandB)
		b.seed(commandA0)
		b.failBuild = true
		b.run("deploy", "now", "--commit", commandB, "--by", laneDeployer)
		b.failBuild = false
		b.run("deploy", "now", "--commit", commandA, "--by", laneDeployer)
		lines, err := deploy.AutomaticLines(b.dir())
		if err != nil || len(lines) != 2 || lines[0].Outcome != deploy.RunFailed || lines[1].Outcome != deploy.RunDeployed {
			t.Fatalf("outcomes = %+v, %v", lines, err)
		}
		automaticStatus(b.deployVerbBed, 0, commandB, deploy.RunFailed, lines[0].Reason)
		for _, outcome := range []string{deploy.AutomaticLaunched, deploy.RunBusy} {
			automaticAppend(b.deployVerbBed, commandB, outcome, "")
			automaticStatus(b.deployVerbBed, 0, commandB, deploy.RunFailed, lines[0].Reason)
		}
		if code, text := b.run("deploy", "now", "--commit", commandB); code != 0 {
			t.Fatalf("person retry = %d, %s", code, text)
		}
		automaticStatus(b.deployVerbBed, 0, "", "", "")
	})
	for _, name := range []string{"unreadable-state", "unreadable-lane", "missing-registration", "covered-descendant", "unreadable-ancestry", "other-project"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundCommandBed(t)
			automaticLane(b.deployVerbBed, commandA)
			b.seed(commandC)
			code := 0
			reason := "it is not after the active " + deploy.Short(commandC)
			if name == "unreadable-state" {
				b.seed(commandA0)
				automaticWrite(t, filepath.Join(b.dir(), "run.json"), []byte("{"))
				code = 1
			}
			b.run("deploy", "now", "--commit", commandA, "--by", laneDeployer)
			lines, err := deploy.AutomaticLines(b.dir())
			if err != nil || len(lines) != 1 || lines[0].Outcome != deploy.RunSkipped {
				t.Fatalf("skip = %+v, %v", lines, err)
			}
			if name == "unreadable-state" {
				reason = lines[0].Reason
			}
			switch name {
			case "unreadable-lane":
				automaticWrite(t, lane.RecordPath(b.home), []byte("{"))
				if _, text := b.run("deploy", "status"); !strings.Contains(text, "the lane's registration can't be read") {
					t.Fatal(text)
				}
			case "missing-registration":
				if err := os.Remove(lane.RecordPath(b.home)); err != nil {
					t.Fatal(err)
				}
			case "covered-descendant":
				b.seed(commandB)
				automaticStatus(b.deployVerbBed, 0, "", "", "")
				return
			case "unreadable-ancestry":
				delete(b.parents, commandC)
			case "other-project":
				root := b.lane(true)
				automaticWrite(t, filepath.Join(root, "metasystem.conf"), nil)
				automaticWrite(t, filepath.Join(root, "deploy.json"), b.landed)
				automaticWrite(t, filepath.Join(plain.Dir(root), "pushes.jsonl"), []byte(`{"commit":"`+commandA+`"}`+"\n"))
				owners := b.owners()
				owners.deploy.origin = func(at string) (string, error) {
					if at == root {
						return "git@github.com:other/project.git", nil
					}
					return "git@github.com:example/project.git", nil
				}
				command, _ := findIntentAction("deploy", "status")
				var out bytes.Buffer
				for _, args := range [][]string{nil, {"--json"}} {
					out.Reset()
					if code := runIntentIn(command, args, &out, &out, b.root, owners); code != 0 || strings.Contains(out.String(), `"automatic"`) || strings.Contains(out.String(), "not deployed automatically: ") {
						t.Fatalf("other project = %d, %s", code, out.String())
					}
				}
				return
			}
			automaticStatus(b.deployVerbBed, code, commandA, deploy.RunSkipped, reason)
		})
	}
	for _, name := range []string{"unreadable-push-record", "unreadable-outcome-record", "unreadable-deploy-record", "empty-push-record", "damaged-push-line"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBoundCommandBed(t)
			automaticLane(b.deployVerbBed, commandA)
			b.seed(commandA0)
			automaticAppend(b.deployVerbBed, commandA, deploy.RunFailed, "build-failed: compile error")
			path := filepath.Join(plain.Dir(b.root), "pushes.jsonl")
			want := ""
			switch name {
			case "unreadable-push-record":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				want = "the lane's push record can't be read"
			case "unreadable-outcome-record":
				automaticWrite(t, filepath.Join(b.dir(), "automatic.jsonl"), []byte("{"))
				want = "the lane's deploys can't be read"
			case "unreadable-deploy-record":
				automaticWrite(t, filepath.Join(b.dir(), "deploys.jsonl"), []byte("{"))
				want = "the lane's deploys can't be read"
			case "empty-push-record":
				automaticWrite(t, path, nil)
				automaticStatus(b.deployVerbBed, 0, "", "", "")
				return
			case "damaged-push-line":
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				automaticWrite(t, path, append(data, []byte("{\n")...))
				automaticStatus(b.deployVerbBed, 0, commandA, deploy.RunFailed, "build-failed: compile error")
				return
			}
			_, text := b.run("deploy", "status")
			if !strings.Contains(text, want) {
				t.Fatalf("status lacks %q: %s", want, text)
			}
			if name == "unreadable-push-record" {
				_, result := b.result("deploy", "status")
				if !strings.Contains(text, "not deployed automatically: "+want) || !strings.Contains(text, "→ metasystem deploy now --commit") || !strings.HasPrefix(result.Summary, "not deployed automatically: "+want) || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem deploy now --commit <main's tip>" {
					t.Fatal(text)
				}
			}
		})
	}
}

func TestLandingPushDeployLaunchFailureKeepsOutcome(t *testing.T) {
	t.Parallel()
	b := newBoundCommandBed(t)
	automaticLane(b.deployVerbBed, commandA)
	b.seed(commandA0)
	b.pushed = plain.PushOutcome{Old: commandA0, Commit: commandA, Changed: true}
	owners := b.pushOwners()
	owners.deploy.launch = func([]string, string, string) (int64, error) { return 0, errors.New("launch unavailable") }
	code, result := b.push(owners)
	if code != 0 || !detailsHave(result, "metasystem deploy now could not be started") {
		t.Fatalf("push = %d, %+v", code, result)
	}
	lines, err := deploy.AutomaticLines(b.dir())
	if err != nil || len(lines) != 1 || lines[0].Commit != commandA || lines[0].Outcome != deploy.AutomaticLaunched {
		t.Fatalf("launch failure = %+v, %v", lines, err)
	}
	automaticStatus(b.deployVerbBed, 0, commandA, deploy.AutomaticLaunched, "its run has reported no outcome")
}
