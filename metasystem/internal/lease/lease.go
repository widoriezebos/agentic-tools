package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Takeover records one seizure of the lease from a dead holder — kept as an
// audit trail on the lease itself.
type Takeover struct {
	FromMainId string `json:"fromMainId"`
	ToMainId   string `json:"toMainId"`
	ClaimEpoch int64  `json:"claimEpoch"`
	TakenAt    string `json:"takenAt"`
	Reason     string `json:"reason"`
}

// Lease is the single checkout write-authority record. Exactly one holder may
// write; a new holder either succeeds the same lineage (epoch preserved) or
// seizes a dead holder's lease (epoch bumped, sweep triggered). revision is a
// compare-and-swap guard against a concurrent writer; claimEpoch stamps which
// generation of work is current so a predecessor's jobs can be swept.
type Lease struct {
	HolderMainId string     `json:"holderMainId"`
	OwnerLineage string     `json:"ownerLineage"`
	Pid          int64      `json:"pid"`
	PidStartedAt int64      `json:"pidStartedAt"`
	CommandHash  string     `json:"commandHash"`
	ClaimedAt    string     `json:"claimedAt"`
	RenewedAt    string     `json:"renewedAt"`
	Takeovers    []Takeover `json:"takeovers"`
	Revision     int64      `json:"revision"`
	ClaimEpoch   int64      `json:"claimEpoch"`
}

// Paths are the four files the lease protocol keeps under artifacts/agents/mains.
type Paths struct {
	Lease       string // the lease record
	Lock        string // the flock file guarding claim/renew
	CommitToken string // the worktree commit token (used by dispatch)
	Stamp       string // reaped-after-claim: proof the epoch's sweep completed
}

func leasePaths(root string) Paths {
	mains := filepath.Join(root, "artifacts/agents/mains")
	return Paths{
		Lease:       filepath.Join(mains, "worktree-lease.json"),
		Lock:        filepath.Join(mains, "worktree-lease.lock"),
		CommitToken: filepath.Join(mains, "worktree-commit-token.json"),
		Stamp:       filepath.Join(mains, "reaped-after-claim.json"),
	}
}

// clock is the time source, overridable in tests.
var clock = time.Now

func nowStamp() string {
	return clock().UTC().Format("2006-01-02T15:04:05Z")
}

// loadLease reads the checkout lease. When required and absent it is an error;
// when not required, absence returns (nil, nil). A lease written before owner
// lineages existed is read, not refused: its holder is one process, so its
// lineage defaults to its own holderMainId, and the default persists on the
// next write.
func loadLease(root string, required bool) (*Lease, error) {
	path := leasePaths(root).Lease
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if required {
			return nil, fmt.Errorf("checkout lease is absent; start or arm an agent main first")
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s is unreadable: %w", path, err)
	}
	var lease Lease
	if err := json.Unmarshal(data, &lease); err != nil {
		return nil, fmt.Errorf("checkout lease schema is invalid")
	}
	if lease.HolderMainId == "" || lease.Pid < 1 || lease.ClaimEpoch < 1 || lease.Revision < 1 {
		return nil, fmt.Errorf("checkout lease schema is invalid")
	}
	if !validLineage(lease.OwnerLineage) {
		lease.OwnerLineage = lease.HolderMainId
	}
	if lease.Takeovers == nil {
		lease.Takeovers = []Takeover{}
	}
	return &lease, nil
}

// saveLease atomically writes the lease record.
func saveLease(root string, lease *Lease) error {
	if lease.Takeovers == nil {
		lease.Takeovers = []Takeover{}
	}
	return atomicJSON(leasePaths(root).Lease, lease)
}

// verifyRevision is the compare-and-swap guard: if the caller expected a
// revision, the lease it is about to overwrite must still be at it.
func verifyRevision(current *Lease, expected *int64) error {
	if expected != nil && (current == nil || current.Revision != *expected) {
		return fmt.Errorf("checkout lease changed before the compare-and-swap write")
	}
	return nil
}

var lineagePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func validLineage(value string) bool {
	return lineagePattern.MatchString(value)
}

// announcementLineage is the logical writer an announcement belongs to:
// its explicit ownerLineage, or its own mainId when it carries none (one
// process is its own lineage, so an ad-hoc session's successor is still a
// foreign takeover).
func announcementLineage(ann *Announcement) string {
	if validLineage(ann.OwnerLineage) {
		return ann.OwnerLineage
	}
	return ann.MainId
}

func leaseLineage(lease *Lease) string {
	if validLineage(lease.OwnerLineage) {
		return lease.OwnerLineage
	}
	return lease.HolderMainId
}

// atomicJSON writes value as indented JSON via a temp file and rename, with
// the parent directory fsynced so the record survives a crash intact.
func atomicJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// readObject reads a JSON object file into a map; absent is (nil, nil).
func readObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object", path)
	}
	return obj, nil
}
