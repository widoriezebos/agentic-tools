package durable_test

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

func TestDarwinKernelSyncRetriesInterruptedSyscall(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "fsync")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	calls := 0
	err = durable.TestKernelSyncWith(f, func(fd int) error {
		calls++
		if fd != int(f.Fd()) {
			t.Fatal("wrong descriptor")
		}
		if calls == 1 {
			return syscall.EINTR
		}
		return syscall.EIO
	})
	var path *os.PathError
	if calls != 2 || !errors.As(err, &path) || path.Op != "sync" || path.Path != f.Name() || !errors.Is(err, syscall.EIO) {
		t.Fatalf("fsync retry/wrapping: calls=%d error=%v", calls, err)
	}
}
