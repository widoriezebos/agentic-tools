package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewStageWritesOnlyThreeFields(t *testing.T) {
	f := newConformanceFixture(t)
	f.writeImplementer("", "source.txt")
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "reviewed change\n")
	expectConformance(t, f, "review", 0, "reviewedTree=")

	path := filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var review map[string]any
	if err := json.Unmarshal(data, &review); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"diffArtifact": true, "implementerJob": true, "reviewedTree": true}
	if len(review) != len(want) {
		t.Fatalf("review.json fields = %v, want exactly %v", review, want)
	}
	for key := range review {
		if !want[key] {
			t.Fatalf("review.json contains unexpected field %q: %v", key, review)
		}
	}
}
