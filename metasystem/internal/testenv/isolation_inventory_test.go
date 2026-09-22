package testenv

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const isolationInventoryUpdate = "ISOLATION_INVENTORY_UPDATE"

type isolationOperation struct {
	source, function, kind, fingerprint, semantic, evidence string
	line                                                    int
}

type isolationInventoryRow struct {
	source, kind, fingerprint, semantic, classification, owner, evidence string
}

func (row isolationInventoryRow) key() string {
	return row.source + "\t" + row.kind + "\t" + row.fingerprint
}

var shellTimeOperation = regexp.MustCompile(`(^|[^[:alnum:]_])(sleep|timeout|date)([[:space:]]|$)|\bSECONDS\b`)
var fixedShellPort = regexp.MustCompile(`(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]):[1-9][0-9]+|--?port([=[:space:]]+)[1-9][0-9]+`)
var fixedShellPath = regexp.MustCompile(`(^|[[:space:]"'=])/(tmp|var/tmp|run|var/run)/[^[:space:]"']+`)

var isolationGoSupportManifest = []string{
	"internal/identity/fixture_custodian.go",
	"internal/testenv/process_group.go",
	"internal/testenv/testenv.go",
	"internal/testexec/testexec.go",
	"internal/testutil/expect.go",
	"internal/testutil/fixture.go",
	"internal/testutil/wait_binary.go",
}

var isolationShellSupportManifest = []string{
	"scripts/agents/acp-fixtures.sh",
	"scripts/agents/actionable-metrics-fixtures.sh",
	"scripts/agents/adapter-deadline-fixtures.sh",
	"scripts/agents/adapters/fake.sh",
	"scripts/agents/authority-regression-fixtures.sh",
	"scripts/agents/brain-fixtures.sh",
	"scripts/agents/channel-fixtures.sh",
	"scripts/agents/checkout-execution-guard-fixtures.sh",
	"scripts/agents/config-identity-fixtures.sh",
	"scripts/agents/conformance-fixtures.sh",
	"scripts/agents/delegate-caps-fixtures.sh",
	"scripts/agents/dispatch-fixtures.sh",
	"scripts/agents/enumerate-suite-fixtures.sh",
	"scripts/agents/evidence-segment-fixtures.sh",
	"scripts/agents/fixture-assert.sh",
	"scripts/agents/fixture-bed-scenarios-fixtures.sh",
	"scripts/agents/fixture-bed-scenarios.sh",
	"scripts/agents/fixture-budget.sh",
	"scripts/agents/fixture-stop-report.sh",
	"scripts/agents/flight-recorder-fixtures.sh",
	"scripts/agents/goal-cli-fixtures.sh",
	"scripts/agents/health-fixtures.sh",
	"scripts/agents/hosts/fake.sh",
	"scripts/agents/land-fixtures.sh",
	"scripts/agents/lease-succession-fixtures.sh",
	"scripts/agents/mission-fixtures.sh",
	"scripts/agents/path-class-fixtures.sh",
	"scripts/agents/pre-commit-guard-fixtures.sh",
	"scripts/agents/record-protocol-fixtures.sh",
	"scripts/agents/return-schema-fixtures.sh",
	"scripts/agents/runtime-hook-fixtures.sh",
	"scripts/agents/second-session-fixtures.sh",
	"scripts/agents/static-reproof-fixtures.sh",
	"scripts/agents/suite-progress-fixtures.sh",
	"scripts/agents/supervision-fixtures.sh",
	"scripts/agents/supervision-go-fixtures.sh",
	"scripts/agents/supervision-hook-fixtures.sh",
	"scripts/agents/telemetry-census-fixtures.sh",
	"scripts/agents/witness-gate-fixtures.sh",
}
var isolationClassifications = map[string]bool{
	"fixture-data":           true,
	"fixture-time":           true,
	"host-readonly":          true,
	"performance-diagnostic": true,
	"process-private":        true,
	"real-clock-contract":    true,
	"terminal-protection":    true,
}

func TestIsolationInventoryClassifiesCompleteSourceSet(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	operations, scanned, err := scanIsolationOperations(root)
	if err != nil {
		t.Fatal(err)
	}
	expected, goFiles, shellFiles, err := completeIsolationSourceInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if problems := compareSourceInventory(expected, scanned); len(problems) != 0 {
		t.Fatalf("isolation source inventory is incomplete:\n%s", strings.Join(problems, "\n"))
	}
	rows := aggregateIsolationOperations(operations)
	path := filepath.Join(root, "internal", "testenv", "testdata", "isolation-inventory.tsv")
	declared, problems := readIsolationInventory(path, root)
	if os.Getenv(isolationInventoryUpdate) == "1" {
		if err := writeIsolationInventory(path, rows, declared); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("%s=1 rewrote internal/testenv/testdata/isolation-inventory.tsv; update mode always fails", isolationInventoryUpdate)
	}
	problems = append(problems, compareIsolationInventory(rows, declared)...)
	if len(problems) != 0 {
		sort.Strings(problems)
		t.Fatalf("isolation inventory refused the source tree:\n%s", strings.Join(problems, "\n"))
	}
	pending := 0
	for _, row := range declared {
		if strings.HasPrefix(row.classification, "pending-") {
			pending++
		}
	}
	t.Logf("isolation inventory joined %d Go test/support files and %d shell fixture files; %d operations classified, %d pending integration", goFiles, shellFiles, len(rows), pending)
}

