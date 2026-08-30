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
	// The cutover landed: the goal ledger's content identity is the
	// accepted goals ref's tip (refs/metasystem/goals/accepted, advanced
	// by every ledger CAS). The retired plans/goals.md hash sat here for
	// three days after the cutover, pinning OpidDigest to the no-ledger
	// sentinel and making every goal movement invisible to stall
	// detection (steward-marks-retired-ledger). An absent ref keeps the
	// sentinel so pre-cutover checkouts still compare totally.
	ledger := "no-ledger"
	refCmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted")
	if refOut, refErr := refCmd.Output(); refErr == nil {
		tip := strings.TrimSpace(string(refOut))
		if tip != "" {
			sum := sha256.Sum256([]byte(tip))
			ledger = hex.EncodeToString(sum[:])
		}
	}
	return Marks{HeadOid: head, OpidDigest: ledger}, nil
}
