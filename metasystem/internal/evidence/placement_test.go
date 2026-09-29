package evidence

// U5i (engine-owns-disk-lifetimes Part B, 3.12 placement rules, R21): a Go
// cache, a module cache, a gocache-x directory and a source copy under an
// evidence root are reported as cache-under-evidence or
// source-copy-under-evidence, are never items, are excluded from the
// segment's bytes, and survive every pass.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func TestCachesAndSourceCopiesUnderARootAreReportedNeverCounted(t *testing.T) {
	t.Parallel()
	bed := newHostBed(t)
	chain := bed.one.chain(t, "kept-chain", 400, 4, "")
	root := bed.one.root
	segment := filepath.Join(root, "agents", bed.one.segment.Git)
	trees := map[string]map[string]string{
		filepath.Join(root, "go-cache"): {"README": "This directory holds cached build artifacts from the Go build system.\n", "trim.txt": "1\n",
			"00/entry-a": strings.Repeat("c", 200*kib)},
		filepath.Join(root, "modules"):          {"cache/download/example.com/lib/@v/list": strings.Repeat("m", 200*kib)},
		filepath.Join(segment, "gocache-x"):     {"00/entry": strings.Repeat("g", 200*kib)},
		filepath.Join(segment, "checkout-copy"): {".git/HEAD": "ref: refs/heads/main\n", "big": strings.Repeat("s", 200*kib)},
	}
	for tree, files := range trees {
		for rel, content := range files {
			path := filepath.Join(tree, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	items, err := bed.one.segment.Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Path != chain {
		t.Fatalf("only the chain is an item of the segment, got %+v", items)
	}
	for _, mode := range []diskstore.Mode{diskstore.ModeApply, diskstore.ModeApply} {
		report := bed.run(t, bed.class(1<<40), mode)
		kinds := map[string]string{}
		for _, line := range report.Misplaced {
			kinds[line.Path] = line.Class
		}
		for tree := range trees {
			want := diskstore.PlacementCache
			if strings.HasSuffix(tree, "checkout-copy") {
				want = diskstore.PlacementSourceCopy
			}
			if kinds[tree] != want {
				t.Fatalf("%s is reported as %s, got %q (%+v)", tree, want, kinds[tree], report.Misplaced)
			}
			if _, err := os.Stat(tree); err != nil {
				t.Fatalf("a pass never removes %s: %v", tree, err)
			}
		}
		if report.Health.Status != diskstore.HealthAttention {
			t.Fatalf("a misplaced tree raises the disk role: %+v", report.Health)
		}
	}
}