func TestIsolationInventoryRejectsNewCollisionAndHostTimeVerdict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAuditFixture(t, root, "cmd/fixture/collision_test.go", `package fixture
import (
		"net"
		"os"
	    "os/exec"
	    "path/filepath"
	    "testing"
	    wall "time"
	    write "github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)
type machine struct { Now func() wall.Time; Sleep func(wall.Duration) }
func rewriteAfterUse(path string) {
	_ = os.WriteFile(path, nil, 0600)
	path = "/tmp/assigned-after-use"
	_ = path
}
func TestCollision(t *testing.T) {
	shared := "/tmp/shared-isolation-collision"
	dynamic := os.Getenv("SHARED_COLLISION_PATH")
	local := filepath.Join(t.TempDir(), "owned")
	ownedConcat := t.TempDir() + "/owned"
	m := machine{Now: wall.Now, Sleep: wall.Sleep}
	if m.Now().IsZero() { t.Fatal("host clock changed the verdict") }
	_ = os.WriteFile(shared, nil, 0600)
	_ = os.WriteFile(os.Args[1], nil, 0600)
	_ = os.WriteFile(filepath.Join(t.TempDir(), os.Args[1]), nil, 0600)
	_ = os.WriteFile(filepath.Join(t.TempDir(), "../escape"), nil, 0600)
	_ = os.WriteFile(t.TempDir() + os.Args[1], nil, 0600)
	_ = os.WriteFile(t.TempDir() + "/../escape", nil, 0600)
	_ = write.WriteFile("/tmp/shared-collision", nil, 0600)
	_ = exec.Command(os.Args[0])
	_ = os.WriteFile(dynamic, nil, 0600)
	_ = os.WriteFile(local, nil, 0600)
	_ = os.WriteFile(ownedConcat, nil, 0600)
	_, _ = net.Listen("tcp", "127.0.0.1:7")
	_, _ = net.Listen("tcp", "0.0.0.0:43123")
	_, _ = net.Listen("tcp", "127.0.0.1:0")
	_ = exec.Command("/bin/sh", "-c", "sleep 120 & subject=$!; printf '%s\\n' \"$subject\"; wait \"$subject\"")
}
`)
	operations, _, err := scanIsolationOperations(root)
	if err != nil {
		t.Fatal(err)
	}
	rows := aggregateIsolationOperations(operations)
	problems := compareIsolationInventory(rows, map[string]isolationInventoryRow{})
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"external-fixed-file", "external-fixed-port", "host-time-read", "host-time-wait", "unknown-external-path"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("synthetic %s was not refused; problems=%s", want, joined)
		}
	}
	fixedPorts := 0
	functionNow, functionSleep := false, false
	dynamicEnvironment, assignedAfterUse, heldSubject := false, false, false
	argumentWrite, dynamicJoin, escapingJoin := false, false, false
	dynamicConcat, escapingConcat, ownedTestBinary := false, false, false
	for _, operation := range operations {
		if operation.kind == "external-fixed-port" {
			fixedPorts++
		}
		if operation.kind == "external-fixed-port" && strings.Contains(operation.evidence, "127.0.0.1:0") {
			t.Fatalf("ephemeral port was classified as fixed: %+v", operation)
		}
		if strings.Contains(operation.evidence, "path[0]=local") || strings.Contains(operation.evidence, "path[0]=ownedConcat") {
			t.Fatalf("provably private path remained in the external inventory: %+v", operation)
		}
		functionNow = functionNow || strings.Contains(operation.evidence, "function-reference wall.Now")
		functionSleep = functionSleep || strings.Contains(operation.evidence, "function-reference wall.Sleep")
		dynamicEnvironment = dynamicEnvironment || strings.Contains(operation.evidence, "path[0]=dynamic")
		assignedAfterUse = assignedAfterUse || strings.Contains(operation.evidence, "rewriteAfterUse") && strings.Contains(operation.evidence, "path[0]=path")
		heldSubject = heldSubject || strings.Contains(operation.evidence, "subject=$!")
		argumentWrite = argumentWrite || strings.Contains(operation.evidence, "os.WriteFile path[0]=os.Args[1]")
		dynamicJoin = dynamicJoin || strings.Contains(operation.evidence, "filepath.Join(t.TempDir(), os.Args[1])")
		escapingJoin = escapingJoin || strings.Contains(operation.evidence, `filepath.Join(t.TempDir(), "../escape")`)
		dynamicConcat = dynamicConcat || strings.Contains(operation.evidence, "t.TempDir() + os.Args[1]")
		escapingConcat = escapingConcat || strings.Contains(operation.evidence, `t.TempDir() + "/../escape"`)
		ownedTestBinary = ownedTestBinary || operation.kind == "external-executable" && strings.Contains(operation.evidence, "exec.Command path[0]=os.Args[0]")
	}
	if fixedPorts != 2 {
		t.Fatalf("fixed listener operations = %d, want IPv4 single-digit and zero-host listeners", fixedPorts)
	}
	if !functionNow || !functionSleep || !dynamicEnvironment || !assignedAfterUse || !heldSubject || !argumentWrite ||
		!dynamicJoin || !escapingJoin || !dynamicConcat || !escapingConcat || !ownedTestBinary {
		t.Fatalf("negative witnesses missing: function-now=%t function-sleep=%t dynamic-environment=%t assigned-after-use=%t held-subject=%t argument-write=%t dynamic-join=%t escaping-join=%t dynamic-concat=%t escaping-concat=%t owned-test-binary=%t",
			functionNow, functionSleep, dynamicEnvironment, assignedAfterUse, heldSubject, argumentWrite, dynamicJoin, escapingJoin, dynamicConcat, escapingConcat, ownedTestBinary)
	}
}

