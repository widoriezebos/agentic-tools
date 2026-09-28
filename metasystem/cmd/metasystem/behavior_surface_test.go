package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withBehaviorSurfaceStreams(t *testing.T, input string, run func() int) int {
	t.Helper()
	inPath := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(inPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	readOnly, err := os.Open(inPath)
	if err != nil {
		in.Close()
		t.Fatal(err)
	}
	priorIn, priorOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, readOnly
	t.Cleanup(func() {
		os.Stdin, os.Stdout = priorIn, priorOut
		_ = in.Close()
		_ = readOnly.Close()
	})
	return run()
}

func TestBehaviorSurfaceSelectFailsOnFlushError(t *testing.T) {
	code := withBehaviorSurfaceStreams(t, "internal/a.go\n", func() int {
		return runBehaviorSurfaceSelect([]string{"--projection", "LANDING"})
	})
	if code == 0 {
		t.Fatal("select returned success after its buffered output failed")
	}
}

func TestBehaviorSurfaceDirectWritersFailOnOutputError(t *testing.T) {
	root := t.TempDir()
	for name, run := range map[string]func() int{
		"digest": func() int {
			return runBehaviorSurfaceDigest([]string{"--root", root, "--projection", "LANDING", "--endpoint", "test"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if code := withBehaviorSurfaceStreams(t, "", run); code == 0 {
				t.Fatal("command returned success after direct output failed")
			}
		})
	}
}
