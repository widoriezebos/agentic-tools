package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// ErrDesignChanged is returned when a design document no longer holds the
// bytes a proposal was made against: someone edited it since.
var ErrDesignChanged = errors.New("DESIGN_DOCUMENT_CHANGED")

// ErrDesignInvalid is returned when a proposal is not the expected design
// record: another kind, id, goal or status than a draft of this record.
var ErrDesignInvalid = errors.New("DESIGN_PROPOSAL_INVALID")

// DesignPublication is one proposal for one design document.
type DesignPublication struct {
	// Destination is the document's absolute path; Root is the directory
	// the atomic write may stage in.
	Destination, Root string
	// Expected is the document's bytes when the proposal was requested;
	// ExpectedPresent false means the document did not exist.
	Expected        []byte
	ExpectedPresent bool
	Draft           []byte
	RecordID, Goal  string
}

// PublishDesign writes a proposal to its design document only when the
// proposal is a draft design record with the expected id and goal, and the
// document still holds exactly the bytes the proposal was made against (or
// is still absent). A document that already holds the identical proposal is
// the same publication, reported as such. Nothing is merged: a changed
// document or an invalid proposal leaves the document as it is.
func PublishDesign(publication DesignPublication) (alreadyPublished bool, err error) {
	record, problems, ok := ParseRecord(filepath.Base(publication.Destination), string(publication.Draft))
	switch {
	case !ok || len(problems) > 0:
		return false, fmt.Errorf("%w: the proposal is not a readable project record (%d problems)", ErrDesignInvalid, len(problems))
	case record.Kind != KindDesign:
		return false, fmt.Errorf("%w: the proposal is a %s record, not a design", ErrDesignInvalid, record.Kind)
	case record.ID != publication.RecordID:
		return false, fmt.Errorf("%w: the proposal's id is %s, not %s", ErrDesignInvalid, record.ID, publication.RecordID)
	case !slices.Contains(record.Goals, publication.Goal):
		return false, fmt.Errorf("%w: the proposal does not name goal %s", ErrDesignInvalid, publication.Goal)
	case record.Status != "draft":
		return false, fmt.Errorf("%w: the proposal's status is %s, not draft", ErrDesignInvalid, record.Status)
	}
	current, readErr := os.ReadFile(publication.Destination)
	present := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return false, readErr
	}
	if present && bytes.Equal(current, publication.Draft) {
		return true, nil
	}
	if present != publication.ExpectedPresent || present && !bytes.Equal(current, publication.Expected) {
		return false, fmt.Errorf("%w: %s changed since the proposal was requested", ErrDesignChanged, publication.Destination)
	}
	if err := os.MkdirAll(filepath.Dir(publication.Destination), 0o755); err != nil {
		return false, err
	}
	_, err = atomicfile.WriteText(publication.Destination, string(publication.Draft), publication.Root)
	return false, err
}
