package launch

// The concrete host answers the sequencer reads identity from: the stamp in
// an engine, the steward's enrolment, the supervision owner lock, and the
// liveness of a launch's own process.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

func TestStampRefusesWhatIsNotAnEngine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := (OSHost{}).Stamp(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("stamp of an absent binary succeeded")
	}
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(text, []byte("not an executable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stamp, err := (OSHost{}).Stamp(text); err == nil {
		t.Fatalf("stamp of a text file = %q, want an error", stamp)
	}
}

// The running test binary is a Go executable with build information; the
// stamp read out of it is one word, whatever its link flags were.
func TestStampReadsAGoExecutablesBuildInformation(t *testing.T) {
	t.Parallel()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := (OSHost{}).Stamp(binary)
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if strings.ContainsAny(stamp, " \t\r\n\"'") {
		t.Fatalf("stamp = %q, want one word", stamp)
	}
}

func TestEnrolledIsFalseForAnInstallationWithNoEnrolledEngine(t *testing.T) {
	t.Parallel()
	if id, enrolled := (OSHost{}).Enrolled(t.TempDir()); enrolled || id != (Identity{}) {
		t.Fatalf("Enrolled = %+v, %v", id, enrolled)
	}
}

func writeSupervisionOwner(t *testing.T, owner any) string {
	t.Helper()
	installation := t.TempDir()
	dir := filepath.Join(installation, "artifacts", "agents", "supervision", "lock.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var data []byte
	if text, isText := owner.(string); isText {
		data = []byte(text)
	} else {
		encoded, err := json.Marshal(owner)
		if err != nil {
			t.Fatal(err)
		}
		data = encoded
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return installation
}

func TestSupervisionUpIsFalseWithoutALiveOwner(t *testing.T) {
	t.Parallel()
	pid := int64(os.Getpid())
	born, alive := identity.ProcessBirth(pid)
	if !alive {
		t.Fatal("this process has no readable birth")
	}
	cases := map[string]string{
		"no lock":   t.TempDir(),
		"malformed": writeSupervisionOwner(t, "{not json"),
		"no pid":    writeSupervisionOwner(t, map[string]any{"pid": 0, "pidStartedAt": born.Unix()}),
		"no start":  writeSupervisionOwner(t, map[string]any{"pid": pid, "pidStartedAt": 0}),
		// This pid, but a start time an hour before it began: a reused pid.
		"another process": writeSupervisionOwner(t, map[string]any{"pid": pid, "pidStartedAt": born.Unix() - 3600}),
	}
	for name, installation := range cases {
		if (OSHost{}).SupervisionUp(installation) {
			t.Fatalf("%s: SupervisionUp = true", name)
		}
	}
}

func TestLiveJudgesTheProcessByItsPidAndStartTime(t *testing.T) {
	t.Parallel()
	pid := os.Getpid()
	own := Identify(pid)
	if own.PID != pid || own.StartedAt <= 0 {
		t.Fatalf("Identify = %+v", own)
	}
	if !Live(own) {
		t.Fatal("this process, identified, is not live")
	}
	if !Live(Process{PID: pid}) {
		t.Fatal("a pid with no recorded start time is not live")
	}
	if Live(Process{PID: pid, StartedAt: own.StartedAt - 3600}) {
		t.Fatal("a reused pid reads as the launch that started")
	}
	for _, dead := range []Process{{}, {PID: -1}} {
		if Live(dead) {
			t.Fatalf("Live(%+v) = true", dead)
		}
	}
}

func TestNewNamespaceIsALaunchesOwnFetchNamespace(t *testing.T) {
	t.Parallel()
	first, err := NewNamespace()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewNamespace()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, seat.FetchNamespacePrefix+"/") || first == second {
		t.Fatalf("namespaces = %q, %q", first, second)
	}
}