func TestIsolationInventoryBindsSupportedWriteFileConsumers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAuditFixture(t, root, "internal/testexec/testexec.go", `package testexec
import "os"
func WriteFile(path string, data []byte, mode os.FileMode) error { return os.WriteFile(path, data, mode) }
`)
	approved := scanIsolationFixtureRows(t, root)
	for key, row := range approved {
		row.classification, row.owner = "fixture-data", "testexec.WriteFile consumer boundary"
		approved[key] = row
	}
	writeAuditFixture(t, root, "internal/fixture/aliased_test.go", `package fixture
import (
	"testing"
	write "github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)
func TestAliasedWrite(t *testing.T) {
	t.Parallel()
	_ = write.WriteFile("/tmp/shared-collision", nil, 0600)
}
`)
	aliased := scanIsolationFixtureRows(t, root)
	if problems := strings.Join(compareIsolationInventory(aliased, approved), "\n"); !strings.Contains(problems, "changed unknown-external-path operation") {
		t.Fatalf("aliased testexec.WriteFile consumer retained prior approval: %s", problems)
	}
	for key, row := range aliased {
		row.classification, row.owner = "fixture-data", "testexec.WriteFile aliased consumer boundary"
		aliased[key] = row
	}
	writeAuditFixture(t, root, "internal/testexec/bare_test.go", `package testexec
func TestBareWrite(path string) { _ = WriteFile(path, nil, 0600) }
`)
	bare := scanIsolationFixtureRows(t, root)
	if problems := strings.Join(compareIsolationInventory(bare, aliased), "\n"); !strings.Contains(problems, "changed unknown-external-path operation") {
		t.Fatalf("bare package testexec WriteFile consumer retained prior approval: %s", problems)
	}
	for key, row := range bare {
		row.classification, row.owner = "fixture-data", "testexec.WriteFile bare consumer boundary"
		bare[key] = row
	}
	writeAuditFixture(t, root, "internal/testpolicy/contractmerge/history_test.go", `package contractmerge
import "testing"
func TestHistoryMergeReplaysParallelTestingContractChanges(t *testing.T) { t.Parallel() }
`)
	contractBaseline := scanIsolationFixtureRows(t, root)
	if problems := compareIsolationInventory(contractBaseline, bare); len(problems) != 0 {
		t.Fatalf("operation-free contractmerge package changed the supported wrapper approval: %v", problems)
	}
	for key, row := range contractBaseline {
		row.classification, row.owner = "fixture-data", "testexec.WriteFile pre-assignment consumer boundary"
		contractBaseline[key] = row
	}
	writeAuditFixture(t, root, "internal/testpolicy/contractmerge/function_value_test.go", `package contractmerge
import (
	"testing"
	write "github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)
func TestWriteFileFunctionValueCollision(t *testing.T) {
	t.Parallel()
	writeFile := write.WriteFile
	_ = writeFile("/tmp/shared-collision", nil, 0600)
}
`)
	assigned := scanIsolationFixtureRows(t, root)
	problems := strings.Join(compareIsolationInventory(assigned, contractBaseline), "\n")
	if !strings.Contains(problems, "changed unknown-external-path operation in internal/testexec/testexec.go") {
		t.Fatalf("contractmerge function-value testexec.WriteFile consumer retained prior approval: %s", problems)
	}
	for _, row := range assigned {
		if row.source == "internal/testpolicy/contractmerge/function_value_test.go" {
			t.Fatalf("function-value witness was mistaken for a direct path operation: %+v", row)
		}
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directories, err := testexecWriteFileConsumerDirectories(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	foundGittree := false
	for _, directory := range directories {
		if directory == "internal/gittree" {
			foundGittree = true
		}
	}
	if !foundGittree {
		t.Fatalf("internal/gittree/disjoint_merge_test.go was absent from the testexec.WriteFile consumer boundary: %v", directories)
	}
}

func TestIsolationInventoryFingerprintBindsUseContextButIgnoresLineShifts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := "internal/fixture/context_test.go"
	writeAuditFixture(t, root, path, `package fixture
import ("testing"; "time")
func TestClock(t *testing.T) { t.Log(time.Now()) }
`)
	approved := scanIsolationFixtureRows(t, root)
	for key, row := range approved {
		row.classification, row.owner = "fixture-time", "TestClock diagnostic timestamp"
		approved[key] = row
	}
	writeAuditFixture(t, root, path, `package fixture

import ("testing"; "time")

func TestClock(t *testing.T) {

	t.Log(time.Now())
}
`)
	shifted := scanIsolationFixtureRows(t, root)
	if problems := compareIsolationInventory(shifted, approved); len(problems) != 0 {
		t.Fatalf("irrelevant line shifts invalidated approval: %v", problems)
	}
	writeAuditFixture(t, root, path, `package fixture
import ("testing"; "time")
func TestClock(t *testing.T) {
	if time.Now().IsZero() { t.Fatal("clock changed the verdict") }
}
`)
	changed := scanIsolationFixtureRows(t, root)
	if problems := strings.Join(compareIsolationInventory(changed, approved), "\n"); !strings.Contains(problems, "changed host-time-read operation") {
		t.Fatalf("changed clock use retained approval: %s", problems)
	}

	helper := "internal/fixture/process_helper_test.go"
	caller := "internal/fixture/process_owner_test.go"
	writeAuditFixture(t, root, helper, `package fixture
import "os"
func isolateProcessEnvironment() { _ = os.Setenv("FIXTURE_VALUE", "owned") }
`)
	writeAuditFixture(t, root, caller, `package fixture
import ("sync"; "testing")
var processEnvironmentMu sync.Mutex
func TestProcessOwner(t *testing.T) {
	processEnvironmentMu.Lock()
	defer processEnvironmentMu.Unlock()
	isolateProcessEnvironment()
}
`)
	protected := scanIsolationFixtureRows(t, root)
	for key, row := range protected {
		if row.kind == "process-environment" {
			row.classification, row.owner = "process-private", "TestProcessOwner package mutex"
			protected[key] = row
		}
	}
	writeAuditFixture(t, root, helper, `package fixture

import "os"

func isolateProcessEnvironment() {
	_ = os.Setenv("FIXTURE_VALUE", "owned")
}
`)
	writeAuditFixture(t, root, caller, `package fixture

import ("sync"; "testing")

var processEnvironmentMu sync.Mutex

func TestProcessOwner(t *testing.T) {
	processEnvironmentMu.Lock()
	defer processEnvironmentMu.Unlock()
	isolateProcessEnvironment()
}
`)
	shifted = scanIsolationFixtureRows(t, root)
	if problems := compareIsolationInventory(shifted, protected); len(problems) != 0 {
		t.Fatalf("irrelevant owner line shifts invalidated approval: %v", problems)
	}
	writeAuditFixture(t, root, caller, `package fixture
import "testing"
func TestProcessOwner(t *testing.T) {
	t.Parallel()
	isolateProcessEnvironment()
}
`)
	unisolated := scanIsolationFixtureRows(t, root)
	if problems := strings.Join(compareIsolationInventory(unisolated, protected), "\n"); !strings.Contains(problems, "changed process-environment operation") {
		t.Fatalf("parallel caller retained shared-mutation approval: %s", problems)
	}

	writeAuditFixture(t, root, helper, `package fixture
import "github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
func writeFixtureExecutable(path string) { _ = testexec.WriteFile(path, nil, 0700) }
`)
	writeAuditFixture(t, root, "internal/testexec/testexec.go", `package testexec
import "os"
func WriteFile(path string, data []byte, mode os.FileMode) error { return os.WriteFile(path, data, mode) }
`)
	writeAuditFixture(t, root, caller, `package fixture
import ("sync"; "testing")
var executableMu sync.Mutex
func TestExecutableOwner(t *testing.T) {
	executableMu.Lock()
	defer executableMu.Unlock()
	writeFixtureExecutable(t.TempDir())
}
`)
	protected = scanIsolationFixtureRows(t, root)
	for key, row := range protected {
		if row.kind == "unknown-external-path" {
			row.classification, row.owner = "pending-fixture-namespace", "writeFixtureExecutable package consumer boundary"
			protected[key] = row
		}
	}
	writeAuditFixture(t, root, caller, `package fixture
import "testing"
func TestExecutableOwner(t *testing.T) {
	t.Parallel()
	writeFixtureExecutable("/tmp/shared-collision")
}
`)
	unisolated = scanIsolationFixtureRows(t, root)
	if problems := strings.Join(compareIsolationInventory(unisolated, protected), "\n"); !strings.Contains(problems, "changed unknown-external-path operation") {
		t.Fatalf("parallel wrapper caller retained shared-mutation approval: %s", problems)
	}
}

