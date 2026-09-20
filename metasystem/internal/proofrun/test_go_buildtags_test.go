package proofrun

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNativeDiscoveryUsesExplicitGoPlatformAndBuildTags(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOOS", "darwin")
	t.Setenv("GOARCH", runtime.GOARCH)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOENV", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOCACHE", filepath.Join(root, "cache"))
	for path, content := range map[string]string{
		"go.mod":                    "module example.invalid/explicitcheck\n\ngo 1.27\n",
		"pkg/plain.go":              "package pkg\n",
		"pkg/target_darwin_test.go": "//go:build checkenv\n\npackage pkg\nimport \"testing\"\nfunc TestDarwin(t *testing.T) {}\n",
		"pkg/target_linux_test.go":  "//go:build checkenv\n\npackage pkg\nimport \"testing\"\nfunc TestLinux(t *testing.T) {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	group := testpolicy.Group{ID: "linux-check", Adapter: "go", CWD: ".", Packages: []string{"pkg"},
		BuildTags: []string{"checkenv"}, Tests: json.RawMessage(`["TestLinux"]`), EnvironmentMode: "explicit",
		Env: map[string]string{"PATH": os.Getenv("PATH"), "GOOS": "linux", "GOARCH": runtime.GOARCH,
			"GOCACHE": os.Getenv("GOCACHE"), "GOENV": "off", "GOTOOLCHAIN": "local"}}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, Groups: []testpolicy.Group{group}}
	if err := CheckNativeDiscovery(context.Background(), root, root, contract, os.Environ()); err != nil {
		t.Fatalf("explicit linux tagged inventory was refused: %v", err)
	}
	group.Env["GOOS"] = "darwin"
	if err := CheckNativeDiscovery(context.Background(), root, root, testpolicy.Contract{SchemaVersion: contract.SchemaVersion,
		Groups: []testpolicy.Group{group}}, os.Environ()); err == nil || !strings.Contains(err.Error(), "TestLinux") {
		t.Fatalf("non-linux environment certified a linux-only test: %v", err)
	}
	t.Setenv("GOOS", "linux")
	group.EnvironmentMode, group.Env, group.Tests = "inherit", nil, json.RawMessage(`["TestDarwin"]`)
	baseEnvironment := []string{"PATH=" + os.Getenv("PATH"), "GOOS=darwin", "GOARCH=" + runtime.GOARCH,
		"GOCACHE=" + os.Getenv("GOCACHE"), "GOENV=off", "GOTOOLCHAIN=local"}
	if err := CheckNativeDiscovery(context.Background(), root, root, testpolicy.Contract{SchemaVersion: contract.SchemaVersion,
		Groups: []testpolicy.Group{group}}, baseEnvironment); err != nil {
		t.Fatalf("inherited inventory ignored supplied execution environment: %v", err)
	}
	baseEnvironment[1] = "GOOS=linux"
	if err := CheckNativeDiscovery(context.Background(), root, root, testpolicy.Contract{SchemaVersion: contract.SchemaVersion,
		Groups: []testpolicy.Group{group}}, baseEnvironment); err == nil || !strings.Contains(err.Error(), "TestDarwin") {
		t.Fatalf("inherited linux environment certified a darwin-only test: %v", err)
	}
}

func TestNativeDiscoveryGoListCompletesUnderHeldResourceCustody(t *testing.T) {
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", engine, "./cmd/metasystem")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build discovery custodian: %v\n%s", err, output)
	}
	root := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":          "module example.invalid/custodied-discovery\n\ngo 1.27\n",
		"pkg/pkg.go":      "package pkg\n",
		"pkg/pkg_test.go": "package pkg\nimport \"testing\"\nfunc TestReady(t *testing.T) {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	admissionRoot, conf := isolatedHostResources(t)
	ctx := WithResourceCustodyExecutable(context.Background(), engine)
	lease, err := AcquireHostResources(ctx, root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithHostResourceLease(ctx, lease)
	group := testpolicy.Group{ID: "go-list", Adapter: "go", CWD: ".", Packages: []string{"pkg"}, Tests: json.RawMessage(`["TestReady"]`)}
	checkErr := CheckNativeDiscovery(ctx, root, root, testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		Groups: []testpolicy.Group{group}}, os.Environ())
	closeErr := lease.Close()
	if checkErr != nil || closeErr != nil {
		t.Fatalf("custodied go discovery: check=%v close=%v", checkErr, closeErr)
	}
	dirty, _, err := hostLeaseState(admissionRoot)
	if err != nil || dirty != 0 {
		t.Fatalf("completed go discovery retained dirty admission: dirty=%d err=%v", dirty, err)
	}
}

func TestGoAdapterBuildTagsBindDiscoveryAndNativeExecution(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":             "module example.invalid/tagged\n\ngo 1.27\n",
		"pkg/plain.go":       "package pkg\n",
		"pkg/plain_test.go":  "package pkg\nimport \"testing\"\nfunc TestPlain(t *testing.T) {}\n",
		"pkg/tagged_test.go": "//go:build batchtest\n\npackage pkg\nimport \"testing\"\nfunc TestOnlyBatchTag(t *testing.T) {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	group := testpolicy.Group{Adapter: "go", Packages: []string{"pkg"}, BuildTags: []string{"batchtest"}, Tests: json.RawMessage(`"all"`)}
	argv, expected, discovery, _, err := goArguments(context.Background(), group, root, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(argv, "batchtest") || !slices.Contains(discovery.Inputs, "pkg/tagged_test.go") {
		t.Fatalf("tagged discovery missing from argv=%v inputs=%v", argv, discovery.Inputs)
	}
	found := false
	for _, identity := range expected {
		if identity.Name == "TestOnlyBatchTag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tagged test absent from expected inventory: %v", expected)
	}
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("tagged native test failed: %v: %s", err, output)
	}
	if !strings.Contains(string(output), `"Test":"TestOnlyBatchTag"`) {
		t.Fatalf("tagged native test did not run: %s", output)
	}
}

func TestGoTaggedFailureRerunUsesOriginalBuildTags(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/tagged-rerun\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "plain.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "tagged_test.go"), []byte(`//go:build batchtest

package pkg
import "testing"
func TestTaggedRed(t *testing.T) { t.Fatal("tagged red") }
`), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: "tagged-red", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "pkg/**"},
		Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"pkg"}, BuildTags: []string{"batchtest"}, Tests: []byte(`"all"`)}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), LogRoot: filepath.Join(root, "logs")}
	request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	result := runTestGroup(context.Background(), request, group)
	if result.Status != "failed" || len(result.Reruns) != 1 || result.Reruns[0].First != "failed" || result.Reruns[0].Second != "failed" {
		t.Fatalf("tagged failure rerun lost native tag: %+v", result)
	}
	rerun, err := os.ReadFile(result.Reruns[0].LogPath)
	if err != nil || !strings.Contains(string(rerun), `"Test":"TestTaggedRed"`) {
		t.Fatalf("tagged rerun did not execute original test: %v: %s", err, rerun)
	}
}
