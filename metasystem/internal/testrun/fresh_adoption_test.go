package testrun

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/uptest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestRearmReadsActualPendingAdoptionEnvelope(t *testing.T) {
	t.Parallel()
	result := uptest.Pending(t)
	var output bytes.Buffer
	if err := verbresult.Write(&output, verbresult.FromError("up", result.ExitCode(), nil, result.Data())); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	answer := filepath.Join(root, "answer.json")
	if err := os.WriteFile(answer, output.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(fmt.Sprintf("#!/bin/sh\nexec /bin/cat '%s'\n", answer)), 0755); err != nil {
		t.Fatal(err)
	}
	outcome, err := landedRearmUp(context.Background(), root, root)
	if err != nil || outcome.Outcome != "armed" {
		t.Fatalf("pending adoption broke rearm reader: %+v %v", outcome, err)
	}
}
