package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots and Record carry the fields this package reads from lifecycle.Roots
// and lifecycle.Record. They are declared here rather than imported because
// internal/ui/lifecycle's own tests import internal/ui/httpd, which imports
// this package to type the resource: taking lifecycle's types closes that loop
// and Go refuses it. The interface's wiring carries the two conversions.
type Roots struct{ Checkout, Installation, StateRoot string }

type Record struct{ EngineBuild, StartedAt, ExecutableDigest string }

// SchemaVersion is the shape of the workspace resource the interface reads.
const SchemaVersion = 1

// The two modes a workspace runs in. Self-hosted is the template's own
// checkout, which serves itself; every other installation is adopted.
const (
	ModeSelfHosted = "self-hosted"
	ModeAdopted    = "adopted"
)

// SelfHostedSubject is the subject of a self-hosted workspace that configures
// none. An adopted workspace falls back to its checkout's base name.
const SelfHostedSubject = "MetaSystem"

// Workspace is what the interface header, the tab title, and Settings' About
// card read. Every field is a fact of this server's own run: nothing here is
// read from the application's records.
type Workspace struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Subject          string `json:"subject"`
	Mode             string `json:"mode"`
	Conflict         bool   `json:"conflict"`
	Checkout         string `json:"checkout"`
	Installation     string `json:"installation"`
	StateRoot        string `json:"stateRoot"`
	EngineBuild      string `json:"engineBuild"`
	StartedAt        string `json:"startedAt"`
	ExecutableDigest string `json:"executableDigest"`
	SourceHead       string `json:"sourceHead"`
	AdoptedFrom      string `json:"adoptedFrom"`
	AdoptionRecord   string `json:"adoptionRecord"`
	// Store is the interface's own private store outside every checkout: where
	// it is, what it holds, and what it is kept to (g1-s54 D3). It is filled by
	// the server's wiring rather than by Describe, because the bounds are this
	// seat's configuration and the measurement is a walk of the account's home
	// — neither is a fact of the three roots.
	Store *Store `json:"store,omitempty"`
}

// Describe answers the workspace resource from the three roots and the
// server's own record. The layout decides the mode, the installation's
// adoption line decides the provenance, and the conflict rule is computed here
// so that it has one owner rather than one per reader.
//
// It is called per request, so an adoption line filled in while the server runs
// is read without a restart.
func Describe(roots Roots, rec Record, configuredSubject string) (Workspace, error) {
	layout, err := stateroot.ResolveLayout(roots.Installation)
	if err != nil {
		return Workspace{}, fmt.Errorf("cannot resolve the layout of the installation at %s: %w", roots.Installation, err)
	}
	mode := ModeAdopted
	if layout.Template {
		mode = ModeSelfHosted
	}
	adoption := ReadAdoption(roots.Installation)
	return Workspace{
		SchemaVersion: SchemaVersion,
		Subject:       subjectOf(configuredSubject, mode, roots.Checkout),
		Mode:          mode,
		// D20's copied-marker case: a self-hosted layout that also records a
		// template SHA is two claims at once, and the interface says so.
		Conflict:         mode == ModeSelfHosted && adoption.Record == Recorded,
		Checkout:         roots.Checkout,
		Installation:     roots.Installation,
		StateRoot:        roots.StateRoot,
		EngineBuild:      rec.EngineBuild,
		StartedAt:        rec.StartedAt,
		ExecutableDigest: rec.ExecutableDigest,
		// The source at HEAD arrives with the slice that reads the checkout.
		SourceHead:     "",
		AdoptedFrom:    adoption.SHA,
		AdoptionRecord: string(adoption.Record),
	}, nil
}

// subjectOf applies the configured subject, or the default the mode implies:
// the template names itself, and an adopted workspace is named by the checkout
// it serves.
func subjectOf(configured, mode, checkout string) string {
	if subject := strings.TrimSpace(configured); subject != "" {
		return subject
	}
	if mode == ModeSelfHosted {
		return SelfHostedSubject
	}
	return filepath.Base(checkout)
}
