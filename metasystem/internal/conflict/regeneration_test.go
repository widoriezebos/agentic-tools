package conflict

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRegenerationArgvUsesLiteralArgumentsAndReportsStartFailure(t *testing.T) {
	t.Parallel()
	log, err := os.CreateTemp(t.TempDir(), "argv.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	literal := "$(no shell); 'literal'"
	started := false
	err = RunRegenerationArgv([]string{"/usr/bin/printf", "%s", literal}, filepath.Dir(log.Name()), log, func(pid int64) error { started = pid > 0; return nil })
	data, readErr := os.ReadFile(log.Name())
	if err != nil || readErr != nil || !started || string(data) != literal {
		t.Fatalf("argv output=%q started=%v err=%v read=%v", data, started, err, readErr)
	}
	refused := errors.New("cannot publish running command")
	if err := RunRegenerationArgv([]string{"/usr/bin/printf", "unused"}, filepath.Dir(log.Name()), log, func(int64) error { return refused }); !errors.Is(err, refused) {
		t.Fatalf("start publication error=%v", err)
	}
	if err := RunRegenerationArgv([]string{"/does/not/exist"}, filepath.Dir(log.Name()), log, func(int64) error { t.Fatal("missing command published"); return nil }); err == nil {
		t.Fatalf("missing command err=%v", err)
	}
}
