package steward

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

type witnessWalkCounts struct {
	logs, batches, archives int
	batchInput              string
}

func observeWitnessWalk(t *testing.T, digester func(context.Context, string, string, behaviorsurface.Policy) (string, error)) *witnessWalkCounts {
	t.Helper()
	originalRunner, originalDigester := witnessGitCommandRunner, witnessTreeDigester
	t.Cleanup(func() {
		witnessGitCommandRunner, witnessTreeDigester = originalRunner, originalDigester
	})
	counts := &witnessWalkCounts{}
	witnessGitCommandRunner = func(ctx context.Context, root string, input []byte, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "log" {
			counts.logs++
		}
		if len(args) > 0 && args[0] == "cat-file" {
			counts.batches++
			counts.batchInput = string(input)
		}
		return originalRunner(ctx, root, input, args...)
	}
	witnessTreeDigester = func(ctx context.Context, toplevel, tree string, policy behaviorsurface.Policy) (string, error) {
		counts.archives++
		if digester != nil {
			return digester(ctx, toplevel, tree, policy)
		}
		return originalDigester(ctx, toplevel, tree, policy)
	}
	return counts
}

func commitOutsideEngine(t *testing.T, root, message string) string {
	t.Helper()
	path := filepath.Join(root, "docs", "history.txt")
	writeRearmFile(t, path, message+"\n")
	rearmGit(t, root, "add", "docs/history.txt")
	rearmGit(t, root, "commit", "-qm", message)
	return rearmGit(t, root, "rev-parse", "HEAD")
}

