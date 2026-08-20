package steward

import (
	"testing"
	"time"
)

func TestArbitrationAdmitsOneContenderAtATime(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	go func() {
		second, err := AcquireArbitration(root)
		if err == nil {
			close(entered)
			second.Release()
		}
	}()
	select {
	case <-entered:
		t.Fatal("the second contender must block while the first holds")
	case <-time.After(150 * time.Millisecond):
	}
	first.Release()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("releasing must admit the waiter")
	}
}
