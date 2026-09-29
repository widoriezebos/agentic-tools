package evidence

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// storeHolds is the checkout's store registry as the collector reads it,
// once per run (engine-owns-disk-lifetimes 3.11, DL2-14): the chains whose
// workspace store is registered and not released, and the directories of
// every unreleased store, known by device and inode. A registry that cannot
// be read whole (an unreadable record, an unknown schema, a store path that
// cannot be examined) holds every collection of the run: fail-closed rule 1.
type storeHolds struct {
	unreadable string
	chains     map[string]string
	roots      map[fileKey]bool
	paths      []string
}

type fileKey struct{ device, inode uint64 }

func keyOf(info fs.FileInfo) (fileKey, bool) {
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileKey{}, false
	}
	return fileKey{uint64(raw.Dev), uint64(raw.Ino)}, true
}

func readStoreHolds(checkoutRoot string) storeHolds {
	holds := storeHolds{chains: map[string]string{}, roots: map[fileKey]bool{}}
	records, unreadable := diskstore.CheckoutRegistry(checkoutRoot).Inventory()
	if len(unreadable) > 0 {
		holds.unreadable = unreadable[0].Path + ": " + unreadable[0].Reason
		return holds
	}
	for _, record := range records {
		if record.State == diskstore.StateReleased {
			continue
		}
		if record.Owner.Kind == diskstore.OwnerDelegate {
			holds.chains[record.Owner.Ref] = record.ID
		}
		paths := []string{record.Path}
		if record.Class == diskstore.WorkspaceClass {
			paths = append(paths, diskstore.WorkspaceTmp(record))
		}
		for _, path := range paths {
			info, err := os.Lstat(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				holds.unreadable = "store " + record.ID + " at " + path + ": " + err.Error()
				return holds
			}
			if key, ok := keyOf(info); ok {
				holds.roots[key] = true
			}
			holds.paths = append(holds.paths, filepath.Clean(path))
		}
	}
	return holds
}

// holdsChain is the reason a chain's payload and job records stay, or empty.
func (h storeHolds) holdsChain(chain string) string {
	if h.unreadable != "" {
		return "the store registry cannot be read (" + h.unreadable + "); nothing is collected this pass"
	}
	if id := h.chains[chain]; id != "" {
		return "its workspace store " + id + " is registered and not released; it is collected after the store's release (metasystem disk show)"
	}
	return ""
}

// protects reports a directory that is a registered store's root, by its
// device and inode; its recorded spelling protects it too, which only ever
// keeps more.
func (h storeHolds) protects(path string, info fs.FileInfo) bool {
	if key, ok := keyOf(info); ok && h.roots[key] {
		return true
	}
	clean := filepath.Clean(path)
	for _, root := range h.paths {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
