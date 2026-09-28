package delegation_test

// Ports of dispatch-fixtures.sh lines 1-1227 and 5675-5867 (slice p1): the
// brain fence scenarios, the steward continuation's dispatcher legs, and
// the roster placeholder refusal. The scenario map lives beside the U6b
// evidence; each test names the fixture leg it replaces.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// corruptBrain writes an unreadable brain declaration: the fence refuses
// every fenced act with the human repair remedy, whatever the ledger.
func (b *bed) corruptBrain() {
	b.t.Helper()
	b.writeFile("artifacts/agents/brain.json", "{broken\n")
}

// agentFiles is every file under artifacts/agents with its bytes, the
// Go form of the fixture's record snapshot.
func (b *bed) agentFiles() map[string]string {
	b.t.Helper()
	files := map[string]string{}
	base := filepath.Join(b.root, "artifacts", "agents")
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, _ := filepath.Rel(base, path)
		files[relative] = string(content)
		return nil
	})
	if err != nil {
		b.t.Fatal(err)
	}
	return files
}

func sameFiles(before, after map[string]string) (string, bool) {
	var drift []string
	for path, content := range before {
		if after[path] != content {
			drift = append(drift, "changed or removed "+path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			drift = append(drift, "added "+path)
		}
	}
	sort.Strings(drift)
	return strings.Join(drift, "; "), len(drift) == 0
}

// fencedActs are the five acts the brain fence guards, each in the argv the
// fixture used against its pending and closed records.
var fencedActs = []struct {
	name string
	argv []string
}{
	{"dispatch", []string{"dispatch"}},
	{"follow-up", []string{"follow-up", "--job", "pending-job", "--message", "/missing"}},
	{"cancel", []string{"cancel", "--job", "pending-job"}},
	{"close", []string{"close", "--job", "closed-root"}},
	{"reap", []string{"reap"}},
}

// brain-delegate-refuses / brain-cancel-close-reap-refuse (corrupt legs):
// an unreadable declaration refuses dispatch, follow-up, cancel, close and
// reap with a typed BRAIN_REFUSED exit 2 naming the human repair, both
// inside the delegate boundary and on the legacy grammar outside it, and
// the refusal writes no job record and mutates none.
func TestPortP1CorruptBrainDeclarationFencesEveryActWithoutTouchingRecords(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeFile("artifacts/agents/jobs/pending-job.json", `{"jobId":"pending-job","status":"pending"}`+"\n")
	b.writeFile("artifacts/agents/jobs/closed-root.json", `{"jobId":"closed-root","status":"completed","chainClosed":true}`+"\n")
	b.corruptBrain()
	before := b.agentFiles()
	for _, act := range fencedActs {
		for _, env := range []struct {
			name string
			env  delegation.Env
		}{
			{"internal", delegation.Env{DelegateInternal: true, RecordOutcome: true}},
			{"legacy", delegation.Env{}},
		} {
			result := b.runEnv(env.env, act.argv...)
			requireExit(t, result, 2, b.stderr.String())
			line := strings.TrimSpace(string(result.Stdout))
			if !strings.HasPrefix(line, `{`) || !strings.Contains(line, `"outcome":"BRAIN_REFUSED"`) ||
				!strings.Contains(line, `"headline":"refused"`) || !strings.Contains(line, "until a human repairs it") {
				t.Fatalf("%s %s: stdout %q", act.name, env.name, result.Stdout)
			}
			if env.env.RecordOutcome && strings.TrimSpace(string(result.Outcome)) != line {
				t.Fatalf("%s %s: recorded outcome %q, printed %q", act.name, env.name, result.Outcome, line)
			}
			if !env.env.RecordOutcome && len(result.Outcome) != 0 {
				t.Fatalf("%s %s: recorded %q without the outcome file", act.name, env.name, result.Outcome)
			}
		}
	}
	if drift, same := sameFiles(before, b.agentFiles()); !same {
		t.Fatalf("the brain fence mutated agent state: %s", drift)
	}
	for _, prefix := range []string{"lease.", "adapter.", "records.", "git ", "guard."} {
		if calls := b.calls(prefix); len(calls) != 0 {
			t.Fatalf("a fenced act reached %s: %v", prefix, calls)
		}
	}
}

// brain-breach-stop-exempt: the stop custodian's breach-stop callback is not
// a fenced act. On a fenced checkout it reaches its own authority check and
// fails there, never with a brain refusal.
func TestPortP1BreachStopCallbackIsNotBrainFenced(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.corruptBrain()
	b.doubles.Lease.AuthorizeFunc = func(_ delegation.Invocation, mode delegation.AuthorityMode, _ string) error {
		return errors.New("control-plane write refused: fixture caller is not the stop custodian")
	}
	result := b.run("__breach-stop-goal", "--goal", "ship-widget", "--revision", "1")
	if result.ExitCode == 0 {
		t.Fatalf("breach-stop succeeded without authority: stdout %q", result.Stdout)
	}
	combined := string(result.Stdout) + string(result.Outcome) + b.stderr.String()
	if strings.Contains(combined, "BRAIN_REFUSED") || strings.Contains(combined, "brain never") || strings.Contains(combined, "until a human repairs it") {
		t.Fatalf("breach-stop was caught by the brain fence: %q", combined)
	}
	if calls := b.calls("lease.Authorize"); len(calls) != 1 || !strings.Contains(calls[0], "mode=stop-custodian") {
		t.Fatalf("breach-stop did not reach the stop-custodian gate: %v", b.doubles.Log.Calls())
	}
}

// steward-continuation, companion flags: a selection flag beside
// --steward-intent refuses before the authorization is consulted and
// before any job state exists.
func TestPortP1StewardIntentAdmitsNoCompanionSelectionFlags(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	before := b.agentFiles()
	for _, argv := range [][]string{
		{"--steward-intent", "n1", "--wait"},
		{"--steward-intent", "n1", "--role", "implementer"},
	} {
		result := b.run(argv...)
		requireExit(t, result, 2, b.stderr.String())
		if !strings.Contains(b.stderr.String(), "--steward-intent admits no other selection flags; the authorization decides") {
			t.Fatalf("%v: stderr %q", argv, b.stderr.String())
		}
		if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
			t.Fatalf("%v: outcome %v", argv, outcome)
		}
	}
	if calls := b.calls("steward."); len(calls) != 0 {
		t.Fatalf("a refused selection consulted the steward: %v", calls)
	}
	if drift, same := sameFiles(before, b.agentFiles()); !same {
		t.Fatalf("a refused selection left job state: %s", drift)
	}
}

