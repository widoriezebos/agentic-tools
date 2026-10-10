package repoproof

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestFullReporterCorrectsLivePackageVerdicts(t *testing.T) {
	t.Parallel()
	for _, final := range []string{"fail", "missing"} {
		t.Run(final, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				live := []proofrun.PackageExecution{
					{Package: "fixture/unit", Shard: 1, Status: "ok"},
					{Package: "fixture/unit", Shard: 2, Status: "ok"},
				}
				request.Progress(3, nil)
				request.Progress(0, live)
				if strings.Count(out.String(), "landing package fixture/unit") != 2 {
					t.Fatalf("live completions were delayed: %s", &out)
				}
				live[0].Status = final
				live = append(live, proofrun.PackageExecution{Package: "fixture/other", Shard: 1, Status: "ok"})
				return proofrun.NativeInventoryResult{Execution: live}, nil
			}}
			code := runHost(&out, &stderr, func(key string) string {
				if key == "LANDING_ONLY" {
					return "fixture/unit"
				}
				return ""
			}, nil, "../../testing.json", hooks)
			text := out.String()
			first := "landing package fixture/unit 1 ok 0\n"
			last := "landing package fixture/unit 1 " + final + " 0\n"
			if code != 1 || strings.Count(text, first) != 1 || strings.Count(text, last) != 1 || strings.Index(text, last) < strings.Index(text, first) || strings.Count(text, "landing package fixture/unit 2 ok 0\n") != 1 || strings.Count(text, "landing package fixture/other 1 ok 0\n") != 1 || !strings.Contains(text, "LANDING-FAILED\tfixture/unit\t\n") {
				t.Fatalf("final verdict lost: exit=%d out=%s stderr=%s", code, text, &stderr)
			}
		})
	}
}

func TestFullReporterMeasuresItsTotalBeforeTheReport(t *testing.T) {
	t.Parallel()
	for _, red := range []bool{false, true} {
		t.Run(fmt.Sprint(red), func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			var out, stderr bytes.Buffer
			nativeRuns := 0
			hooks := HostRunners{Now: func() time.Time { return now }, Environment: func() (string, error) { return "fixture", nil }, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				nativeRuns++
				if request.Progress == nil {
					t.Fatal("reporter supplied no live progress callback")
				}
				request.Progress(1, nil)
				completed := []proofrun.PackageExecution{{Package: "fixture/unit", Shard: 1, Status: "ok"}}
				request.Progress(0, completed)
				if !strings.Contains(out.String(), "landing planned 1\nlanding package fixture/unit 1 ok 0\n") {
					t.Fatalf("progress was not published during native execution: %s", &out)
				}
				now = now.Add(2 * time.Minute)
				return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/unit", Shard: 1, Status: "ok"}}}, nil
			}}
			hooks.Groups = func(ids []string) ([]proofrun.NamedGroupResult, error) {
				if len(ids) != 1 || ids[0] != "fast-static-build" {
					t.Fatalf("static selection: %v", ids)
				}
				now = now.Add(time.Minute)
				status := "green"
				if red {
					status = "red"
				}
				return []proofrun.NamedGroupResult{{ID: "fast-static-build", Status: status, DurationMS: 60000}}, nil
			}
			command := func(argv []string, stdout, stderr io.Writer) error {
				fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				return nil
			}
			code := runHost(&out, &stderr, func(string) string { return "" }, command, "../../testing.json", hooks)
			want := 0
			status := "green"
			if red {
				want = 1
				status = "red"
			}
			text := out.String()
			if code != want || strings.Count(text, "landing package fixture/unit 1 ok 0\n") != nativeRuns || !strings.Contains(text, "landing group fast-static-build "+status+" 60000\n") || strings.Count(text, "landing clock total 300000\n") != 1 || strings.Index(text, "landing clock total ") > strings.Index(text, "LANDING-CHECKED\t") || red && strings.Index(text, "landing clock total ") > strings.Index(text, "LANDING-FAILED\t") || !strings.HasSuffix(text, fmt.Sprintf("LANDING-CHECKED\t%d\n", want)) {
				t.Fatalf("exit=%d out=%s stderr=%s", code, text, &stderr)
			}
		})
	}
	t.Run("not-run", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
		var out, stderr bytes.Buffer
		hooks := HostRunners{Now: func() time.Time { return now }, Environment: func() (string, error) {
			now = now.Add(time.Minute)
			return "", &os.PathError{Op: "read", Path: "fixture", Err: os.ErrNotExist}
		}}
		code := runHost(&out, &stderr, func(string) string { return "" }, nil, "../../testing.json", hooks)
		text := out.String()
		if code != 1 || strings.Count(text, "landing clock total 60000\n") != 1 || !strings.HasPrefix(text, "landing clock total 60000\nLANDING-NOT-RUN\t") {
			t.Fatalf("not-run exit=%d out=%s stderr=%s", code, text, &stderr)
		}
	})

}
