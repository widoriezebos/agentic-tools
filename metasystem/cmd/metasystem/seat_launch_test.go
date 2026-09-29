package main

// `seat launch` at its edge under g1-s72: a record a signed-in session
// enrolled is authoritative for its machine and destination on a fresh
// invocation as on a resume (S72-01, round 2), and the pair beside it is
// refused. Both refusals come before any write and any step.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
)

func sessionEnrolledRecordFor(t *testing.T, from, machine, destination string) (string, []byte) {
	t.Helper()
	const id = "01M3SESSIONLAUNCH000000000"
	record := launch.Record{
		SchemaVersion: launch.SchemaVersion, Launch: id, Machine: machine, Destination: destination,
		Outcome: launch.OutcomeStarting, Steps: []launch.Step{},
		Enrollment: &launch.Enrollment{
			Kind: launch.EnrollmentHumanSession, Provider: "browser", Human: "wido",
			Session: "01M3SESSIONREFERENCE000000", At: "2026-09-29T08:00:00Z",
		},
	}
	if err := launch.Save(from, record); err != nil {
		t.Fatal(err)
	}
	path, err := launch.Path(from, id)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, data
}

func TestAFreshInvocationCannotRedirectASessionEnrolledRecord(t *testing.T) {
	t.Parallel()
	from := t.TempDir()
	parent := t.TempDir()
	destinationA := filepath.Join(parent, "agentic-tools-m1f")
	destinationB := filepath.Join(parent, "agentic-tools-m1g")
	for what, args := range map[string][]string{
		"another machine and destination": {"--machine", "m1g", "--destination", destinationB},
		"another destination":             {"--machine", "m1f", "--destination", destinationB},
		"another machine":                 {"--machine", "m1g", "--destination", destinationA},
		"the pair beside the record":      {"--machine", "m1f", "--destination", destinationA, "--temporary-human-word", "Wido says so", "--review-by", "2026-10-02"},
	} {
		path, before := sessionEnrolledRecordFor(t, from, "m1f", destinationA)
		var stdout bytes.Buffer
		code := runSeatLaunchTo(append([]string{"--from", from, "--record", path}, args...), &stdout)
		if code == 0 {
			t.Fatalf("%s: the verb ran (%q)", what, stdout.String())
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%s: the record changed:\n%s", what, after)
		}
		for _, destination := range []string{destinationA, destinationB} {
			if _, err := os.Lstat(destination); err == nil {
				t.Fatalf("%s: %s was created", what, destination)
			}
		}
		read, err := launch.LoadAt(path)
		if err != nil || read.Enrollment == nil || read.Machine != "m1f" || read.Destination != destinationA || len(read.Steps) != 0 {
			t.Fatalf("%s: record = %+v, %v", what, read, err)
		}
	}
}
