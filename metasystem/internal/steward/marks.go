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
	"fmt"
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
	out, headErr := cmd.Output()
	if headErr == nil {
		head = strings.TrimSpace(string(out))
	} else if _, statErr := os.Stat(filepath.Join(repoRoot, ".git")); statErr == nil {
		// The repository exists but HEAD is unreadable: a sentinel
		// here would differ from the stored mark and read as
		// PROGRESS, resetting the aging and the dry cap on the
		// strength of a read failure. Refuse instead.
		return Marks{}, fmt.Errorf("HEAD unreadable in an existing repository: %v", headErr)
	}
	ledger := "no-ledger"
	if data, err := os.ReadFile(filepath.Join(repoRoot, "plans", "goals.md")); err == nil {
		sum := sha256.Sum256(data)
		ledger = hex.EncodeToString(sum[:])
	} else if !os.IsNotExist(err) {
		return Marks{}, fmt.Errorf("goal ledger unreadable: %v", err)
	}
	return Marks{HeadOid: head, OpidDigest: ledger}, nil
}
