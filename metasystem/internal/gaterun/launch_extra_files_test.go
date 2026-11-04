package gaterun

import (
	"os"
	"testing"
)

func TestADetachedLaunchPassesItsExtraFiles(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "custody")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	done := make(chan error, 1)
	if _, err := LaunchDetached(DetachedLaunch{Argv: []string{"/bin/sh", "-c", "test -e /dev/fd/3"}, ExtraFiles: []*os.File{file}, Completed: done}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("detached child lost its extra file: %v", err)
	}
}
