package batch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func TestEveryBatchThreeWayApplyUsesTestingMergeDriver(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := []string{"landing/batch", "landing", "gittree", "goal/branch"}
	var missing []string
	for _, dir := range dirs {
		paths, err := filepath.Glob(filepath.Join(root, dir, "*.go"))
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
			for number, line := range strings.Split(string(data), "\n") {
				contentMerge := strings.Contains(line, `"--3way"`) || strings.Contains(line, `"cherry-pick"`) ||
					strings.Contains(line, `"rebase", upstream`) || strings.Contains(line, `"merge"`)
				// Argument construction is checked at the call that consumes it;
				// it is not itself a Git invocation.
				if strings.Contains(line, `args = append(args, "--3way")`) {
					continue
				}
				if !contentMerge || strings.Contains(line, `"rebase", "--abort"`) || strings.Contains(line, `"--ff-only"`) {
					continue
				}
				carriesDriver := strings.Contains(line, "runBatchMergeGit(") || strings.Contains(line, "runPlanningMergeGit(") ||
					strings.Contains(line, "gitInput(") || strings.Contains(line, "gitOutput(") || strings.Contains(line, "gitArgs")
				if !carriesDriver {
					missing = append(missing, fmt.Sprintf("%s:%d", filepath.ToSlash(path), number+1))
				}
			}
		}
	}
	if len(missing) != 0 {
		t.Fatalf("content-merging Git call sites without testing merge-driver arguments: %s", strings.Join(missing, ", "))
	}
}

func TestContentMergingVerbsRefuseWithoutTheDriver(t *testing.T) {
	saved := batchMergeDriverArgs
	batchMergeDriverArgs = func() ([]string, error) {
		return contractgit.DriverArgs(func() (string, error) { return filepath.Join(t.TempDir(), "missing-metasystem"), nil })
	}
	t.Cleanup(func() { batchMergeDriverArgs = saved })
	_, err := AssembleBranchMembers("not-a-repository", "not-a-tree", []BranchMember{{GoalID: "goal-a"}})
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.DriverUnresolvedCode {
		t.Fatalf("batch composition refusal = %v", err)
	}
}

func TestOrdinaryFileCannotOptIntoTheTestingMergeDriver(t *testing.T) {
	bed, endpoint, member, contentCommit := ordinaryDriverBatchFixture(t)
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	_, err := AssembleBranchMembers(bed.root, endpoint, []BranchMember{member})
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.AttributeMisuseCode ||
		!strings.Contains(err.Error(), "ordinary.json") || !strings.Contains(err.Error(), contentCommit) {
		t.Fatalf("ordinary merge-driver selection refusal = %v", err)
	}
}