// steward-continuation, replay: an authorization the steward owner refuses
// (a consumed intent) launches nothing and leaves no record.
func TestPortP1StewardIntentReplayLaunchesNothing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.doubles.Steward.AuthorizeFunc = func(_ delegation.Invocation, intent string) (steward.DispatchAuthorization, error) {
		return steward.DispatchAuthorization{}, errors.New("intent " + intent + " already launched; a replay authorizes nothing")
	}
	before := b.agentFiles()
	result := b.run("--steward-intent", "consumed-nonce")
	requireExit(t, result, 1, b.stderr.String())
	stderr := b.stderr.String()
	if !strings.Contains(stderr, "steward authorize-dispatch: intent consumed-nonce already launched; a replay authorizes nothing") ||
		!strings.Contains(stderr, "steward continuation refused: the authorization did not verify") {
		t.Fatalf("stderr %q", stderr)
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
		t.Fatalf("outcome %v", outcome)
	}
	if calls := b.calls("steward.AuthorizeDispatch"); len(calls) != 1 || !strings.Contains(calls[0], "intent=consumed-nonce") {
		t.Fatalf("authorization calls %v", calls)
	}
	for _, prefix := range []string{"adapter.", "records.", "guard."} {
		if calls := b.calls(prefix); len(calls) != 0 {
			t.Fatalf("a replayed authorization reached %s: %v", prefix, calls)
		}
	}
	if drift, same := sameFiles(before, b.agentFiles()); !same {
		t.Fatalf("a replayed authorization left job state: %s", drift)
	}
}

// steward-continuation, placeholder model: a role whose roster resolves to
// the template '<model>' refuses with REFUSED-ROSTER naming the key and the
// command that sets it, before any job record, claim or launch exists.
func TestPortP1RosterPlaceholderRefusesBeforeAnyJobState(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeFile("metasystem.conf", "metasystem.runtimes=fake\nrole.default.runtime=fake\nrole.default.model.fake=<model>\nrole.steward-continuation.runtime=fake\nevidence.root="+filepath.Join(b.root, "evidence")+"\n")
	for _, asset := range []string{"scripts/agents/roles/steward-continuation.md", "scripts/agents/roles/steward-continuation.requirements.json"} {
		b.installAsset(asset)
	}
	brief := b.writeFile("brief.md", "Working Mode: implement\n\nRepair it.\n")
	before := b.agentFiles()
	result := b.run("dispatch", "--role", "steward-continuation", "--brief", brief, "--destructive-reach", "MECHANICAL", "--job-id", "placeholder")
	requireExit(t, result, 1, b.stderr.String())
	stderr := b.stderr.String()
	for _, want := range []string{
		"role steward-continuation resolves to fake:<model>, a template placeholder from role.default.model.fake; set it with: metasystem settings set role.default.model.fake <the fake model this seat runs>, which writes",
		"metasystem.conf.local",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr %q lacks %q", stderr, want)
		}
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-ROSTER" {
		t.Fatalf("outcome %v", outcome)
	}
	if drift, same := sameFiles(before, b.agentFiles()); !same {
		t.Fatalf("a placeholder refusal left job state: %s", drift)
	}
	for _, prefix := range []string{"adapter.", "records."} {
		if calls := b.calls(prefix); len(calls) != 0 {
			t.Fatalf("a placeholder refusal reached %s: %v", prefix, calls)
		}
	}
}
