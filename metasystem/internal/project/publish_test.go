package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func designDraft(id, goal, status string) []byte {
	return []byte("# Reader design\n\n- Kind: design\n- Id: " + id + "\n- Status: " + status + "\n- Goals: " + goal + "\n\nBody.\n")
}

// TestDesignPublicationConflict: a valid draft is published only over the
// exact bytes it was proposed against; a changed document or an invalid
// proposal leaves the document intact, and the same publication repeats.
func TestDesignPublicationConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	destination := filepath.Join(root, "plans", "designs", "g.md")
	id := "01M3EFDSFTKWEMSDCP1BB7TDGA"
	draft := designDraft(id, "g", "draft")
	publication := DesignPublication{Destination: destination, Root: root, Draft: draft, RecordID: id, Goal: "g"}
	if already, err := PublishDesign(publication); err != nil || already {
		t.Fatalf("first publication: %v %v", already, err)
	}
	if already, err := PublishDesign(publication); err != nil || !already {
		t.Fatalf("replayed publication: %v %v", already, err)
	}
	os.WriteFile(destination, []byte("edited by the person\n"), 0o644)
	next := publication
	next.Draft, next.Expected, next.ExpectedPresent = designDraft(id, "g", "draft")[:0:0], draft, true
	next.Draft = append(designDraft(id, "g", "draft"), []byte("More.\n")...)
	if _, err := PublishDesign(next); !errors.Is(err, ErrDesignChanged) {
		t.Fatalf("a changed document: %v", err)
	}
	if current, _ := os.ReadFile(destination); string(current) != "edited by the person\n" {
		t.Fatalf("the person's edit was overwritten: %q", current)
	}
	for _, invalid := range [][]byte{designDraft(id, "g", "accepted"), designDraft("01M3EFDSFTKWEMSDCP1BB7TDGB", "g", "draft"), designDraft(id, "other", "draft"), []byte("not a record\n")} {
		bad := next
		bad.Expected, bad.Draft = []byte("edited by the person\n"), invalid
		if _, err := PublishDesign(bad); !errors.Is(err, ErrDesignInvalid) {
			t.Fatalf("invalid proposal %q: %v", invalid, err)
		}
	}
}
