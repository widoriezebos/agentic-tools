package main

// Public homes for the machinery verbs people and agents are told to run by
// hand (plans/designs/verbs-object-action.md, sections 3.1, 3.4 and 3.6;
// U9a): test add, test merge, test baseline and settings set (system register was folded into system setup in U9b).
// Each routes through the public router to the owner the retired internal
// verb reached, and each has an idempotency row with its witness here.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func init() {
	registerIdempotency("test add", idemStateful, "tests already in the group: success, the contract's bytes unchanged", witnessTestAddRepeat)
	registerIdempotency("test remove", idemStateful, "tests already absent from the group: success, the contract's bytes unchanged", witnessTestRemoveRepeat)
	registerIdempotency("test merge", idemStateful, "the same three contracts merge to the same bytes; a repeat rewrites them unchanged", witnessTestMergeRepeat)
	registerIdempotency("test baseline", idemCreation, "--gate records that the gate passed at this moment, so the baseline's age restarts from each call; --check only reads", nil)
	registerIdempotency("settings set", idemStateful, "a key already holding the value: success, the local configuration unchanged", witnessSettingsSetRepeat)
}

// newHomesAddTestsFixture is a contract with one group over a package whose
// source declares TestBase and TestAdded.
func newHomesAddTestsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	packageDir := filepath.Join(root, "internal", "example")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "example_test.go"), []byte("package example\nfunc TestBase(t any) {}\nfunc TestAdded(t any) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contract := testingMergeFixture()
	contract.Groups[0].Packages = []string{"internal/example"}
	path := filepath.Join(root, "testing.json")
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestTestAddIsThePublicHomeOfAddTests: docs tell people to register tests
// with the contract's add-tests; the public test add does it.
func TestTestAddIsThePublicHomeOfAddTests(t *testing.T) {
	path := newHomesAddTestsFixture(t)
	if code := dispatch([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}); code != 0 {
		t.Fatalf("test add exit = %d", code)
	}
	loaded, err := testpolicy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, names, _ := testpolicy.GoTests(loaded.Groups[0]); !reflect.DeepEqual(names, []string{"TestAdded", "TestBase"}) {
		t.Fatalf("test add wrote tests %v", names)
	}
}

func witnessTestAddRepeat(t *testing.T) {
	path := newHomesAddTestsFixture(t)
	args := []string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}
	if code := dispatch(args); code != 0 {
		t.Fatalf("first test add exit = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := dispatch(args); code != 0 {
		t.Fatalf("repeated test add exit = %d", code)
	}
	if second, err := os.ReadFile(path); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated test add changed the contract: err=%v", err)
	}
}

// newHomesMergeFixture writes a base contract and two edits of it that each
// add one test to the first group, and returns the four paths the merge
// takes.
func newHomesMergeFixture(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	base := testingMergeFixture()
	ours, theirs := testingMergeClone(t, base), testingMergeClone(t, base)
	ours.Groups[0].Tests = json.RawMessage(`["TestBase","TestOurs"]`)
	theirs.Groups[0].Tests = json.RawMessage(`["TestBase","TestTheirs"]`)
	paths := []string{filepath.Join(root, "base.json"), filepath.Join(root, "ours.json"), filepath.Join(root, "theirs.json"), filepath.Join(root, "out.json")}
	for i, contract := range []testpolicy.Contract{base, ours, theirs} {
		data, err := contractmerge.Render(contract)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[i], data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

// TestTestMergeIsThePublicHomeOfTheContractMerge: the documented manual merge
// of concurrent contract edits is test merge.
func TestTestMergeIsThePublicHomeOfTheContractMerge(t *testing.T) {
	paths := newHomesMergeFixture(t)
	if code := dispatch([]string{"test", "merge", "--base", paths[0], "--ours", paths[1], "--theirs", paths[2], "--out", paths[3]}); code != 0 {
		t.Fatalf("test merge exit = %d", code)
	}
	merged, err := testpolicy.Load(paths[3])
	if err != nil {
		t.Fatal(err)
	}
	if _, names, _ := testpolicy.GoTests(merged.Groups[0]); !reflect.DeepEqual(names, []string{"TestBase", "TestOurs", "TestTheirs"}) {
		t.Fatalf("test merge wrote tests %v", names)
	}
}

func witnessTestMergeRepeat(t *testing.T) {
	paths := newHomesMergeFixture(t)
	args := []string{"test", "merge", "--base", paths[0], "--ours", paths[1], "--theirs", paths[2], "--out", paths[3]}
	if code := dispatch(args); code != 0 {
		t.Fatalf("first test merge exit = %d", code)
	}
	first, err := os.ReadFile(paths[3])
	if err != nil {
		t.Fatal(err)
	}
	if code := dispatch(args); code != 0 {
		t.Fatalf("repeated test merge exit = %d", code)
	}
	if second, err := os.ReadFile(paths[3]); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated test merge wrote different bytes: err=%v", err)
	}
}

// TestTestBaselineRoutesRecordAndCheck: the refactor skill's record and
// check of the trusted baseline route to the baseline owner; a record
// without a gate and a check with a gate are the owner's usage refusals.
func TestTestBaselineRoutesRecordAndCheck(t *testing.T) {
	if _, ok := findIntentAction("test", "baseline"); !ok {
		t.Fatal("test baseline is not a public action")
	}
	for _, args := range [][]string{{"--gate", "go test ./..."}, {"--check"}} {
		words, err := testBaselineArgs(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		want := "record"
		if args[0] == "--check" {
			want = "check"
		}
		if words[0] != want {
			t.Fatalf("%v routed to %q, want %q", args, words[0], want)
		}
	}
	if _, err := testBaselineArgs([]string{"--check", "--gate", "x"}); err == nil {
		t.Fatal("--check with --gate was accepted")
	}
	if _, err := testBaselineArgs(nil); err == nil {
		t.Fatal("a baseline with neither --gate nor --check was accepted")
	}
}

// newHomesSettingsInstallation is an installation with a shipped
// metasystem.conf and no local configuration.
func newHomesSettingsInstallation(t *testing.T) (string, intentOwners) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable)}
}

