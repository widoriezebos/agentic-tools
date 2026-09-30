package main

import (
	"io"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// up --json (structured-output U2, T4/T5): stdout holds exactly one
// envelope whose data carries up's outcome and failed component as typed
// fields; up's own lines go to stderr, for a person.
func TestUpJSONAnswersWithOneEnvelope(t *testing.T) {
	root := t.TempDir()
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
	stdout, stderr, code := captureRelay(t, func(stdout, stderr io.Writer) int {
		return runUpWith([]string{"--metasystem-root", root, "--repo", root, "--retire", "--json",
			"--session", "up-json", "--pid", "1", "--start-time", "1"}, repositoryTop, stdout, stderr)
	})
	result, err := verbresult.Read([]byte(stdout), "up", code, stderr)
	if err != nil {
		t.Fatalf("up --json printed no readable envelope: %v\nstdout=%q", err, stdout)
	}
	var data up.Data
	if decodeErr := result.DecodeData(&data); decodeErr != nil || data.Outcome == "" {
		t.Fatalf("up --json data = %s (%v)", result.Data, decodeErr)
	}
	if strings.Contains(stdout, "up outcome=") || !strings.Contains(stderr, "up outcome="+data.Outcome) {
		t.Fatalf("up's lines belong on stderr under --json: stdout=%q stderr=%q", stdout, stderr)
	}
}

// A failed up is a failed envelope naming the component it stopped at.
func TestUpEnvelopeOfAFailedUpNamesItsStep(t *testing.T) {
	t.Parallel()
	armed := upEnvelope(up.Result{Outcome: "armed", ReArmed: "generation=2 previous=1"})
	var data up.Data
	if armed.Outcome != verbresult.Confirmed || armed.DecodeData(&data) != nil || data.Outcome != "armed" || data.ReArmed != "generation=2 previous=1" {
		t.Fatalf("armed envelope = %+v (%s)", armed, armed.Data)
	}
	failed := upEnvelope(up.Result{Outcome: "failed", Failed: "session-identity", Remedy: "pass --pid and --start-time"})
	data = up.Data{}
	if failed.Outcome != verbresult.Failed || failed.DecodeData(&data) != nil || data.Failed != "session-identity" ||
		!strings.Contains(failed.Summary, "session-identity") {
		t.Fatalf("failed envelope = %+v (%s)", failed, failed.Data)
	}
}
