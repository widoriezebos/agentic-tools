package main

// The relay side of the delegate seam: runtime-named verbs stay (the
// architecture sanctions them as the shell's addressing), but their
// bodies go through the seam's port registry instead of calling the
// owner packages directly — the seam is the one lookup point the
// later slices swap implementations behind.

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// delegatePorts resolves a runtime's registered read-side ports for a
// relay. A missing registration is a wiring defect: refuse loudly.
func delegatePorts(verb, runtime string) (delegate.Ports, bool) {
	ports, err := delegate.PortsFor(runtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", verb, err)
		return delegate.Ports{}, false
	}
	return ports, true
}