func runSettingsSet(t *testing.T, root string, owners intentOwners, args ...string) (int, intentResult) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "settings set"), append(args, "--json"), &stdout, &stderr, root, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

// TestSettingsSetWritesTheLocalConfiguration: the placeholder-model refusals
// tell a person to set a key in this seat's local configuration; settings set
// writes it there, creating the file when it is absent, and leaves the
// shipped configuration alone.
func TestSettingsSetWritesTheLocalConfiguration(t *testing.T) {
	t.Parallel()
	root, owners := newHomesSettingsInstallation(t)
	code, result := runSettingsSet(t, root, owners, "role.default.model.claude", "claude-opus-5-5")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("settings set = %d %+v", code, result)
	}
	local, err := os.ReadFile(filepath.Join(root, "metasystem.conf.local"))
	if err != nil || !strings.Contains(string(local), "role.default.model.claude=claude-opus-5-5\n") {
		t.Fatalf("local configuration = %q err=%v", local, err)
	}
	if shipped, _ := os.ReadFile(filepath.Join(root, "metasystem.conf")); string(shipped) != "metasystem.runtimes=claude\n" {
		t.Fatalf("settings set changed the shipped configuration: %q", shipped)
	}
	if code, result := runSettingsSet(t, root, owners, "role.default.model.claude"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("settings set without a value = %d %+v", code, result)
	}
}

func witnessSettingsSetRepeat(t *testing.T) {
	root, owners := newHomesSettingsInstallation(t)
	if code, result := runSettingsSet(t, root, owners, "launch.read.model", "fixture-model"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first settings set = %d %+v", code, result)
	}
	first, err := os.ReadFile(filepath.Join(root, "metasystem.conf.local"))
	if err != nil {
		t.Fatal(err)
	}
	if code, result := runSettingsSet(t, root, owners, "launch.read.model", "fixture-model"); code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("repeated settings set = %d %+v", code, result)
	}
	if second, err := os.ReadFile(filepath.Join(root, "metasystem.conf.local")); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated settings set rewrote the local configuration: err=%v", err)
	}
}

// TestTestRemoveTakesTestsOutOfAGroup: deleting a test leaves its name in the
// contract until test remove takes it out; the name need not exist in the
// packages any more, and a name the group does not list is refused.
func TestTestRemoveTakesTestsOutOfAGroup(t *testing.T) {
	path := newHomesAddTestsFixture(t)
	if code := dispatch([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}); code != 0 {
		t.Fatalf("test add exit = %d", code)
	}
	if code := dispatch([]string{"test", "remove", "--file", path, "--group", "app-group", "--tests", "TestAdded"}); code != 0 {
		t.Fatalf("test remove exit = %d", code)
	}
	loaded, err := testpolicy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, names, _ := testpolicy.GoTests(loaded.Groups[0]); !reflect.DeepEqual(names, []string{"TestBase"}) {
		t.Fatalf("test remove left tests %v", names)
	}
	if code := dispatch([]string{"test", "remove", "--file", path, "--group", "no-such-group", "--tests", "TestBase"}); code == 0 {
		t.Fatal("test remove of an unknown group succeeded")
	}
}

func witnessTestRemoveRepeat(t *testing.T) {
	path := newHomesAddTestsFixture(t)
	if code := dispatch([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}); code != 0 {
		t.Fatalf("test add exit = %d", code)
	}
	args := []string{"test", "remove", "--file", path, "--group", "app-group", "--tests", "TestAdded"}
	if code := dispatch(args); code != 0 {
		t.Fatalf("first test remove exit = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := dispatch(args); code != 0 {
		t.Fatalf("repeated test remove exit = %d", code)
	}
	if second, err := os.ReadFile(path); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated test remove changed the contract: err=%v", err)
	}
}

// TestTestRemoveWithoutTestsTakesAWholeGroupOut: a group whose tests are all
// gone leaves the contract, and every surface, the always lists, the unknown
// list and the cadence stop naming it.
func TestTestRemoveWithoutTestsTakesAWholeGroupOut(t *testing.T) {
	contract := testingMergeFixture()
	spare := contract.Groups[0]
	spare.ID = "spare-group"
	contract.Groups = append(contract.Groups, spare)
	contract.Surfaces[0].Standard = append(contract.Surfaces[0].Standard, "spare-group")
	contract.Always.Standard = []string{"spare-group"}
	contract.Unknown = append(contract.Unknown, "spare-group")
	contract.Cadence = []string{"spare-group"}
	path := filepath.Join(t.TempDir(), "testing.json")
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := dispatch([]string{"test", "remove", "--file", path, "--group", "spare-group"}); code != 0 {
		t.Fatalf("test remove of a group exit = %d", code)
	}
	loaded, err := testpolicy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(loaded)
	if len(loaded.Groups) != 1 || strings.Contains(string(encoded), "spare-group") {
		t.Fatalf("the removed group is still named: %s", encoded)
	}
	if code := dispatch([]string{"test", "remove", "--file", path, "--group", "spare-group"}); code != 0 {
		t.Fatalf("a repeated group removal exit = %d", code)
	}
}
