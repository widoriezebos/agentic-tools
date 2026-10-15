package commandenv_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestEngineEnvironmentUsesResolvedDirectory(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, directory string
		input, want     []string
	}{
		{"resolved", "/installed/bin", []string{"PATH=/earlier", "VALUE=kept", "PATH=/declared"}, []string{"VALUE=kept", "PATH=/installed/bin" + string(os.PathListSeparator) + "/declared"}},
		{"empty path", "/installed/bin", []string{"PATH="}, []string{"PATH=/installed/bin" + string(os.PathListSeparator)}},
		{"no path", "/installed/bin", []string{"VALUE=kept"}, []string{"VALUE=kept"}},
		{"no directory", "", []string{"PATH=/declared", "VALUE=kept"}, []string{"PATH=/declared", "VALUE=kept"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			before := append([]string{}, row.input...)
			got := commandenv.EngineEnvironmentInDirectory(row.input, row.directory)
			if !reflect.DeepEqual(got, row.want) || !reflect.DeepEqual(row.input, before) {
				t.Fatalf("resolved environment=%v, want %v; input=%v, before=%v", got, row.want, row.input, before)
			}
			got[0] = "CHANGED=copy"
			if !reflect.DeepEqual(row.input, before) {
				t.Fatal("result aliases the declared environment")
			}
		})
	}
}

func TestEngineEnvironmentPreservesDeclaredValuesAndLastPath(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name   string
		input  []string
		path   string
		values []string
	}{
		{"duplicates", []string{"PATH=/earlier", "VALUE=kept", "PATH=/declared"}, "/declared", []string{"VALUE=kept"}},
		{"missing", []string{"VALUE=kept"}, "", []string{"VALUE=kept"}},
		{"empty", []string{}, "", []string{}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			before := append([]string{}, row.input...)
			got := commandenv.EngineEnvironment(row.input)
			want := "PATH=" + filepath.Dir(executable) + string(os.PathListSeparator) + row.path
			paths := 0
			for _, entry := range got {
				if strings.HasPrefix(entry, "PATH=") {
					paths++
					if entry != want {
						t.Fatalf("engine path: %q, want %q", entry, want)
					}
				} else if entry != "VALUE=kept" {
					t.Fatalf("declared value changed: %q", entry)
				}
			}
			// An input without a PATH gains none: the shell keeps its own default search (no empty entry either).
			wantPaths, wantGot := 1, append(append([]string{}, row.values...), want)
			if row.path == "" {
				wantPaths, wantGot = 0, row.values
			}
			if paths != wantPaths || !reflect.DeepEqual(row.input, before) {
				t.Fatalf("path count=%d (want %d); input=%v, before=%v", paths, wantPaths, row.input, before)
			}
			if !reflect.DeepEqual(got, wantGot) {
				t.Fatalf("declared values changed: %v, want %v", got, wantGot)
			}
		})
	}
}

func TestInstallationEnvironmentValidatesItsOwnEngineAndSuppliesMissingPath(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"path", "absent", "missing", "not executable", "directory"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			bin := filepath.Join(install, "bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(bin, "metasystem")
			if mode == "directory" {
				if err := os.Mkdir(binary, 0700); err != nil {
					t.Fatal(err)
				}
			} else if mode != "missing" {
				perm := os.FileMode(0700)
				if mode == "not executable" {
					perm = 0600
				}
				if err := testexec.WriteFile(binary, []byte("fixture"), perm); err != nil {
					t.Fatal(err)
				}
			}
			input := []string{"VALUE=kept", "PATH=/earlier", "PATH=/competitor"}
			if mode == "absent" {
				input = []string{"VALUE=kept"}
			}
			before := append([]string(nil), input...)
			got, err := commandenv.InstallationEnvironment(input, install)
			if mode == "missing" || mode == "not executable" || mode == "directory" {
				if err == nil || got != nil || !strings.Contains(err.Error(), "no command was started") {
					t.Fatalf("invalid installation accepted: %v %v", got, err)
				}
				return
			}
			wantPath := "PATH=" + bin
			if mode == "path" {
				wantPath += string(os.PathListSeparator) + "/competitor"
			} else {
				wantPath += string(os.PathListSeparator) + "/usr/bin:/bin"
			}
			if err != nil || !reflect.DeepEqual(got, []string{"VALUE=kept", wantPath}) || !reflect.DeepEqual(input, before) {
				t.Fatalf("installation path: %v %v; original %v", got, err, input)
			}
			got[0] = "CHANGED=value"
			if !reflect.DeepEqual(input, before) {
				t.Fatal("environment aliases its input")
			}
		})
	}
}
