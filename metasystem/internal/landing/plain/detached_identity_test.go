package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"golang.org/x/sys/unix"
)

const detachedIdentityReport = "PLAIN_TEST_DETACHED_IDENTITY_REPORT"

type detachedIdentityObservation struct {
	Session, CallerSession   int
	Terminal, CallerTerminal bool
	Unit, Tree, Commit       string
}

func TestClassifyReplayDetachedFromEnrolledTerminal(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	sid, err := unix.Getsid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(b.checkout, "enrolled-terminal.json")
	data := []byte(fmt.Sprintf(`{"%d":{"terminal":true},"%d":{"terminal":true}}`, sid, os.Getpid()))
	if sid == os.Getpid() {
		data = []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, sid))
	}
	if err := os.WriteFile(table, data, 0600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(b.checkout, "replay-identity.json")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=" + shellquote.Token(table) + " " + detachedIdentityReport + "=" + shellquote.Token(report) + " PLAIN_TEST_CALLER_SESSION=" + strconv.Itoa(sid) + " " + shellquote.Token(executable) + " -test.run ^TestDetachedCheckIdentityReporter$ -test.timeout=30m"
	originalGit := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify parent^{tree}" {
			return "main-tree", nil
		}
		out, err := originalGit(dir, args...)
		if err == nil && len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			err = os.WriteFile(filepath.Join(args[3], "metasystem", "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600)
		}
		return out, err
	}
	b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) { return nil, nil }
	b.seams.CommandForCommit = func(string) (string, error) { return command, nil }
	b.seams.Person = &ActProvenance{Person: "Wido", Kind: "proof"}
	red := Result{Attempt: "saved-red", Commit: "commit", Tree: "tree", Result: Red, Scope: "impact", Cause: &Cause{Kind: "unclassified"}, Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}}
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(resultsPath(b.install), red); err != nil {
		t.Fatal(err)
	}
	classified, err := ClassifyAttempt(b.install, b.checkout, red.Attempt, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(report)
	if err != nil {
		t.Fatalf("replay did not report its identity: %v; result=%+v", err, classified)
	}
	var got detachedIdentityObservation
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Terminal || !got.CallerTerminal || got.Session == sid || got.CallerSession != sid || got.Unit != "u/a" || got.Tree != "main-tree" || got.Commit != "parent" {
		t.Fatalf("replay carried the caller's terminal: %+v", got)
	}
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || running.Person == nil || running.Person.Kind != "classification" || running.Person.Person != "Wido" || classified.ClassificationPerson == nil || classified.ClassificationPerson.Person != "Wido" || classified.ClassificationPerson.Kind != "classification" || classified.Result != Red {
		t.Fatalf("classification lost the person's act: %+v running=%+v err=%v", classified, running, err)
	}
}

func TestDetachedCheckIdentityReporter(t *testing.T) {
	t.Parallel()
	report := os.Getenv(detachedIdentityReport)
	if report == "" {
		t.Skip("only runs as a proof reporter")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := fixtureauth.New(root)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := unix.Getsid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	caller, err := strconv.Atoi(os.Getenv("PLAIN_TEST_CALLER_SESSION"))
	if err != nil {
		t.Fatal(err)
	}
	terminal, known := identity.ControllingTerminal(int64(sid), auth.Identity())
	callerTerminal, callerKnown := identity.ControllingTerminal(int64(caller), auth.Identity())
	if !known || !callerKnown {
		t.Fatal("terminal observations are unavailable")
	}
	got := detachedIdentityObservation{sid, caller, terminal, callerTerminal, os.Getenv("LANDING_ONLY"), os.Getenv("LANDING_TREE"), os.Getenv("LANDING_COMMIT")}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, data, 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Print("LANDING-CHECKED\t0\n")
}
