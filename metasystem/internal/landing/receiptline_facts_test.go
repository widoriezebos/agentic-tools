package landing

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// receiptLineFixture keeps filesystem layout real while each run declares its
// own immutable tree reads. No fixture operation consults a Git repository.
type receiptLineFixture struct {
	t         *testing.T
	repo      string
	root      string
	ownerRoot string
	prefix    string
	ledger    string
	manifest  []byte
}

func newReceiptLineFixture(t *testing.T, adopted bool) *receiptLineFixture {
	t.Helper()
	repo := t.TempDir()
	prefix := "metasystem"
	if adopted {
		prefix = "vendor/metasystem"
	}
	f := &receiptLineFixture{
		t: t, repo: repo, root: filepath.Join(repo, filepath.FromSlash(prefix)), prefix: prefix,
		ledger: path.Join(prefix, "memory/receipts.log"),
	}
	if adopted {
		f.ledger = "memory/receipts.log"
	} else {
		f.writeTop("development/metasystem-design.md", "fixture\n")
	}
	f.writeInstall("metasystem.conf", "metasystem.runtimes=\n")
	ownerRoot, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		t.Fatalf("resolve receipt-line installation: %v", err)
	}
	f.ownerRoot = ownerRoot
	f.writeInstall("product.txt", "before\n")
	manifest, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "path-classes.txt"))
	if err != nil {
		t.Fatalf("read shipped path-class manifest: %v", err)
	}
	f.manifest = append([]byte(nil), manifest...)
	f.writeInstall(pathclass.ManifestPath, string(manifest))
	f.writeTop(f.ledger, "receipt=existing\n")
	return f
}

func (f *receiptLineFixture) writeTop(relative, content string) {
	f.t.Helper()
	target := filepath.Join(f.repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *receiptLineFixture) writeInstall(relative, content string) {
	f.t.Helper()
	f.writeTop(path.Join(f.prefix, relative), content)
}

func (f *receiptLineFixture) removeTop(relative string) {
	f.t.Helper()
	if err := os.Remove(filepath.Join(f.repo, filepath.FromSlash(relative))); err != nil {
		f.t.Fatal(err)
	}
}

type receiptLineFact struct {
	method, first, second string
	bytes                 []byte
	present               bool
	paths                 []string
}

type receiptLineRun struct {
	fixture         *receiptLineFixture
	base, candidate string
	calls           []receiptLineFact
	nextCall        int
}

func (f *receiptLineFixture) newRun(base, candidate string) *receiptLineRun {
	return &receiptLineRun{fixture: f, base: base, candidate: candidate}
}

func (r *receiptLineRun) expect(method, first, second string) {
	r.calls = append(r.calls, receiptLineFact{method: method, first: first, second: second})
}

func (r *receiptLineRun) expectLocation() {
	if r.fixture.prefix == "vendor/metasystem" {
		r.expect("repositoryTop", r.fixture.root, "")
	}
	r.expect("TopLevel", "", "")
}

func (r *receiptLineRun) expectHead() {
	r.expect("headTree", r.fixture.root, "")
}

func (r *receiptLineRun) expectBaseLedger(content string, present bool) {
	r.expectFile(r.base, r.fixture.ledger, content, present)
}

func (r *receiptLineRun) expectChanged(paths ...string) {
	r.calls = append(r.calls, receiptLineFact{
		method: "ChangedPaths", first: r.base, second: r.candidate,
		paths: append([]string(nil), paths...),
	})
}

func (r *receiptLineRun) expectCandidateLedger(content string, present bool) {
	r.expectFile(r.candidate, r.fixture.ledger, content, present)
}

func (r *receiptLineRun) expectManifest() {
	r.expect("Prefix", "", "")
	r.expectFile(r.base, path.Join(r.fixture.prefix, pathclass.ManifestPath), string(r.fixture.manifest), true)
}

func (r *receiptLineRun) expectOwner() {
	r.expect("repositoryTop", r.fixture.ownerRoot, "")
}

func (r *receiptLineRun) expectPresentLedgerChange(baseLedger, candidateLedger string, changed ...string) {
	r.expectLocation()
	r.expectHead()
	r.expectBaseLedger(baseLedger, true)
	r.expectChanged(changed...)
	r.expectCandidateLedger(candidateLedger, true)
}

func (r *receiptLineRun) expectFile(tree, file, content string, present bool) {
	r.calls = append(r.calls, receiptLineFact{
		method: "FileAt", first: tree, second: file,
		bytes: []byte(content), present: present,
	})
}

func (r *receiptLineRun) take(method, first, second string) receiptLineFact {
	r.fixture.t.Helper()
	if r.nextCall >= len(r.calls) {
		r.fixture.t.Fatalf("unexpected %s(%q, %q) after all declared receipt-line facts", method, first, second)
	}
	fact := r.calls[r.nextCall]
	r.nextCall++
	if fact.method != method || fact.first != first || fact.second != second {
		r.fixture.t.Fatalf("receipt-line fact %d: got %s(%q, %q), want %s(%q, %q)",
			r.nextCall, method, first, second, fact.method, fact.first, fact.second)
	}
	return fact
}

func (r *receiptLineRun) TopLevel() (string, error) {
	r.take("TopLevel", "", "")
	return r.fixture.repo, nil
}

func (r *receiptLineRun) Prefix() (string, error) {
	r.take("Prefix", "", "")
	return r.fixture.prefix, nil
}

func (r *receiptLineRun) FileAt(tree, file string) ([]byte, bool, error) {
	fact := r.take("FileAt", tree, file)
	return append([]byte(nil), fact.bytes...), fact.present, nil
}

func (r *receiptLineRun) ChangedPaths(base, candidate string) ([]string, error) {
	fact := r.take("ChangedPaths", base, candidate)
	return append([]string(nil), fact.paths...), nil
}

func (r *receiptLineRun) decide(goal, directFix string) ReceiptLineDecision {
	r.fixture.t.Helper()
	resolver := stateroot.NewResolver(func(root string) (string, error) {
		r.take("repositoryTop", root, "")
		return r.fixture.repo, nil
	}, func() (string, error) {
		r.fixture.t.Fatal("receipt-line policy unexpectedly requested the executable path")
		return "", fmt.Errorf("unexpected executable path lookup")
	})
	decision, err := observeReceiptLineWithDependencies(ReceiptLineParams{
		RepoRoot: r.fixture.root, CandidateTree: r.candidate, Goal: goal, DirectFix: directFix,
	}, r, func(root string) (string, error) {
		r.take("headTree", root, "")
		return r.base, nil
	}, resolver.RootForInstallation, resolver.OwnerForInstallation)
	if err != nil {
		r.fixture.t.Fatalf("receipt-line check failed: %v", err)
	}
	if r.nextCall != len(r.calls) {
		fact := r.calls[r.nextCall]
		r.fixture.t.Fatalf("receipt-line check missed declared fact %d: %s(%q, %q)",
			r.nextCall+1, fact.method, fact.first, fact.second)
	}
	return decision
}
