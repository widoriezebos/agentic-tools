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
	resolveCounselorRoot = func() (string, error) { return "/fixture/metasystem", nil }
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
	for _, expected := range []string{"Cost:", "Limitation — Risk-class evidence:", "Limitation — Path classification:", "Limitation — Register-edit classification:"} {
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

func TestCounselorBriefReportsRootResolutionFailure(t *testing.T) {
	originalResolve := resolveCounselorRoot
	t.Cleanup(func() { resolveCounselorRoot = originalResolve })
	resolveCounselorRoot = func() (string, error) { return "", errors.New("missing installation root") }
	if code := runCounselorBrief([]string{"--dry-run"}); code != 1 {
		t.Fatalf("root resolution failure exit = %d, want 1", code)
	}
}
