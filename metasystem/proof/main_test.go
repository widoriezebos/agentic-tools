package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// This integration drives the reporter's real process entrypoint and marker
// lifetime; a stubbed launcher cannot prove the child's kernel ancestry.
func TestReporterRegistersLiveAncestorAndRemovesMarker(t *testing.T) {
	t.Parallel()
	reporter := filepath.Join(t.TempDir(), "reporter")
	build := testenv.Go("build", "-buildvcs=false", "-o", reporter, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build reporter: %v\n%s", err, output)
	}
	module, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Load("../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	const id = "section/go-engine-gate"
	for i, group := range contract.Groups {
		if group.ID == id {
			group.Argv = []string{binary, "section-marker-probe", module}
			group.CWD = "."
			group.Inputs = []string{"bed.txt"}
			group.Tools = nil
			contract.Groups[i] = group
		}
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"testing.json": data, "bed.txt": []byte("section input")} {
		if err := os.WriteFile(filepath.Join(module, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{"0", "1"} {
		command := exec.Command(reporter, "--section", id)
		command.Dir = module
		command.Env = append(os.Environ(), "SECTION_MARKER_PROBE_EXIT="+status)
		output, err := command.CombinedOutput()
		if (err == nil) != (status == "0") || !strings.Contains(string(output), "authenticated reporter ancestor") {
			t.Fatalf("section status=%s err=%v output=%s", status, err, output)
		}
		survey := gaterun.Survey(module)
		if len(survey.Live) != 0 || len(survey.Unreadable) != 0 {
			t.Fatalf("reporter left its marker: %+v", survey)
		}
	}
}

func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "section-marker-probe" {
		for _, marker := range gaterun.Survey(os.Args[2]).Live {
			if marker.Gate == "repository proof" && proofrun.AuthenticateAncestor(int64(os.Getpid()),
				proofrun.ProcessIdentity{Pid: marker.Pid, PidStartedAt: marker.PidStartedAt}) == nil {
				fmt.Print("authenticated reporter ancestor\n")
				if os.Getenv("SECTION_MARKER_PROBE_EXIT") == "1" {
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		fmt.Fprintln(os.Stderr, "no live reporter ancestor")
		os.Exit(1)
	}
	if exit, handled := repoproof.Custodian(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	os.Exit(testenv.Main(m))
}
