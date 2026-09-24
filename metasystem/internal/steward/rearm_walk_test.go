package steward

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type witnessWalkCounts struct {
	logs, batches, archives int
	batchInput              string
}

func observeWitnessWalk(t *testing.T, deps *rearmResolverDeps, digester func(context.Context, string, string, behaviorsurface.Policy) (string, error)) *witnessWalkCounts {
	t.Helper()
	originalRunner, originalDigester := deps.witnessGit, deps.witnessTreeDigest
	counts := &witnessWalkCounts{}
	deps.witnessGit = func(ctx context.Context, root string, input []byte, progress func(), args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "log" {
			counts.logs++
		}
		if len(args) > 0 && args[0] == "cat-file" {
			counts.batches++
			counts.batchInput = string(input)
		}
		return originalRunner(ctx, root, input, progress, args...)
	}
	deps.witnessTreeDigest = func(ctx context.Context, toplevel, tree string, policy behaviorsurface.Policy, clock RearmClock, seconds int) (string, error) {
		counts.archives++
		if digester != nil {
			return digester(ctx, toplevel, tree, policy)
		}
		return originalDigester(ctx, toplevel, tree, policy, clock, seconds)
	}
	return counts
}

func TestWitnessResolverWalksInOneProcessAndDigestsOnlyDistinctProjections(t *testing.T) {
	targetDigest, otherDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	t.Run("200 changed projections", func(t *testing.T) {
		root := canonicalPath(t.TempDir())
		ref := "refs/remotes/origin/trunk"
		first := rearmID(1)
		var history strings.Builder
		for candidate := 199; candidate >= 0; candidate-- {
			parent := ""
			old := strings.Repeat("0", 40)
			if candidate > 0 {
				parent = rearmID(candidate)
				old = rearmID(candidate + 200)
			}
			history.WriteString(rearmHistory(rearmID(candidate+1), rearmID(candidate+101), parent, rearmRawChange("cmd/surface.txt", old, rearmID(candidate+201))))
		}
		deps := rearmTestDeps(t, rearmWitnessExpectations(root, ref, history.String())...)
		counts := observeWitnessWalk(t, &deps, func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy) (string, error) {
			if gotRoot != root {
				t.Fatalf("archive root %q", gotRoot)
			}
			if spec == first {
				return targetDigest, nil
			}
			return otherDigest, nil
		})
		resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("200-candidate resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 200 {
			t.Fatalf("200-candidate walk used log=%d batch=%d archives=%d, want 1, 0, 200", counts.logs, counts.batches, counts.archives)
		}
	})
	t.Run("65 candidates in five projection classes", func(t *testing.T) {
		root := canonicalPath(t.TempDir())
		ref := "refs/remotes/origin/trunk"
		first := rearmID(1)
		var history strings.Builder
		for candidate := 64; candidate >= 0; candidate-- {
			parent := ""
			if candidate > 0 {
				parent = rearmID(candidate)
			}
			projection := candidate
			if projection > 4 {
				projection = 4
			}
			change := rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201))
			if candidate > 0 && candidate < 5 {
				change = rearmRawChange("cmd/surface.txt", rearmID(candidate+200), rearmID(candidate+201))
			}
			if candidate >= 5 {
				change = rearmRawChange("docs/history.txt", rearmID(candidate+300), rearmID(candidate+301))
			}
			history.WriteString(rearmHistory(rearmID(candidate+1), rearmID(projection+101), parent, change))
		}
		expected := append([]testgit.Expectation{}, rearmWitnessExpectations(root, ref, history.String())...)
		expected = append(expected, rearmWitnessExpectations(root, ref, history.String())...)
		deps := rearmTestDeps(t, expected...)
		counts := observeWitnessWalk(t, &deps, func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy) (string, error) {
			if gotRoot != root {
				t.Fatalf("archive root %q", gotRoot)
			}
			if spec == first {
				return targetDigest, nil
			}
			return otherDigest, nil
		})
		resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("cold class resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 5 {
			t.Fatalf("cold class walk used log=%d batch=%d archives=%d, want 1, 0, 5", counts.logs, counts.batches, counts.archives)
		}
		counts.logs, counts.batches, counts.archives = 0, 0, 0
		resolved, err = resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12])
		if err != nil || resolved != first {
			t.Fatalf("warm class resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
			t.Fatalf("warm class walk used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
		}
	})
	t.Run("root and merge", func(t *testing.T) {
		root := canonicalPath(t.TempDir())
		ref := "refs/remotes/origin/trunk"
		first, side, trunk, merge := rearmID(1), rearmID(2), rearmID(3), rearmID(4)
		tree := rearmID(101)
		rootHistory := rearmHistory(first, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
		mergeHistory := fmt.Sprintf("\x01%s %s %s %s\x00", merge, tree, trunk, side) + rearmHistory(trunk, tree, first, rearmRawChange("docs/trunk.txt", strings.Repeat("0", 40), rearmID(203))) + rearmHistory(side, tree, first, rearmRawChange("docs/side.txt", strings.Repeat("0", 40), rearmID(202))) + rootHistory
		expected := append([]testgit.Expectation{}, rearmWitnessExpectations(root, ref, rootHistory)...)
		expected = append(expected, rearmWitnessExpectations(root, ref, mergeHistory)...)
		deps := rearmTestDeps(t, expected...)
		counts := observeWitnessWalk(t, &deps, func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy) (string, error) {
			if gotRoot != root || spec != first {
				t.Fatalf("archive root=%q spec=%q", gotRoot, spec)
			}
			return targetDigest, nil
		})
		if resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12]); err != nil || resolved != first {
			t.Fatalf("root resolution: got=%s want=%s err=%v", resolved, first, err)
		}
		if counts.logs != 1 || counts.archives != 1 {
			t.Fatalf("root walk used log=%d archives=%d, want 1, 1", counts.logs, counts.archives)
		}
		counts.logs, counts.archives = 0, 0
		if resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12]); err != nil || resolved != merge {
			t.Fatalf("merge resolution: got=%s want=%s err=%v", resolved, merge, err)
		}
		if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
			t.Fatalf("cached merge class used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
		}
	})
}

