package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// RetiredFile is the pointer a retired evidence root carries, so a disk
// census reads it as "retired root of these checkouts" instead of a silent
// orphan.
const RetiredFile = "RETIRED.json"

// retiredPointer is RETIRED.json in its fixed field order.
type retiredPointer struct {
	SchemaVersion int      `json:"schemaVersion"`
	RetiredAt     string   `json:"retiredAt"`
	Checkouts     []string `json:"checkouts"`
	Successor     string   `json:"successor"`
	Rule          string   `json:"rule"`
}

// WriteRetired writes <root>/RETIRED.json naming the checkouts that used the
// root before they moved onto the per-checkout default. It is idempotent: an
// existing pointer keeps its retiredAt, its checkouts are the union of the
// file's and the call's, and a call that adds nothing leaves the file's bytes
// alone (changed is false). A checkout is added, never removed. The write is
// atomic.
func WriteRetired(root string, checkouts []string, now time.Time) (changed bool, err error) {
	if !filepath.IsAbs(root) {
		return false, fmt.Errorf("retire an evidence root: the root must be absolute, got %q", root)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return false, fmt.Errorf("retire an evidence root: %s is not a directory", root)
	}
	if len(checkouts) == 0 {
		return false, fmt.Errorf("retire an evidence root: name at least one checkout")
	}
	for _, checkout := range checkouts {
		if !filepath.IsAbs(checkout) {
			return false, fmt.Errorf("retire an evidence root: a checkout must be absolute, got %q", checkout)
		}
	}
	path := filepath.Join(root, RetiredFile)
	pointer := retiredPointer{SchemaVersion: 1, RetiredAt: now.UTC().Format(time.RFC3339),
		Successor: "per-checkout default", Rule: "evidence-root-default"}
	existing, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		var held retiredPointer
		if err := json.Unmarshal(existing, &held); err != nil || held.SchemaVersion != 1 || held.RetiredAt == "" {
			return false, fmt.Errorf("retire an evidence root: %s is not a retirement pointer; move it aside first", path)
		}
		pointer.RetiredAt = held.RetiredAt
		checkouts = append(append([]string{}, held.Checkouts...), checkouts...)
	case !os.IsNotExist(readErr):
		return false, readErr
	}
	seen := map[string]bool{}
	for _, checkout := range checkouts {
		clean := filepath.Clean(checkout)
		if !seen[clean] {
			seen[clean] = true
			pointer.Checkouts = append(pointer.Checkouts, clean)
		}
	}
	sort.Strings(pointer.Checkouts)
	encoded, err := json.Marshal(pointer)
	if err != nil {
		return false, err
	}
	text := string(encoded) + "\n"
	if readErr == nil && string(existing) == text {
		return false, nil
	}
	if _, err := atomicfile.WriteText(path, text, root); err != nil {
		return false, err
	}
	return true, nil
}
