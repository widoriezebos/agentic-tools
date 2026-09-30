package lane

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// accountPrefix marks a lane accounting identity: a proof of a batch whose
// members are all changes is charged to the lane, never to a goal (U11b).
const accountPrefix = "lane:"

// AccountID is the accounting identity of the lane whose checkout is root.
func AccountID(root string) string {
	sum := sha256.Sum256([]byte(resolved(root)))
	return accountPrefix + hex.EncodeToString(sum[:])[:12]
}

// IsAccount reports whether id is a lane accounting identity rather than a
// goal (or change) id.
func IsAccount(id string) bool { return strings.HasPrefix(id, accountPrefix) }

// ResolveAccount answers the accounting identity of the host's registered
// lane when controlRoot lies inside its checkout. No record, an unreadable
// or gone lane, or a control root outside it is an error: a proof is never
// charged to a lane that cannot be named.
//
// Its refusals are coded CodeAccountUnresolved: a caller matches the code
// with errors.As, a person reads the plain reason.
func ResolveAccount(home, controlRoot string) (string, error) {
	unresolved := func(facts string, reason error) error {
		return &refusal.Coded{Code: CodeAccountUnresolved, Facts: facts, Reason: reason}
	}
	record, ok, err := Read(home)
	switch {
	case err != nil:
		return "", unresolved("", fmt.Errorf("this computer's landing lane record can't be read: %w", err))
	case !ok:
		return "", unresolved("", errors.New("no landing lane is registered on this computer"))
	case gone(record.Root):
		return "", unresolved("lane="+record.Root, fmt.Errorf("the landing lane %s no longer exists", record.Root))
	}
	relative, err := filepath.Rel(resolved(record.Root), resolved(controlRoot))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", unresolved("lane="+record.Root, fmt.Errorf("%s is not inside the landing lane %s", controlRoot, record.Root))
	}
	return AccountID(record.Root), nil
}
