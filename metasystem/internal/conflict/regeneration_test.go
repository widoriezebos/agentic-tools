package conflict

import (
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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

func TestInstallationRegenerationKeepsLiteralArgumentsAndCancellation(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	bin := filepath.Join(install, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "metasystem")
	if err := testexec.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s' \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.CreateTemp(t.TempDir(), "regeneration.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	literal := "literal; $(no shell)"
	cwd := t.TempDir()
	started := false
	run := RegenerationInInstallation(install)
	if err := run([]string{"metasystem", literal}, cwd, log, func(pid int64) error { started = pid > 0; return nil }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log.Name())
	if err != nil || !started || string(data) != literal {
		t.Fatalf("literal output: %s %v started %v", data, err, started)
	}
	refused := errors.New("ownership publication refused")
	if err := run([]string{"metasystem", literal}, cwd, log, func(int64) error { return refused }); !errors.Is(err, refused) {
		t.Fatalf("cancellation: %v", err)
	}
	for _, missing := range []bool{false, true} {
		if missing {
			err = os.Remove(binary)
		} else {
			err = os.Chmod(binary, 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"/bin/sh", "-c", "metasystem forbidden"}, cwd, log, func(int64) error { t.Fatal("invalid installation launched a child"); return nil }); err == nil {
			t.Fatal("invalid installation was accepted")
		}
	}
}
