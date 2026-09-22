package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type freezeOverlapWriter struct {
	bytes.Buffer
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newFreezeOverlapWriter() *freezeOverlapWriter {
	return &freezeOverlapWriter{started: make(chan struct{}), release: make(chan struct{})}
}

func (writer *freezeOverlapWriter) Write(data []byte) (int, error) {
	writer.once.Do(func() {
		close(writer.started)
		<-writer.release
	})
	return writer.Buffer.Write(data)
}

func TestGateWitnessFreezeReturnsAndCleansExactSnapshot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "standalone")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "source.go"), []byte("package internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runGateWitnessFreezeWithWriters([]string{"--root", root}, &stdout, &stderr)
	fields := strings.Split(strings.TrimSpace(stdout.String()), "\t")
	if code != 0 || stderr.Len() != 0 || len(fields) != 3 || fields[1] != fields[2] {
		t.Fatalf("freeze CLI code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	owner := filepath.Dir(fields[2])
	if _, err := os.Stat(fields[1]); err != nil {
		t.Fatalf("freeze CLI did not publish execution root: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runGateWitnessFreezeWithWriters([]string{"--cleanup", fields[2]}, &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("freeze cleanup CLI code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(owner); !os.IsNotExist(err) {
		t.Fatalf("freeze cleanup left owned root %s: %v", owner, err)
	}
}
