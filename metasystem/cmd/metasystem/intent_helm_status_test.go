package main

import (
	"strings"
	"testing"
)

// helm status reads who holds the helm, since when and why, and changes
// nothing; with no signature the machinery is at the helm.
func TestHelmStatusReadsTheHelm(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, true)
	code, out := b.run("helm", "status")
	if code != 0 || !strings.Contains(out, "the machinery is at the helm") {
		t.Fatalf("helm status with no helm = %d:\n%s", code, out)
	}
	if code, out := b.run("helm", "take", "--reason", "coordinating by hand"); code != 0 {
		t.Fatalf("take = %d:\n%s", code, out)
	}
	before, _ := b.signature()
	code, out = b.run("helm", "status")
	after, _ := b.signature()
	if code != 0 || !strings.Contains(out, "Wido") || !strings.Contains(out, "coordinating by hand") || before != after {
		t.Fatalf("helm status while taken = %d:\n%s", code, out)
	}
}

func init() {
	registerIdempotency("helm status", idemRead, "reads the helm signature, its enrollment and the seat's account; changes nothing", nil)
}
