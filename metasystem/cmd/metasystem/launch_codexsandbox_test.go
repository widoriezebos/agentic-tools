package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// supervisorStart stands in for the supervisor process the launcher starts.
type supervisorStart func(id, stateDir string) (identity.Ref, error)

func (start supervisorStart) StartSupervisor(id, stateDir string) (identity.Ref, error) {
	return start(id, stateDir)
}

// A unit's Codex child runs under the sandbox of the installation the unit
// runs for, wherever the engine sits: in that installation's bin, in a
// steward pin under it, or in another installation. The unit runner's
// manager admits the launch; the argv is built as the supervisor process
// builds it, by a manager of its own from the same engine reading the stored
// record.
func TestUnitCodexSandboxFollowsTheSelectedInstallation(t *testing.T) {
	previousExecutable, previousLookup, previousMachine := launchExecutable, launchLookupEnv, launchMachine
	t.Cleanup(func() {
		launchExecutable, launchLookupEnv, launchMachine = previousExecutable, previousLookup, previousMachine
	})
	launchLookupEnv = func(string) (string, bool) { return "", false }
	launchMachine = func(string) (string, error) { return "", errors.New("the fixture names no machine") }
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	conf := "metasystem.runtimes=codex\nlaunch.build.runtime=codex\n"
	for _, shape := range []string{"bin-control", "pinned-engine", "separate-selected-installation"} {
		t.Run(shape, func(t *testing.T) {
			installation := t.TempDir()
			write(filepath.Join(installation, "metasystem.conf"), conf)
			write(filepath.Join(installation, "metasystem.conf.local"), config.CodexSandboxKey+"="+config.CodexSandboxFullAccess+"\n")
			executable := filepath.Join(installation, "bin", "metasystem")
			switch shape {
			case "pinned-engine":
				executable = filepath.Join(installation, "artifacts", "agents", "steward", "engine-pins", "generation-9-fixture")
			case "separate-selected-installation":
				other := t.TempDir()
				write(filepath.Join(other, "metasystem.conf"), conf)
				executable = filepath.Join(other, "bin", "metasystem")
			}
			launchExecutable = func() (string, error) { return executable, nil }
			brief := filepath.Join(installation, "brief.md")
			write(brief, "build it\n\n| unit | lines |\n|---|---|\n| u1 | 10 |\n")
			store := launch.Store{Root: t.TempDir()}
			admitting := (&intentInvocation{}).work().units(stateroot.Layout{InstallationRoot: stateroottest.Installation(t, installation)}).Manager
			admitting.Store = store
			var argv []string
			admitting.Supervisor = supervisorStart(func(id, stateDir string) (identity.Ref, error) {
				supervisor := launchManager()
				supervisor.Store = store
				record, err := supervisor.Store.Read(id)
				if err != nil {
					return identity.Ref{}, err
				}
				command, err := supervisor.Adapters[record.Adapter].Command(record, stateDir)
				if err != nil {
					return identity.Ref{}, err
				}
				argv = command.Args
				ref := identity.Ref{Pid: 10, StartedAtSec: 10}
				_, err = store.Update(id, func(current *launch.Record) error {
					current.Supervisor, current.Child, current.State = &ref, &ref, launch.Running
					return nil
				})
				return ref, err
			})
			if _, err := admitting.Start(launch.StartSpec{Kind: "build", Goal: "sandbox-fixture", WorkingDirectory: installation, Brief: brief}); err != nil {
				t.Fatal(err)
			}
			flag := slices.Index(argv, "-s")
			if flag < 0 || flag+1 >= len(argv) || argv[flag+1] != config.CodexSandboxFullAccess {
				t.Fatalf("engine %s: argv %q, want -s %s", executable, argv, config.CodexSandboxFullAccess)
			}
		})
	}
}
