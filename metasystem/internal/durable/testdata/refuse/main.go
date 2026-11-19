// Command refuse is a normal (non-test) binary that tries to lower its own
// durability. It exits 0 only when the lowering is refused and its sync runs
// at full.
package main

import (
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

func main() {
	if durable.Current() != "full" || durable.UseKernelInThisTestProcess() == nil || durable.Current() != "full" {
		os.Exit(1)
	}
	// A sync in a normal binary freezes and runs at full.
	file, err := os.CreateTemp("", "durable-refuse-")
	if err != nil {
		os.Exit(1)
	}
	syncErr := durable.Sync(file)
	closeErr := file.Close()
	removeErr := os.Remove(file.Name())
	if syncErr != nil || closeErr != nil || removeErr != nil || durable.Current() != "full" {
		os.Exit(1)
	}
}
