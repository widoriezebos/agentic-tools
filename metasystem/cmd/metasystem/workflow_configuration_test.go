package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The refactor gate's cadence resolves from the installation's
// metasystem.conf, and a flag outranks it (ported from the retired
// validate-metasystem.sh workflow-tooling section; the commit backstop itself
// is TestRefactorBaselineCheckRefusals).
func TestRefactorCadenceResolvesConfigurationThenFlag(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("refactor.max-age-minutes=1440\nrefactor.max-commits=0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var fromConf validate.RefactorBaselineParams
	if code := resolveRefactorCadence(&fromConf, map[string]bool{}, "", "", conf, io.Discard); code != 0 || fromConf.MaxCommits != 0 || fromConf.MaxAgeMinutes != 1440 {
		t.Fatalf("refactor cadence ignored metasystem.conf: code=%d %+v", code, fromConf)
	}
	var fromFlag validate.RefactorBaselineParams
	if code := resolveRefactorCadence(&fromFlag, map[string]bool{"max-commits": true}, "", "2", conf, io.Discard); code != 0 || fromFlag.MaxCommits != 2 {
		t.Fatalf("refactor cadence did not prefer the flag: code=%d %+v", code, fromFlag)
	}
	if code := resolveRefactorCadence(&validate.RefactorBaselineParams{}, map[string]bool{"max-commits": true}, "", "many", conf, io.Discard); code != 2 {
		t.Fatalf("a non-numeric cadence was accepted: code=%d", code)
	}
}

// The job watcher arms with the installation's configured stale and cap
// ceilings (ported from the retired validate-metasystem.sh workflow-tooling
// section).
func TestWatchJobsArmsWithConfiguredCeilings(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("watch.stale-min=7\nwatch.cap-min=33\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(installation, "jobs")
	state := filepath.Join(installation, "state")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := runReportWatchJobsTo([]string{"--root", installation, "--dir", jobs, "--state", state, "--once"}, &stdout, &stderr)
	out := stdout.String()
	if code != 0 || !strings.Contains(out, "stale=7m cap=33m") {
		t.Fatalf("watcher ignored metasystem.conf ceilings: code=%d\n%s", code, out)
	}
}