func TestWitnessHistoryParserPreservesNULDelimitedPathsAndRenames(t *testing.T) {
	root := canonicalPath(t.TempDir())
	oldPath := "cmd/space name.txt"
	newPath := "cmd/renamed name.txt"
	newlinePath := "cmd/line\nbreak.txt"
	zero := strings.Repeat("0", 40)
	first, newest := rearmID(1), rearmID(2)
	history := rearmHistory(newest, rearmID(102), first, rearmRawChange(oldPath, rearmID(201), zero)+rearmRawChange(newPath, zero, rearmID(201))) +
		rearmHistory(first, rearmID(101), "", rearmRawChange(oldPath, zero, rearmID(201))+rearmRawChange(newlinePath, zero, rearmID(202)))
	deps := rearmTestDeps(t, rearmExpected(root, history, nil, "log", "--topo-order", "--no-abbrev", "--raw", "-z", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--diff-merges=first-parent", "--format=%x01%H %T %P", "HEAD"))
	counts := observeWitnessWalk(t, &deps, func(context.Context, string, string, behaviorsurface.Policy) (string, error) { return "", nil })
	candidates, err := readWitnessCandidatesWithDeps(deps, SystemRearmClock(), deps.resolveSeconds(root), root, "HEAD")
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
	root := filepath.Join(outer, "metasystem")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/nested\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "nested\n")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	source, landed := rearmID(1), rearmID(2)
	tree := rearmID(101)
	ref := "refs/remotes/origin/trunk"
	history := rearmHistory(landed, rearmID(102), source, rearmRawChange("plans/goals/peer.md", strings.Repeat("0", 40), rearmID(202))) +
		rearmHistory(source, tree, "", rearmRawChange("metasystem/cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	input := landed + ":metasystem\n" + source + ":metasystem\n"
	expected := []testgit.Expectation{
		rearmExpected(root, outer+"\n", nil, "rev-parse", "--show-toplevel"),
		rearmExpected(root, history, nil, "log", "--topo-order", "--no-abbrev", "--raw", "-z", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--diff-merges=first-parent", "--format=%x01%H %T %P", ref),
		rearmBatch(root, input, tree+"\n"+tree+"\n"),
	}
	deps := rearmTestDeps(t, expected...)
	counts := observeWitnessWalk(t, &deps, func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy) (string, error) {
		if gotRoot != outer || spec != landed+":metasystem" {
			t.Fatalf("nested archive root=%q spec=%q", gotRoot, spec)
		}
		return digest, nil
	})
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), outer, root, ref, digest[:12])
	if err != nil || resolved != landed {
		t.Fatalf("nested resolution: got=%s want=%s err=%v", resolved, landed, err)
	}
	if counts.logs != 1 || counts.batches != 1 || counts.archives != 1 || strings.Count(counts.batchInput, "\n") != 2 {
		t.Fatalf("nested walk used log=%d batch=%d archives=%d batch-lines=%d, want 1, 1, 1, 2", counts.logs, counts.batches, counts.archives, strings.Count(counts.batchInput, "\n"))
	}
}

func TestWitnessResolverLedgerOnlyLandingUsesCachedProjection(t *testing.T) {
	root := canonicalPath(t.TempDir())
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "source\n")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	source, landed, tree := rearmID(1), rearmID(2), rearmID(101)
	ref := "refs/remotes/origin/trunk"
	sourceHistory := rearmHistory(source, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	landedHistory := rearmHistory(landed, tree, source, rearmRawChange("plans/goals/peer.md", strings.Repeat("0", 40), rearmID(202))) + sourceHistory
	expected := append([]testgit.Expectation{}, rearmWitnessExpectations(root, ref, sourceHistory)...)
	expected = append(expected, rearmWitnessExpectations(root, ref, landedHistory)...)
	deps := rearmTestDeps(t, expected...)
	counts := observeWitnessWalk(t, &deps, func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy) (string, error) {
		if gotRoot != root || spec != source {
			t.Fatalf("archive root=%q spec=%q", gotRoot, spec)
		}
		return digest, nil
	})
	if resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12]); err != nil || resolved != source {
		t.Fatalf("warm source cache: got=%s want=%s err=%v", resolved, source, err)
	}
	if counts.archives != 1 {
		t.Fatalf("cold source used %d archives, want 1", counts.archives)
	}
	writeRearmFile(t, filepath.Join(root, "plans", "goals", "peer.md"), "ledger-only landing\n")
	counts.logs, counts.batches, counts.archives = 0, 0, 0
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12])
	if err != nil || resolved != landed {
		t.Fatalf("ledger-only resolution: got=%s want=%s err=%v", resolved, landed, err)
	}
	if counts.logs != 1 || counts.batches != 0 || counts.archives != 0 {
		t.Fatalf("ledger-only next run used log=%d batch=%d archives=%d, want 1, 0, 0", counts.logs, counts.batches, counts.archives)
	}
}
