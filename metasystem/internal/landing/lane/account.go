package lane

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
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
// Its refusals lead with CodeAccountUnresolved, the code machines match; a
// person's command shows the words after it and the code with --verbose.
func ResolveAccount(home, controlRoot string) (string, error) {
	record, ok, err := Read(home)
	switch {
	case err != nil:
		return "", fmt.Errorf("%s: this computer's landing lane record can't be read: %w", CodeAccountUnresolved, err)
	case !ok:
		return "", fmt.Errorf("%s: no landing lane is registered on this computer", CodeAccountUnresolved)
	case gone(record.Root):
		return "", fmt.Errorf("%s: the landing lane %s no longer exists", CodeAccountUnresolved, record.Root)
	}
	relative, err := filepath.Rel(resolved(record.Root), resolved(controlRoot))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: %s is not inside the landing lane %s", CodeAccountUnresolved, controlRoot, record.Root)
	}
	return AccountID(record.Root), nil
}
