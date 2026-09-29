package evidence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// budget is a context whose budget runs out after a number of checks: a
// deterministic pass budget for the resumption witnesses.
type budget struct {
	context.Context
	left *int
}

func newBudget(checks int) budget {
	return budget{Context: context.Background(), left: &checks}
}

func (b budget) Err() error {
	if *b.left <= 0 {
		return context.DeadlineExceeded
	}
	*b.left--
	return nil
}

type citationBed struct {
	*boundBed
	records string
	index   *Citations
}

func newCitationBed(t *testing.T) citationBed {
	t.Helper()
	bed := citationBed{boundBed: newBoundBed(t)}
	bed.records = filepath.Join(filepath.Dir(bed.root), "records")
	if err := os.MkdirAll(bed.records, 0o755); err != nil {
		t.Fatal(err)
	}
	bed.index = bed.fresh()
	return bed
}

// fresh is the index as a new pass sees it.
func (bed citationBed) fresh() *Citations {
	return &Citations{Dir: filepath.Join(bed.home, "stores", "citations"), Now: boundNow,
		Roots: func() ([]string, error) { return []string{bed.records}, nil }}
}

func (bed citationBed) write(t *testing.T, rel string, data []byte) string {
	t.Helper()
	path := filepath.Join(bed.records, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (bed citationBed) complete(t *testing.T) {
	t.Helper()
	published, err := bed.index.Step(context.Background())
	if err != nil || !published {
		t.Fatalf("a generation within one budget publishes: %v %v", published, err)
	}
}

func (bed citationBed) cited(t *testing.T, name string) []string {
	t.Helper()
	files, unknown, pending := bed.fresh().Cited(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: name})
	if unknown != "" || pending != "" {
		t.Fatalf("cited(%s): unknown=%q pending=%q", name, unknown, pending)
	}
	return files
}

func TestCitationsFindEverySpellingButABareId(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	segment := bed.segment.Git
	large := append(make([]byte, 5<<20), []byte("see ~/metasystem-evidence/agentic-tools/agents/"+segment+"/huge-chain/rounds/1/return.json")...)
	bed.write(t, "large.md", large)
	bed.write(t, "binary.bin", append([]byte{0, 0, 1, 0}, []byte(segment+"/nul-chain\x00\x00")...))
	bed.write(t, "relative.md", []byte("the chain at agents/"+segment+"/relative-chain holds it"))
	bed.write(t, "ids.md", []byte("delegate=claude:opus:id-only-chain proof-mtviifgb-af048a24c818711d"))
	bed.complete(t)
	for _, name := range []string{"huge-chain", "nul-chain", "relative-chain"} {
		if files := bed.cited(t, name); len(files) != 1 {
			t.Fatalf("%s must be cited: %v", name, files)
		}
	}
	if files := bed.cited(t, "id-only-chain"); len(files) != 0 {
		t.Fatalf("a bare job id cites nothing: %v", files)
	}
	if files := bed.cited(t, "huge"); len(files) != 0 {
		t.Fatalf("a prefix of a longer name is another item: %v", files)
	}
}

// A record copied into place with a preserved old mtime is found by the
// apply-time inventory diff, never by mtime order (DL4C-02).
func TestACitationCopiedInAfterTheGenerationIsFoundByTheInventoryDiff(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	bed.write(t, "first.md", []byte("nothing here"))
	bed.complete(t)
	copied := bed.write(t, "copied/note.md", []byte("cites "+bed.segment.Git+"/late-chain"))
	old := boundNow.Add(-400 * 24 * time.Hour)
	if err := os.Chtimes(copied, old, old); err != nil {
		t.Fatal(err)
	}
	if files := bed.cited(t, "late-chain"); len(files) != 1 || files[0] != copied {
		t.Fatalf("the changed inventory is scanned at apply: %v", files)
	}
}

// One file larger than a pass budget is scanned across passes from its
// byte offset, and a citation straddling the chunk boundary is found
// (DL4D-10).
func TestACitationStraddlingAChunkBoundaryIsFoundAcrossPasses(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	token := bed.segment.Git + "/straddling-chain"
	data := append(make([]byte, citationChunk-8), []byte(token)...)
	data = append(data, make([]byte, citationChunk)...)
	bed.write(t, "big.log", data)
	passes := 0
	for {
		passes++
		// Each pass's budget allows the walk and one chunk.
		published, err := bed.index.Step(newBudget(3))
		if err != nil {
			t.Fatal(err)
		}
		if published {
			break
		}
		if passes > 10 {
			t.Fatal("the generation never completes")
		}
	}
	if passes < 2 {
		t.Fatalf("the file needed more than one pass, got %d", passes)
	}
	if files := bed.cited(t, "straddling-chain"); len(files) != 1 {
		t.Fatalf("the straddling citation is found: %v", files)
	}
}

func TestAnInventoryLargerThanABudgetCompletes(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	for index := 0; index < 30; index++ {
		bed.write(t, filepath.Join("dir"+string(rune('a'+index%26))+string(rune('a'+index/26)), "note.md"), []byte("plain"))
	}
	bed.write(t, "zz/cite.md", []byte(bed.segment.Git+"/walked-chain"))
	passes := 0
	for published := false; !published; passes++ {
		var err error
		published, err = bed.index.Step(newBudget(5))
		if err != nil {
			t.Fatal(err)
		}
		if passes > 40 {
			t.Fatal("the walk never completes")
		}
	}
	if passes < 3 {
		t.Fatalf("the walk resumed across passes, got %d", passes)
	}
	if files := bed.cited(t, "walked-chain"); len(files) != 1 {
		t.Fatalf("found after a resumed walk: %v", files)
	}
}

func TestAnUnreadableCitationFileIsUnknownForEveryItem(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	secret := bed.write(t, "secret.md", []byte("anything"))
	if err := os.Chmod(secret, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secret, 0o644) })
	bed.complete(t)
	_, unknown, _ := bed.fresh().Cited(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "any"})
	if !strings.Contains(unknown, secret) {
		t.Fatalf("the unreadable file is named: %q", unknown)
	}
}

func TestNoPublishedGenerationKeepsEveryItem(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	_, _, pending := bed.fresh().Cited(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "any"})
	if pending == "" {
		t.Fatal("before the first generation completes every item is held")
	}
}

// A generation whose scan needs three pass budgets completes, and only
// then is an uncited chain clear; a cited chain stays held.
func TestAnItemIsClearOnlyOnceTheGenerationCompletes(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	cited := bed.chain(t, "cited", 300, 400, "")
	free := bed.chain(t, "free", 300, 400, "")
	bed.write(t, "design.md", append(make([]byte, 2*citationChunk), []byte("the evidence is "+bed.segment.Git+"/cited")...))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	exclusions := bed.exclusions(fake)
	for passes := 0; ; passes++ {
		exclusions.Citations = bed.fresh()
		if pass := bed.judge(exclusions); pass.clear[free] || pass.clear[cited] {
			t.Fatal("nothing is clear before the generation completes")
		}
		if published, err := bed.index.Step(newBudget(3)); err != nil {
			t.Fatal(err)
		} else if published {
			break
		}
		if passes > 10 {
			t.Fatal("never completes")
		}
	}
	exclusions = bed.exclusions(fake)
	exclusions.Citations = bed.fresh()
	if pass := bed.judge(exclusions); !pass.clear[free] || pass.clear[cited] {
		t.Fatalf("after the generation the uncited chain is clear and the cited one held: %v %v", pass.clear[free], pass.clear[cited])
	}
}
