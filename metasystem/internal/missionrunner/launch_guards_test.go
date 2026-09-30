package missionrunner

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The launch spine's guard ladder and armAndPreflight's refusal branches,
// driven with a stub arming neighbor (Phase 6). The stub stands in for the
// checkout engine's `up` entry only — arming itself has its own fixtures; the
// unit here is the ORCHESTRATION: sequence, refusal wording, and the handoff
// into contract preflight.

func stubArming(engine *Engine, result verbresult.Result, err error) {
	engine.ArmSupervision = func([]string) (verbresult.Result, error) { return result, err }
}

// upArmedEnvelope is the line an arming engine stub prints for up --json.
const upArmedEnvelope = `{"schemaVersion":1,"verb":"up","targets":[],"outcome":"confirmed","summary":"supervision is armed","data":{"outcome":"armed"}}`

// upAnswered is up's --json envelope with outcome as its typed data, read
// the way the runner reads the real one.
func upAnswered(t testing.TB, outcome string, exit int) verbresult.Result {
	t.Helper()
	result := verbresult.FromError("up", exit, nil, up.Data{Outcome: outcome})
	result.Summary = "up ended " + outcome
	var printed bytes.Buffer
	if err := verbresult.Write(&printed, result); err != nil {
		t.Fatal(err)
	}
	read, err := verbresult.Read(printed.Bytes(), "up", exit, "")
	if err != nil {
		t.Fatal(err)
	}
	return read
}

func TestLaunchGuardLadder(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-launch"}
	os.MkdirAll(engine.missionDir(), 0o755)
	statePath := filepath.Join(engine.missionDir(), "state.json")

	// Resume with no state at all.
	if err := engine.launch("resume", false); err == nil ||
		!strings.Contains(err.Error(), "has started here") {
		t.Fatalf("resume without state: %v", err)
	}
	// Start over an existing state steers to resume.
	os.WriteFile(statePath, []byte(`{}`), 0o644)
	if err := engine.launch("start", false); err == nil ||
		!strings.Contains(err.Error(), "already exists; metasystem mission resume mr-launch continues it") {
		t.Fatalf("start over state: %v", err)
	}
	// Resume over a malformed state surfaces the verifier's refusal.
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	if err := engine.launch("resume", false); err == nil {
		t.Fatal("resume verified a malformed state")
	}
}

func TestArmAndPreflightRefusals(t *testing.T) {
	// No checkout engine at all: the arm step refuses by name.
	bare := &Engine{Root: t.TempDir(), Mission: "mr-arm-a"}
	if err := bare.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") {
		t.Fatalf("armless root: %v", err)
	}

	// An arming that fails: same named refusal, up's summary carried.
	failing := &Engine{Root: t.TempDir(), Mission: "mr-arm-b"}
	refused := upAnswered(t, "failed", 1)
	refused.Summary = "deliberate refusal"
	stubArming(failing, refused, nil)
	if err := failing.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") ||
		!strings.Contains(err.Error(), "deliberate refusal") {
		t.Fatalf("failing armer: %v", err)
	}

	// An answer that could not be read is no arming, whatever it said.
	unread := &Engine{Root: t.TempDir(), Mission: "mr-arm-d"}
	stubArming(unread, verbresult.Result{Outcome: verbresult.Unknown}, errors.New("up printed no readable result"))
	if err := unread.armAndPreflight("start"); err == nil || !strings.Contains(err.Error(), "supervision did not arm") {
		t.Fatalf("unreadable armer: %v", err)
	}

	// Mission startup keeps requiring armed: a confirmed up that is only
	// an advisor is no arming.
	advisor := &Engine{Root: t.TempDir(), Mission: "mr-arm-e"}
	stubArming(advisor, upAnswered(t, "advisor", 0), nil)
	if err := advisor.armAndPreflight("start"); err == nil || !strings.Contains(err.Error(), "not armed") {
		t.Fatalf("advisor armer: %v", err)
	}

	// An armer that reports the typed armed outcome hands off to contract preflight, which
	// refuses the absent contract by name.
	armed := &Engine{Root: t.TempDir(), Mission: "mr-arm-c"}
	stubArming(armed, upAnswered(t, "armed", 0), nil)
	if err := armed.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "refused by preflight") {
		t.Fatalf("preflight handoff: %v", err)
	}
}

// Driven against the real engine's up (T5): a checkout it refuses arrives
// as the envelope's summary, and the mission does not start.
func TestArmAndPreflightReadsTheRealUpsEnvelope(t *testing.T) {
	t.Setenv("METASYSTEM_BIN", freshEngineBinary(t))
	engine := &Engine{Root: t.TempDir(), Mission: "mr-arm-real"}
	err := engine.armAndPreflight("start")
	if err == nil || !strings.Contains(err.Error(), "supervision did not arm") || !strings.Contains(err.Error(), "not inside a git repository") {
		t.Fatalf("the real up's refusal read as %v", err)
	}
}

// EM-13, EM-24: a start names a mission without a contract before anything
// is armed, and a resume names a mission that never started, with the
// public command that starts it.
func TestLaunchNamesAMissingMission(t *testing.T) {
	t.Parallel()
	engine := &Engine{Root: t.TempDir(), Mission: "mr-none"}
	armed := 0
	answer := upAnswered(t, "armed", 0)
	engine.ArmSupervision = func([]string) (verbresult.Result, error) { armed++; return answer, nil }
	err := engine.launch("start", false)
	if err == nil || !strings.Contains(err.Error(), "no mission contract mr-none") || armed != 0 {
		t.Fatalf("start without a contract: %v (armed %d)", err, armed)
	}
	err = engine.launch("resume", false)
	if err == nil || !strings.Contains(err.Error(), "no mission mr-none has started here") || !strings.Contains(err.Error(), "metasystem mission start mr-none") {
		t.Fatalf("resume without state: %v", err)
	}
}