func TestIsolationInventoryRefreshLeavesNewVerdictUnclassified(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := "internal/fixture/refresh_test.go"
	writeAuditFixture(t, root, source, `package fixture
import ("testing"; "time")
func TestClock(t *testing.T) { t.Log(time.Now()) }
`)
	approved := scanIsolationFixtureRows(t, root)
	for key, row := range approved {
		row.classification, row.owner = "fixture-time", "TestClock diagnostic timestamp"
		approved[key] = row
	}
	writeAuditFixture(t, root, source, `package fixture
import ("testing"; "time")
func TestClock(t *testing.T) {
	t.Log(time.Now())
	if time.Now().Unix()%2 == 0 { t.Fatal("new host-time verdict") }
}
`)
	found := scanIsolationFixtureRows(t, root)
	path := filepath.Join(root, "inventory.tsv")
	if err := writeIsolationInventory(path, found, approved); err != nil {
		t.Fatal(err)
	}
	refreshed, readProblems := readIsolationInventory(path, root)
	if problems := strings.Join(readProblems, "\n"); !strings.Contains(problems, "is unclassified") {
		t.Fatalf("refresh did not leave changed operations unclassified: %s", problems)
	}
	if problems := compareIsolationInventory(found, refreshed); len(problems) != 0 {
		t.Fatalf("refresh proposal does not describe current operations: %v", problems)
	}
	for key, row := range refreshed {
		row.classification, row.owner = "pending-native-time", "TestClock new verdict review"
		refreshed[key] = row
	}
	if problems := compareIsolationInventory(found, refreshed); len(problems) != 0 {
		t.Fatalf("reviewed classifications did not satisfy the current operations: %v", problems)
	}
	if err := writeIsolationInventory(path, found, refreshed); err != nil {
		t.Fatal(err)
	}
	if _, pendingProblems := readIsolationInventory(path, root); !strings.Contains(strings.Join(pendingProblems, "\n"), "is unclassified") {
		t.Fatalf("reader accepted a pending inventory row: %v", pendingProblems)
	}
}

func scanIsolationFixtureRows(t *testing.T, root string) map[string]isolationInventoryRow {
	t.Helper()
	operations, _, err := scanIsolationOperations(root)
	if err != nil {
		t.Fatal(err)
	}
	return aggregateIsolationOperations(operations)
}

func completeIsolationSourceInventory(root string) (map[string]bool, int, int, error) {
	complete := make(map[string]bool)
	packages, _, err := loadPackageTests(root)
	if err != nil {
		return nil, 0, 0, err
	}
	goFiles := 0
	for directory, group := range packages {
		for _, file := range group.files {
			complete[filepath.ToSlash(filepath.Join(directory, file.name))] = true
			goFiles++
		}
	}
	for _, source := range isolationGoSupportManifest {
		if err := requireIsolationSupportSource(root, source); err != nil {
			return nil, 0, 0, err
		}
		if !complete[source] {
			complete[source] = true
			goFiles++
		}
	}
	for _, source := range isolationShellSupportManifest {
		if err := requireIsolationSupportSource(root, source); err != nil {
			return nil, 0, 0, err
		}
		complete[source] = true
	}
	return complete, goFiles, len(isolationShellSupportManifest), nil
}

func requireIsolationSupportSource(root, source string) error {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(source)))
	if err != nil {
		return fmt.Errorf("isolation support manifest %s: %w", source, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("isolation support manifest %s is not a regular file", source)
	}
	return nil
}

func discoveredShellFixtureSources(root string) ([]string, error) {
	var sources []string
	scripts := filepath.Join(root, "scripts", "agents")
	if _, err := os.Stat(scripts); os.IsNotExist(err) {
		return sources, nil
	} else if err != nil {
		return nil, err
	}
	err := filepath.WalkDir(scripts, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sh") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if isolationShellFixtureCandidate(relative) {
			sources = append(sources, relative)
		}
		return nil
	})
	sort.Strings(sources)
	return sources, err
}

func isolationShellFixtureCandidate(relative string) bool {
	base := filepath.Base(relative)
	return strings.Contains(base, "fixture") || relative == "scripts/agents/adapters/fake.sh" || relative == "scripts/agents/hosts/fake.sh"
}

