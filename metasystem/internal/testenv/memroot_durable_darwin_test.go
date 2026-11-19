package testenv

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

func TestMemoryRootMatchesDurabilityMount(t *testing.T) {
	t.Parallel()
	if durable.MemoryVolumeRoot() != "/private"+defaultMemoryRoot.path() {
		t.Fatal("durability mount name differs from the test harness memory root")
	}
}
