package missionrunner

import (
	"bytes"
	"testing"

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
