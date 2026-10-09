package repoproof

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestFullReporterMeasuresItsTotalBeforeTheReport(t *testing.T) {
	t.Parallel()
	for _, red := range []bool{false, true} {
		t.Run(fmt.Sprint(red), func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			var out, stderr bytes.Buffer
			hooks := HostRunners{Now: func() time.Time { return now }, Environment: func() (string, error) { return "fixture", nil }, Native: func(proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				now = now.Add(2 * time.Minute)
				return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/unit", Shard: 1, Status: "ok"}}}, nil
			}}
			command := func(argv []string, stdout, stderr io.Writer) error {
				if argv[0] == "go" {
					now = now.Add(time.Minute)
					if red {
						return exec.Command("/usr/bin/false").Run()
					}
					return nil
				}
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
			if code != want || !strings.Contains(text, "landing group fast-static-build "+status+" 60000\n") || strings.Count(text, "landing clock total 300000\n") != 1 || strings.Index(text, "landing clock total ") > strings.Index(text, "LANDING-CHECKED\t") || red && strings.Index(text, "landing clock total ") > strings.Index(text, "LANDING-FAILED\t") || !strings.HasSuffix(text, fmt.Sprintf("LANDING-CHECKED\t%d\n", want)) {
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
