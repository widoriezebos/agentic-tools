package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// personTrimBed is a checkout whose settings set the steward's pass budget
// and the person's budget, and a clock each fake pass advances by its
// budget.
type personTrimBed struct {
	top   string
	now   time.Time
	calls []personTrimCall
}

type personTrimCall struct {
	caches []string
	budget time.Duration
}

func newPersonTrimBed(t *testing.T, conf string) *personTrimBed {
	t.Helper()
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "metasystem.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	return &personTrimBed{top: top, now: time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC)}
}

// pass fakes one trim pass: the engine Go cache needs measuring passes
// more than the steward's budget allows before it completes; every other
// cache completes at once.
func (bed *personTrimBed) pass(measuringPasses int) CacheTrimPass {
	return func(_ context.Context, caches []string, _ time.Time, budget time.Duration) ([]gocache.TrimReport, error) {
		bed.calls = append(bed.calls, personTrimCall{caches: caches, budget: budget})
		bed.now = bed.now.Add(budget)
		if len(caches) == 0 {
			caches = gocache.MachineCaches()
		}
		var reports []gocache.TrimReport
		for _, cache := range caches {
			report := gocache.TrimReport{Cache: cache, EndedBy: "complete", Phase: "idle", CapBytes: 30 << 30, BytesAfter: 1 << 30}
			if cache == "engine-go-build" && len(bed.calls) <= measuringPasses {
				report.EndedBy, report.Phase = "budget", "measure"
				report.Checkpoint = gocache.TrimCheckpoint{Shard: "c0", BytesSoFar: int64(len(bed.calls)) * 10 << 30}
			}
			reports = append(reports, report)
		}
		return reports, nil
	}
}

func (bed *personTrimBed) run(t *testing.T, measuringPasses int) ([]gocache.TrimReport, []int) {
	t.Helper()
	var progress []int
	reports, err := TrimMachineCachesForPerson(context.Background(), CacheTrimRun{
		Top: bed.top, Clock: func() time.Time { return bed.now }, Pass: bed.pass(measuringPasses),
	}, func(pass int, _ []gocache.TrimReport) { progress = append(progress, pass) })
	if err != nil {
		t.Fatal(err)
	}
	return reports, progress
}

// A person running disk clean expects it to finish: it runs pass after pass
// of the steward's budget, each continuing only the caches the last one
// left unfinished, until every cache is done, telling each pass.
func TestPersonTrimContinuesUntilEveryCacheIsDone(t *testing.T) {
	t.Parallel()
	bed := newPersonTrimBed(t, "disk.cache-trim-budget-sec=10\ndisk.cache-trim-person-budget-sec=300\n")
	reports, progress := bed.run(t, 3)
	if len(bed.calls) != 4 || !slices.Equal(progress, []int{1, 2, 3, 4}) {
		t.Fatalf("passes = %+v, progress %v; want four", bed.calls, progress)
	}
	if bed.calls[0].caches != nil || !slices.Equal(bed.calls[1].caches, []string{"engine-go-build"}) || bed.calls[0].budget != 10*time.Second {
		t.Fatalf("pass 1 must trim every cache and later passes only the unfinished one, each at the steward's budget: %+v", bed.calls)
	}
	if len(reports) != 4 || reports[0].Cache != "engine-go-build" || reports[0].EndedBy != "complete" {
		t.Fatalf("final reports = %+v", reports)
	}
}

// The person's budget bounds it: when it is spent the last pass's report
// stands, still measuring, and the last pass gets only what is left.
func TestPersonTrimStopsAtThePersonBudget(t *testing.T) {
	t.Parallel()
	bed := newPersonTrimBed(t, "disk.cache-trim-budget-sec=10\ndisk.cache-trim-person-budget-sec=25\n")
	reports, _ := bed.run(t, 100)
	if len(bed.calls) != 3 || bed.calls[2].budget != 5*time.Second {
		t.Fatalf("passes = %+v; want 10s, 10s and the 5s left", bed.calls)
	}
	if reports[0].Cache != "engine-go-build" || reports[0].EndedBy != "budget" || reports[0].Phase != "measure" {
		t.Fatalf("final engine report = %+v", reports[0])
	}
}

// A disk setting the trim cannot read is a SettingsError by type, so disk
// clean tells a person's setting from a failed pass without its words; a
// failed pass is not one.
func TestPersonTrimSettingRefusalIsTyped(t *testing.T) {
	t.Parallel()
	bed := newPersonTrimBed(t, "disk.cache-trim-budget-sec=0\n")
	var setting *CacheSettingsError
	if _, err := TrimMachineCachesForPerson(context.Background(), CacheTrimRun{
		Top: bed.top, Clock: func() time.Time { return bed.now }, Pass: bed.pass(0),
	}, nil); !errors.As(err, &setting) {
		t.Fatalf("an invalid disk setting = %v, want a CacheSettingsError", err)
	}
	good := newPersonTrimBed(t, "disk.cache-trim-budget-sec=10\ndisk.cache-trim-person-budget-sec=300\n")
	failing := func(context.Context, []string, time.Time, time.Duration) ([]gocache.TrimReport, error) {
		return nil, errors.New("disk.cache: the pass failed")
	}
	if _, err := TrimMachineCachesForPerson(context.Background(), CacheTrimRun{
		Top: good.top, Clock: func() time.Time { return good.now }, Pass: failing,
	}, nil); err == nil || errors.As(err, &setting) {
		t.Fatalf("a failed pass = %v, want an error that is no setting refusal", err)
	}
}
