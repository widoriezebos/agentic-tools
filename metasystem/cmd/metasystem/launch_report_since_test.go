package main

import (
	"os"
	"strings"
	"testing"
)

func TestLaunchReportRejectsMalformedSince(t *testing.T) {
	t.Parallel()
	code, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runLaunchReport([]string{"--since", "yesterday"})
	})
	if code != 2 || !strings.Contains(stderr, "invalid --since") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestRetroSkillCollectsLaunchEvidence(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../skills/retro/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(data)
	for _, want := range []string{
		"launch report --since",
		"builds over the cap",
		"compactions per build job",
		"compacted reads",
		"calls above 200K context",
	} {
		if !strings.Contains(contents, want) {
			t.Errorf("retro skill does not name %q", want)
		}
	}
}
