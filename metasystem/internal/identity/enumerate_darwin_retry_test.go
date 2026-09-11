package identity

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

func TestAllPidsRetriesAMomentaryENOMEM(t *testing.T) {
	original := sysctlRaw
	t.Cleanup(func() { sysctlRaw = original })
	calls := 0
	sysctlRaw = func(name string, args ...int) ([]byte, error) {
		calls++
		if calls < 3 {
			return nil, unix.ENOMEM
		}
		return original(name, args...)
	}
	pids, err := AllPids()
	if err != nil || len(pids) == 0 || calls != 3 {
		t.Fatalf("a momentary ENOMEM was not retried: calls=%d pids=%d err=%v", calls, len(pids), err)
	}
	calls = 0
	sysctlRaw = func(string, ...int) ([]byte, error) { calls++; return nil, errors.New("permanent") }
	if _, err := AllPids(); err == nil || calls != 1 {
		t.Fatalf("a permanent failure must not be retried: calls=%d err=%v", calls, err)
	}
}
