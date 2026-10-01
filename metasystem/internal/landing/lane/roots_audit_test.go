package lane

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rootGuess is a lane path taken by guessing a root instead of from the
// registered Layout: probing for the nested module (batch.ModuleRoot) or
// asking git for the toplevel.
var rootGuess = regexp.MustCompile(`ModuleRoot\(|TopLevel\(\)|--show-toplevel`)

// guessesAtK_a are the guesses left when K-a landed, per file, in the
// lane's own packages. They sit in the owner and the batch machinery that
// units K-b to D rewrite or delete; each of those units lowers its count,
// and D's audit finds none. layout.go is the one place a root is resolved.
var guessesAtK_a = map[string]int{
	"batch/join_admission.go": 1,
	"batch/publish.go":        1,
	"batch/seal.go":           3,
	"batchowner/admission.go": 3,
	"batchowner/change.go":    3,
	"batchowner/cost.go":      2,
	"batchowner/early.go":     3,
	"batchowner/join.go":      4,
	"batchowner/land.go":      5,
	"batchowner/lane.go":      1,
	"batchowner/owner.go":     7,
	"batchowner/ownerlog.go":  1,
	"batchowner/prefix.go":    4,
	"batchowner/prove.go":     10,
	"batchowner/red.go":       4,
	"lane/keeper.go":          2,
	"lane/layout.go":          2,
}

// L1 (r6 U1, design r10 K-a): every lane path comes from the registered
// Layout. No file of the lane's packages guesses a root more often than it
// did when the layout landed, and no new file guesses at all. (The Go
// language adapter, batch/goadapter, finds Go modules by design and is not
// a lane path.)
func TestLaneNeverGuessesRoots(t *testing.T) {
	t.Parallel()
	seen := map[string]int{}
	for _, dir := range []string{"batch", "batchowner", "lane"} {
		paths, err := filepath.Glob(filepath.Join("..", dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if count := len(rootGuess.FindAll(data, -1)); count != 0 {
				seen[dir+"/"+filepath.Base(path)] = count
			}
		}
	}
	if len(seen) == 0 {
		t.Fatalf("the audit read no lane source; it is not looking where the lane lives")
	}
	for file, count := range seen {
		if allowed := guessesAtK_a[file]; count > allowed {
			t.Errorf("%s guesses a root %d times (allowed %d): take the path from the registered lane.Layout (Checkout, Install or Execution) instead", file, count, allowed)
		}
	}
}
