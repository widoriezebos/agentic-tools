package contractgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryAttributesNameTestingMergeDriver(t *testing.T) {
	data, err := os.ReadFile("../../../../.gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "metasystem/testing.json merge=metasystem-testing\n") {
		t.Fatalf("root .gitattributes does not assign the testing contract merge driver:\n%s", data)
	}
}

func TestDriverArgsRunQuotedExecutable(t *testing.T) {
	repo, ours, theirs := contractMergeRepository(t)
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(t.TempDir(), "driver path with space", "metasystem")
	if err := os.MkdirAll(filepath.Dir(driver), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(testBinary, driver); err != nil {
		t.Fatal(err)
	}
	args, err := DriverArgs(func() (string, error) { return driver, nil })
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", append(append([]string{"-C", repo}, args...), "merge", "--no-edit", theirs)...)
	command.Env = append(os.Environ(), "METASYSTEM_CONTRACT_DRIVER_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("merge through quoted driver: %v\n%s", err, output)
	}
	if got, want := git(t, repo, "show", "HEAD:metasystem/testing.json"), strings.TrimSpace(string(historyContract(t, "want"))); got != want {
		t.Fatalf("merge through quoted driver differs from the semantic merge:\n%s", got)
	}
	if head := git(t, repo, "rev-parse", "HEAD^"); head != ours {
		t.Fatalf("merge first parent = %s, want %s", head, ours)
	}
}

func TestDriverArgsUnresolvableExecutableRefuses(t *testing.T) {
	testDriverArgsUnresolvableExecutableRefuses(t)
}

func TestDriverArgsUnresolvableExecutableFallsBack(t *testing.T) {
	testDriverArgsUnresolvableExecutableRefuses(t)
}

func testDriverArgsUnresolvableExecutableRefuses(t *testing.T) {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "missing", "metasystem")
	args, err := DriverArgs(func() (string, error) { return missing, nil })
	if err == nil || len(args) != 0 || !strings.Contains(err.Error(), DriverUnresolvedCode) || !strings.Contains(err.Error(), "installed metasystem binary") {
		t.Fatalf("unresolvable executable args=%v err=%v", args, err)
	}
}

func contractMergeRepository(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "user.name", "fixture")
	git(t, repo, "config", "user.email", "fixture@example.invalid")
	writeFile(t, repo, "metasystem/testing.json", string(historyContract(t, "base")))
	writeFile(t, repo, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-qm", "base")
	base := git(t, repo, "rev-parse", "HEAD")

	writeFile(t, repo, "metasystem/testing.json", string(historyContract(t, "ours")))
	git(t, repo, "add", "metasystem/testing.json")
	git(t, repo, "commit", "-qm", "ours")
	ours := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "switch", "--quiet", "-c", "theirs", base)
	writeFile(t, repo, "metasystem/testing.json", string(historyContract(t, "theirs")))
	git(t, repo, "add", "metasystem/testing.json")
	git(t, repo, "commit", "-qm", "theirs")
	theirs := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "switch", "--quiet", "main")
	return repo, ours, theirs
}

func historyContract(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "contractmerge", "testdata", "history-"+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
