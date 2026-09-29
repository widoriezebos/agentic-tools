package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A gate given no TMPDIR stages in its process's registered scratch root,
// never in an unowned os.TempDir() entry (Part B U1b-2); a TMPDIR it is
// given is used as given.
func TestGateTempDirWithoutTMPDIRIsTheProcessScratch(t *testing.T) {
	t.Parallel()
	scratch, err := diskstore.ProcessScratch()
	if err != nil {
		t.Fatal(err)
	}
	if got := tempDir(newEnvironment(nil)); got != scratch {
		t.Fatalf("the gate's temporary root without TMPDIR = %q, want the process scratch root %q", got, scratch)
	}
	if got := tempDir(newEnvironment([]string{"TMPDIR=/given"})); got != "/given" {
		t.Fatalf("the gate's temporary root with TMPDIR=/given = %q", got)
	}
}
