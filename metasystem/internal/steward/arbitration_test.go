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
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("releasing must admit the waiter")
	}
}
