package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// engineChildExited names a FIFO an engine child of these tests holds open
// for writing from its start to its exit, so a test waits for a detached
// child's end by reading the FIFO to its end (waitForEngineChildExit).
const engineChildExited = "METASYSTEM_TEST_ENGINE_CHILD_EXITED"

// engineChildExitHold is that FIFO's write end, held for the process's life.
var engineChildExitHold *os.File

// The command's tests share one process-wide home, and a lane registered by
// one test would refuse the next test's landing checkout (U12). Every test
// here therefore runs without a host lane, on each seat's own
// landing.batch-root as before U12; a test of the lane passes its own home
// through the lane seams or the helpers that take one. This binary run as
// an engine child (GO_WANT_BATCH_E2E_COMMAND=1, such as a detached landing
// prove) resolves the host lane as the engine does, from the run-scoped
// home its test gave it.
func init() {
	if os.Getenv("GO_WANT_BATCH_E2E_COMMAND") == "1" {
		// Only after TestMain's re-exec under its own name: a hold opened
		// before it would close at the exec.
		if fifo := os.Getenv(engineChildExited); fifo != "" && filepath.Base(os.Args[0]) == proofrun.TestHostLoadCommandName("0") {
			engineChildExitHold, _ = os.OpenFile(fifo, os.O_WRONLY, 0)
		}
		return
	}
	batchowner.LandingLaneHome = func() (string, error) {
		return "", errors.New("the command's tests keep no host landing lane")
	}
}
