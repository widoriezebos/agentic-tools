package launch

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestCheckProcessDistinguishesCancellationAndStartFailure(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, want     string
		before, during bool
		err            error
	}{
		{name: "already cancelled", want: "cancelled", before: true},
		{name: "completion races cancellation", want: "cancelled", during: true},
		{name: "cancelled owner", want: "cancelled", err: context.Canceled},
		{name: "custody start", want: "environment", err: ErrCheckStart},
		{name: "unsettled cancellation", want: "environment", during: true, err: ErrCheckUnsettled},
		{name: "command failure", want: "completed", err: errors.New("command exited")},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if row.before {
				cancel()
			}
			called := false
			clock := CheckContext{After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, RunCommand: func(context.Context, *exec.Cmd) error {
				called = true
				if row.during {
					cancel()
				}
				return row.err
			}}
			why, _ := checkProcess(ctx, &exec.Cmd{}, nil, time.Minute, time.Minute, clock)
			if why != row.want || called == row.before {
				t.Fatalf("reason=%s called=%t", why, called)
			}
		})
	}
}
