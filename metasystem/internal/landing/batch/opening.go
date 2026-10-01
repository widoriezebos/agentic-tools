package batch

// The record shape of the deleted landing begin: older batch records keep
// their openings and composition evidence, and an engine reading them
// still decodes them. Nothing writes them any more.

import (
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// The trailers an older lane agent marked its own commits with.
const (
	LaneResolvedTrailer    = "Lane-Resolved"
	LaneIntegrationTrailer = "Lane-Integration"
)

// Opening is one landing begin an older engine recorded.
type Opening struct {
	OpID      string         `json:"opId"`
	At        string         `json:"at"`
	Actor     string         `json:"actor"`
	Members   []string       `json:"members"`
	Base      string         `json:"base"`
	BaseTree  string         `json:"baseTree"`
	Head      string         `json:"head"`
	Candidate string         `json:"candidate"`
	Tree      string         `json:"tree"`
	Series    []SeriesCommit `json:"series"`
	Deviation int            `json:"deviation"`
}

// SeriesCommit is one commit of an older opening's series.
type SeriesCommit struct {
	Commit    string `json:"commit"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Member    string `json:"member,omitempty"`
	Pin       string `json:"pin,omitempty"`
	Deviation int    `json:"deviation,omitempty"`
	Resolved  string `json:"resolved,omitempty"`
}

// CompositionEvidence is a composition refusal an older engine recorded.
type CompositionEvidence struct {
	OpID      string   `json:"opId"`
	At        string   `json:"at"`
	Actor     string   `json:"actor"`
	Kind      string   `json:"kind"`
	Members   []string `json:"members"`
	Base      string   `json:"base,omitempty"`
	Onto      string   `json:"onto,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Deviation int      `json:"deviation,omitempty"`
	Detail    string   `json:"detail"`
}

// realGit is a git command in root that reads the real objects: replace
// refs are off by config and by environment, whatever refs/replace holds.
func realGit(root string, args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-C", root, "-c", "core.useReplaceRefs=false"}, args...)...)
}

// realObjectsEnviron is the scrubbed environment with replacement off.
func realObjectsEnviron(extra ...string) []string {
	return gittree.ScrubbedEnviron(append([]string{"GIT_NO_REPLACE_OBJECTS=1"}, extra...)...)
}
