package plain

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadReportAllowsOnlyOrdinaryLinesAfterChecked(t *testing.T) {
	t.Parallel()
	failed := "landing planned 1\nLANDING-FAILED\tp/q\tTestA TestB\nLANDING-LOAD\t2.75\nLANDING-CHECKED\t1\n"
	render := "landing prove: failed\n✗ commit is proven red\n→ metasystem landing status\n"
	want := checkReport{kind: "complete", failed: []FailedUnit{{Unit: "p/q", Tests: []string{"TestA", "TestB"}}}, load: 2.75}
	for _, test := range []struct {
		name, log string
		want      checkReport
	}{
		{"final checked", failed, want},
		{"render after checked", failed + render, want},
		{"unterminated render", failed + "landing prove: failed", want},
		{"blank lines", failed + "\n\n", want},
		{"green render", "LANDING-CHECKED\t0\n" + render, checkReport{kind: "complete"}},
		{"carriage returns", "LANDING-FAILED\tp/q\tTestA TestB\r\nLANDING-LOAD\t2.75\r\nLANDING-CHECKED\t1\r\n" + render, want},
		{"no checked", "LANDING-FAILED\tp/q\tTestA TestB\n" + render, checkReport{}},
		{"only render", render, checkReport{}},
		{"protocol after checked", failed + "LANDING-FAILED\tr/s\tTestC\n" + render, checkReport{}},
		{"unknown protocol after checked", failed + "LANDING-UNKNOWN\t1\n" + render, checkReport{}},
		{"partial protocol after checked", failed + "LANDING-", checkReport{}},
		{"malformed final checked", failed + "LANDING-CHECKED\tbad\n" + render, checkReport{}},
		{"count mismatch", "LANDING-FAILED\tp/q\tTestA\nLANDING-CHECKED\t2\n" + render, checkReport{}},
		{"negative count", "LANDING-CHECKED\t-1\n" + render, checkReport{}},
		{"missing unit", "LANDING-FAILED\t\tTestA\nLANDING-CHECKED\t1\n" + render, checkReport{}},
		{"malformed failure", "LANDING-FAILED\tp/q\nLANDING-CHECKED\t1\n" + render, checkReport{}},
		{"invalid load", "LANDING-LOAD\tNaN\nLANDING-CHECKED\t0\n" + render, checkReport{}},
		{"not run", "LANDING-NOT-RUN\tunavailable\n", checkReport{kind: "not-run"}},
		{"not run with render", "LANDING-NOT-RUN\tunavailable\n" + render, checkReport{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := readReport([]byte(test.log)); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("report = %+v, want %+v", got, test.want)
			}
			if got := FailedChecks([]byte(test.log)); !reflect.DeepEqual(got, test.want.failed) {
				t.Fatalf("incident failures = %+v, want %+v", got, test.want.failed)
			}
		})
	}
}

func TestRunCheckAppendLogStartsAtItsSize(t *testing.T) {
	t.Parallel()
	for _, knownPath := range []bool{false, true} {
		t.Run(map[bool]string{false: "output name", true: "running log"}[knownPath], func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "proof.log")
			if err := os.WriteFile(path, []byte("landing environment earlier\nlanding package old/unit 0 fail 99\nLANDING-CHECKED\t0\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			running := Running{}
			if knownPath {
				running.Log = path
			}
			observed := &proofOutput{output: io.Discard}
			report, err := runCheck(ProveSeams{}, dir, "printf 'landing environment current\nLANDING-CHECKED\\t0\\n'", running, "p/q", scopeDecision{}, output, observed)
			if err != nil || report.kind != "complete" || observed.environment != "current" || len(observed.packages) != 0 {
				t.Fatalf("append log included earlier bytes: report=%+v environment=%q packages=%+v err=%v", report, observed.environment, observed.packages, err)
			}
		})
	}
}

func TestProofOutputReadLogRequiresARegularPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, test := range []struct {
		name, path string
		offset     int64
	}{
		{"missing", filepath.Join(dir, "missing"), 0},
		{"directory", dir, 0},
		{"device", os.DevNull, 0},
		{"unknown offset", filepath.Join(dir, "missing"), -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			observed := &proofOutput{output: io.Discard}
			observed.readLog(test.path, test.offset)
			if observed.environment != "" || observed.report.kind != "" || len(observed.ran) != 0 || len(observed.packages) != 0 {
				t.Fatalf("unreadable log supplied observations: %+v", observed)
			}
		})
	}
}
