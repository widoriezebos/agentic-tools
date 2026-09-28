package batch

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The lane is language-neutral: its non-test sources import no Go toolchain
// package and name no Go tool; its test sources import none either. Every
// language fact reaches it through internal/testpolicy/adapter. There is no
// exception list.
var (
	leakForbiddenImports = []string{"go/ast", "go/build", "go/format", "go/parser", "go/token",
		"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"}
	leakForbiddenStrings = []string{"go test", "-count=1", "GOCACHE", "gofmt", `"go"`}
)

// laneAuditFiles lists the lane's sources relative to the module root: the
// batch package and its subpackages except goadapter, the command's
// landing_batch_*.go files, and internal/quality once it exists.
func laneAuditFiles(module string) ([]string, error) {
	var files []string
	walk := func(dir string, skip string) error {
		root := filepath.Join(module, dir)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			return nil
		}
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && skip != "" && entry.Name() == skip {
				return filepath.SkipDir
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".go") {
				relative, err := filepath.Rel(module, path)
				files = append(files, filepath.ToSlash(relative))
				return err
			}
			return nil
		})
	}
	if err := walk(filepath.Join("internal", "landing", "batch"), "goadapter"); err != nil {
		return nil, err
	}
	if err := walk(filepath.Join("internal", "quality"), ""); err != nil {
		return nil, err
	}
	command, err := os.ReadDir(filepath.Join(module, "cmd", "metasystem"))
	if err != nil {
		return nil, err
	}
	for _, entry := range command {
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && strings.HasPrefix(name, "landing_batch_") {
			files = append(files, "cmd/metasystem/"+name)
		}
	}
	sort.Strings(files)
	return files, nil
}

// goImports reads a Go file's import paths without the Go parser, which the
// audit itself may not import.
func goImports(source []byte) []string {
	var imports []string
	inBlock := false
	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			imports = append(imports, importPath(line))
		case line == "import (":
			inBlock = true
		case strings.HasPrefix(line, "import "):
			imports = append(imports, importPath(strings.TrimPrefix(line, "import ")))
		}
	}
	return imports
}

func importPath(spec string) string {
	fields := strings.Fields(spec)
	for _, field := range fields {
		if unquoted, err := strconv.Unquote(field); err == nil {
			return unquoted
		}
	}
	return ""
}

func laneLeaks(path string, source []byte) []string {
	var leaks []string
	for _, imported := range goImports(source) {
		for _, forbidden := range leakForbiddenImports {
			if imported == forbidden {
				leaks = append(leaks, path+" imports "+imported)
			}
		}
	}
	if strings.HasSuffix(path, "_test.go") {
		return leaks
	}
	for _, forbidden := range leakForbiddenStrings {
		if bytes.Contains(source, []byte(forbidden)) {
			leaks = append(leaks, path+" names "+forbidden)
		}
	}
	return leaks
}

func TestLaneImportsNoGoToolchainPackages(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files, err := laneAuditFiles(module)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, path := range files {
		seen[path] = true
		source, err := os.ReadFile(filepath.Join(module, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, leak := range laneLeaks(path, source) {
			t.Error(leak)
		}
	}
	for _, required := range []string{"internal/landing/batch/gate.go", "internal/landing/batch/red.go",
		"internal/landing/batch/leak_audit_test.go", "cmd/metasystem/landing_batch_red.go"} {
		if !seen[required] {
			t.Errorf("the audit scope misses %s", required)
		}
	}
	for path := range seen {
		if strings.Contains(path, "/goadapter/") {
			t.Errorf("the audit scope includes the Go adapter file %s", path)
		}
	}
}

func TestLaneLeakAuditFlagsEachForbiddenForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path, source string
		leak         bool
	}{
		{"internal/landing/batch/x.go", "package batch\nimport \"go/ast\"\n", true},
		{"internal/landing/batch/x.go", "package batch\nimport (\n\tparse \"go/parser\"\n)\n", true},
		{"internal/landing/batch/x.go", "package batch\nimport (\n\t\"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages\"\n)\n", true},
		{"internal/landing/batch/x.go", "package batch\nvar step = []string{\"go\", \"vet\"}\n", true},
		{"internal/landing/batch/x.go", "package batch\n// runs go test on the tip\n", true},
		{"cmd/metasystem/landing_batch_x.go", "package main\nvar flag = \"-count=1\"\n", true},
		{"cmd/metasystem/gate_unit.go", "package main\nvar env = \"GOCACHE=/x\"\n", true},
		{"internal/landing/batch/x.go", "package batch\nvar formatter = \"gofmt\"\n", true},
		{"internal/landing/batch/x_test.go", "package batch\nimport \"go/token\"\n", true},
		{"internal/landing/batch/x_test.go", "package batch\nvar step = []string{\"go\", \"test\", \"-count=1\"}\n", false},
		{"internal/landing/batch/x.go", "package batch\nimport \"strings\"\nvar s = strings.ToLower(\"Go\")\n", false},
	} {
		if got := len(laneLeaks(tc.path, []byte(tc.source))) != 0; got != tc.leak {
			t.Errorf("%s %q leak=%v want %v", tc.path, tc.source, got, tc.leak)
		}
	}
}
