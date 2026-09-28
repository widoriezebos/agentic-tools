package supervisor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestEngineDispatcherRunsTheDelegateEntry: the production dispatcher runs
// each lifecycle callback as `ENGINE internal delegate ARGS` under the
// installation's METASYSTEM_DELEGATE_ROOT (the retired runtime-common.sh
// delegate_callback, now that dispatch.sh is gone), and relays the
// callback's output and exit status; the self-test's status read answers the
// callback's stdout line and an empty string on a refusal.
func TestEngineDispatcherRunsTheDelegateEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "install")
	log := filepath.Join(dir, "delegate.log")
	engine := filepath.Join(dir, "engine")
	if err := testexec.WriteFile(engine, []byte(`#!/bin/sh
printf '%s|root=%s\n' "$*" "${METASYSTEM_DELEGATE_ROOT-}" >>"`+log+`"
case "$3" in
  status) echo running ;;
  __record-cas) exit 3 ;;
esac
`), 0o755); err != nil {
		t.Fatal(err)
	}
	dispatcher := EngineDispatcher{Root: root, Engine: engine, Environ: []string{"PATH=" + os.Getenv("PATH")}}
	var stdout bytes.Buffer
	if code := dispatcher.Run(&stdout, &stdout, "__record-cas", "--job", "j1"); code != 3 {
		t.Fatalf("callback exit = %d, want the callback's own 3", code)
	}
	d := Deps{Dispatch: dispatcher}
	if got := d.lifecycleStatus("j1"); got != "running" {
		t.Fatalf("lifecycle status = %q, want running", got)
	}
	d.lifecycleReap("j1")
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal delegate __record-cas --job j1|root=" + root,
		"internal delegate status --job j1|root=" + root,
		"internal delegate reap --job j1|root=" + root,
	}
	if got := strings.Split(strings.TrimSpace(string(data)), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("engine argv =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	refusing := Deps{Dispatch: EngineDispatcher{Root: root, Engine: filepath.Join(dir, "missing-engine")}}
	if got := refusing.lifecycleStatus("j1"); got != "" {
		t.Fatalf("status through a failing entry = %q, want empty", got)
	}
}
