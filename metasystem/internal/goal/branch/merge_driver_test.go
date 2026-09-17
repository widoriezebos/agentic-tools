package branch

import (
	"os"
	"strings"
	"testing"
)

func TestGoalBranchGitRunnersSupplyTestingMergeDriver(t *testing.T) {
	for _, path := range []string{"digest.go", "commit.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `exec.Command("git", branchGitCommand(repo, args...)...)`) {
			t.Fatalf("%s Git runner does not supply the testing merge driver", path)
		}
	}
}