func scanIsolationOperations(root string) ([]isolationOperation, map[string]bool, error) {
	var operations []isolationOperation
	scanned := make(map[string]bool)
	support := make(map[string]bool, len(isolationGoSupportManifest))
	for _, source := range isolationGoSupportManifest {
		support[source] = true
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			if scannerIgnoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			} else if !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		relative := relativeToRoot(root, path)
		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") ||
			(!strings.HasSuffix(entry.Name(), "_test.go") && !support[relative]) {
			return nil
		}
		scanned[relative] = true
		found, err := scanGoIsolationOperations(relative, path)
		if err != nil {
			return err
		}
		operations = append(operations, found...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err := bindSharedMutationOwnerContext(root, operations); err != nil {
		return nil, nil, err
	}
	shell, err := discoveredShellFixtureSources(root)
	if err != nil {
		return nil, nil, err
	}
	for _, relative := range shell {
		scanned[relative] = true
		found, err := scanShellIsolationOperations(relative, filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, nil, err
		}
		operations = append(operations, found...)
	}
	return operations, scanned, nil
}

// bindSharedMutationOwnerContext makes an approval for a helper-owned process
// mutation depend on the normalized Go package that calls it. This is a
// conservative invalidation boundary, not a claim that static syntax proves
// arbitrary dynamic calls: any semantic package change requires review again.
func bindSharedMutationOwnerContext(root string, operations []isolationOperation) error {
	contexts := make(map[string]string)
	var testexecContext string
	for index := range operations {
		operation := &operations[index]
		if operation.source == "internal/testexec/testexec.go" && operation.function == "WriteFile" &&
			operation.kind == "unknown-external-path" && strings.Contains(operation.evidence, "os.WriteFile") {
			if testexecContext == "" {
				var err error
				testexecContext, err = normalizedTestexecWriteFileConsumerContext(root)
				if err != nil {
					return err
				}
			}
			operation.semantic = isolationFingerprint(operation.source, operation.kind, operation.semantic+"\x00consumer-context\x00"+testexecContext)
			continue
		}
		if !sharedMutationNeedsOwnerContext(*operation) {
			continue
		}
		directory := filepath.Dir(operation.source)
		context, ok := contexts[directory]
		if !ok {
			var err error
			context, err = normalizedGoPackageContext(root, directory)
			if err != nil {
				return err
			}
			contexts[directory] = context
		}
		operation.semantic = isolationFingerprint(operation.source, operation.kind, operation.semantic+"\x00owner-context\x00"+context)
	}
	return nil
}

func sharedMutationNeedsOwnerContext(operation isolationOperation) bool {
	if strings.HasPrefix(operation.function, "Test") || strings.HasPrefix(operation.function, "Benchmark") ||
		strings.HasPrefix(operation.function, "Fuzz") || strings.HasPrefix(operation.function, "Example") {
		return false
	}
	switch operation.kind {
	case "external-fixed-file", "external-fixed-port", "process-environment", "process-signal", "process-stream", "process-working-directory":
		return true
	default:
		return false
	}
}

func normalizedTestexecWriteFileConsumerContext(root string) (string, error) {
	directories, err := testexecWriteFileConsumerDirectories(root)
	if err != nil {
		return "", err
	}
	var context strings.Builder
	for _, directory := range directories {
		packageContext, err := normalizedGoPackageContext(root, directory)
		if err != nil {
			return "", err
		}
		context.WriteString(directory)
		context.WriteByte(0)
		context.WriteString(packageContext)
		context.WriteByte(0)
	}
	return context.String(), nil
}

func testexecWriteFileConsumerDirectories(root string) ([]string, error) {
	directories := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && scannerIgnoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			if path != root {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		}
		relative := relativeToRoot(root, path)
		if !strings.HasSuffix(entry.Name(), "_test.go") && relative != "internal/testexec/testexec.go" {
			return nil
		}
		files := token.NewFileSet()
		parsed, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		aliases := make(map[string]string)
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			aliases[name] = importPath
		}
		found := false
		ast.Inspect(parsed, func(candidate ast.Node) bool {
			if expression, ok := candidate.(ast.Expr); ok && isolationCallNameInPackage(expression, aliases, parsed.Name.Name) == "testexec.WriteFile" {
				found = true
				return false
			}
			return !found
		})
		if found && relative != "internal/testexec/testexec.go" {
			directories[filepath.ToSlash(filepath.Dir(relative))] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ordered := make([]string, 0, len(directories))
	for directory := range directories {
		ordered = append(ordered, directory)
	}
	sort.Strings(ordered)
	return ordered, nil
}

func normalizedGoPackageContext(root, relativeDirectory string) (string, error) {
	directory := filepath.Join(root, filepath.FromSlash(relativeDirectory))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}
	var context strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		files := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return "", fmt.Errorf("parse owner context %s: %w", path, parseErr)
		}
		context.WriteString(entry.Name())
		context.WriteByte(0)
		context.WriteString(canonicalGoNode(files, parsed))
		context.WriteByte(0)
	}
	return context.String(), nil
}

func scannerIgnoredDirectory(name string) bool {
	return name == "artifacts" || name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func relativeToRoot(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative)
}

func scanGoIsolationOperations(relative, path string) ([]isolationOperation, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	aliases := make(map[string]string)
	for _, spec := range parsed.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[name] = importPath
	}
	var operations []isolationOperation
	ordinals := make(map[string]int)
	for _, declaration := range parsed.Decls {
		function := "package"
		if declared, ok := declaration.(*ast.FuncDecl); ok {
			function = declared.Name.Name
		}
		semantic := canonicalGoNode(files, declaration)
		operations = append(operations, scanGoIsolationNode(files, relative, parsed.Name.Name, function, semantic, declaration, aliases, ordinals)...)
	}
	return operations, nil
}

