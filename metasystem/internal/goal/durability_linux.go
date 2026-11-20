//go:build linux

package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

func (e Endpoint) syncPublished(commit, recoveredOp string) {
	home, err := board.Home()
	if err != nil {
		return
	}
	stores, _ := ownedStores(home, e.Root)
	if len(stores) == 0 {
		return
	}
	ref := e.Branch
	receivers := []string{e.Root}
	if e.LocalMode() {
		ref = LocalLedgerBranch
	} else {
		receivers = pushReceivers(e.Root, e.Remote)
	}
	syncDir := e.syncDirectory
	if syncDir == nil {
		syncDir = durable.SyncDir
	}
	for _, receiver := range receivers {
		common := commonStore(receiver)
		if _, ok := stores[common]; !ok {
			continue
		}
		delete(stores, common)
		if err := syncLedgerNames(common, ref, commit, recoveredOp, syncDir); err != nil {
			w := e.durabilityWarnings
			if w == nil {
				w = os.Stderr
			}
			fmt.Fprintf(w, "%s published; durability in doubt for %s: %v\n", ref, common, err)
		}
	}
}

func syncLedgerNames(common, ref, commit, recoveredOp string, syncDir func(string) error) error {
	if recoveredOp != "" {
		var err error
		commit, err = commitWithTrailer(common, ref, "Goal-Transaction", recoveredOp)
		if err != nil {
			return err
		}
	}
	out, err := gitIn(common, "--no-replace-objects", "--no-lazy-fetch", "rev-list", "--objects", "--no-object-names", commit+"^!")
	if err != nil {
		return err
	}
	dirs := map[string]bool{}
	for _, oid := range strings.Fields(out) {
		fan := filepath.Join("objects", oid[:2])
		if _, err := os.Stat(filepath.Join(common, fan, oid[2:])); os.IsNotExist(err) {
			fan = "objects/pack"
		} else if err != nil {
			return err
		}
		dirs[fan] = true
	}
	for d := range dirs {
		if err := syncDir(filepath.Join(common, d)); err != nil {
			return err
		}
	}
	if err := syncDir(filepath.Join(common, "objects")); err != nil {
		return err
	}
	// Loose refs and newly created ancestors, or the packed-refs rename
	// in the common directory. Reftable installs its tables and manifest together.
	refDirs := []string{filepath.Dir(ref)}
	if target, err := gitIn(common, "symbolic-ref", "-q", ref); err == nil {
		refDirs = append(refDirs, filepath.Dir(strings.TrimSpace(target)))
	}
	if _, err := os.Stat(filepath.Join(common, "reftable")); err == nil {
		refDirs = []string{"reftable"}
	} else {
		for _, dir := range refDirs {
			refDirs = append(refDirs, filepath.Join("logs", dir))
		}
	}
	for _, dir := range refDirs {
		for ; dir != "."; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(common, dir)); os.IsNotExist(err) {
				continue
			}
			if err := syncDir(filepath.Join(common, dir)); err != nil {
				return err
			}
		}
	}
	return syncDir(common)
}
