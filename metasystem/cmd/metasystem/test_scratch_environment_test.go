package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The launcher's own filter must deliver the caller's Go env selection to
// scratch preparation: testrun.Environment -> testrun.RunRequest -> Prepare.
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
		prepared := testrun.Preparation{EffectiveContract: contract, Plan: plan, Environment: testrun.Environment(hostEnv)}
		if slices.ContainsFunc(prepared.Environment, func(entry string) bool { return strings.HasPrefix(entry, "UNRELATED_SECRET=") }) {
			t.Fatal("filter passed an unrelated host variable")
		}
		request := testrun.RunRequest(prepared, "attempt", t.TempDir(), "", "", "")
		control := t.TempDir()
		// Cleanup proves the scratch writers gone by taking the writer lock
		// through a fresh description. A parallel test's fork/exec copies the
		// launcher's writer descriptor into its child until that child execs,
		// which keeps the lock held and retains the run as writer-lock-held.
		// Excluding forks while the descriptor is open keeps it this run's own.
		checkScratchLifetime := func() error {
			run, err := proofrun.CreateScratchRun(control)
			if err != nil {
				return err
			}
			if err := proofrun.PrepareScratchEnvironment(&request, run); err != nil {
				return errors.Join(fmt.Errorf("%s: %w", test.name, err), run.Cleanup(nil))
			}
			location := filepath.Join(run.Dir("goenv"), "env")
			if test.name == "default" {
				// An unset GOENV is resolved per group: the caller's default file
				// is snapshotted at the group's managed default config location.
				// Under v2 the group's managed tree lies in its lease slot.
				home := filepath.Join(run.Dir("groups"), group.ID, ".environment", "home")
				if lease := request.ScratchEnvironment.Groups[0].Lease; lease != "" {
					home = filepath.Join(lease, ".environment", "home")
				}
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
			return run.Cleanup(nil)
		}
		if err := testexec.Locked(checkScratchLifetime); err != nil {
			t.Fatal(err)
		}
	}
}