func scanGoIsolationNode(files *token.FileSet, source, packageName, function, semantic string, node ast.Node, aliases map[string]string, ordinals map[string]int) []isolationOperation {
	bindings := isolationExpressionBindings(node, aliases)
	directClock := make(map[*ast.SelectorExpr]bool)
	ast.Inspect(node, func(candidate ast.Node) bool {
		if call, ok := candidate.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				directClock[selector] = true
			}
		}
		return true
	})
	var operations []isolationOperation
	add := func(kind, operation string, anchor ast.Node) {
		operations = append(operations, newGoIsolationOperation(files, source, function, semantic, kind, operation, anchor, ordinals))
	}
	ast.Inspect(node, func(candidate ast.Node) bool {
		switch typed := candidate.(type) {
		case *ast.CallExpr:
			name := isolationCallNameInPackage(typed.Fun, aliases, packageName)
			if kind := isolationDirectCallKind(name); kind != "" {
				add(kind, "call "+canonicalGoNode(files, typed), typed)
			}
			if kind, operation := listenerPortOperation(name, typed, files, bindings, aliases); kind != "" {
				add(kind, operation, typed)
			}
			for _, pathOperation := range externalPathOperations(name, typed, files, bindings, aliases) {
				add(pathOperation.kind, pathOperation.operation, typed)
			}
			if kind, operation := embeddedCommandTimeOperation(name, typed, files, bindings, aliases); kind != "" {
				add(kind, operation, typed)
			}
		case *ast.SelectorExpr:
			if directClock[typed] {
				return true
			}
			if kind := clockOperationKind(isolationCallName(typed, aliases)); kind != "" {
				add(kind, "function-reference "+canonicalGoNode(files, typed), typed)
			}
		case *ast.AssignStmt:
			for _, target := range typed.Lhs {
				name := isolationCallName(target, aliases)
				if name == "os.Stdin" || name == "os.Stdout" || name == "os.Stderr" {
					add("process-stream", "assignment "+canonicalGoNode(files, typed), typed)
					break
				}
			}
		}
		return true
	})
	return operations
}

func isolationCallName(expression ast.Expr, aliases map[string]string) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	path := aliases[identifier.Name]
	if path == "" {
		return identifier.Name + "." + selector.Sel.Name
	}
	return filepath.Base(path) + "." + selector.Sel.Name
}

func isolationCallNameInPackage(expression ast.Expr, aliases map[string]string, packageName string) string {
	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "WriteFile" &&
		(packageName == "testexec" || filepath.Base(aliases["."]) == "testexec") {
		return "testexec.WriteFile"
	}
	return isolationCallName(expression, aliases)
}

func isolationDirectCallKind(name string) string {
	if kind := clockOperationKind(name); kind != "" {
		return kind
	}
	switch name {
	case "os.Setenv", "os.Unsetenv":
		return "process-environment"
	case "os.Chdir":
		return "process-working-directory"
	case "signal.Ignore", "signal.Notify", "signal.NotifyContext", "signal.Reset", "signal.Stop":
		return "process-signal"
	}
	if strings.HasSuffix(name, ".Setenv") {
		return "process-environment"
	}
	return ""
}

func clockOperationKind(name string) string {
	switch name {
	case "time.Now", "time.Since", "time.Until", "identity.BootClock":
		return "host-time-read"
	case "time.Sleep", "time.After", "time.AfterFunc", "time.NewTicker", "time.NewTimer", "time.Tick", "context.WithDeadline", "context.WithTimeout":
		return "host-time-wait"
	default:
		return ""
	}
}

func listenerPortOperation(name string, call *ast.CallExpr, files *token.FileSet, bindings map[string][]ast.Expr, aliases map[string]string) (string, string) {
	index := -1
	switch name {
	case "net.Listen", "net.ListenPacket":
		index = 1
	case "http.ListenAndServe", "http.ListenAndServeTLS":
		index = 0
	}
	if index < 0 || index >= len(call.Args) {
		return "", ""
	}
	argument := call.Args[index]
	value, known := isolationConstantString(argument, bindings, aliases, nil)
	operation := name + " address=" + canonicalGoNode(files, argument)
	if !known {
		return "unknown-external-port", operation
	}
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return "unknown-external-port", operation
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return "unknown-external-port", operation
	}
	if number == 0 {
		return "", ""
	}
	return "external-fixed-port", operation
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

type isolationPathOperation struct {
	kind, operation string
}

func externalPathOperations(name string, call *ast.CallExpr, files *token.FileSet, bindings map[string][]ast.Expr, aliases map[string]string) []isolationPathOperation {
	indexes, executable := isolationPathArgumentIndexes(name)
	var operations []isolationPathOperation
	for _, index := range indexes {
		if index >= len(call.Args) {
			continue
		}
		argument := call.Args[index]
		operation := fmt.Sprintf("%s path[%d]=%s", name, index, canonicalGoNode(files, argument))
		if executable && isolationOwnedTestBinary(argument) {
			operations = append(operations, isolationPathOperation{kind: "external-executable", operation: operation})
			continue
		}
		if value, known := isolationConstantString(argument, bindings, aliases, nil); known {
			if executable {
				operations = append(operations, isolationPathOperation{kind: "external-executable", operation: operation})
			} else if filepath.IsAbs(value) {
				operations = append(operations, isolationPathOperation{kind: "external-fixed-file", operation: operation})
			}
			continue
		}
		if isolationPrivatePath(argument, bindings, aliases, nil) {
			continue
		} else {
			operations = append(operations, isolationPathOperation{kind: "unknown-external-path", operation: operation})
		}
	}
	return operations
}

func isolationPathArgumentIndexes(name string) ([]int, bool) {
	switch name {
	case "os.Open", "os.OpenFile", "os.ReadFile", "os.ReadDir", "os.WriteFile", "os.Create", "os.Mkdir", "os.MkdirAll", "os.Remove", "os.RemoveAll", "os.Stat", "os.Lstat", "os.Chtimes":
		return []int{0}, false
	case "os.Rename", "os.Symlink":
		return []int{0, 1}, false
	case "exec.Command":
		return []int{0}, true
	case "exec.CommandContext":
		return []int{1}, true
	default:
		return nil, false
	}
}

func isolationOwnedTestBinary(expression ast.Expr) bool {
	indexed, ok := expression.(*ast.IndexExpr)
	if !ok || isolationCallName(indexed.X, nil) != "os.Args" {
		return false
	}
	index, ok := indexed.Index.(*ast.BasicLit)
	return ok && index.Kind == token.INT && index.Value == "0"
}

