// Package workspace answers what the workspace in front of a human is: its
// subject, the layout its installation runs in, and what that installation
// records about the template it was adopted from.
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// adoptionLinePrefix is the line scripts/adopt.sh writes into the installed
// docs/project-rules.md, and the placeholder it leaves until a provenance
// slice fills it in.
const (
	adoptionLinePrefix  = "- Adopted from template SHA:"
	adoptionPlaceholder = "<template sha>"
	adoptionRelativeDoc = "docs/project-rules.md"
	shaLength           = 40
)

// AdoptionRecord says what the installation's adoption line holds. The four
// values are the only ones this reader reports, so every caller can enumerate
// them.
type AdoptionRecord string

const (
	// Absent: no docs/project-rules.md, or one carrying no adoption line.
	Absent AdoptionRecord = "absent"
	// Unreadable: the file exists but cannot be read, or its adoption line
	// holds something this reader cannot make a SHA of.
	Unreadable AdoptionRecord = "unreadable"
	// Placeholder: the line still carries the literal the adoption script
	// writes, so nothing has recorded a template SHA yet.
	Placeholder AdoptionRecord = "placeholder"
	// Recorded: the line names a template SHA, carried in SHA.
	Recorded AdoptionRecord = "recorded"
)

// Adoption is what the installation records about its own provenance. SHA is
// set only for Recorded; genuine provenance, verified against the template, is
// a later slice's.
type Adoption struct {
	Record AdoptionRecord
	SHA    string
}

// ReadAdoption reads the adoption line from the installation's own
// docs/project-rules.md, the path the adoption script writes in every layout.
// It never fails: an installation whose provenance cannot be read is a fact the
// interface states, not a reason to refuse the whole workspace.
func ReadAdoption(installation string) Adoption {
	content, err := os.ReadFile(filepath.Join(installation, filepath.FromSlash(adoptionRelativeDoc)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Adoption{Record: Absent}
		}
		return Adoption{Record: Unreadable}
	}
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(strings.TrimRight(line, "\r"), adoptionLinePrefix) {
			continue
		}
		return adoptionFrom(line)
	}
	return Adoption{Record: Absent}
}

// adoptionFrom reads the text between the line's first pair of backticks, the
// form the adoption script writes.
func adoptionFrom(line string) Adoption {
	_, after, found := strings.Cut(line, "`")
	if !found {
		return Adoption{Record: Unreadable}
	}
	value, _, closed := strings.Cut(after, "`")
	switch {
	case !closed:
		return Adoption{Record: Unreadable}
	case value == adoptionPlaceholder:
		return Adoption{Record: Placeholder}
	case isTemplateSHA(value):
		return Adoption{Record: Recorded, SHA: value}
	default:
		return Adoption{Record: Unreadable}
	}
}

// isTemplateSHA reports whether value is forty lower-case hexadecimal digits:
// what git rev-parse writes, and the only form this reader accepts.
func isTemplateSHA(value string) bool {
	if len(value) != shaLength {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
