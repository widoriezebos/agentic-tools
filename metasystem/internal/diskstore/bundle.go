package diskstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A suite-failure bundle's owner (3.5, 3.12 clause 1; DL4D-06): every bundle
// carries OWNER.json naming the attempt that wrote it and the goal that
// attempt was accounted to, so the evidence bound can hold a bundle of an
// open goal long after the attempt record itself is pruned. The distiller
// never distils it and the move carries it.

// OwnerFileName is the bundle's owner file.
const OwnerFileName = "OWNER.json"

// BundleOwnerSchema names the owner file's format.
const BundleOwnerSchema = "metasystem.bundle-owner/1"

// The goal values that are not goal ids.
const (
	// GoalNone: the attempt names no goal, or the bundle is standalone; the
	// bound's goal clause does not apply.
	GoalNone = "none"
	// GoalUnknown: a legacy bundle whose attempt record is gone; the goal
	// clause is Unknown and machinery never compacts it.
	GoalUnknown = "unknown"
	// AttemptStandalone: written outside a proof attempt.
	AttemptStandalone = "standalone"
)

// BundleOwner is OWNER.json.
type BundleOwner struct {
	Schema         string    `json:"schema"`
	Attempt        string    `json:"attempt"`
	Goal           string    `json:"goal"`
	GitRoot        string    `json:"gitRoot,omitempty"`
	Installation   string    `json:"installation,omitempty"`
	RootCommit     string    `json:"rootCommit,omitempty"`
	LedgerIdentity string    `json:"ledgerIdentity,omitempty"`
	WrittenBy      string    `json:"writtenBy"`
	WrittenAt      time.Time `json:"writtenAt"`
}

// Valid reports whether the owner names an attempt and a goal.
func (o BundleOwner) Valid() error {
	if o.Attempt == "" || o.Goal == "" {
		return errors.New("an owner file names an attempt and a goal")
	}
	return nil
}

// WriteBundleOwner writes OWNER.json into a bundle directory, creating the
// directory; an existing owner file is kept as it is (the first writer
// names the owner) and reported unchanged.
func WriteBundleOwner(bundle string, owner BundleOwner, sync Syncer) (written bool, err error) {
	if err := owner.Valid(); err != nil {
		return false, err
	}
	path := filepath.Join(bundle, OwnerFileName)
	if _, err := os.Lstat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	owner.Schema = BundleOwnerSchema
	data, err := json.MarshalIndent(owner, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(bundle, 0o700); err != nil {
		return false, err
	}
	stage := fmt.Sprintf("owner-%d", owner.WrittenAt.UnixNano())
	return true, sync.WriteDurable(path, append(data, '\n'), stage)
}

// ReadBundleOwner reads OWNER.json; absent is os.ErrNotExist.
func ReadBundleOwner(bundle string) (BundleOwner, error) {
	data, err := os.ReadFile(filepath.Join(bundle, OwnerFileName))
	if err != nil {
		return BundleOwner{}, err
	}
	var owner BundleOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return BundleOwner{}, fmt.Errorf("%s is unreadable: %w", filepath.Join(bundle, OwnerFileName), err)
	}
	if err := owner.Valid(); err != nil {
		return BundleOwner{}, fmt.Errorf("%s: %w", filepath.Join(bundle, OwnerFileName), err)
	}
	return owner, nil
}

var unsafeBundleName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// BundleRunName is the part of a suite-failure bundle's name that says
// which run wrote it (3.5): the proof attempt's id when the writer runs
// inside one, else standalone-<pid>.
func BundleRunName(attempt string, pid int) string {
	attempt = strings.Trim(unsafeBundleName.ReplaceAllString(attempt, "-"), "-")
	if attempt == "" {
		return fmt.Sprintf("%s-%d", AttemptStandalone, pid)
	}
	return attempt
}