func embeddedCommandTimeOperation(name string, call *ast.CallExpr, files *token.FileSet, bindings map[string][]ast.Expr, aliases map[string]string) (string, string) {
	programIndex := 0
	if name == "exec.CommandContext" {
		programIndex = 1
	} else if name != "exec.Command" {
		return "", ""
	}
	if programIndex >= len(call.Args) {
		return "", ""
	}
	program, known := isolationConstantString(call.Args[programIndex], bindings, aliases, nil)
	if !known {
		return "", ""
	}
	base := filepath.Base(program)
	if base == "sleep" || base == "timeout" || base == "date" {
		return "host-time-wait", "command-time " + canonicalGoNode(files, call)
	}
	if base != "sh" && base != "bash" {
		return "", ""
	}
	for index := programIndex + 1; index+1 < len(call.Args); index++ {
		option, ok := isolationConstantString(call.Args[index], bindings, aliases, nil)
		if !ok || option != "-c" {
			continue
		}
		script, ok := isolationConstantString(call.Args[index+1], bindings, aliases, nil)
		if !ok {
			return "unknown-embedded-command", "embedded-shell " + canonicalGoNode(files, call.Args[index+1])
		}
		if shellTimeOperation.MatchString(script) {
			return "host-time-wait", "embedded-shell " + strings.Join(strings.Fields(script), " ")
		}
		return "", ""
	}
	return "", ""
}

func isolationExpressionBindings(node ast.Node, aliases map[string]string) map[string][]ast.Expr {
	bindings := make(map[string][]ast.Expr)
	bind := func(expression ast.Expr, value ast.Expr) {
		if identifier, ok := expression.(*ast.Ident); ok && identifier.Name != "_" {
			bindings[identifier.Name] = append(bindings[identifier.Name], value)
		}
	}
	ast.Inspect(node, func(candidate ast.Node) bool {
		switch typed := candidate.(type) {
		case *ast.AssignStmt:
			if len(typed.Lhs) == len(typed.Rhs) {
				for index := range typed.Lhs {
					bind(typed.Lhs[index], typed.Rhs[index])
				}
			} else if len(typed.Rhs) == 1 && len(typed.Lhs) > 0 {
				if call, ok := typed.Rhs[0].(*ast.CallExpr); ok && isolationCallName(call.Fun, aliases) == "os.MkdirTemp" {
					bind(typed.Lhs[0], typed.Rhs[0])
				}
			}
		case *ast.ValueSpec:
			if len(typed.Names) == len(typed.Values) {
				for index := range typed.Names {
					bind(typed.Names[index], typed.Values[index])
				}
			}
		}
		return true
	})
	return bindings
}

func isolationConstantString(expression ast.Expr, bindings map[string][]ast.Expr, aliases map[string]string, visiting map[string]bool) (string, bool) {
	if value, ok := stringLiteral(expression); ok {
		return value, true
	}
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	switch typed := expression.(type) {
	case *ast.Ident:
		values := bindings[typed.Name]
		if len(values) != 1 || values[0].Pos() >= typed.Pos() || visiting[typed.Name] {
			return "", false
		}
		visiting[typed.Name] = true
		value, ok := isolationConstantString(values[0], bindings, aliases, visiting)
		delete(visiting, typed.Name)
		return value, ok
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := isolationConstantString(typed.X, bindings, aliases, visiting)
		right, rightOK := isolationConstantString(typed.Y, bindings, aliases, visiting)
		return left + right, leftOK && rightOK
	case *ast.CallExpr:
		if isolationCallName(typed.Fun, aliases) != "filepath.Join" {
			return "", false
		}
		parts := make([]string, len(typed.Args))
		for index, argument := range typed.Args {
			value, ok := isolationConstantString(argument, bindings, aliases, visiting)
			if !ok {
				return "", false
			}
			parts[index] = value
		}
		return filepath.Join(parts...), true
	default:
		return "", false
	}
}

func isolationPrivatePath(expression ast.Expr, bindings map[string][]ast.Expr, aliases map[string]string, visiting map[string]bool) bool {
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	switch typed := expression.(type) {
	case *ast.Ident:
		values := bindings[typed.Name]
		if len(values) != 1 || values[0].Pos() >= typed.Pos() || visiting[typed.Name] {
			return false
		}
		visiting[typed.Name] = true
		private := isolationPrivatePath(values[0], bindings, aliases, visiting)
		delete(visiting, typed.Name)
		return private
	case *ast.CallExpr:
		name := isolationCallName(typed.Fun, aliases)
		if name == "t.TempDir" || name == "os.TempDir" || name == "os.MkdirTemp" {
			return true
		}
		if name == "filepath.Join" && len(typed.Args) > 0 {
			if !isolationPrivatePath(typed.Args[0], bindings, aliases, visiting) {
				return false
			}
			parts := make([]string, 0, len(typed.Args)-1)
			for _, argument := range typed.Args[1:] {
				part, known := isolationConstantString(argument, bindings, aliases, nil)
				if !known || filepath.IsAbs(part) {
					return false
				}
				parts = append(parts, part)
			}
			return isolationContainedRelativePath(filepath.Join(parts...))
		}
	case *ast.BinaryExpr:
		if typed.Op != token.ADD || !isolationPrivatePath(typed.X, bindings, aliases, visiting) {
			return false
		}
		suffix, known := isolationConstantString(typed.Y, bindings, aliases, nil)
		if !known || suffix == "" {
			return known
		}
		if !strings.HasPrefix(suffix, string(filepath.Separator)) {
			return false
		}
		return isolationContainedRelativePath(strings.TrimLeft(suffix, string(filepath.Separator)))
	}
	return false
}

