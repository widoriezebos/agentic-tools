package launch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

func TestCheckSectionNamesOnlyFullyAccountedFailures(t *testing.T) {
	t.Parallel()
	const named = "--- FAIL: TestAudit (0.01s)\nFAIL\nFAIL\texample.test/m/pkg/unit\t0.01s\n"
	for _, tc := range []struct {
		name, output, prefix string
		want                 []loopstop.RedUnit
	}{
		{name: "top level", output: named, want: []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}}},
		{name: "template", prefix: "metasystem/", output: named, want: []loopstop.RedUnit{{Unit: "metasystem/pkg/unit", Tests: []string{"TestAudit"}}}},
		{name: "subtest ignored", output: "    --- FAIL: TestAudit/sub (0.01s)\n" + named, want: []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}}},
		{name: "subtest alone", output: "--- FAIL: TestAudit/sub (0.01s)\nFAIL\nFAIL\texample.test/m/pkg/unit\t0.01s\n"},
		{name: "indented top level ignored", output: "    " + named},
		{name: "two packages", output: named + "--- FAIL: TestOther (0.02s)\nFAIL\nFAIL\texample.test/m/other\t0.02s\n", want: []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}, {Unit: "other", Tests: []string{"TestOther"}}}},
		{name: "module root", output: "--- FAIL: TestRoot (0.01s)\nFAIL\nFAIL\texample.test/m\t0.01s\n", want: []loopstop.RedUnit{{Unit: ".", Tests: []string{"TestRoot"}}}},
		{name: "unfinished", output: named + "--- FAIL: TestUnfinished (0.01s)\n"},
		{name: "process exit", output: "--- FAIL: TestAudit (0.01s)\nFAIL\texample.test/m/pkg/unit\t0.01s\nFAIL\n"},
		{name: "fatal error", output: "--- FAIL: TestAudit (0.01s)\nfatal error: stack overflow\n\ngoroutine 1 [running]:\nFAIL\texample.test/m/pkg/unit\t1.5s\nFAIL\n"},
		{name: "bare FAIL before failure", output: "FAIL\n--- FAIL: TestAudit (0.01s)\nFAIL\texample.test/m/pkg/unit\t0.01s\n"},
		{name: "bare FAIL before last failure", output: "--- FAIL: TestOther (0.02s)\nFAIL\n--- FAIL: TestAudit (0.01s)\nFAIL\texample.test/m/pkg/unit\t0.01s\n"},
		{name: "second package crashed", output: named + "--- FAIL: TestOther (0.02s)\nFAIL\texample.test/m/other\t0.02s\n"},
		{name: "earlier command finished", output: named + "FAIL\n--- FAIL: TestOther (0.02s)\nFAIL\texample.test/m/other\t0.02s\n"},
		{name: "exit status after bare FAIL", output: "--- FAIL: TestAudit (0.01s)\nFAIL\nexit status 1\nFAIL\texample.test/m/pkg/unit\t0.01s\n", want: []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}}},
		{name: "timeout after named failure", output: "--- FAIL: TestAudit (0.01s)\npanic: test timed out after 30m\n\trunning tests:\n\t\tTestHang (30m0s)\nFAIL\texample.test/m/pkg/unit\t1800.1s\n"},
		{name: "package without name", output: named + "FAIL\texample.test/m/other\t0.01s\n"},
		{name: "outside module", output: named + "--- FAIL: TestOther (0.01s)\nFAIL\nFAIL\texample.test/more/other\t0.01s\n"},
		{name: "build after name", output: "--- FAIL: TestAudit (0.01s)\nFAIL\texample.test/m/pkg/unit [build failed]\n"},
		{name: "no package", output: "--- FAIL: TestAudit (0.01s)\nFAIL\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sectionRedUnits(tc.output, "example.test/m", tc.prefix); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("section units=%+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCheckSectionUsesTheInstallationModuleAndTemplateMode(t *testing.T) {
	t.Parallel()
	for _, layout := range []string{"root adopted", "nested adopted", "template", "missing module"} {
		t.Run(layout, func(t *testing.T) {
			t.Parallel()
			root, execution := t.TempDir(), t.TempDir()
			directory := root
			if layout != "root adopted" {
				directory = filepath.Join(root, "metasystem")
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if layout != "missing module" {
				if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("// fixture\nmodule \"example.test/m\"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}}
			if layout == "template" {
				if err := os.WriteFile(filepath.Join(directory, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
					t.Fatal(err)
				}
				want[0].Unit = "metasystem/pkg/unit"
			}
			if layout == "missing module" {
				want = nil
			}
			if err := writeUnitJSON(filepath.Join(execution, "result.json"), struct{ Exits []CheckExit }{[]CheckExit{
				{Name: "cheap", Output: "--- FAIL: TestCheap (0.01s)\nFAIL\nFAIL\texample.test/m/pkg/unit\t0.01s\n"},
				{Name: "audits", Output: "--- FAIL: TestAudit (0.01s)\nFAIL\nFAIL\texample.test/m/pkg/unit\t0.01s\n"},
			}}, execution); err != nil {
				t.Fatal(err)
			}
			check := CheckExecution{Directory: execution}
			if got := check.sectionUnits("launch:check:audits", directory); !reflect.DeepEqual(got, want) {
				t.Fatalf("installation units=%+v, want %+v", got, want)
			}
		})
	}
}

func TestCheckSectionsMergeTestsWithoutMovingRegistrations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		units []loopstop.RedUnit
		want  []loopstop.RedUnit
	}{
		{name: "first seen order", units: []loopstop.RedUnit{
			{Unit: "pkg/unit", Tests: []string{"TestRed", "TestUnregistered"}, Registered: []string{"flake:red"}},
			{Unit: "other", Tests: []string{"TestOther"}},
			{Unit: "pkg/unit", Tests: []string{"TestRed", "TestAudit", "TestAudit"}},
		}, want: []loopstop.RedUnit{
			{Unit: "pkg/unit", Tests: []string{"TestRed", "TestUnregistered", "TestAudit"}, Registered: []string{"flake:red", "", ""}},
			{Unit: "other", Tests: []string{"TestOther"}, Registered: []string{""}},
		}},
		{name: "unnamed first", units: []loopstop.RedUnit{{Unit: "pkg/unit"}, {Unit: "pkg/unit", Tests: []string{"TestAudit"}}}, want: []loopstop.RedUnit{{Unit: "pkg/unit"}}},
		{name: "unnamed last", units: []loopstop.RedUnit{{Unit: "pkg/unit", Tests: []string{"TestAudit"}}, {Unit: "pkg/unit"}}, want: []loopstop.RedUnit{{Unit: "pkg/unit"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := mergeRedUnits(tc.units); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("merged units=%+v, want %+v", got, tc.want)
			}
		})
	}
}
