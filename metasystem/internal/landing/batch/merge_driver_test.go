package batch

import (
	"os"
	"strings"
	"testing"
)

func TestEveryBatchThreeWayApplyUsesTestingMergeDriver(t *testing.T) {
	uses, threeWay := 0, 0
	for _, path := range []string{"join.go", "land_apply.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		uses += strings.Count(text, `exec.Command("git", batchMergeGitCommand(`)
		threeWay += strings.Count(text, `"--3way"`)
	}
	if uses != threeWay || threeWay != 3 {
		t.Fatalf("batch three-way apply sites using the testing merge driver = %d/%d, want 3/3", uses, threeWay)
	}
}