func isolationContainedRelativePath(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	cleaned := filepath.Clean(path)
	return cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func canonicalGoNode(files *token.FileSet, node ast.Node) string {
	var rendered bytes.Buffer
	_ = format.Node(&rendered, files, node)
	return strings.Join(strings.Fields(rendered.String()), " ")
}

func newGoIsolationOperation(files *token.FileSet, source, function, semantic, kind, operation string, node ast.Node, ordinals map[string]int) isolationOperation {
	base := isolationFingerprint(source, kind, function+"\x00"+operation)
	ordinals[base]++
	fingerprint := base
	if ordinals[base] > 1 {
		fingerprint += "-" + strconv.Itoa(ordinals[base])
	}
	line := files.Position(node.Pos()).Line
	return isolationOperation{source: source, kind: kind, fingerprint: fingerprint,
		function: function, semantic: isolationFingerprint(source, kind, function+"\x00"+semantic), line: line,
		evidence: fmt.Sprintf("%s@%d %s", function, line, operation)}
}

func scanShellIsolationOperations(relative, path string) ([]isolationOperation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var operations []isolationOperation
	ordinals := make(map[string]int)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		canonical := strings.Join(strings.Fields(text), " ")
		for _, candidate := range []struct {
			kind string
			find func(string) bool
		}{
			{"host-time-wait", shellTimeOperation.MatchString},
			{"external-fixed-port", fixedShellPort.MatchString},
			{"external-fixed-file", fixedShellPath.MatchString},
		} {
			if candidate.find(text) {
				base := isolationFingerprint(relative, candidate.kind, canonical)
				ordinals[base]++
				fingerprint := base
				if ordinals[base] > 1 {
					fingerprint += "-" + strconv.Itoa(ordinals[base])
				}
				operations = append(operations, isolationOperation{source: relative, kind: candidate.kind, line: line,
					fingerprint: fingerprint, semantic: isolationFingerprint(relative, candidate.kind, canonical),
					evidence: fmt.Sprintf("shell@%d %s", line, canonical)})
			}
		}
	}
	return operations, scanner.Err()
}

func isolationFingerprint(source, kind, operation string) string {
	digest := sha256.Sum256([]byte(source + "\x00" + kind + "\x00" + operation))
	return hex.EncodeToString(digest[:8])
}

func aggregateIsolationOperations(operations []isolationOperation) map[string]isolationInventoryRow {
	rows := make(map[string]isolationInventoryRow, len(operations))
	for _, operation := range operations {
		row := isolationInventoryRow{source: operation.source, kind: operation.kind, fingerprint: operation.fingerprint,
			semantic: operation.semantic, evidence: operation.evidence}
		rows[row.key()] = row
	}
	return rows
}

func compareSourceInventory(expected, scanned map[string]bool) []string {
	var problems []string
	for source := range expected {
		if !scanned[source] {
			problems = append(problems, "source was not scanned: "+source)
		}
	}
	for source := range scanned {
		if !expected[source] {
			problems = append(problems, "scanner read a source absent from the complete inventory: "+source)
		}
	}
	sort.Strings(problems)
	return problems
}

func readIsolationInventory(path, root string) (map[string]isolationInventoryRow, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer file.Close()
	rows := make(map[string]isolationInventoryRow)
	var problems []string
	previous := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 7 {
			problems = append(problems, fmt.Sprintf("%s:%d: expected seven tab-separated fields", path, line))
			continue
		}
		row := isolationInventoryRow{source: fields[0], kind: fields[1], fingerprint: fields[2], semantic: fields[3],
			classification: fields[4], owner: fields[5], evidence: fields[6]}
		key := row.key()
		if row.fingerprint == "" || row.semantic == "" || strings.TrimSpace(row.classification) == "" || strings.TrimSpace(row.owner) == "" || strings.TrimSpace(row.evidence) == "" {
			problems = append(problems, fmt.Sprintf("%s:%d: invalid or empty inventory field", path, line))
		}
		if !isolationClassifications[row.classification] {
			problems = append(problems, fmt.Sprintf("%s:%d: %s %s is unclassified", path, line, row.source, row.kind))
		}
		if strings.TrimSpace(row.owner) == "UNOWNED" {
			problems = append(problems, fmt.Sprintf("%s:%d: %s %s has no owner", path, line, row.source, row.kind))
		}
		if key <= previous {
			problems = append(problems, fmt.Sprintf("%s:%d: rows are not sorted: %s follows %s", path, line, key, previous))
		}
		previous = key
		if _, duplicate := rows[key]; duplicate {
			problems = append(problems, fmt.Sprintf("%s:%d: duplicate row %s", path, line, key))
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(row.source))); statErr != nil || !info.Mode().IsRegular() {
			problems = append(problems, fmt.Sprintf("%s:%d: source is not a regular file: %s (%v)", path, line, row.source, statErr))
		}
		rows[key] = row
	}
	if err := scanner.Err(); err != nil {
		problems = append(problems, err.Error())
	}
	return rows, problems
}

func compareIsolationInventory(found, declared map[string]isolationInventoryRow) []string {
	var problems []string
	for key, actual := range found {
		expected, ok := declared[key]
		if !ok {
			problems = append(problems, fmt.Sprintf("unclassified %s operation in %s: fingerprint=%s evidence=%s", actual.kind, actual.source, actual.fingerprint, actual.evidence))
			continue
		}
		if expected.semantic != actual.semantic {
			problems = append(problems, fmt.Sprintf("changed %s operation in %s: fingerprint=%s declared semantic=%s found semantic=%s evidence=%s", actual.kind, actual.source, actual.fingerprint, expected.semantic, actual.semantic, actual.evidence))
		}
	}
	for key, expected := range declared {
		if _, ok := found[key]; !ok {
			problems = append(problems, fmt.Sprintf("stale %s operation in %s: fingerprint=%s", expected.kind, expected.source, expected.fingerprint))
		}
	}
	return problems
}

func writeIsolationInventory(path string, found, existing map[string]isolationInventoryRow) error {
	keys := make([]string, 0, len(found))
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var contents strings.Builder
	contents.WriteString("# source\toperation\tfingerprint\tsemantic-fingerprint\tclassification\towner\tevidence\n")
	for _, key := range keys {
		row := found[key]
		classification, owner := "UNCLASSIFIED", "UNOWNED"
		if prior, ok := existing[key]; ok && prior.semantic == row.semantic {
			classification, owner = prior.classification, prior.owner
		}
		fmt.Fprintf(&contents, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.source, row.kind, row.fingerprint, row.semantic, classification, owner, row.evidence)
	}
	return os.WriteFile(path, []byte(contents.String()), 0o644)
}
