package landing

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorktreeDrift(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *observeFixture)
		index       byte
		worktree    byte
		withoutFlag string
		withFlag    string
		tolerated   bool
	}{
		{name: "clean"},
		{
			name: "register append", setup: func(t *testing.T, f *observeFixture) {
				appendReceiptFixtureFile(t, filepath.Join(f.root, "memory", "receipts.log"), "receipt=append\n")
			}, index: ' ', worktree: 'M', tolerated: true,
		},
		{
			name: "register rewrite", setup: func(_ *testing.T, f *observeFixture) {
				f.write("memory/receipts.log", "receipt=rewritten\n")
			}, index: ' ', worktree: 'M', withoutFlag: "register-not-append", withFlag: "register-not-append",
		},
		{
			name: "staged and unstaged register", setup: func(t *testing.T, f *observeFixture) {
				f.write("memory/receipts.log", "receipt=existing\nreceipt=staged\n")
				f.git("add", "--", "memory/receipts.log")
				appendReceiptFixtureFile(t, filepath.Join(f.root, "memory", "receipts.log"), "receipt=unstaged\n")
			}, index: 'M', worktree: 'M', withoutFlag: "", withFlag: "staged", tolerated: true,
		},
		{
			name: "added and unstaged register", setup: func(t *testing.T, f *observeFixture) {
				f.git("rm", "-q", "--", "memory/receipts.log")
				f.git("commit", "-qm", "remove receipts")
				f.write("memory/receipts.log", "receipt=added\n")
				f.git("add", "--", "memory/receipts.log")
				appendReceiptFixtureFile(t, filepath.Join(f.root, "memory", "receipts.log"), "receipt=unstaged\n")
			}, index: 'A', worktree: 'M', withoutFlag: "", withFlag: "staged", tolerated: true,
		},
		{
			name: "type-changed register", setup: func(t *testing.T, f *observeFixture) {
				path := filepath.Join(f.root, "memory", "receipts.log")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("first-target", path); err != nil {
					t.Fatal(err)
				}
				f.git("add", "--", "memory/receipts.log")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("second-target", path); err != nil {
					t.Fatal(err)
				}
			}, index: 'T', worktree: 'M', withoutFlag: "unstaged", withFlag: "staged",
		},
		{
			name: "staged product", setup: func(_ *testing.T, f *observeFixture) {
				f.write("product.txt", "staged\n")
				f.git("add", "--", "product.txt")
			}, index: 'M', worktree: ' ', withFlag: "staged",
		},
		{
			name: "unstaged product", setup: func(_ *testing.T, f *observeFixture) {
				f.write("product.txt", "unstaged\n")
			}, index: ' ', worktree: 'M', withoutFlag: "unstaged", withFlag: "unstaged",
		},
		{
			name: "deleted register", setup: func(t *testing.T, f *observeFixture) {
				if err := os.Remove(filepath.Join(f.root, "memory", "receipts.log")); err != nil {
					t.Fatal(err)
				}
			}, index: ' ', worktree: 'D', withoutFlag: "unstaged", withFlag: "unstaged",
		},
		{
			name: "untracked", setup: func(_ *testing.T, f *observeFixture) {
				f.write("untracked.txt", "untracked\n")
			}, index: '?', worktree: '?', withoutFlag: "untracked", withFlag: "untracked",
		},
	}

	for _, test := range tests {
		for _, requireEmptyIndex := range []bool{false, true} {
			name := "candidate-index"
			wantKind := test.withoutFlag
			if requireEmptyIndex {
				name = "empty-index"
				wantKind = test.withFlag
			}
			t.Run(test.name+"/"+name, func(t *testing.T) {
				f := newObserveFixture(t)
				prepareNestedDriftFixture(f)
				if test.setup != nil {
					test.setup(t, f)
				}
				drift, tolerated, err := WorktreeDrift(f.root, requireEmptyIndex)
				if err != nil {
					t.Fatal(err)
				}
				wantTolerated := []string{}
				if test.tolerated && (!requireEmptyIndex || test.index == ' ') {
					wantTolerated = []string{"metasystem/memory/receipts.log"}
				}
				if !reflect.DeepEqual(tolerated, wantTolerated) {
					t.Fatalf("tolerated = %v, want %v", tolerated, wantTolerated)
				}
				if wantKind == "" {
					if len(drift) != 0 {
						t.Fatalf("drift = %+v, want none", drift)
					}
					return
				}
				if len(drift) != 1 {
					t.Fatalf("drift = %+v, want one entry", drift)
				}
				wantPath := "metasystem/"
				if test.index == '?' {
					wantPath += "untracked.txt"
				} else if test.name == "staged product" || test.name == "unstaged product" {
					wantPath += "product.txt"
				} else {
					wantPath += "memory/receipts.log"
				}
				want := DriftEntry{Kind: wantKind, Index: test.index, Worktree: test.worktree, Path: wantPath}
				if drift[0] != want {
					t.Fatalf("drift = %+v, want %+v", drift[0], want)
				}
			})
		}
	}

	t.Run("adopted root path space", func(t *testing.T) {
		f := newAdoptedObserveFixture(t)
		appendReceiptFixtureFile(t, filepath.Join(f.root, "records", "narrator-digest.log"), "digest=append\n")
		drift, tolerated, err := WorktreeDrift(f.root, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(drift) != 0 || !reflect.DeepEqual(tolerated, []string{"records/narrator-digest.log"}) {
			t.Fatalf("adopted-root drift = %+v, tolerated = %v", drift, tolerated)
		}
	})
}

func prepareNestedDriftFixture(f *observeFixture) {
	f.t.Helper()
	f.git("add", "--", "../development/metasystem-design.md")
	f.git("commit", "-qm", "track repository sibling")
}
