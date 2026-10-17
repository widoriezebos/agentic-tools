package missionrunner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/uptest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestRequireArmedAcceptsActualPendingAdoption(t *testing.T) {
	t.Parallel()
	result := uptest.Pending(t)
	envelope := verbresult.FromError("up", result.ExitCode(), nil, result.Data())
	var output bytes.Buffer
	if err := verbresult.Write(&output, envelope); err != nil {
		t.Fatal(err)
	}
	read, err := verbresult.Read(output.Bytes(), "up", result.ExitCode(), "")
	if err := requireArmed(read, err); err != nil {
		t.Fatalf("pending adoption broke mission launch: %v", err)
	}
	result.Outcome = "partial"
	if err := requireArmed(verbresult.FromError("up", result.ExitCode(), nil, result.Data()), nil); err == nil {
		t.Fatal("mission launch accepted partial supervision")
	}
}

func TestRequireArmedRefusesPersonTerminal(t *testing.T) {
	t.Parallel()
	result := up.Result{Outcome: "person-terminal", TerminalPid: 4300}
	if err := requireArmed(verbresult.FromError("up", result.ExitCode(), nil, result.Data()), nil); err == nil || !strings.Contains(err.Error(), "person-terminal, not armed") {
		t.Fatalf("a terminal cannot launch a mission as an agent main: %v", err)
	}
}
