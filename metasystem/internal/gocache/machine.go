package gocache

import (
	"context"
	"errors"
	"time"
)

// MachineTrim is one steward pass over the four machine caches.
type MachineTrim struct {
	// UserCacheDir is the directory the caches live under; nil is
	// os.UserCacheDir. The caches are never taken from GOCACHE.
	UserCacheDir func() (string, error)
	// StateDir holds each cache's flock, report and candidates.
	StateDir string
	// The hard caps, in bytes.
	EngineGoCapBytes, DelegateGoCapBytes, StaticcheckCapBytes int64
	// Keep is the window inside which nothing is deleted.
	Keep time.Duration
	// Now is the pass's time; Clock reads time for the reports (nil is Now).
	Now   time.Time
	Clock func() time.Time
	// Stopped is asked between batches.
	Stopped func() bool
}

// TrimMachine trims the engine's Go and staticcheck caches, then the
// delegates', each to its cap, under the one context: its deadline is the
// pass's budget, shared by the four caches in that order.
func TrimMachine(ctx context.Context, machine MachineTrim) ([]TrimReport, error) {
	engine, engineErr := DomainPathsUsing(DomainEngine, machine.UserCacheDir)
	delegate, delegateErr := DomainPathsUsing(DomainDelegate, machine.UserCacheDir)
	if err := errors.Join(engineErr, delegateErr); err != nil {
		return nil, err
	}
	var reports []TrimReport
	for _, cache := range []struct {
		name string
		root string
		cap  int64
	}{
		{"engine-go-build", engine.GoCache, machine.EngineGoCapBytes},
		{"engine-staticcheck", engine.StaticcheckCache, machine.StaticcheckCapBytes},
		{"delegate-go-build", delegate.GoCache, machine.DelegateGoCapBytes},
		{"delegate-staticcheck", delegate.StaticcheckCache, machine.StaticcheckCapBytes},
	} {
		report, err := Trim(ctx, TrimConfig{Name: cache.name, Root: cache.root, CapBytes: cache.cap, Keep: machine.Keep,
			StateDir: machine.StateDir, Now: machine.Now, Clock: machine.Clock, Stopped: machine.Stopped})
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
