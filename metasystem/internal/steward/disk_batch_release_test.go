package steward

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// Round D3 N6: the checkout pass retries its seat's batch members'
// unfinished release sets in every landing lane: a report pass lists and
// runs nothing, an apply pass runs each batch and says what is left, and an
// unresolvable lane holds the class.
func TestCheckoutPassRetriesBatchMembersReleaseSets(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	unfinished := map[string]bool{"b1": true}
	var retried []string
	class := BatchReleaseSets{Seat: "/seat", Lanes: []string{"/lane"}, Retry: BatchReleaseRetry{
		Unfinished: func(lane, seat string) ([]string, error) {
			if lane != "/lane" || seat != "/seat" {
				t.Fatalf("asked lane %s seat %s", lane, seat)
			}
			var ids []string
			for id := range unfinished {
				ids = append(ids, id)
			}
			return ids, nil
		},
		Retry: func(_ context.Context, lane, seat, id string, _ time.Time) error {
			retried = append(retried, id)
			delete(unfinished, id)
			return nil
		}}}
	registry := diskstore.CheckoutRegistry(t.TempDir())
	pass := func(mode diskstore.Mode, class diskstore.Class) diskstore.Report {
		report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: "seat", Registry: registry, Mode: mode,
			Now: now, Clock: func() time.Time { return now }, Classes: []diskstore.Class{class}})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := pass(diskstore.ModeReport, class); len(report.Planned) != 1 || len(retried) != 0 {
		t.Fatalf("a report pass = %+v, retried %v", report, retried)
	}
	if report := pass(diskstore.ModeApply, class); len(report.Actions) != 1 || len(retried) != 1 || retried[0] != "b1" {
		t.Fatalf("an apply pass = %+v, retried %v", report, retried)
	}
	if report := pass(diskstore.ModeApply, class); len(report.Actions) != 0 || len(retried) != 1 {
		t.Fatalf("a finished set is not visited again: %+v", report)
	}
	class.LaneErr = context.Canceled
	if report := pass(diskstore.ModeApply, class); len(report.Pending) != 1 {
		t.Fatalf("an unresolvable lane holds the class: %+v", report)
	}
}
