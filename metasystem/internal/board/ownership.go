package board

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Ownership is what the accepted ledger says of the goals peer messages are
// addressed to (batch-lane design D14-r3, R26): every live goal with the
// machine that holds it ("" when nobody does), and every concluded goal,
// done or abandoned, with its fact. The board takes it as a value; it never
// reads the ledger.
type Ownership struct {
	Live      map[string]string `json:"live"`
	Concluded map[string]string `json:"concluded"`
}

// ownershipCache is the host's one shared projection of the ledger's
// ownership, keyed by the accepted tip it was read at.
type ownershipCache struct {
	Tip       string    `json:"tip"`
	Ownership Ownership `json:"ownership"`
}

func ownershipCachePath(home string) string {
	return filepath.Join(home, "host", "peer-ownership.json")
}

// ReadOwnershipCache is the cached ownership at tip; false when the cache is
// missing, unreadable or of another tip (the read's F-3).
func ReadOwnershipCache(home, tip string) (Ownership, bool) {
	if home == "" || tip == "" {
		return Ownership{}, false
	}
	data, err := os.ReadFile(ownershipCachePath(home))
	if err != nil {
		return Ownership{}, false
	}
	var cache ownershipCache
	if json.Unmarshal(data, &cache) != nil || cache.Tip != tip || cache.Ownership.Live == nil {
		return Ownership{}, false
	}
	return cache.Ownership, true
}

// WriteOwnershipCache publishes the ownership read at tip for every seat of
// the host, atomically, 0600 in the private host directory.
func WriteOwnershipCache(home, tip string, ownership Ownership) error {
	if home == "" || tip == "" {
		return nil
	}
	if _, err := boardDir(home); err != nil {
		return err
	}
	if ownership.Live == nil {
		ownership.Live = map[string]string{}
	}
	if ownership.Concluded == nil {
		ownership.Concluded = map[string]string{}
	}
	data, err := json.Marshal(ownershipCache{Tip: tip, Ownership: ownership})
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(ownershipCachePath(home), string(data)+"\n", home)
	return err
}
