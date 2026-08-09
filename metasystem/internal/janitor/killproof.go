// Package janitor implements D-4: the machine-wide sweep that closes
// dead claims and stops their surviving sets — killing ONLY what it
// can prove (REG-6), in the order the incident taught (owner first,
// SLC-R7-001).
package janitor

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Shape is one known invocation form (REG-6): a command whose argv
// carries the claim's tag in a defined position. The shapes cover
// both the shipped shell components and the Go binary's verbs, so a
// janitor can prove ownership across the migration.
type Shape struct {
	// Name labels the shape in reports.
	Name string
	// Includes are substrings that must ALL appear among argv words
	// for the shape to match (command words, subcommands).
	Includes []string
	// TagFlag is the flag whose FOLLOWING argv word must equal the
	// claim's tag ("--tag", "--instance-tag"). The tag must appear as
	// that flag's value — a tag merely mentioned anywhere in argv
	// never matches (KI-14's lesson, restated by REG-6).
	TagFlag string
}

// DefaultShapes covers the committed shell supervision and the Go
// binary's owner verb.
func DefaultShapes() []Shape {
	return []Shape{
		{Name: "shell-owner", Includes: []string{"arm-supervision.sh", "__owner"}, TagFlag: "--tag"},
		{Name: "shell-watcher", Includes: []string{"watch-background-jobs.sh"}, TagFlag: "--instance-tag"},
		{Name: "shell-reaper", Includes: []string{"dispatch.sh", "reap"}, TagFlag: "--instance-tag"},
		{Name: "go-owner", Includes: []string{"metasystem", "supervise"}, TagFlag: "--tag"},
	}
}

// MatchShape reports whether argv matches a known invocation shape
// with the given tag in the tag position.
func MatchShape(shapes []Shape, argv []string, tag string) (Shape, bool) {
	for _, shape := range shapes {
		if matchOne(shape, argv, tag) {
			return shape, true
		}
	}
	return Shape{}, false
}

func matchOne(shape Shape, argv []string, tag string) bool {
	for _, required := range shape.Includes {
		found := false
		for _, word := range argv {
			// Command words may arrive as full paths; match on the
			// path's base or an exact word, never on arbitrary
			// substrings of unrelated arguments.
			if word == required || strings.HasSuffix(word, "/"+required) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for i, word := range argv {
		if word == shape.TagFlag && i+1 < len(argv) && argv[i+1] == tag {
			return true
		}
		// Also accept --flag=value spelling.
		if word == shape.TagFlag+"="+tag {
			return true
		}
	}
	return false
}

// Killable applies REG-6's triple to one LIVE observation: pid,
// pidStartedAt, AND claim-consistent argv, captured in one probe
// immediately before signalling. `recorded` may be nil for
// signature-only proof (an establishment orphan whose identities were
// never recorded); then the shape match with the claim's tag IS the
// proof and the observed identity merely names the target.
func Killable(observed identity.Exact, recorded *registry.ProcessRef, shapes []Shape, claimTags []string) (string, bool) {
	if recorded != nil {
		if observed.Pid != recorded.Pid || observed.StartedAt.Unix() != recorded.PidStartedAt {
			return "", false // a stranger on a recycled pid (SLC-R8-006)
		}
	}
	if len(observed.Argv) == 0 {
		// No readable argv means no third factor: report, never kill.
		return "", false
	}
	for _, tag := range claimTags {
		if shape, ok := MatchShape(shapes, observed.Argv, tag); ok {
			return shape.Name, true
		}
	}
	return "", false
}
