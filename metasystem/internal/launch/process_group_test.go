package launch

import (
	"errors"
	"syscall"
	"testing"
)

func TestGroupAliveReadsEPERMAgainstTheKernelList(t *testing.T) {
	t.Parallel()
	listErr := errors.New("kernel list unavailable")
	for _, test := range []struct {
		name      string
		signalErr error
		listed    bool
		listErr   error
		wantAlive bool
		wantErr   error
		wantReads int
	}{
		{name: "signal succeeds", wantAlive: true},
		{name: "no group", signalErr: syscall.ESRCH},
		{name: "EPERM with a member", signalErr: syscall.EPERM, listed: true, wantAlive: true, wantReads: 1},
		{name: "EPERM without members", signalErr: syscall.EPERM, wantReads: 1},
		{name: "EPERM with unreadable list", signalErr: syscall.EPERM, listErr: listErr, wantAlive: true, wantReads: 1},
		{name: "unexpected signal error", signalErr: syscall.EINVAL, wantErr: syscall.EINVAL},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			const pgid int64 = 1234
			signals, reads := 0, 0
			alive, err := groupAlive(pgid, func(pid int, signal syscall.Signal) error {
				signals++
				if pid != -int(pgid) || signal != 0 {
					t.Errorf("signal arguments = (%d, %d), want (%d, 0)", pid, signal, -pgid)
				}
				return test.signalErr
			}, func(got int64) (bool, error) {
				reads++
				if got != pgid {
					t.Errorf("list group = %d, want %d", got, pgid)
				}
				return test.listed, test.listErr
			})
			if alive != test.wantAlive || !errors.Is(err, test.wantErr) {
				t.Errorf("group alive = (%v, %v), want (%v, %v)", alive, err, test.wantAlive, test.wantErr)
			}
			if signals != 1 || reads != test.wantReads {
				t.Errorf("signal calls = %d, list reads = %d; want 1, %d", signals, reads, test.wantReads)
			}
		})
	}
}
