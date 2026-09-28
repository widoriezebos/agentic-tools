package delegation

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPollIntervalHonoursOnlyAPositiveOverride(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want time.Duration
	}{{"", 250 * time.Millisecond}, {"20", 20 * time.Millisecond}} {
		s := &session{env: Env{HandshakePollMS: tc.raw}, stderr: &bytes.Buffer{}}
		if got, err := s.pollInterval(250); err != nil || got != tc.want {
			t.Fatalf("pollInterval(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{"0", "-5", "020", "fast"} {
		var stderr bytes.Buffer
		s := &session{env: Env{HandshakePollMS: raw}, stderr: &stderr}
		_, err := s.pollInterval(250)
		if ExitCode(err) != 2 || !strings.Contains(stderr.String(), "poll interval must be a positive integer") {
			t.Fatalf("pollInterval(%q) = %v (stderr %q); want exit 2 with the refusal", raw, err, stderr.String())
		}
	}
}

func TestConfigGetReadsTheRootConfigurationOrItsDefault(t *testing.T) {
	t.Parallel()
	const key = "delegation.test-only-setting"
	conf := func(content string) *session {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return &session{root: root, stderr: &bytes.Buffer{}}
	}
	if got, err := conf("other.setting=1\n").configGet(key, "3"); err != nil || got != "3" {
		t.Fatalf("configGet of an unset key = %q, %v; want the default", got, err)
	}
	if got, err := conf(key+"=5\n").configGet(key, "3"); err != nil || got != "5" {
		t.Fatalf("configGet = %q, %v; want the configured 5", got, err)
	}
	for name, s := range map[string]*session{
		"absent configuration": {root: t.TempDir()},
		"ambiguous key":        conf(key + "=5\n" + key + "=6\n"),
	} {
		var stderr bytes.Buffer
		s.stderr = &stderr
		if _, err := s.configGet(key, "3"); ExitCode(err) != 1 || stderr.Len() == 0 {
			t.Fatalf("%s: configGet = %v (stderr %q); want exit 1 with a diagnostic", name, err, stderr.String())
		}
	}
}

func TestExitCodeOfCarriesAProcessExitAndDefaultsToOne(t *testing.T) {
	t.Parallel()
	if got := exitCodeOf(errors.New("not a process exit")); got != 1 {
		t.Fatalf("exitCodeOf(plain error) = %d, want 1", got)
	}
	err := exec.Command("/bin/sh", "-c", "exit 7").Run()
	if got := exitCodeOf(err); got != 7 {
		t.Fatalf("exitCodeOf(exit 7) = %d (err %v), want 7", got, err)
	}
}
