package repoproof

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestFullJoinsStaticVerdictAndOperationalFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, static, suite     string
		staticError, suiteError bool
		want                    string
	}{
		{name: "static red", static: "red", suite: "ok", want: "LANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\n"},
		{name: "suite red", static: "green", suite: "fail", want: "LANDING-FAILED\tfixture/internal/fixture\t\nLANDING-CHECKED\t1\n"},
		{name: "static error", static: "red", suite: "ok", staticError: true, want: "LANDING-NOT-RUN\tenvironment\n"},
		{name: "suite error", static: "red", suite: "ok", suiteError: true, want: "LANDING-NOT-RUN\tenvironment\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			staticStarted, suiteRan, staticFinished := make(chan struct{}), make(chan string, 1), make(chan struct{})
			hooks := HostRunners{
				Environment: func() (string, error) { return "fixture", nil },
				Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
					if len(ids) != 1 || ids[0] != "refusal-register-standard" {
						t.Errorf("prerequisite selection %v", ids)
					}
					return []proofrun.NamedGroupResult{{ID: "refusal-register-standard", Status: "green"}}, nil
				},
				Static: func(completed []proofrun.NamedGroupResult) ([]proofrun.NamedGroupResult, error) {
					defer close(staticFinished)
					if len(completed) != 1 || completed[0].ID != "refusal-register-standard" {
						t.Error("static lost prerequisites")
					}
					close(staticStarted)
					var root string
					select {
					case root = <-suiteRan:
					case <-t.Context().Done():
						return nil, t.Context().Err()
					}
					if _, err := os.Stat(root); err != nil {
						t.Error("scratch removed before static joined")
					}
					var err error
					if tc.staticError {
						err = errors.New("static launch failed")
					}
					return []proofrun.NamedGroupResult{{ID: "fast-static-build", Status: tc.static, Output: "static diagnostic"}}, err
				},
				Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
					if len(r.BuildTags) > 0 {
						return proofrun.NativeInventoryResult{}, nil
					}
					select {
					case <-staticStarted:
					case <-t.Context().Done():
						return proofrun.NativeInventoryResult{}, t.Context().Err()
					}
					suiteRan <- r.LogRoot
					if tc.suiteError {
						return proofrun.NativeInventoryResult{}, errors.New("suite launch failed")
					}
					return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/internal/fixture", Shard: 1, Status: tc.suite}}}, nil
				},
			}
			exit := runHost(&out, &stderr, func(string) string { return "" }, nil, "testdata/testing.json", hooks)
			select {
			case <-staticFinished:
			default:
				t.Fatal("returned before static finished")
			}
			if exit != 1 || !strings.HasSuffix(out.String(), tc.want) {
				t.Fatalf("exit=%d output=%s stderr=%s", exit, &out, &stderr)
			}
			if !strings.Contains(out.String(), "landing group fast-static-build "+tc.static) {
				t.Fatal("static reporting line lost")
			}
			if tc.static == "red" && !strings.Contains(stderr.String(), "static diagnostic") {
				t.Fatal("static diagnostic lost")
			}
		})
	}
}
