package testexec

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteFileLeavesNoWriterForAFork(t *testing.T) {
	t.Parallel()

	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = exec.Command(truePath).Run()
				}
			}
		}()
	}
	defer func() {
		close(stop)
		workers.Wait()
	}()

	directory := t.TempDir()
	for iteration := range 200 {
		path := filepath.Join(directory, fmt.Sprintf("script-%03d.sh", iteration))
		if err := WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("iteration %d write: %v", iteration, err)
		}
		if err := exec.Command(path).Run(); err != nil {
			t.Fatalf("iteration %d execute: %v", iteration, err)
		}
	}
}
