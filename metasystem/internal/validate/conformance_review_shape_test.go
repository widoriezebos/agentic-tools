package validate

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReviewStageWritesOnlyThreeFields(t *testing.T) {
	data, err := encodeConformanceReview(filepath.Join("artifacts", "diff.patch"), "impl", "opaque-reviewed-tree")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("review.json has no trailing newline: %q", data)
	}
	var review map[string]string
	if err := json.Unmarshal(data, &review); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"diffArtifact": "diff.patch", "implementerJob": "impl", "reviewedTree": "opaque-reviewed-tree",
	}
	if !reflect.DeepEqual(review, want) {
		t.Fatalf("review.json fields = %v, want exactly %v", review, want)
	}
}
