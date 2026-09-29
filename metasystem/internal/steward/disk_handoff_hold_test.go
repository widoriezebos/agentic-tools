package steward

// U5d, fail-closed rule 1: a handoff the inspection cannot read holds the
// whole class for the pass; nothing is released beside it.

import (
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func TestAnUnreadableHandoffHoldsTheWholeClass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	class := HandoffClass{Root: root, Keep: 14 * 24 * time.Hour,
		Inspect: func(string, time.Time, time.Time) ([]string, []error) {
			return []string{"6900000000000001"}, []error{errors.New("decode handoff state: unexpected end of JSON input")}
		}}
	registry := diskstore.CheckoutRegistry(root)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: root, Registry: registry,
		LockPath: filepath.Join(registry.Dir, ".sweep.flock"), ReportPath: diskstore.CheckoutReportPath(root), Mode: diskstore.ModeApply,
		Now: now, Clock: func() time.Time { return now }, Entropy: rand.Reader, Classes: []diskstore.Class{class}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("nothing is released beside an unreadable handoff: %+v", report.Actions)
	}
	if len(report.Pending) == 0 {
		t.Fatalf("the report says why the class is held: %+v", report)
	}
}
