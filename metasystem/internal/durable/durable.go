// Package durable owns the durability level of file and directory syncs.
//
// Every process starts at level full: Sync is File.Sync, which on macOS asks
// the drive to flush its cache. Only a Go test process can lower its own level
// to kernel (plain fsync(2) on macOS, File.Sync elsewhere), and only by the
// explicit call UseKernelInThisTestProcess, which testenv.MainWithSetup makes
// after it prepared the test environment. The level freezes at the first sync
// or the opt-in, whichever comes first, so a process that synced before the
// opt-in stays full. No path, environment variable or other ambient input
// changes the level, and this package imports nothing of the test harness.
//
// Limitation of this unit: there is no way yet to ask for full in a test run.
// A test process under testenv.Main or MainWithSetup is always kernel (unless
// it synced before the opt-in); every other process is always full. Every
// file and directory sync of the engine goes through this package; an audit in
// internal/testenv refuses a call of (*os.File).Sync anywhere else in the
// module's non-test code.
package durable

import (
	"errors"
	"os"
	"sync"
	"testing"
)

const (
	levelFull   = "full"
	levelKernel = "kernel"
)

type policy struct {
	sync.Mutex
	frozen, opted bool
	level         string
}

var process policy

// useKernel freezes the level once. A process that already synced stays full.
func (p *policy) useKernel(isTest bool) error {
	p.Lock()
	defer p.Unlock()
	if !isTest {
		return errors.New("durability opt-in requires a Go test process")
	}
	if p.opted {
		return errors.New("durability opt-in already called")
	}
	p.opted = true
	if !p.frozen {
		p.frozen = true
		p.level = levelKernel
	}
	return nil
}

// current returns the level; with freeze, an unfrozen process freezes at full.
func (p *policy) current(freeze bool) string {
	p.Lock()
	defer p.Unlock()
	if !p.frozen {
		if !freeze {
			return levelFull
		}
		p.frozen = true
		p.level = levelFull
	}
	return p.level
}

// UseKernelInThisTestProcess lowers this test process's level to kernel, once.
// It refuses outside a Go test binary and when called a second time; a sync
// before the call has already frozen the process at full, and then the call
// changes nothing.
func UseKernelInThisTestProcess() error {
	return process.useKernel(testing.Testing())
}

// Current reports the effective level ("full" or "kernel") without freezing
// an unused adapter.
func Current() string { return process.current(false) }

// Sync freezes the selected level before synchronizing f.
func Sync(f *os.File) error {
	return syncAt(process.current(true), f, (*os.File).Sync, kernelSync)
}

func syncAt(level string, f *os.File, full, kernel func(*os.File) error) error {
	if level == levelKernel {
		return kernel(f)
	}
	return full(f)
}

// SyncDir synchronizes a directory and returns its sync or close error.
func SyncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := Sync(dir); err != nil {
		_ = dir.Close()
		return err
	}
	return dir.Close()
}
