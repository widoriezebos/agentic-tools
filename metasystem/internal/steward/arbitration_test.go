package steward

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArbitrationMissingCheckoutCreatesNothing(t *testing.T) {
	t.Parallel()
	for name, acquire := range map[string]func(string) (*ArbitrationLock, error){
		"blocking": AcquireArbitration, "nonblocking": TryAcquireArbitration,
		"probe": func(root string) (*ArbitrationLock, error) {
			free, err := ProbeArbitration(root)
			if free {
				t.Error("a missing checkout must not read as free")
			}
			return nil, err
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "gone", "metasystem")
			held, err := acquire(root)
			if held != nil {
				held.Release()
			}
			if !errors.Is(err, ErrCheckoutGone) {
				t.Errorf("missing checkout error = %v; want ErrCheckoutGone", err)
			}
			if _, err := os.Stat(filepath.Dir(root)); !os.IsNotExist(err) {
				t.Fatalf("missing checkout was recreated: %v", err)
			}
		})
	}
}

func TestArbitrationExistingCheckoutCreatesLockDirectories(t *testing.T) {
	t.Parallel()
	for name, acquire := range map[string]func(string) (*ArbitrationLock, error){"blocking": AcquireArbitration, "nonblocking": TryAcquireArbitration} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			held, err := acquire(root)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			if _, err := os.Stat(ArbitrationLockPath(root)); err != nil {
				t.Fatal(err)
			}
			if free, err := ProbeArbitration(root); err != nil || free {
				t.Fatalf("held lock: free=%v err=%v", free, err)
			}
		})
	}
}

func TestArbitrationAdmitsOneContenderAtATime(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	secondAtLock := make(chan struct{})
	priorBeforeWait := beforeArbitrationWait
	beforeArbitrationWait = func() { secondAtLock <- struct{}{} }
	t.Cleanup(func() { beforeArbitrationWait = priorBeforeWait })
	entered := make(chan struct{})
	go func() {
		second, err := AcquireArbitration(root)
		if err == nil {
			close(entered)
			second.Release()
		}
	}()
	<-secondAtLock
	select {
	case <-entered:
		t.Fatal("the second contender must block while the first holds")
	default:
	}
	first.Release()
	<-entered
}
