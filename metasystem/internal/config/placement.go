package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/placement"
)

// placementMeasureBudget bounds the size walk of one misplaced tree in a
// refusal; a larger tree is named "at least" what the walk reached.
const placementMeasureBudget = 2 * time.Second

// machineCacheRoots are the engine's and the delegates' Go and staticcheck
// caches; a root that cannot be resolved is left out (it contains nothing).
func machineCacheRoots() []string {
	var roots []string
	for _, domain := range []gocache.Domain{gocache.DomainEngine, gocache.DomainDelegate} {
		if paths, err := gocache.DomainPaths(domain); err == nil {
			roots = append(roots, paths.Directories()...)
		}
	}
	return roots
}

// placementProblems is 3.12's first placement rule for one evidence root
// (R21): it may neither contain a cache root nor lie inside one (compared
// by file identity), and its top two levels may hold no cache-shaped tree
// and no source copy. Each refusal names the tree, its size and what a
// person runs instead. A root that does not exist yet holds nothing.
func placementProblems(root string, cacheRoots []string) []string {
	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return nil
	}
	var problems []string
	for _, cache := range cacheRoots {
		cacheInfo, err := os.Stat(cache)
		if err != nil || !cacheInfo.IsDir() {
			continue
		}
		switch {
		case within(cache, rootInfo):
			problems = append(problems, fmt.Sprintf("%s %s contains the cache root %s: caches never live under an evidence root; move the evidence root elsewhere (a directory of its own outside every cache)", EvidenceRootKey, root, cache))
		case within(root, cacheInfo):
			problems = append(problems, fmt.Sprintf("%s %s lies inside the cache root %s: caches never live under an evidence root; move the evidence root elsewhere (a directory of its own outside every cache)", EvidenceRootKey, root, cache))
		}
	}
	for _, tree := range topTwoLevels(root) {
		misplaced := placement.Of(tree)
		if misplaced.Kind == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), placementMeasureBudget)
		bytes, complete := measure(ctx, tree)
		cancel()
		size := formatSize(bytes)
		if !complete {
			size = "at least " + size
		}
		problems = append(problems, fmt.Sprintf("%s %s holds %s at %s (%s, %s): caches and source copies never live under an evidence root and it is not evidence; a person removes it once nothing needs it (rm -rf -- %s), and a bed goes in metasystem work workspace G",
			EvidenceRootKey, root, misplaced.Shape, tree, misplaced.Kind, size, "'"+strings.ReplaceAll(tree, "'", `'\''`)+"'"))
	}
	return problems
}

// within reports whether path or one of its physical ancestors (symbolic
// links resolved first) is the directory target, by device and inode.
func within(path string, target os.FileInfo) bool {
	current := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}
	for {
		if info, err := os.Stat(current); err == nil && os.SameFile(info, target) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

// topTwoLevels lists the directories directly under root and directly
// under each of those, never following a symbolic link; a directory
// recognized as a misplaced tree is not descended into.
func topTwoLevels(root string) []string {
	var trees []string
	first, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, entry := range first {
		path := filepath.Join(root, entry.Name())
		if !entry.IsDir() {
			continue
		}
		trees = append(trees, path)
		if placement.Of(path).Kind != "" {
			continue
		}
		second, err := os.ReadDir(path)
		if errors.Is(err, os.ErrNotExist) || err != nil {
			continue
		}
		for _, child := range second {
			if child.IsDir() {
				trees = append(trees, filepath.Join(path, child.Name()))
			}
		}
	}
	return trees
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1<<20))
	}
	return fmt.Sprintf("%d KiB", bytes>>10)
}

// measure is a stat walk of allocated bytes, never following a symbolic
// link, bounded by ctx; complete is false when ctx cut it short.
func measure(ctx context.Context, root string) (int64, bool) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			if raw, ok := info.Sys().(*syscall.Stat_t); ok {
				total += int64(raw.Blocks) * 512
			} else {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err == nil
}
