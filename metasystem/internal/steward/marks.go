package steward

// Reading the current high-water marks. The first mark is the
// checkout HEAD commit id. The second is the goal ledger's content
// identity: before the multi-machine cutover that is the digest of
// plans/goals.md (every goal mutation rewrites it); after cutover it
// becomes the claim-History opid digest the design names — same
// meaning, different ledger format.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CurrentMarks reads both marks. Sentinel values stand in where a
// mark has no referent yet (an unborn branch, an absent ledger), so
// comparisons stay total.
func CurrentMarks(repoRoot string) (Marks, error) {
	head := "no-head"
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	if out, err := cmd.Output(); err == nil {
		head = strings.TrimSpace(string(out))
	}
	ledger := "no-ledger"
	if data, err := os.ReadFile(filepath.Join(repoRoot, "plans", "goals.md")); err == nil {
		sum := sha256.Sum256(data)
		ledger = hex.EncodeToString(sum[:])
	}
	return Marks{HeadOid: head, OpidDigest: ledger}, nil
}
