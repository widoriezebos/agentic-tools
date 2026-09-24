package landing

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var chainExpectedTree = strings.Repeat("d", 40)
var chainReviewedTree = strings.Repeat("e", 40)

type observationTreePair struct{ from, to string }

type chainObservationFacts struct {
	expected, consumed map[string]int
	paths              map[observationTreePair][]string
	applyBase          string
	applyPatch         []byte
	expectedTree       string
	reviewedTree       string
}

type chainFileChange struct {
	path   string
	before *string
	after  *string
}

func chainAddition(path, after string) chainFileChange {
	return chainFileChange{path: path, after: observationText(after)}
}

func chainReplacement(path, before, after string) chainFileChange {
	return chainFileChange{path: path, before: observationText(before), after: observationText(after)}
}

func chainDiff(changes ...chainFileChange) []byte {
	var patch strings.Builder
	for _, change := range changes {
		fmt.Fprintf(&patch, "diff --git a/%s b/%s\n", change.path, change.path)
		if change.before == nil {
			patch.WriteString("new file mode 100644\n")
			fmt.Fprintf(&patch, "--- /dev/null\n+++ b/%s\n", change.path)
		} else {
			fmt.Fprintf(&patch, "--- a/%s\n+++ b/%s\n", change.path, change.path)
		}
		beforeLines, afterLines := 0, 0
		if change.before != nil {
			beforeLines = strings.Count(*change.before, "\n")
		}
		if change.after != nil {
			afterLines = strings.Count(*change.after, "\n")
		}
		if change.before == nil {
			fmt.Fprintf(&patch, "@@ -0,0 +1,%d @@\n", afterLines)
		} else {
			fmt.Fprintf(&patch, "@@ -1,%d +1,%d @@\n", beforeLines, afterLines)
		}
		if change.before != nil {
			for _, line := range strings.Split(strings.TrimSuffix(*change.before, "\n"), "\n") {
				fmt.Fprintf(&patch, "-%s\n", line)
			}
		}
		if change.after != nil {
			for _, line := range strings.Split(strings.TrimSuffix(*change.after, "\n"), "\n") {
				fmt.Fprintf(&patch, "+%s\n", line)
			}
		}
	}
	return []byte(patch.String())
}

func chainBlobOID(data []byte) string {
	payload := []byte(fmt.Sprintf("blob %d\x00", len(data)))
	payload = append(payload, data...)
	sum := sha1.Sum(payload)
	return fmt.Sprintf("%x", sum)
}

func (f *repositoryObservationFixture) chainCase(candidate string, changes ...chainFileChange) *observationCase {
	f.t.Helper()
	paths := make([]string, len(changes))
	for i, change := range changes {
		paths[i] = change.path
	}
	c := f.comparison(candidate, string(chainDiff(changes...)), paths...)
	c.chain = &chainObservationFacts{
		expected: map[string]int{}, consumed: map[string]int{},
		paths: map[observationTreePair][]string{},
	}
	c.wantChain("head", 1)
	c.wantChain("diff:"+observeBaseTree+":"+candidate, 1)
	for _, change := range changes {
		c.declare(change.path, change.before, change.after)
	}
	return c
}

func (c *observationCase) wantChain(key string, count int) {
	c.fixture.t.Helper()
	if c.chain == nil || c.sealed {
		c.fixture.t.Fatal("chain facts must be declared before observation")
	}
	c.chain.expected[key] += count
}

func (c *observationCase) wantChainPolicy(paths ...string) {
	c.wantChain("head", 1)
	c.wantChain("file:"+observeBaseTree+":scripts/agents/landing-classes.json", 1)
	c.wantChain("file:"+observeBaseTree+":memory/rulings.md", 1)
	c.wantChain("file:"+observeBaseTree+":scripts/agents/path-classes.txt", 1)
	c.wantChain("paths:"+observeBaseTree+":"+c.candidate, 1)
	c.wantChain("prefix", 1)
	for _, path := range paths {
		c.wantChain("owner:"+path, 1)
	}
}

