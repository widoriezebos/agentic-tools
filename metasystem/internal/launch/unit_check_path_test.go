package launch

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var unitCheckPathChild = flag.String("unit-check-path-child", "", "run the recorded frozen check")

func TestUnitCheckFindsRunningEngineWithoutCallerPath(t *testing.T) {
	t.Parallel()
	if *unitCheckPathChild != "" {
		data, err := os.ReadFile(*unitCheckPathChild)
		var check UnitCheck
		if err != nil || json.Unmarshal(data, &check) != nil {
			t.Fatalf("frozen check record: %s, %v", data, err)
		}
		environment := slices.Clone(check.Environment)
		_, exits, err := check.Run(check.Directory, t.TempDir(), CheckContext{})
		if err != nil || len(exits) != 2 {
			t.Fatalf("declared engine checks: %+v, %v", exits, err)
		}
		for _, exit := range exits {
			if exit.Exit != 0 || !slices.Contains(strings.Split(exit.Output, "\n"), "engine found; value=kept; "+environment[len(environment)-1]) {
				t.Fatalf("declared %s command: %+v", exit.Name, exit)
			}
		}
		if !slices.Equal(check.Environment, environment) {
			t.Fatal("the frozen record's environment changed")
		}
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\necho \"engine found; value=$CHECK_VALUE; PATH=$PATH\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	check := UnitCheck{Directory: directory, Minutes: 1, Base: "fixture-parent",
		Environment: commandenv.EngineEnvironmentInDirectory([]string{"PATH=" + directory, "CHECK_VALUE=kept"}, filepath.Dir(engine)),
		Cheap:       `metasystem`,
		Audits:      `metasystem`}
	record := filepath.Join(t.TempDir(), "check.json")
	if err := writeUnitJSON(record, check, filepath.Dir(record)); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(t.TempDir(), "runner")
	if err := testexec.WriteFile(runner, data, 0700); err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(t.Context(), runner, "-test.run=^TestUnitCheckFindsRunningEngineWithoutCallerPath$", "-test.timeout=30m", "-unit-check-path-child="+record)
	child.Env = append(child.Environ(), "PATH="+t.TempDir())
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("fixture engine: %v\n%s", err, output)
	}
}