func ordinaryDriverBatchFixture(t *testing.T) (goalBranchBed, string, BranchMember, string) {
	t.Helper()
	root := t.TempDir()
	branchGit(t, root, "init", "-q", "-b", "main")
	branchGit(t, root, "config", "user.name", "Batch Fixture")
	branchGit(t, root, "config", "user.email", "batch@example.invalid")
	branchWrite(t, root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeBatchContractAt(t, root, "ordinary.json", batchContractFixture())
	branchGit(t, root, "add", ".")
	branchGit(t, root, "commit", "-qm", "base")
	base := branchGit(t, root, "rev-parse", "HEAD")

	branchWrite(t, root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\nordinary.json merge=metasystem-testing\n")
	branchGit(t, root, "add", ".gitattributes")
	branchGit(t, root, "commit", "-qm", "opt ordinary file into driver")
	attributeCommit := branchGit(t, root, "rev-parse", "HEAD")
	writeBatchContractAt(t, root, "ordinary.json", batchContractWithAddition(batchContractFixture(), "incoming", 1200))
	branchGit(t, root, "add", "ordinary.json")
	branchGit(t, root, "commit", "-qm", "incoming ordinary change")
	contentCommit := branchGit(t, root, "rev-parse", "HEAD")

	build := func(commit string) BranchBuild {
		digest, err := goalbranch.UnitDigest(root, commit)
		must(t, err)
		return BranchBuild{Units: []string{commit[:8]}, Commit: commit, Digest: digest}
	}
	member := BranchMember{GoalID: "goal-a", Tip: contentCommit, Builds: []BranchBuild{build(attributeCommit), build(contentCommit)}}

	branchGit(t, root, "switch", "--quiet", "--detach", base)
	writeBatchContractAt(t, root, "ordinary.json", batchContractWithAddition(batchContractFixture(), "endpoint", 1100))
	branchGit(t, root, "add", "ordinary.json")
	branchGit(t, root, "commit", "-qm", "endpoint ordinary change")
	endpoint := branchGit(t, root, "rev-parse", "HEAD^{tree}")
	return goalBranchBed{root: root, base: base}, endpoint, member, contentCommit
}

func TestHandRebasedTestingContractHunkRefusesToLand(t *testing.T) {
	bed, endpoint, member, hand := handMergedBatchFixture(t)
	saved := batchMergeDriverArgs
	batchMergeDriverArgs = func() ([]string, error) {
		return []string{"-c", "merge.metasystem-testing.name=fixture", "-c", "merge.metasystem-testing.driver=cp '" + hand + "' %A"}, nil
	}
	t.Cleanup(func() { batchMergeDriverArgs = saved })

	_, err := AssembleBranchMembers(bed.root, endpoint, []BranchMember{member})
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.ContractHandMergedCode ||
		!strings.Contains(err.Error(), member.Builds[0].Commit) || !strings.Contains(err.Error(), "byte ") {
		t.Fatalf("hand-merged testing contract refusal = %v", err)
	}

	batchMergeDriverArgs = saved
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	if prefixes, err := AssembleBranchMembers(bed.root, endpoint, []BranchMember{member}); err != nil || len(prefixes) != 1 {
		t.Fatalf("semantic testing contract result prefixes=%v err=%v", prefixes, err)
	}
}

func handMergedBatchFixture(t *testing.T) (goalBranchBed, string, BranchMember, string) {
	t.Helper()
	root := t.TempDir()
	branchGit(t, root, "init", "-q", "-b", "main")
	branchGit(t, root, "config", "user.name", "Batch Fixture")
	branchGit(t, root, "config", "user.email", "batch@example.invalid")
	branchWrite(t, root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeBatchContract(t, root, batchContractFixture())
	branchGit(t, root, "add", ".")
	branchGit(t, root, "commit", "-qm", "base")
	base := branchGit(t, root, "rev-parse", "HEAD")

	writeBatchContract(t, root, batchContractWithAddition(batchContractFixture(), "incoming", 1200))
	branchGit(t, root, "add", "metasystem/testing.json")
	branchGit(t, root, "commit", "-qm", "incoming testing contract")
	commit := branchGit(t, root, "rev-parse", "HEAD")
	digest, err := goalbranch.UnitDigest(root, commit)
	must(t, err)
	member := BranchMember{GoalID: "goal-a", Tip: commit, Builds: []BranchBuild{{Units: []string{"testing"}, Commit: commit, Digest: digest}}}

	branchGit(t, root, "switch", "--quiet", "--detach", base)
	writeBatchContract(t, root, batchContractWithAddition(batchContractFixture(), "endpoint", 1100))
	branchGit(t, root, "add", "metasystem/testing.json")
	branchGit(t, root, "commit", "-qm", "endpoint testing contract")
	endpoint := branchGit(t, root, "rev-parse", "HEAD^{tree}")

	handContract := batchContractWithAddition(batchContractWithAddition(batchContractFixture(), "endpoint", 1100), "incoming", 1200)
	handContract.Surfaces[2].Standard = append(handContract.Surfaces[2].Standard, "incoming-group")
	handContract.Surfaces[3].Standard = []string{}
	handBytes, err := contractmerge.Render(handContract)
	must(t, err)
	hand := filepath.Join(t.TempDir(), "hand.json")
	must(t, os.WriteFile(hand, handBytes, 0o644))
	return goalBranchBed{root: root, base: base}, endpoint, member, hand
}

func writeBatchContractAt(t *testing.T, root, path string, contract testpolicy.Contract) {
	t.Helper()
	data, err := contractmerge.Render(contract)
	must(t, err)
	branchWrite(t, root, path, string(data))
}
