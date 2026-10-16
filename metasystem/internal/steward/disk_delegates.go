package steward

// Delegate workspaces in the checkout pass (engine-owns-disk-lifetimes Part
// B, 3.1's delegate row, 3.10, U5f): the job worktrees dispatch registers
// at creation are released by the delegate proof; the ones made before
// registration existed are adopted here at apply, never by a preview.
// Confirmed records-gone worktrees are absorbed and kept for a person's
// guarded discard through disk clean.

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// DelegateWorktrees recognizes the unregistered job worktrees of a
// checkout.
type DelegateWorktrees struct {
	Top string
}

func (DelegateWorktrees) Name() string { return "unregistered job worktrees" }

func (c DelegateWorktrees) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	root := filepath.Join(c.Top, "artifacts", "agents", "worktrees")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	registry := diskstore.CheckoutRegistry(c.Top)
	records, unreadable := registry.Inventory()
	if len(unreadable) > 0 {
		return []diskstore.Item{{Class: c.Name(), Key: "~registry", Path: registry.Dir, Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the store registry cannot be read (" + unreadable[0].Reason + "); nothing is adopted this pass", Command: "metasystem disk show"}}}, nil
	}
	registered := map[[2]uint64]bool{}
	for _, record := range records {
		if record.State == diskstore.StateReleased {
			continue
		}
		if device, inode, ok := pathID(record.Path); ok {
			registered[[2]uint64{device, inode}] = true
		}
	}
	var items []diskstore.Item
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		if device, inode, ok := pathID(path); ok && registered[[2]uint64{device, inode}] {
			continue
		}
		item := diskstore.Item{Class: c.Name(), Key: entry.Name(), Path: path}
		gone, recordErr := dispatch.DelegateRecordsGone(c.Top, entry.Name(), path)
		if recordErr != nil {
			item.Verdict = diskstore.Verdict{Decision: diskstore.Pending, Absorbable: errors.Is(recordErr, dispatch.ErrDispatchLockReclaim), Reason: recordErr.Error(), Command: "metasystem disk show"}
			items = append(items, item)
			continue
		}
		if gone {
			proof, err := DelegateProof(c.Top, pass.Now)
			identity, identityErr := diskstore.ReadGitIdentity(path)
			if err != nil || identityErr != nil {
				item.Verdict = diskstore.Verdict{Decision: diskstore.Pending, Reason: "its identity or delegate proof cannot be read: " + errors.Join(err, identityErr).Error(), Command: "metasystem disk show"}
			} else {
				item.Verdict = proof.Observe(ctx, diskstore.Record{Path: path, Class: diskstore.DelegateClass, Layout: diskstore.LayoutCopy,
					Identity: identity, Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: entry.Name()}})
			}
			items = append(items, item)
			continue
		}
		if _, err := diskstore.ReadGitIdentity(path); err != nil {
			item.Verdict = diskstore.Verdict{Decision: diskstore.Pending, Reason: "its identity cannot be read (" + err.Error() + "); it is not adopted", Command: "metasystem disk show"}
			items = append(items, item)
			continue
		}
		item.Verdict = diskstore.Verdict{Decision: diskstore.Release, Reason: "adopt as chain " + entry.Name() + "'s delegate workspace"}
		items = append(items, item)
	}
	return items, nil
}

// Apply writes the adopted record, accepted with the worktree's identity;
// a path registered meanwhile is left to its record (Register answers it).
func (c DelegateWorktrees) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	registry := diskstore.CheckoutRegistry(c.Top)
	if item.Verdict.Absorbable {
		proof, err := DelegateProof(c.Top, pass.Now)
		if err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
		}
		record, err := diskstore.AdmitDelegate(ctx, registry, c.Top, proof.GitRoot, item.Path, proof.Git, pass.Now)
		if err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
		}
		stores := diskstore.RegisteredStores{Registry: registry, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: proof}}
		item.Record = record.ID
		return stores.Apply(ctx, pass, item)
	}
	identity, err := diskstore.ReadGitIdentity(item.Path)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "its identity cannot be read: " + err.Error(), Command: "metasystem disk show"}
	}
	record, err := registry.Register(diskstore.Registration{Path: item.Path, Git: true, Class: diskstore.DelegateClass,
		Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: item.Key}, Checkout: c.Top, Lifetime: diskstore.LifetimeOwner,
		CapBytes: 8 << 30, CapKind: diskstore.CapTarget, Layout: diskstore.LayoutCopy, Adopted: true}, pass.Now, rand.Reader)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "it could not be adopted: " + err.Error(), Command: "metasystem disk show"}
	}
	if record.State == diskstore.StateReserved {
		if _, err := registry.Transition(record.ID, []diskstore.State{diskstore.StateReserved}, diskstore.StateAccepted,
			func(record *diskstore.Record) { record.Identity = identity }); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "its adoption could not be accepted: " + err.Error(), Command: "metasystem disk show"}
		}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "adopted as chain " + item.Key + "'s delegate workspace"}
}

func pathID(path string) (uint64, uint64, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, false
	}
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(raw.Dev), uint64(raw.Ino), true
}

// DelegateProof is the delegate owner kind's proof for a checkout: the
// chain's records under top, the checkout's repository, the kernel's
// custody observations, and the pass's instant.
func DelegateProof(top string, now time.Time) (dispatch.DelegateWorkspaceProof, error) {
	layout, err := stateroot.ResolveLayout(top)
	if err != nil {
		return dispatch.DelegateWorkspaceProof{}, err
	}
	stage, err := diskstore.NewID(now, rand.Reader)
	if err != nil {
		return dispatch.DelegateWorkspaceProof{}, err
	}
	return dispatch.DelegateWorkspaceProof{Repo: top, GitRoot: layout.GitRoot, Git: ExecWorkspaceGit, Stage: stage, Now: now,
		Custody: dispatch.CustodyDeathDependencies{MatchesTag: dispatchproc.PositionedJobTagAt(top)}}, nil
}
