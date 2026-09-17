package branch

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func TestGoalBranchGitRunnersSupplyTestingMergeDriver(t *testing.T) {
	for _, path := range []string{"digest.go", "commit.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `branchGitCommand(repo, args...)`) {
			t.Fatalf("%s Git runner does not supply the testing merge driver", path)
		}
	}
}

func TestContentMergingVerbsRefuseWithoutTheDriver(t *testing.T) {
	saved := branchMergeDriverArgs
	branchMergeDriverArgs = func() ([]string, error) {
		return contractgit.DriverArgs(func() (string, error) { return filepath.Join(t.TempDir(), "missing-metasystem"), nil })
	}
	t.Cleanup(func() { branchMergeDriverArgs = saved })
	_, err := PrepareLanding(LandRequest{})
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.DriverUnresolvedCode {
		t.Fatalf("branch landing refusal = %v", err)
	}
}

func TestOrdinaryFileCannotOptIntoTheTestingMergeDriver(t *testing.T) {
	repo, endpoint, attributeCommit, contentCommit := ordinaryDriverBranchFixture(t)
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	mergeDriverGit(t, repo, "reset", "--hard", endpoint)
	if err := applyCommit(repo, attributeCommit); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(repo, "write-tree"); err != nil {
		t.Fatal(err)
	}
	err := applyCommit(repo, contentCommit)
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.AttributeMisuseCode ||
		!strings.Contains(err.Error(), "ordinary.json") || !strings.Contains(err.Error(), contentCommit) {
		t.Fatalf("ordinary merge-driver selection refusal = %v", err)
	}
}

func ordinaryDriverBranchFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	repo := newMergeDriverRepo(t, "ordinary.json")
	base := mergeDriverGit(t, repo, "rev-parse", "HEAD")
	mergeDriverWrite(t, repo, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\nordinary.json merge=metasystem-testing\n")
	mergeDriverGit(t, repo, "add", ".gitattributes")
	mergeDriverGit(t, repo, "commit", "-qm", "opt ordinary file into driver")
	attributeCommit := mergeDriverGit(t, repo, "rev-parse", "HEAD")
	writeMergeDriverContract(t, repo, "ordinary.json", mergeDriverContractWithAddition(mergeDriverContractFixture(), "incoming", 1200))
	mergeDriverGit(t, repo, "add", "ordinary.json")
	mergeDriverGit(t, repo, "commit", "-qm", "incoming ordinary change")
	contentCommit := mergeDriverGit(t, repo, "rev-parse", "HEAD")
	mergeDriverGit(t, repo, "switch", "--quiet", "--detach", base)
	writeMergeDriverContract(t, repo, "ordinary.json", mergeDriverContractWithAddition(mergeDriverContractFixture(), "endpoint", 1100))
	mergeDriverGit(t, repo, "add", "ordinary.json")
	mergeDriverGit(t, repo, "commit", "-qm", "endpoint ordinary change")
	return repo, mergeDriverGit(t, repo, "rev-parse", "HEAD"), attributeCommit, contentCommit
}

func TestHandRebasedTestingContractHunkRefusesToLand(t *testing.T) {
	repo, endpoint, incoming, hand := handMergedBranchFixture(t)
	saved := branchMergeDriverArgs
	branchMergeDriverArgs = func() ([]string, error) {
		return []string{"-c", "merge.metasystem-testing.name=fixture", "-c", "merge.metasystem-testing.driver=cp '" + hand + "' %A"}, nil
	}
	t.Cleanup(func() { branchMergeDriverArgs = saved })
	mergeDriverGit(t, repo, "reset", "--hard", endpoint)
	err := applyCommit(repo, incoming)
	var refusal *contractgit.Refusal
	if !errors.As(err, &refusal) || refusal.Code != contractgit.ContractHandMergedCode ||
		!strings.Contains(err.Error(), incoming) || !strings.Contains(err.Error(), "byte ") {
		t.Fatalf("hand-merged testing contract refusal = %v", err)
	}

	branchMergeDriverArgs = saved
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	mergeDriverGit(t, repo, "reset", "--hard", endpoint)
	if err := applyCommit(repo, incoming); err != nil {
		t.Fatalf("semantic testing contract result: %v", err)
	}
}

func handMergedBranchFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	repo := newMergeDriverRepo(t, contractgit.TestingContractPath)
	base := mergeDriverGit(t, repo, "rev-parse", "HEAD")
	writeMergeDriverContract(t, repo, contractgit.TestingContractPath, mergeDriverContractWithAddition(mergeDriverContractFixture(), "incoming", 1200))
	mergeDriverGit(t, repo, "add", contractgit.TestingContractPath)
	mergeDriverGit(t, repo, "commit", "-qm", "incoming testing contract")
	incoming := mergeDriverGit(t, repo, "rev-parse", "HEAD")
	mergeDriverGit(t, repo, "switch", "--quiet", "--detach", base)
	writeMergeDriverContract(t, repo, contractgit.TestingContractPath, mergeDriverContractWithAddition(mergeDriverContractFixture(), "endpoint", 1100))
	mergeDriverGit(t, repo, "add", contractgit.TestingContractPath)
	mergeDriverGit(t, repo, "commit", "-qm", "endpoint testing contract")
	endpoint := mergeDriverGit(t, repo, "rev-parse", "HEAD")

	handContract := mergeDriverContractWithAddition(mergeDriverContractWithAddition(mergeDriverContractFixture(), "endpoint", 1100), "incoming", 1200)
	handContract.Surfaces[2].Standard = append(handContract.Surfaces[2].Standard, "incoming-group")
	handContract.Surfaces[3].Standard = []string{}
	handBytes, err := contractmerge.Render(handContract)
	if err != nil {
		t.Fatal(err)
	}
	hand := filepath.Join(t.TempDir(), "hand.json")
	if err := os.WriteFile(hand, handBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, endpoint, incoming, hand
}

func newMergeDriverRepo(t *testing.T, contractPath string) string {
	t.Helper()
	repo := t.TempDir()
	mergeDriverGit(t, repo, "init", "-q", "-b", "main")
	mergeDriverGit(t, repo, "config", "user.name", "Branch Fixture")
	mergeDriverGit(t, repo, "config", "user.email", "branch@example.invalid")
	mergeDriverWrite(t, repo, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeMergeDriverContract(t, repo, contractPath, mergeDriverContractFixture())
	mergeDriverGit(t, repo, "add", ".")
	mergeDriverGit(t, repo, "commit", "-qm", "base")
	return repo
}

func mergeDriverContractFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			{ID: "base", Paths: []string{"base/**"}, DependsOn: []string{}, Standard: []string{"base-group"}, Deep: []string{}, Critical: []string{}},
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"base-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups: []testpolicy.Group{mergeDriverGroup("base-group", 1000)}, Always: testpolicy.Always{Canary: []string{}, Standard: []string{}},
		Unknown: []string{"base-group"}, Cadence: []string{}}
}

func mergeDriverContractWithAddition(contract testpolicy.Contract, name string, target int64) testpolicy.Contract {
	id := name + "-group"
	contract.Groups = append(contract.Groups, mergeDriverGroup(id, target))
	surface := testpolicy.Surface{ID: name, Paths: []string{name + "/**"}, DependsOn: []string{}, Standard: []string{id}, Deep: []string{}, Critical: []string{}}
	contract.Surfaces = append(contract.Surfaces, surface)
	for index := range contract.Surfaces {
		if contract.Surfaces[index].ID == contract.Fallback {
			contract.Surfaces[index].Standard = append(contract.Surfaces[index].Standard, id)
		}
	}
	contract.Unknown = append(contract.Unknown, id)
	return contract
}

func mergeDriverGroup(id string, target int64) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{},
		Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: target,
		Packages: []string{"./example"}, Tests: json.RawMessage(`["TestExample"]`)}
}

func writeMergeDriverContract(t *testing.T, root, path string, contract testpolicy.Contract) {
	t.Helper()
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	mergeDriverWrite(t, root, path, string(data))
}

func mergeDriverWrite(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mergeDriverGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
