package main

// Public homes for the machinery verbs people and agents are told to run by
// hand (plans/designs/verbs-object-action.md, sections 3.1, 3.4 and 3.6;
// U9a): test add, test baseline and settings set (U9b folded system register into system setup and left test merge to git's merge driver).
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
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func init() {
	registerIdempotency("test add", idemStateful, "tests, inputs or surface paths already present: success, the contract's bytes unchanged", witnessTestAddRepeat)
	registerIdempotency("test remove", idemStateful, "tests, inputs, surface paths or a surface already absent: success, the contract's bytes unchanged", witnessTestRemoveRepeat)
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
	if code := dispatchOn([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}, t.Output(), t.Output()); code != 0 {
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
	if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
		t.Fatalf("first test add exit = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
		t.Fatalf("repeated test add exit = %d", code)
	}
	if second, err := os.ReadFile(path); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated test add changed the contract: err=%v", err)
	}
	// Inputs and surface paths already present: success, the same bytes.
	for _, args := range [][]string{
		{"test", "add", "--file", path, "--group", "app-group", "--inputs", "go.mod"},
		{"test", "add", "--file", path, "--surface", "app", "--paths", "app/**"},
	} {
		if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
			t.Fatalf("%v exit = %d", args, code)
		}
		if again, err := os.ReadFile(path); err != nil || !bytes.Equal(first, again) {
			t.Fatalf("%v changed the contract: err=%v", args, err)
		}
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
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	return root, intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable), prove: enrolledPersonProver(t, root, now), commandNow: func(string) (time.Time, error) { return now, nil }}
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

