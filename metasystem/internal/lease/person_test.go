package lease

import (
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// W9: a caller a live general power of attorney admits is a person for the
// person-act sites, carrying the grant; ClassifyAt's own answer is kept for
// everyone else, and a person stays a person without asking the grant.
func TestClassifyPersonAtAsksTheGrantAfterTheClassifier(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)
	grant := humanauthority.HelmGrant{By: "wido", Class: ClassMain, Grant: "01M-grant"}
	var asked []string
	admit := func(root string, pid int64, at time.Time) (humanauthority.HelmGrant, bool) {
		asked = append(asked, root)
		if !at.Equal(now) || pid != 80 {
			t.Errorf("asked at %v for %d", at, pid)
		}
		return grant, root == "/state"
	}
	main := Classification{Class: ClassMain, MainId: "main-1"}
	classify := func(string, string, int64) (Classification, error) { return main, nil }
	got, err := classifyPersonAt("/checkout", "/state", 80, now, classify, admit)
	if err != nil || got.Class != ClassHuman || got.Attorney == nil || *got.Attorney != grant || got.MainId != "main-1" {
		t.Fatalf("the grantee is a person here: %+v %v", got, err)
	}
	if !reflect.DeepEqual(asked, []string{"/checkout", "/state"}) {
		t.Fatalf("asked %v, want the checkout then the state root", asked)
	}
	asked = nil
	if got, err := classifyPersonAt("/checkout", "/elsewhere", 80, now, classify, admit); err != nil || !reflect.DeepEqual(got, main) {
		t.Fatalf("without a grant the classifier's answer stands: %+v %v", got, err)
	}
	if got, _ := classifyPersonAt("/state", "/state", 80, now, classify, nil); !reflect.DeepEqual(got, main) {
		t.Fatalf("with no seam the classifier's answer stands: %+v", got)
	}
	asked = nil
	human := Classification{Class: ClassHuman}
	if got, _ := classifyPersonAt("/state", "/state", 80, now, func(string, string, int64) (Classification, error) { return human, nil }, admit); !reflect.DeepEqual(got, human) || len(asked) != 0 {
		t.Fatalf("a person is not asked about: %+v %v", got, asked)
	}
	if _, err := classifyPersonAt("/state", "/state", 80, time.Time{}, classify, admit); err != nil {
		t.Fatalf("a zero clock is no grant, not an error: %v", err)
	}
	if got, _ := classifyPersonAt("/state", "/state", 80, time.Time{}, classify, admit); got.Class != ClassMain {
		t.Fatalf("a zero clock admitted: %+v", got)
	}
}
