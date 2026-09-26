package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// TestIntentManualDiffReview drives the public diagnostic review subjects
// through the launch owner's standalone read in a real Git checkout with a
// fake reader: current changes captured from a nested directory, a supplied
// patch, the rejoin of an identical request, show and wait by the public ref,
// and the source checkout's HEAD, index and files left unchanged. The result
// never claims to be a goal review.
func TestIntentManualDiffReview(t *testing.T) {
	c := newConnectionBed(t)
	owners := c.connectionOwners()
	root := c.root()
	nested := filepath.Join(root, "app", "deep")
	os.MkdirAll(nested, 0o700)
	os.WriteFile(filepath.Join(root, "top.txt"), []byte("top-level change\n"), 0o644)
	connectionGit(t, root, "add", "top.txt")
	connectionGit(t, root, "commit", "-q", "-m", "tracked top-level file")
	os.WriteFile(filepath.Join(root, "top.txt"), []byte("top-level change, edited\n"), 0o644)
	os.WriteFile(filepath.Join(root, "app", "new.txt"), []byte("untracked\n"), 0o644)
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Examine the change for defects.\n"), 0o600)
	head := connectionGit(t, root, "rev-parse", "HEAD")
	indexBefore, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	statusBefore := connectionGit(t, root, "status", "--porcelain")

	run := func(cwd string, args ...string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, cwd, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	code, first := run(nested, "review", "changes", "--brief", brief)
	data, _ := first.Data.(map[string]any)
	ref, _ := data["ref"].(string)
	files := strings.Fields(strings.Trim(strings.ReplaceAll(strings.ReplaceAll(jsonText(data["files"]), `"`, " "), ",", " "), "[] "))
	if code != 0 || first.Outcome != intentConfirmed || !strings.HasPrefix(ref, "read-") || data["note"] != diagnosticReviewNote || data["complete"] != true ||
		!slices.Contains(files, "top.txt") || !slices.Contains(files, "app/new.txt") || data["base"] != head {
		t.Fatalf("review changes from a nested directory: %d %+v", code, first)
	}
	if strings.Contains(first.Summary, "approved") || first.Next != nil && slices.Contains(first.Next.Argv, "land") {
		t.Fatalf("diagnostic feedback offered delivery: %+v", first)
	}
	launched := c.starterLaunches()
	code, again := run(root, "review", "changes", "--brief", brief)
	if againData, _ := again.Data.(map[string]any); code != 0 || againData["ref"] != ref || c.starterLaunches() != launched {
		t.Fatalf("an identical request did not rejoin: %d %+v (launches %d then %d)", code, again, launched, c.starterLaunches())
	}
	for _, verb := range []string{"show", "wait"} {
		if code, shown := run(root, verb, "review", ref); code != 0 || shown.Outcome != intentConfirmed {
			t.Fatalf("%s review %s: %d %+v", verb, ref, code, shown)
		}
	}
	if connectionGit(t, root, "rev-parse", "HEAD") != head || connectionGit(t, root, "status", "--porcelain") != statusBefore {
		t.Fatal("the diagnostic review changed the source checkout")
	}
	if indexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index")); !bytes.Equal(indexBefore, indexAfter) {
		t.Fatal("the diagnostic review changed the source index")
	}
	if content, _ := os.ReadFile(filepath.Join(root, "top.txt")); string(content) != "top-level change, edited\n" {
		t.Fatalf("the source file changed: %q", content)
	}

	patch := filepath.Join(t.TempDir(), "change.patch")
	os.WriteFile(patch, []byte("diff --git a/top.txt b/top.txt\n--- a/top.txt\n+++ b/top.txt\n@@ -1 +1 @@\n-top-level change\n+patched\n"), 0o600)
	code, supplied := run(root, "review", "diff", patch, "--brief", brief)
	suppliedData, _ := supplied.Data.(map[string]any)
	if code != 0 || supplied.Outcome != intentConfirmed || suppliedData["kind"] != "patch" || suppliedData["ref"] == ref {
		t.Fatalf("review diff: %d %+v", code, supplied)
	}
	code, empty := run(root, "review", "changes")
	if code != 2 || empty.Outcome != intentRefused || !strings.Contains(empty.Summary, "--brief") {
		t.Fatalf("review changes without a brief: %d %+v", code, empty)
	}
	code, unknown := run(root, "show", "review", "read-000000000000000000000000")
	if code == 0 || unknown.Outcome == intentConfirmed {
		t.Fatalf("an unknown ref was shown: %d %+v", code, unknown)
	}
}

