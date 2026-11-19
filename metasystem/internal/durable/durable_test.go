package durable_test

import (
	"bytes"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// earlySyncFileEnv names a file the parent test made. A re-run of this binary
// that finds it syncs the file through the adapter before testenv.Main, so the
// harness opt-in comes after a sync. The helper opens the parent's file and
// allocates no temp entry of its own before the harness.
const earlySyncFileEnv = "DURABLE_EARLY_SYNC_FILE"

func TestMain(m *testing.M) {
	if path := os.Getenv(earlySyncFileEnv); path != "" {
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			os.Exit(2)
		}
		syncErr := durable.Sync(file)
		closeErr := file.Close()
		if syncErr != nil || closeErr != nil {
			os.Exit(2)
		}
	}
	os.Exit(testenv.Main(m))
}

func TestDurabilityOptInOnceAndFreeze(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		isTest, early bool
		level         string
		refused       bool
	}{
		{name: "test", isTest: true, level: "kernel"},
		{name: "synced before opt-in", isTest: true, early: true, level: "full"},
		{name: "normal binary", isTest: false, level: "full", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var p durable.TestPolicy
			if got := p.TestCurrent(false); got != "full" {
				t.Fatalf("fresh level = %q, want full", got)
			}
			if tc.early {
				if got := p.TestCurrent(true); got != "full" {
					t.Fatalf("first sync level = %q, want full", got)
				}
			}
			err := p.TestOptIn(tc.isTest)
			if (err != nil) != tc.refused {
				t.Fatalf("opt-in refusal = %v, want refused %t", err, tc.refused)
			}
			if got := p.TestCurrent(false); got != tc.level {
				t.Fatalf("level after opt-in = %q, want %q", got, tc.level)
			}
			if got := p.TestCurrent(true); got != tc.level {
				t.Fatalf("level at first sync after opt-in = %q, want %q", got, tc.level)
			}
			if !tc.refused && p.TestOptIn(true) == nil {
				t.Fatal("second opt-in accepted")
			}
			if got := p.TestCurrent(true); got != tc.level {
				t.Fatalf("level moved after a second opt-in: %q, want %q", got, tc.level)
			}
		})
	}
}

// A refused opt-in freezes nothing, and a sync after it freezes the process
// at full, which no later opt-in lowers.
func TestDurabilityRefusedOptInDoesNotLowerAfterSync(t *testing.T) {
	t.Parallel()
	var p durable.TestPolicy
	if p.TestOptIn(false) == nil {
		t.Fatal("non-test opt-in accepted")
	}
	if got := p.TestCurrent(true); got != "full" {
		t.Fatalf("sync after refusal = %q, want full", got)
	}
	_ = p.TestOptIn(true)
	if got := p.TestCurrent(true); got != "full" {
		t.Fatalf("level lowered after a sync froze it: %q", got)
	}
}

func TestDurabilitySelectsOperationByLevel(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("selected")
	for _, level := range []string{"full", "kernel"} {
		selected := ""
		err := durable.SyncAtForTest(level, nil,
			func(*os.File) error { selected = "full"; return sentinel },
			func(*os.File) error { selected = "kernel"; return sentinel })
		if selected != level || !errors.Is(err, sentinel) {
			t.Fatalf("selection %s: %s %v", level, selected, err)
		}
	}
}

func TestDurabilityFileErrorsAndDirectory(t *testing.T) {
	t.Parallel()
	if !errors.Is(durable.TestKernelSync(nil), os.ErrInvalid) {
		t.Fatal("nil file handling")
	}
	if !errors.Is(durable.Sync(nil), os.ErrInvalid) {
		t.Fatal("nil file handling through Sync")
	}
	f, err := os.CreateTemp(t.TempDir(), "sync")
	if err != nil {
		t.Fatal(err)
	}
	if err = durable.TestKernelSync(f); err != nil {
		t.Fatal(err)
	}
	if err = durable.Sync(f); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	for name, sync := range map[string]func(*os.File) error{"kernel": durable.TestKernelSync, "Sync": durable.Sync} {
		got, want := sync(f), f.Sync()
		var path *os.PathError
		if !errors.As(got, &path) || path.Op != "sync" || path.Path != f.Name() || !errors.Is(got, os.ErrClosed) || got.Error() != want.Error() {
			t.Fatalf("%s closed file mismatch: %v / %v", name, got, want)
		}
	}
	if err = durable.SyncDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err = durable.SyncDir(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing directory: %v", err)
	}
}

// The injected selector test cannot detect swapped production callbacks.
func TestSyncWiresFullAndKernelOperations(t *testing.T) {
	t.Parallel()
	files := token.NewFileSet()
	source, err := parser.ParseFile(files, "durable.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range source.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name != "Sync" {
			continue
		}
		if len(function.Body.List) != 1 {
			t.Fatal("Sync must delegate to the selector")
		}
		statement, ok := function.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(statement.Results) != 1 {
			t.Fatal("Sync must return the selected operation")
		}
		var expression bytes.Buffer
		if err := format.Node(&expression, files, statement.Results[0]); err != nil {
			t.Fatal(err)
		}
		if expression.String() != "syncAt(process.current(true), f, (*os.File).Sync, kernelSync)" {
			t.Fatal("Sync does not wire the frozen level, full File.Sync and kernelSync in selector order")
		}
		return
	}
	t.Fatal("Sync declaration missing")
}

// A test binary under testenv.Main runs at kernel; the same binary re-run as
// a helper that synced before Main stays at full.
func TestDurabilityLevelUnderMain(t *testing.T) {
	t.Parallel()
	want := "kernel"
	if os.Getenv(earlySyncFileEnv) != "" {
		want = "full"
	}
	if got := durable.Current(); got != want {
		t.Fatalf("level under Main = %q, want %q", got, want)
	}
	if durable.UseKernelInThisTestProcess() == nil {
		t.Fatal("a second opt-in after the harness was accepted")
	}
	if got := durable.Current(); got != want {
		t.Fatalf("level moved after a second opt-in: %q, want %q", got, want)
	}
}

func TestDurabilitySyncBeforeOptInFreezesFull(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(t.TempDir(), "synced-before-main")
	if err := os.WriteFile(fixture, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestDurabilityLevelUnderMain$", "-test.count=1", "-test.v")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, earlySyncFileEnv+"=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, earlySyncFileEnv+"="+fixture)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "--- PASS: TestDurabilityLevelUnderMain") {
		t.Fatalf("helper that synced before Main did not stay full: %v\n%s", err, output)
	}
}

func TestDurabilityNormalBinaryRefusesOptIn(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "refuse")
	build := testenv.Go("build", "-p", "2", "-o", binary, "./internal/durable/testdata/refuse")
	build.Dir = module
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build refusal witness: %v\n%s", err, output)
	}
	output, err := exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("normal binary lowered durability or failed to sync: %v\n%s", err, output)
	}
}
