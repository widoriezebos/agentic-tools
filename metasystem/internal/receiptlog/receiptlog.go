// Package receiptlog is the one writer that appends to a task-receipt
// ledger (memory/receipts.log). Every engine append holds the host's
// evidence bound lock shared around its write (design
// engine-owns-disk-lifetimes Part B 3.12 clause 2, DL4D-03): the bound
// judges an item's "named by a receipt no retro covered" exclusion under
// the lock held exclusively, so an engine line is either seen by the
// judgement or waits for the one item in flight. A line that reaches the
// ledger by git or by hand holds no lock; the bound re-hashes the ledger at
// its commit point instead.
package receiptlog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Options shape one append.
type Options struct {
	// Create creates the ledger when it is missing.
	Create bool
	// Sync is the durability barrier; nil is the file's Sync.
	Sync func(*os.File) error
}

// AppendLine appends one line (which carries its own newline) to the ledger
// at path. With Sync set, durability is claimed only after it succeeds.
func AppendLine(path, line string, options Options) error {
	release := holdShared(path)
	defer release()
	flags := os.O_APPEND | os.O_WRONLY
	if options.Create {
		flags |= os.O_CREATE
	}
	handle, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(line); err != nil {
		handle.Close()
		return err
	}
	if options.Sync != nil {
		if err := options.Sync(handle); err != nil {
			handle.Close()
			return fmt.Errorf("receipt append: not durably written: %w", err)
		}
	}
	return handle.Close()
}

// holdShared takes the bound lock shared for a ledger at
// <installation>/memory/receipts.log; nothing is taken (never refused)
// when no evidence root resolves for that installation or the host's
// state root cannot be named.
func holdShared(ledger string) func() {
	installation := filepath.Dir(filepath.Dir(ledger))
	if _, err := config.ResolveEvidenceRoot(config.EvidenceRootParams{ConfPath: filepath.Join(installation, "metasystem.conf")}); err != nil {
		return func() {}
	}
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return func() {}
	}
	lock, err := diskstore.BoundShared(diskstore.BoundLockPath(filepath.Dir(registryPath)))
	if err != nil {
		return func() {}
	}
	return lock.Release
}

// HoldBoundShared is holdShared for a ledger write that is not an append
// (goal reopen and carry publish the goal ledger): the lock is held until
// release is called; release is never nil. installation is the checkout's
// installation.
func HoldBoundShared(installation string) (release func()) {
	return holdShared(filepath.Join(installation, "memory", "receipts.log"))
}