func jsonText(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

// starterLaunches counts the launch records the bed's manager holds.
func (c *connectionBed) starterLaunches() int {
	records, err := c.manager.Store.List()
	if err != nil {
		c.t.Fatal(err)
	}
	return len(records)
}

// holdingReadStarter keeps read launches running with a live supervisor and
// child until released; other launches go to the bed's own starter.
type holdingReadStarter struct {
	c    *connectionBed
	hold bool
}

func (s *holdingReadStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, _ := s.c.manager.Store.Read(id)
	if !s.hold || record.Kind != "read" {
		return s.c.StartSupervisor(id, state)
	}
	s.c.manager.Store.Update(id, func(current *launch.Record) error {
		supervisor, child := workRef(10), workRef(20)
		current.Supervisor, current.Child, current.State = &supervisor, &child, launch.Running
		return nil
	})
	return workRef(10), nil
}

// flipProber reports the held processes alive until they are declared dead.
type flipProber struct{ dead bool }

func (p *flipProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if !p.dead && (pid == 10 || pid == 20) {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

// TestIntentManualDiffReviewStopAndRetry drives stop and retry of a
// diagnostic review through the public commands and the launch manager's
// real custody: a running read whose processes cannot be proved stopped is
// reported uncertain and a retry is refused toward stop; once the processes
// are proved dead the stop is recorded, and the printed retry runs exactly
// one new attempt of the same frozen request.
func TestIntentManualDiffReviewStopAndRetry(t *testing.T) {
	c := newConnectionBed(t)
	owners := c.connectionOwners()
	root := c.root()
	starter, prober := &holdingReadStarter{c: c, hold: true}, &flipProber{}
	c.manager.Supervisor, c.manager.Prober = starter, prober
	os.WriteFile(filepath.Join(root, "change.txt"), []byte("to be read\n"), 0o644)
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Examine the change.\n"), 0o600)
	run := func(args ...string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, root, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	code, started := run("review", "changes", "--brief", brief)
	ref, _ := resultData(t, started)["ref"].(string)
	if started.Outcome != intentInProgress || ref == "" || started.Next == nil || !slices.Equal(started.Next.Argv[1:4], []string{"wait", "review", ref}) {
		t.Fatalf("a held read is in progress: code=%d %+v", code, started)
	}
	launches := c.starterLaunches()
	code, stopped := run("stop", "review", ref)
	if stopped.Outcome != intentPartial || resultData(t, stopped)["uncertain"] == nil || stopped.Next == nil || !slices.Equal(stopped.Next.Argv[1:], []string{"stop", "review", ref}) {
		t.Fatalf("an unprovable stop is uncertain: code=%d %+v", code, stopped)
	}
	code, early := run("review", "changes", "--brief", brief, "--retry", "1")
	if early.Outcome != intentRefused || early.Next == nil || !slices.Equal(early.Next.Argv[1:], []string{"stop", "review", ref}) || c.starterLaunches() != launches {
		t.Fatalf("a retry before the attempt ends is refused toward stop: code=%d %+v", code, early)
	}
	prober.dead = true
	code, proved := run("stop", "review", ref)
	if proved.Outcome != intentInProgress || resultData(t, proved)["uncertain"] != nil || !strings.Contains(proved.Summary, "stopping") ||
		proved.Next == nil || !slices.Equal(proved.Next.Argv[1:4], []string{"wait", "review", ref}) {
		t.Fatalf("a proved stop: code=%d %+v", code, proved)
	}
	code, ended := run("wait", "review", ref, "--timeout", "1s")
	data := resultData(t, ended)
	if ended.Outcome != intentPartial || data["state"] != "stopped" || ended.Next == nil || !slices.Contains(ended.Next.Argv, "--retry") || c.starterLaunches() != launches {
		t.Fatalf("the stopped attempt offers its retry and starts nothing: code=%d %+v", code, ended)
	}
	starter.hold = false
	code, retried := run(ended.Next.Argv[1:]...)
	retriedData := resultData(t, retried)
	if code != 0 || retried.Outcome != intentConfirmed || retriedData["ref"] != ref || retriedData["attempt"] != float64(2) {
		t.Fatalf("the printed retry runs one new attempt of the same request: code=%d %+v", code, retried)
	}
	if code, again := run(ended.Next.Argv[1:]...); code != 0 || resultData(t, again)["attempt"] != float64(2) {
		t.Fatalf("repeating the retry rejoins it: code=%d %+v", code, again)
	}
}