func TestSettingsSetDeclaredKeys(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		key, before string
		refused     bool
	}{
		{"role.code-critic.model", "", true},
		{"role.code-critic.model", "role.code-critic.model=old\n", true},
		{"role.code-crtic.model.claude", "", true},
		{"role.code-critic.model.claude", "", false},
		{"role.code-critic.model.codex", "", false},
		{"launch.read.model.fake", "", false},
		{"mode.refactor.role.code-critic.model.codex", "", false},
		{"evidence.citation-roots", "", false},
	} {
		t.Run(test.key+test.before, func(t *testing.T) {
			t.Parallel()
			root, owners := newHomesSettingsInstallation(t)
			local := filepath.Join(root, "metasystem.conf.local")
			if test.before != "" {
				if err := os.WriteFile(local, []byte(test.before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			code, result := runSettingsSet(t, root, owners, test.key, "X")
			after, err := os.ReadFile(local)
			if test.refused {
				want := test.key + " is not a setting; nearest: role.code-critic.model.claude, role.code-critic.model.codex, role.code-critic.model.devin"
				if code == 0 || result.Outcome != intentRefused || result.Summary != want || string(after) != test.before || (test.before == "" && !os.IsNotExist(err)) || (test.before != "" && err != nil) {
					t.Fatalf("refusal = %d %+v; local=%q, %v", code, result, after, err)
				}
			} else if code != 0 || result.Outcome != intentConfirmed || err != nil || string(after) != test.key+"=X\n" {
				t.Fatalf("declared setting = %d %+v; local=%q, %v", code, result, after, err)
			}
		})
	}
}

func TestSettingsCheckUndeclaredLocalKey(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	root, owners := bed.root(), bed.workOwners()
	local := filepath.Join(root, "metasystem.conf.local")
	body := "# ignored.typo=X\nrole.code-critic.model=X\nrole.code-critic.model=X\nlaunch.read.model=X\n"
	if err := os.WriteFile(local, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "settings check"), nil, &stdout, &stderr, root, owners)
	output := stdout.String() + stderr.String()
	if code == 0 || strings.Count(output, "is not a setting; nearest:") != 1 || !strings.Contains(output, "role.code-critic.model is not a setting; nearest: role.code-critic.model.claude") {
		t.Fatalf("settings check = %d, %q", code, output)
	}
	if after, err := os.ReadFile(local); err != nil || string(after) != body {
		t.Fatalf("settings check changed the local file: %q, %v", after, err)
	}
}

// Valid settings fixtures declare the repository proof commands before validation.
func TestSettingsReadKeysWriteAndValidate(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	root, owners := bed.root(), bed.workOwners()
	owners.work.config = configSettingWithDefault
	owners.contractReady = func(root string, _ bool) (string, int, error) {
		_, contract, path, err := testrun.LoadContract(root)
		return path, len(contract.Groups), err
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\nproof.full=true\nproof.cheap=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := contractmerge.Render(testingMergeFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "testing.json"), contract, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{".claude/agents", ".claude/skills"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, setting := range []struct{ key, value string }{
		{"testing.concurrency", "4"},
		{"dispatch.return-margin-min", "10"},
		{"steward.proof-admission-red-min", "10"},
		{"dispatch.permissions.network", "allow"},
	} {
		if code, result := runSettingsSet(t, root, owners, setting.key, setting.value); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("settings set %s = %d %+v", setting.key, code, result)
		}
		local, err := os.ReadFile(filepath.Join(root, "metasystem.conf.local"))
		if err != nil || !strings.Contains(string(local), setting.key+"="+setting.value+"\n") {
			t.Fatalf("local setting %s missing: %q, %v", setting.key, local, err)
		}
		var stdout, stderr bytes.Buffer
		if code := runIntentIn(mustIntentCommand(t, "settings show"), []string{setting.key}, &stdout, &stderr, root, owners); code != 0 || !strings.Contains(stdout.String(), setting.key) || !strings.Contains(stdout.String(), setting.value) {
			t.Fatalf("settings show %s = %d: %s%s", setting.key, code, &stdout, &stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runConfigValidate([]string{"--conf", filepath.Join(root, "metasystem.conf"), "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("config validate = %d: %s%s", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runIntentIn(mustIntentCommand(t, "settings check"), nil, &stdout, &stderr, root, owners); code != 0 {
		t.Fatalf("settings check = %d: %s%s", code, &stdout, &stderr)
	}
	if code, result := runSettingsSet(t, root, owners, "role.code-critic.model", "claude-opus-5-5"); code == 0 || !strings.Contains(result.Summary, "nearest: role.code-critic.model.claude") {
		t.Fatalf("undeclared model = %d %+v", code, result)
	}
}

// TestSettingsSetRefusesAnUnknownCodexSandbox: launch.codex.sandbox takes
// workspace-write or danger-full-access; any other value is refused naming
// both, and nothing is written.
func TestSettingsSetRefusesAnUnknownCodexSandbox(t *testing.T) {
	t.Parallel()
	root, owners := newHomesSettingsInstallation(t)
	code, result := runSettingsSet(t, root, owners, "launch.codex.sandbox", "other")
	if code != 2 || result.Outcome != intentRefused ||
		!strings.Contains(result.Summary, "workspace-write") || !strings.Contains(result.Summary, "danger-full-access") {
		t.Fatalf("settings set launch.codex.sandbox other = %d %+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf.local")); !os.IsNotExist(err) {
		t.Fatalf("a refused value wrote the local configuration: err=%v", err)
	}
	if code, result := runSettingsSet(t, root, owners, "launch.codex.sandbox", "danger-full-access"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("settings set launch.codex.sandbox danger-full-access = %d %+v", code, result)
	}
	local, err := os.ReadFile(filepath.Join(root, "metasystem.conf.local"))
	if err != nil || string(local) != "launch.codex.sandbox=danger-full-access\n" {
		t.Fatalf("local configuration = %q err=%v", local, err)
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
	if code := dispatchOn([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}, t.Output(), t.Output()); code != 0 {
		t.Fatalf("test add exit = %d", code)
	}
	if code := dispatchOn([]string{"test", "remove", "--file", path, "--group", "app-group", "--tests", "TestAdded"}, t.Output(), t.Output()); code != 0 {
		t.Fatalf("test remove exit = %d", code)
	}
	loaded, err := testpolicy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, names, _ := testpolicy.GoTests(loaded.Groups[0]); !reflect.DeepEqual(names, []string{"TestBase"}) {
		t.Fatalf("test remove left tests %v", names)
	}
	if code := dispatchOn([]string{"test", "remove", "--file", path, "--group", "no-such-group", "--tests", "TestBase"}, t.Output(), t.Output()); code == 0 {
		t.Fatal("test remove of an unknown group succeeded")
	}
}

func witnessTestRemoveRepeat(t *testing.T) {
	path := newHomesAddTestsFixture(t)
	if code := dispatchOn([]string{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestAdded"}, t.Output(), t.Output()); code != 0 {
		t.Fatalf("test add exit = %d", code)
	}
	args := []string{"test", "remove", "--file", path, "--group", "app-group", "--tests", "TestAdded"}
	if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
		t.Fatalf("first test remove exit = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
		t.Fatalf("repeated test remove exit = %d", code)
	}
	if second, err := os.ReadFile(path); err != nil || !bytes.Equal(first, second) {
		t.Fatalf("a repeated test remove changed the contract: err=%v", err)
	}
	// Inputs, surface paths and a whole surface already gone: success, the
	// same bytes. A group keeps at least one input and a surface at least
	// one path (the contract refuses an empty list), so the witness removes
	// a second one of each that it added.
	for _, args := range [][]string{
		{"test", "add", "--file", path, "--group", "app-group", "--inputs", "extra.json"},
		{"test", "add", "--file", path, "--surface", "app", "--paths", "extra/**"},
	} {
		if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
			t.Fatalf("%v exit = %d", args, code)
		}
	}
	for _, args := range [][]string{
		{"test", "remove", "--file", path, "--group", "app-group", "--inputs", "extra.json"},
		{"test", "remove", "--file", path, "--surface", "app", "--paths", "extra/**"},
		{"test", "remove", "--file", path, "--surface", "no-longer-there"},
	} {
		if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
			t.Fatalf("first %v exit = %d", args, code)
		}
		once, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
			t.Fatalf("repeated %v exit = %d", args, code)
		}
		if twice, err := os.ReadFile(path); err != nil || !bytes.Equal(once, twice) {
			t.Fatalf("a repeated %v changed the contract: err=%v", args, err)
		}
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
	if code := dispatchOn([]string{"test", "remove", "--file", path, "--group", "spare-group"}, t.Output(), t.Output()); code != 0 {
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
	if code := dispatchOn([]string{"test", "remove", "--file", path, "--group", "spare-group"}, t.Output(), t.Output()); code != 0 {
		t.Fatalf("a repeated group removal exit = %d", code)
	}
}

// TestTestRemoveAndAddEditGroupInputsAndSurfacePaths: a deleted or moved file
// leaves the contract's inputs and surface paths through the same two verbs,
// and a moved file is re-pointed by removing the old path and adding the new;
// a surface whose paths all went leaves with every dependsOn naming it.
func TestTestRemoveAndAddEditGroupInputsAndSurfacePaths(t *testing.T) {
	contract := testingMergeFixture()
	contract.Groups[0].Inputs = []string{"go.mod", "scripts/old.json", "scripts/gone.sh"}
	contract.Surfaces = append(contract.Surfaces, testpolicy.Surface{ID: "fixture", Paths: []string{"scripts/fixture.sh"}, DependsOn: []string{}, Standard: []string{"app-group"}, Deep: []string{}, Critical: []string{}})
	contract.Surfaces[0].DependsOn = []string{"fixture"}
	contract.Surfaces[0].Paths = []string{"app/**", "scripts/old.json"}
	path := filepath.Join(t.TempDir(), "testing.json")
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"test", "remove", "--file", path, "--group", "app-group", "--inputs", "scripts/old.json,scripts/gone.sh"},
		{"test", "add", "--file", path, "--group", "app-group", "--inputs", "floors.json"},
		{"test", "remove", "--file", path, "--surface", "app", "--paths", "scripts/old.json"},
		{"test", "add", "--file", path, "--surface", "app", "--paths", "floors.json"},
		{"test", "remove", "--file", path, "--surface", "fixture"},
	} {
		if code := dispatchOn(args, t.Output(), t.Output()); code != 0 {
			t.Fatalf("%v exit = %d", args, code)
		}
	}
	loaded, err := testpolicy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Groups[0].Inputs; !reflect.DeepEqual(got, []string{"go.mod", "floors.json"}) {
		t.Fatalf("group inputs = %v", got)
	}
	if len(loaded.Surfaces) != 2 || !reflect.DeepEqual(loaded.Surfaces[0].Paths, []string{"app/**", "floors.json"}) || len(loaded.Surfaces[0].DependsOn) != 0 {
		t.Fatalf("surfaces = %+v", loaded.Surfaces)
	}
	for _, args := range [][]string{
		{"test", "remove", "--file", path, "--group", "no-such-group", "--inputs", "go.mod"},
		{"test", "add", "--file", path, "--surface", "no-such-surface", "--paths", "x"},
		{"test", "add", "--file", path, "--group", "app-group", "--surface", "app", "--paths", "x"},
		{"test", "add", "--file", path, "--group", "app-group", "--tests", "TestBase", "--inputs", "x"},
		{"test", "remove", "--file", path, "--surface", "app", "--inputs", "x"},
	} {
		if code := dispatchOn(args, t.Output(), t.Output()); code == 0 {
			t.Fatalf("%v succeeded", args)
		}
	}
}
