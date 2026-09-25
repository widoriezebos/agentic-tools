package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The launcher's own filter must deliver the caller's Go env selection to
// scratch preparation: testingEnvironment -> testingRunRequest -> Prepare.
func TestTestingEnvironmentCarriesGoConfigIntoScratchPreparation(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	write := func(path, contents string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	explicit := filepath.Join(host, "explicit-env")
	xdg := filepath.Join(host, "xdg")
	write(explicit, "GOFLAGS=-tags=explicit\n")
	write(filepath.Join(xdg, "go", "env"), "GOFLAGS=-tags=xdg\n")
	write(filepath.Join(host, "Library", "Application Support", "go", "env"), "GOFLAGS=-tags=darwin\n")
	defaultWant := "GOFLAGS=-tags=xdg\n"
	if runtime.GOOS == "darwin" {
		defaultWant = "GOFLAGS=-tags=darwin\n"
	}
	group := testpolicy.Group{ID: "unit", Adapter: "command", EnvironmentMode: "inherit"}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{SelectedGroups: []string{group.ID}}
	cases := []struct {
		name, goEnv, want string
		off               bool
	}{
		{"off", "GOENV=off", "", true},
		{"explicit", "GOENV=" + explicit, "GOFLAGS=-tags=explicit\n", false},
		{"default", "", defaultWant, false},
	}
	for _, test := range cases {
		hostEnv := []string{"HOME=" + host, "PATH=/usr/bin:/bin", "XDG_CONFIG_HOME=" + xdg, "UNRELATED_SECRET=1"}
		if test.goEnv != "" {
			hostEnv = append(hostEnv, test.goEnv)
		}
		prepared := testingPreparation{EffectiveContract: contract, Plan: plan, Environment: testingEnvironment(hostEnv)}
		if slices.ContainsFunc(prepared.Environment, func(entry string) bool { return strings.HasPrefix(entry, "UNRELATED_SECRET=") }) {
			t.Fatal("filter passed an unrelated host variable")
		}
		request := testingRunRequest(prepared, "attempt", t.TempDir(), "", "", "")
		run, err := proofrun.CreateScratchRun(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := proofrun.PrepareScratchEnvironment(&request, run); err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		location := filepath.Join(run.Dir("goenv"), "env")
		if test.name == "default" {
			// An unset GOENV is resolved per group: the caller's default file
			// is snapshotted at the group's managed default config location.
			home := filepath.Join(run.Dir("groups"), group.ID, ".environment", "home")
			location = filepath.Join(home, ".config", "go", "env")
			if runtime.GOOS == "darwin" {
				location = filepath.Join(home, "Library", "Application Support", "go", "env")
			}
		}
		snapshot, err := os.ReadFile(location)
		if test.off {
			if request.ScratchEnvironment.GoEnv != "off" || !os.IsNotExist(err) {
				t.Errorf("off: mode %q, snapshot err %v", request.ScratchEnvironment.GoEnv, err)
			}
		} else if err != nil || string(snapshot) != test.want {
			t.Errorf("%s snapshot = %q %v, want %q", test.name, snapshot, err, test.want)
		}
		if err := proofrun.ValidateScratchEnvironment(request, run); err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
		if err := run.Cleanup(nil); err != nil {
			t.Fatal(err)
		}
	}
}
