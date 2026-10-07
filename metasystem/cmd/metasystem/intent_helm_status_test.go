package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
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
	if code != 0 || !strings.Contains(out, "Wido") || !strings.Contains(out, "coordinating by hand") || !reflect.DeepEqual(before, after) {
		t.Fatalf("helm status while taken = %d:\n%s", code, out)
	}
}

func TestHelmStatusReturnRemedyNamesThePerson(t *testing.T) {
	t.Parallel()
	for _, malformed := range []bool{false, true} {
		name := "held"
		if malformed {
			name = "malformed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newHelmBed(t, 20, true)
			b.wantTake(nil, 0, "proven", "Wido")
			if malformed {
				helmMust(t, os.WriteFile(filepath.Join(b.root, ".git", "metasystem", "helm.json"), []byte("{"), 0600))
			}
			code, out := b.run("helm", "status", "--json")
			if code != 0 || !strings.Contains(out, "metasystem helm return --repo ") || !strings.Contains(out, humanauthority.PersonActRemedy("")[:20]) {
				t.Fatalf("status must name the person's return at their enrolled terminal: exit %d:\n%s", code, out)
			}
		})
	}
}

func init() {
	registerIdempotency("helm status", idemRead, "reads the helm signature, its enrollment and the seat's account; changes nothing", nil)
}
