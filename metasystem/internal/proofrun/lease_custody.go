package proofrun

// A richer lease's custody record (engine-owns-disk-lifetimes Part B, 3.5
// "Admission leases", DL2-01, DL3B-02): every custodian started for the
// lease, with its process group, appended durably beside the lease before
// the worker it guards is born. The lease record itself is immutable after
// acquisition (only its fixed-width cleared field changes), so the custody
// list lives in custody-<lease name>.jsonl, a name outside the lease-*,
// slot-* and resource-* patterns the scanners glob.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// leaseCustodian is one custodian of a lease: its encoded exact reference
// and its process group.
type leaseCustodian struct {
	Ref   string `json:"ref"`
	Group int64  `json:"group"`
}

const leaseCustodyPrefix = "custody-"

// leaseCustodyPath is the custody record beside a lease marker.
func leaseCustodyPath(lease string) string {
	return filepath.Join(filepath.Dir(lease), leaseCustodyPrefix+filepath.Base(lease)+".jsonl")
}

// createLeaseCustody creates the empty custody record of a new lease,
// durably, before its claim is published: a richer lease without one is
// unreadable, never "no custodians".
func createLeaseCustody(lease string) error {
	path := leaseCustodyPath(lease)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close(), syncLeaseDirectory(filepath.Dir(path)))
}

// recordLeaseCustodian appends a custodian to the custody record of every
// richer lease marker among files.
func recordLeaseCustodian(files []*os.File, custodian identity.Ref, group int64) error {
	encoded, err := identity.EncodeRef(custodian)
	if err != nil {
		return err
	}
	line, err := json.Marshal(leaseCustodian{Ref: encoded, Group: group})
	if err != nil {
		return err
	}
	for _, file := range files {
		if !strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			continue
		}
		// A lease an older engine acquired (no fixture owner) has no custody
		// record; it is judged by the older rules.
		lease, _, err := readHostLeaseRecord(file)
		if err != nil {
			return err
		}
		if lease.FixtureOwner == nil {
			continue
		}
		record, err := os.OpenFile(leaseCustodyPath(file.Name()), os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("lease custody record of %s: %w", filepath.Base(file.Name()), err)
		}
		_, writeErr := record.Write(append(line, '\n'))
		if err := errors.Join(writeErr, record.Sync(), record.Close()); err != nil {
			return fmt.Errorf("lease custody record of %s: %w", filepath.Base(file.Name()), err)
		}
	}
	return nil
}

// readLeaseCustodians reads a lease's custody record strictly: absent,
// torn, an unknown field or an invalid reference is an error.
func readLeaseCustodians(lease string) ([]leaseCustodian, error) {
	data, err := os.ReadFile(leaseCustodyPath(lease))
	if err != nil {
		return nil, err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("the custody record of %s is torn", filepath.Base(lease))
	}
	var custodians []leaseCustodian
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		var custodian leaseCustodian
		if err := decoder.Decode(&custodian); err != nil {
			return nil, fmt.Errorf("the custody record of %s is unreadable: %w", filepath.Base(lease), err)
		}
		if _, err := identity.ParseRef(custodian.Ref); err != nil || custodian.Group <= 0 {
			return nil, fmt.Errorf("the custody record of %s names an invalid custodian", filepath.Base(lease))
		}
		custodians = append(custodians, custodian)
	}
	return custodians, scanner.Err()
}

// removeLeaseCustody removes a lease's custody record once the lease is
// gone; an absent record is success.
func removeLeaseCustody(lease string) error {
	if err := os.Remove(leaseCustodyPath(lease)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func syncLeaseDirectory(directory string) error {
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

// removeOrphanCustody removes every custody record whose lease marker no
// longer exists; the caller holds admission.lock.
func removeOrphanCustody(directory string) error {
	records, err := filepath.Glob(filepath.Join(directory, leaseCustodyPrefix+"lease-*.jsonl"))
	if err != nil {
		return err
	}
	for _, record := range records {
		lease := filepath.Join(directory, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(record), leaseCustodyPrefix), ".jsonl"))
		if _, err := os.Lstat(lease); errors.Is(err, os.ErrNotExist) {
			if err := os.Remove(record); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