// The patch is a real review artifact. Apply supplies only its raw tree result;
// the observer still compares the certified paths and actual tree entries.
func (c *observationCase) bindChain(patch []byte, reviewedTree string, certified chainFileChange) {
	c.fixture.t.Helper()
	certifiedPath := certified.path
	if certified.after == nil {
		c.fixture.t.Fatal("chain fixture requires a certified postimage")
	}
	c.chain.applyBase = observeBaseTree
	c.chain.applyPatch = append([]byte(nil), patch...)
	c.chain.expectedTree = chainExpectedTree
	c.chain.reviewedTree = reviewedTree
	c.chain.paths[observationTreePair{observeBaseTree, chainExpectedTree}] = []string{certifiedPath}
	c.wantChain("head", 1)
	c.wantChain("apply:"+observeBaseTree+":"+string(patch), 1)
	c.wantChain("paths:"+observeBaseTree+":"+chainExpectedTree, 1)
	c.wantChain("paths:"+observeBaseTree+":"+c.candidate, 1)
	pathSet := []string{certifiedPath}
	after := map[string]gittree.Entry{certifiedPath: {Mode: "100644", OID: chainBlobOID([]byte(*certified.after))}}
	c.declareEntries(chainExpectedTree, pathSet, after)
	c.declareEntries(reviewedTree, pathSet, after)
	for _, tree := range []string{chainExpectedTree, reviewedTree, observeBaseTree, c.candidate} {
		c.wantChain("entries:"+tree+":"+certifiedPath, 1)
	}
	// pathChangeDigest reads the base and expected trees, then the base and candidate.
	c.wantChain("entries:"+observeBaseTree+":"+certifiedPath, 1)
	c.wantChain("entries:"+chainExpectedTree+":"+certifiedPath, 1)
	if _, ok := c.entries[entriesKey(c.candidate, pathSet)]; !ok {
		c.fixture.t.Fatalf("certified path %s is not declared in candidate", certifiedPath)
	}
}

func (c *observationCase) Apply(base string, patch []byte) (string, error) {
	c.consumeExact("apply:" + base + ":" + string(patch))
	if base != c.chain.applyBase || !bytes.Equal(patch, c.chain.applyPatch) {
		c.fixture.t.Fatalf("unexpected certified patch replay at %s", base)
	}
	return c.chain.expectedTree, nil
}

func (c *observationCase) wantChainGoal(id string) {
	c.wantChain("file:"+observeBaseTree+":plans/goals/"+id+".md", 1)
}

func (c *observationCase) wantChainReceiptAppend(times int) {
	path := "memory/receipts.log"
	for _, tree := range []string{observeBaseTree, c.candidate} {
		c.wantChain("entries:"+tree+":"+path, times)
		c.wantChain("file:"+tree+":"+path, times)
	}
}

func (c *observationCase) wantChainRegister(paths ...string) {
	c.wantChain("head", 1)
	c.wantChain("file:"+observeBaseTree+":scripts/agents/landing-classes.json", 1)
	c.wantChain("file:"+observeBaseTree+":memory/rulings.md", 1)
	c.wantChain("file:"+observeBaseTree+":scripts/agents/path-classes.txt", 1)
	c.wantChain("prefix", 1)
	for _, path := range paths {
		c.wantChain("owner:"+path, 1)
		c.wantChain("entries:"+observeBaseTree+":"+path, 1)
		c.wantChain("entries:"+c.candidate+":"+path, 1)
	}
}

func (f *repositoryObservationFixture) writeChainRecord(chain string, record map[string]any) {
	f.t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(filepath.Join("artifacts", "agents", "jobs", chain+".json"), string(append(data, '\n')))
}

func (f *repositoryObservationFixture) writeChainReview(chain string, round int, implementerJob, reviewedTree string, patch []byte) {
	f.t.Helper()
	review, err := json.Marshal(map[string]any{
		"diffArtifact": "diff.patch", "implementerJob": implementerJob, "reviewedTree": reviewedTree,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	directory := filepath.Join("artifacts", "agents", chain, "rounds", fmt.Sprint(round))
	f.write(filepath.Join(directory, "review.json"), string(append(review, '\n')))
	f.write(filepath.Join(directory, "diff.patch"), string(patch))
}

func chainRoot(id string) map[string]any {
	return map[string]any{
		"jobId": id, "parentJob": nil, "role": "implementer", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "chainClosed": true,
	}
}

func (f *repositoryObservationFixture) baseHeldGoalAtRevision(id, machine, lineage string, revision uint64) {
	f.t.Helper()
	history := make([]goal.HistoryLine, revision)
	for index := range history {
		at := fmt.Sprintf("2026-09-03T08:%02d:00Z", index)
		history[index] = goal.HistoryLine{
			At: at, Opid: fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FAW-%s-%08x", machine, index+1),
			Verb: "edit", Actor: machine + "+" + lineage, Targets: []string{id}, Keep: -1,
		}
	}
	history[revision-1].Verb = "claim"
	f.base(filepath.Join("plans", "goals", id+".md"), string(goal.RenderFile(&goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Exercise landing binding.", OpenedAt: "2026-09-03T08:00:00Z", Revision: revision,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: history[revision-1].At, Revision: revision, AccountingRevision: revision},
		History: history,
	})))
}
