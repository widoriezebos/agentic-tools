package run

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestWaitSurvivesDarwinBootTimeAdjustment(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)
	boot := time.Minute
	bootReads := int64(0)
	reads := 0
	options := WaitOptions{
		Now: func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) {
			boot += time.Second
			bootReads++
			id, err := identity.DarwinBootIdentity(identity.DarwinBootReadings{
				SessionUUID:  testDarwinWaitSessionUUID,
				BootTimeSec:  1788592681,
				BootTimeUsec: 131526 - bootReads,
				Now:          now,
				Elapsed:      boot,
			})
			return id, boot, err
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Sleep: func(_ context.Context, duration time.Duration) error {
			now = now.Add(duration)
			boot += duration
			return nil
		},
		Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			reads++
			return SourceObservation{
				Pending: reads <= 3,
				Incarnation: WaiterTarget{
					Generation:  1,
					LaunchNonce: "n",
				},
				ExitCode: ExitGreen,
				Outcome:  "green",
				Evidence: "run:pending:g1",
			}, nil
		},
	}
	result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{
		Selector: WaitSelector{
			Kind:     "run",
			TargetID: "darwin-clock-adjustment",
		},
		Owner:          mainCaller,
		RuntimeSession: "session-a",
		Timeout:        time.Minute,
	}, options)
	row, _, err := FindWaiterByID(root, result.WaitID)
	if err != nil || reads != 4 || result.ExitCode != ExitGreen {
		t.Fatalf("result=%+v row=%+v reads=%d err=%v", result, row, reads, err)
	}
	if row.RegisteredBootID != testDarwinWaitSessionUUID || row.LastObservedBootID != row.RegisteredBootID {
		t.Fatalf("registered identity %q and observed identity %q differ", row.RegisteredBootID, row.LastObservedBootID)
	}
}

const testDarwinWaitSessionUUID = "16CD714C-2D75-4544-8B28-69576F9570B9"