func TestWitnessResolverWalksInOneProcessAndDigestsOnlyDistinctProjections(t *testing.T) {
	targetDigest := strings.Repeat("a", 64)
	otherDigest := strings.Repeat("b", 64)

	t.Run("200 changed projections", func(t *testing.T) {
		root := initRearmRepo(t)
		first := commitRearmTree(t, root, "candidate-000")
		for candidate := 1; candidate < 200; candidate++ {
			commitRearmTree(t, root, fmt.Sprintf("candidate-%03d", candidate))
		}
		ref := "refs/remotes/origin/trunk"
		rearmGit(t, root, "update-ref", ref, "HEAD")
		counts := observeWitnessWalk(t, func(_ context.Context, _, tree string, _ behaviorsurface.Policy) (string, error) {
			if tree == first {
				return targetDigest, nil
			}
			return otherDigest, nil
		})
		resolved, err := resolveWitnessStamp(root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("200-candidate resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 200 {
			t.Fatalf("200-candidate walk used log=%d batch=%d archives=%d, want 1, 0, 200", counts.logs, counts.batches, counts.archives)
		}
	})

	t.Run("65 candidates in five projection classes", func(t *testing.T) {
		root := initRearmRepo(t)
		first := commitRearmTree(t, root, "projection-0")
		for projection := 1; projection < 5; projection++ {
			commitRearmTree(t, root, fmt.Sprintf("projection-%d", projection))
		}
		for candidate := 0; candidate < 60; candidate++ {
			commitOutsideEngine(t, root, fmt.Sprintf("ledger-%02d", candidate))
		}
		ref := "refs/remotes/origin/trunk"
		rearmGit(t, root, "update-ref", ref, "HEAD")
		counts := observeWitnessWalk(t, func(_ context.Context, _, tree string, _ behaviorsurface.Policy) (string, error) {
			if tree == first {
				return targetDigest, nil
			}
			return otherDigest, nil
		})
		resolved, err := resolveWitnessStamp(root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("cold class resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 5 {
			t.Fatalf("cold class walk used log=%d batch=%d archives=%d, want 1, 0, 5", counts.logs, counts.batches, counts.archives)
		}
		counts.logs, counts.batches, counts.archives = 0, 0, 0
		resolved, err = resolveWitnessStamp(root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("warm class resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
			t.Fatalf("warm class walk used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
		}
	})

	t.Run("root and merge", func(t *testing.T) {
		root := initRearmRepo(t)
		first := commitRearmTree(t, root, "root")
		ref := "refs/remotes/origin/trunk"
		rearmGit(t, root, "update-ref", ref, first)
		counts := observeWitnessWalk(t, func(context.Context, string, string, behaviorsurface.Policy) (string, error) {
			return targetDigest, nil
		})
		if resolved, err := resolveWitnessStamp(root, root, ref, targetDigest[:12]); err != nil || resolved != first {
			t.Fatalf("root resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.archives != 1 {
			t.Fatalf("root walk used log=%d archives=%d, want 1, 1", counts.logs, counts.archives)
		}

		rearmGit(t, root, "checkout", "-qb", "side", first)
		writeRearmFile(t, filepath.Join(root, "docs", "side.txt"), "side\n")
		rearmGit(t, root, "add", "docs/side.txt")
		rearmGit(t, root, "commit", "-qm", "side")
		rearmGit(t, root, "checkout", "-q", "trunk")
		commitOutsideEngine(t, root, "trunk")
		rearmGit(t, root, "merge", "--no-ff", "-qm", "merge", "side")
		merge := rearmGit(t, root, "rev-parse", "HEAD")
		rearmGit(t, root, "update-ref", ref, merge)
		counts.logs, counts.archives = 0, 0
		if resolved, err := resolveWitnessStamp(root, root, ref, targetDigest[:12]); err != nil || resolved != merge {
			t.Fatalf("merge resolution: got=%s want=%s err=%v", resolved, merge, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
			t.Fatalf("cached merge class used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
		}
	})
}

func TestWitnessHistoryParserPreservesNULDelimitedPathsAndRenames(t *testing.T) {
	root := initRearmRepo(t)
	oldPath := "cmd/space name.txt"
	newPath := "cmd/renamed name.txt"
	newlinePath := "cmd/line\nbreak.txt"
	writeRearmFile(t, filepath.Join(root, filepath.FromSlash(oldPath)), "space\n")
	writeRearmFile(t, filepath.Join(root, filepath.FromSlash(newlinePath)), "newline\n")
	rearmGit(t, root, "add", ".")
	rearmGit(t, root, "commit", "-qm", "paths")
	rearmGit(t, root, "mv", oldPath, newPath)
	rearmGit(t, root, "commit", "-qm", "rename")
	counts := observeWitnessWalk(t, func(context.Context, string, string, behaviorsurface.Policy) (string, error) {
		return "", nil
	})
	candidates, err := readWitnessCandidates(context.Background(), root, "HEAD")
	if err != nil || len(candidates) != 2 {
		t.Fatalf("read NUL-delimited history: candidates=%d err=%v", len(candidates), err)
	}
	paths := map[string]projectionDiffEntry{}
	for _, entry := range candidates[0].changes {
		paths[entry.path] = entry
	}
	if deleted, ok := paths[oldPath]; !ok || deleted.newMode != "000000" {
		t.Fatalf("rename source path was not preserved as a delete: %+v", deleted)
	}
	if added, ok := paths[newPath]; !ok || added.oldMode != "000000" {
		t.Fatalf("rename destination path was not preserved as an add: %+v", added)
	}
	foundNewline := false
	for _, entry := range candidates[1].changes {
		foundNewline = foundNewline || entry.path == newlinePath
	}
	if !foundNewline || counts.logs != 1 {
		t.Fatalf("newline path or single log stream missing: newline=%t logs=%d", foundNewline, counts.logs)
	}
}

func TestWitnessResolverUsesOneBatchCheckForNestedPrefix(t *testing.T) {
	outer := canonicalPath(t.TempDir())
	rearmGit(t, outer, "init", "-q", "-b", "trunk")
	rearmGit(t, outer, "config", "user.name", "test")
	rearmGit(t, outer, "config", "user.email", "test@example.invalid")
	root := filepath.Join(outer, "metasystem")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/nested\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "nested\n")
	rearmGit(t, outer, "add", ".")
	rearmGit(t, outer, "commit", "-qm", "nested")
	writeRearmFile(t, filepath.Join(outer, "plans", "goals", "peer.md"), "ledger\n")
	rearmGit(t, outer, "add", "plans/goals/peer.md")
	rearmGit(t, outer, "commit", "-qm", "ledger")
	landed := rearmGit(t, outer, "rev-parse", "HEAD")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, outer, "update-ref", ref, landed)
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	counts := observeWitnessWalk(t, nil)
	resolved, err := resolveWitnessStamp(outer, root, ref, digest[:12])
	if err != nil || resolved != landed {
		t.Fatalf("nested resolution: got=%s want=%s err=%v", resolved, landed, err)
	}
	if counts.logs != 1 || counts.batches != 1 || counts.archives != 1 || strings.Count(counts.batchInput, "\n") != 2 {
		t.Fatalf("nested walk used log=%d batch=%d archives=%d batch-lines=%d, want 1, 1, 1, 2", counts.logs, counts.batches, counts.archives, strings.Count(counts.batchInput, "\n"))
	}
}

func TestWitnessResolverLedgerOnlyLandingUsesCachedProjection(t *testing.T) {
	root := initRearmRepo(t)
	source := commitRearmTree(t, root, "source")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, source)
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	counts := observeWitnessWalk(t, nil)
	if resolved, err := resolveWitnessStamp(root, root, ref, digest[:12]); err != nil || resolved != source {
		t.Fatalf("warm source cache: got=%s want=%s err=%v", resolved, source, err)
	}
	if counts.archives != 1 {
		t.Fatalf("cold source used %d archives, want 1", counts.archives)
	}
	writeRearmFile(t, filepath.Join(root, "plans", "goals", "peer.md"), "ledger-only landing\n")
	rearmGit(t, root, "add", "plans/goals/peer.md")
	rearmGit(t, root, "commit", "-qm", "ledger-only landing")
	landed := rearmGit(t, root, "rev-parse", "HEAD")
	rearmGit(t, root, "update-ref", ref, landed)
	counts.logs, counts.batches, counts.archives = 0, 0, 0
	resolved, err := resolveWitnessStamp(root, root, ref, digest[:12])
	if err != nil || resolved != landed {
		t.Fatalf("ledger-only resolution: got=%s want=%s err=%v", resolved, landed, err)
	}
	if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
		t.Fatalf("ledger-only next run used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
	}
}
