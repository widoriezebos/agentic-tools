package main

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
)

func TestCounselorBriefDryRunRendersEvidenceLimits(t *testing.T) {
	originalBuild := buildCounselorBrief
	originalRender := renderCounselorBrief
	originalResolve := resolveCounselorRoot
	t.Cleanup(func() {
		buildCounselorBrief = originalBuild
		renderCounselorBrief = originalRender
		resolveCounselorRoot = originalResolve
	})
	resolveCounselorRoot = func(explicit string) (string, error) {
		if explicit != "" {
			t.Fatalf("default explicit counselor root = %q, want empty", explicit)
		}
		return "/fixture/metasystem", nil
	}
	buildCounselorBrief = func(options counselor.Options) counselor.Brief {
		if options.Root != "/fixture/metasystem" {
			t.Fatalf("brief root = %q, want resolved metasystem checkout", options.Root)
		}
		return counselor.Compute(counselor.RecordSet{}, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	}

	output, code := captureStdout(t, func() int {
		return dispatch([]string{"counselor", "brief", "--dry-run"})
	})
	if code != 0 {
		t.Fatalf("dry run exit = %d, output=%q", code, output)
	}
	for _, expected := range []string{"Cost:", "Limitation — Risk-class evidence:", "Limitation — Path classification:", "Limitation — Register-edit classification:", "Accepted risk and near-miss register reads records/counselor/accepted-risk-register.jsonl"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dry run omitted %q:\n%s", expected, output)
		}
	}

	if code := runCounselorBrief(nil); code != 2 {
		t.Fatalf("missing --dry-run exit = %d, want usage exit 2", code)
	}
	if code := runCounselorBrief([]string{"--dry-run", "extra"}); code != 2 {
		t.Fatalf("positional argument exit = %d, want usage exit 2", code)
	}
	if code := runCounselorBrief([]string{"--unknown"}); code != 2 {
		t.Fatalf("unknown flag exit = %d, want usage exit 2", code)
	}

	renderCounselorBrief = func(io.Writer, counselor.Brief) error { return errors.New("closed output") }
	if code := runCounselorBrief([]string{"--dry-run"}); code != 1 {
		t.Fatalf("output failure exit = %d, want 1", code)
	}
}

func TestCounselorBriefAcceptsExplicitMetasystemRoot(t *testing.T) {
	originalBuild := buildCounselorBrief
	originalRender := renderCounselorBrief
	originalResolve := resolveCounselorRoot
	t.Cleanup(func() {
		buildCounselorBrief = originalBuild
		renderCounselorBrief = originalRender
		resolveCounselorRoot = originalResolve
	})
	resolveCounselorRoot = func(explicit string) (string, error) {
		if explicit != "/explicit/metasystem" {
			t.Fatalf("explicit counselor root = %q, want /explicit/metasystem", explicit)
		}
		return explicit, nil
	}
	buildCounselorBrief = func(options counselor.Options) counselor.Brief {
		if options.Root != "/explicit/metasystem" {
			t.Fatalf("brief root = %q, want explicit metasystem checkout", options.Root)
		}
		return counselor.Brief{}
	}
	renderCounselorBrief = func(writer io.Writer, _ counselor.Brief) error {
		_, err := writer.Write([]byte("explicit root ok\n"))
		return err
	}

	output, code := captureStdout(t, func() int {
		return dispatch([]string{"counselor", "brief", "--dry-run", "--metasystem-root", "/explicit/metasystem"})
	})
	if code != 0 || output != "explicit root ok\n" {
		t.Fatalf("explicit root dry run: code=%d output=%q", code, output)
	}
}

func TestCounselorBriefReportsRootResolutionFailure(t *testing.T) {
	originalResolve := resolveCounselorRoot
	t.Cleanup(func() { resolveCounselorRoot = originalResolve })
	resolveCounselorRoot = upMetasystemRoot
	errOut, code := captureStderr(t, func() int {
		return runCounselorBrief([]string{"--dry-run"})
	})
	if code != 1 || !strings.Contains(errOut, "counselor brief root: cannot derive the metasystem root") ||
		!strings.Contains(errOut, "--metasystem-root") {
		t.Fatalf("root resolution failure: code=%d stderr=%q", code, errOut)
	}
}
