package proofrun

import (
	"context"
	"errors"
	"testing"
)

func checkHostCapacityWaitObservesCallerFence(t *testing.T, ctx context.Context) {
	t.Run("held_slot", func(t *testing.T) {
		directory, conf := isolatedHostResources(t)
		holder, err := AcquireHostResources(ctx, directory, conf, "heavy", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		closed := errors.New("caller fence closed")
		reads := 0
		contender, err := AcquireHostResourcesWithWaitCheck(ctx, directory, conf, "heavy", nil, func() error {
			reads++
			if reads > 2 {
				return closed
			}
			return nil
		})
		if contender != nil || !errors.Is(err, closed) || reads != 3 {
			t.Fatalf("queued admission = lease %v, error %v, fence reads %d", contender, err, reads)
		}
		active, err := activeHostResourceSlots(directory)
		if err != nil || active != 1 {
			t.Fatalf("closed contender changed capacity: active %d, error %v", active, err)
		}
		if err := MarkHostResourcesClean(holder.Files()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("held_guard", func(t *testing.T) {
		directory, conf := isolatedHostResources(t)
		guard, held, err := tryHostFile(directory + "/admission.lock")
		if err != nil || !held {
			t.Fatalf("hold admission guard: %v (%t)", err, held)
		}
		defer guard.Close()
		closed := errors.New("caller fence closed")
		reads := 0
		lease, err := AcquireHostResourcesWithWaitCheck(ctx, directory, conf, "heavy", nil, func() error {
			reads++
			if reads > 2 {
				return closed
			}
			return nil
		})
		if lease != nil || !errors.Is(err, closed) || reads != 3 {
			t.Fatalf("guard wait = lease %v, error %v, fence reads %d", lease, err, reads)
		}
	})
}
