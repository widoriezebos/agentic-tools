package delegation

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A refusal never leaves its caller guessing (Wido, 2026-09-30: "An error
// message that confuses you IS REALLY BAD."): a form the entrypoint does not
// take is answered by name, never by a bare exit status.
func TestARefusedFormIsNamedNeverSilent(t *testing.T) {
	t.Parallel()
	for _, argv := range [][]string{{"__cancel-owned", "u9b-not-a-job-flag"}, {"__handshake-timeout"}, {"__engine-skew-preflight", "a", "b"}} {
		var stderr bytes.Buffer
		life := &Lifecycle{root: t.TempDir()}
		result := life.Run(context.Background(), Request{Stderr: &stderr, Env: Env{DelegateInternal: true}}, argv)
		if result.ExitCode != 2 || !strings.Contains(stderr.String(), argv[0]+" is started by the machinery") ||
			!strings.Contains(stderr.String(), "nothing was done") {
			t.Errorf("%v: code %d stderr %q", argv, result.ExitCode, stderr.String())
		}
	}
}

// A callback's flag error is answered in the public style.
func TestACallbacksUnknownFlagIsNamed(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	s := &session{stderr: &stderr}
	set := s.flags("__record-create")
	set.String("job", "", "job id")
	if set.Parse([]string{"--u9b-bogus"}) == nil {
		t.Fatal("unknown flag parsed")
	}
	if got := stderr.String(); strings.Contains(got, "flag provided but not defined") || strings.Contains(got, "Usage of") ||
		!strings.Contains(got, "metasystem internal delegate __record-create: does not take --u9b-bogus; it takes --job; nothing was done") {
		t.Fatalf("stderr %q", got)
	}
}
