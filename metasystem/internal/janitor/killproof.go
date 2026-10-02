// Package janitor implements the machine-wide sweep that closes dead claims
// and stops only the surviving processes whose ownership it can prove. Owners
// are stopped before survivors because a surviving owner may relaunch them.
package janitor

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// Shape is one known invocation form whose argv carries the claim's tag in a
// defined position. The shapes cover both shipped shell components and Go
// verbs, so the janitor can prove ownership across the migration.
type Shape struct {
	// Name labels the shape in reports.
	Name string
	// Includes are substrings that must ALL appear among argv words
	// for the shape to match (command words, subcommands).
	Includes []string
	// TagFlag is the flag whose FOLLOWING argv word must equal the
	// claim's tag ("--tag", "--instance-tag"). The tag must appear as
	// that flag's value — a tag merely mentioned anywhere in argv
	// never matches.
	TagFlag string
	// TagPrefix is an optional exact prefix inside the TagFlag value. It
	// supports structured flag values such as key="tag" without accepting a
	// tag in any other argument.
	TagPrefix string
	// TagPathBase accepts the tag as the exact base name of the flag's path
	// value. It covers a CLI whose only inert per-invocation argv carrier is
	// its private configuration file.
	TagPathBase bool
}

// DefaultShapes covers the committed supervision processes. The delegate
// and host supervisors' shapes come from the runtime registry's one process
// definition, the same one their launcher builds its argv from.
func DefaultShapes() []Shape {
	shapes := []Shape{
		{Name: "go-owner", Includes: []string{"metasystem", "supervise"}, TagFlag: "--tag"},
	}
	for _, supervisor := range runtimes.SupervisorShapes() {
		shapes = append(shapes, Shape{Name: supervisor.Name, Includes: supervisor.Includes, TagFlag: supervisor.TagFlag})
	}
	for _, runtime := range []string{"codex", "claude", "devin"} {
		for _, cli := range runtimes.CLIInvocations(runtime) {
			shapes = append(shapes, Shape{Name: "adapter-cli-" + runtime, Includes: cli.Includes, TagFlag: cli.TagFlag,
				TagPrefix: cli.TagPrefix, TagPathBase: cli.TagPathBase})
		}
	}
	return append(shapes,
		Shape{Name: "tagged-hold", Includes: []string{"metasystem", "util", "hold"}, TagFlag: "--tag"},
		Shape{Name: "mission-run-loop", Includes: []string{"metasystem", "mission", "run-loop"}, TagFlag: "--instance-tag"},
	)
}

// ShapesAt is DefaultShapes for an installation root: the committed shapes
// plus each external runtime's and override's supervisor shapes and the
// claim-bound CLI invocation shapes its describe declares (VOA-29). The
// kill proof stays here in Go; a runtime's declaration only says where its
// claim tag sits, never that a process is its own.
func ShapesAt(root string) []Shape {
	shapes := DefaultShapes()
	if root == "" {
		return shapes
	}
	reg, err := external.Load(root)
	if err != nil {
		return shapes
	}
	for _, entry := range reg.Externals() {
		if entry.Refused != nil {
			continue
		}
		if !entry.Builtin {
			for _, supervisor := range runtimes.RuntimeSupervisorShapes(entry.Name, true, entry.Description.Capabilities.Host) {
				shapes = append(shapes, Shape{Name: supervisor.Name, Includes: supervisor.Includes, TagFlag: supervisor.TagFlag})
			}
		}
		for _, cli := range entry.Description.Invocations {
			if len(cli.Includes) == 0 || cli.TagFlag == "" {
				continue
			}
			shapes = append(shapes, Shape{Name: "adapter-cli-" + entry.Name, Includes: cli.Includes, TagFlag: cli.TagFlag,
				TagPrefix: cli.TagPrefix, TagPathBase: cli.TagPathBase})
		}
	}
	return shapes
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
		if word == shape.TagFlag && i+1 < len(argv) && tagValueMatches(shape, argv[i+1], tag) {
			return true
		}
		// Also accept --flag=value spelling.
		if shape.TagPrefix == "" && word == shape.TagFlag+"="+tag {
			return true
		}
	}
	return false
}

func tagValueMatches(shape Shape, value, tag string) bool {
	if shape.TagPathBase {
		return filepath.Base(value) == tag
	}
	if shape.TagPrefix == "" {
		return value == tag
	}
	return value == shape.TagPrefix+tag || value == shape.TagPrefix+`"`+tag+`"`
}

// GroupOwnershipOutcome is the signal predicate's tri-state result. Only a
// verified positioned tag authorizes signalling; an unreadable observation
// remains distinguishable so the fake-runtime compatibility path can defer.
type GroupOwnershipOutcome string

const (
	GroupOwned         GroupOwnershipOutcome = "OWNED"
	GroupNotOwned      GroupOwnershipOutcome = "NOT-OWNED"
	GroupIndeterminate GroupOwnershipOutcome = "INDETERMINATE"
)

// GroupOwnership scans each current member through the identity sandwich and
// the shipped positional shapes. A process that merely mentions the tag is a
// known non-match, including when it is the group leader.
func GroupOwnership(pgid int64, tag string) GroupOwnershipOutcome {
	return GroupOwnershipAt("", pgid, tag)
}

// GroupOwnershipAt is GroupOwnership against an installation's shapes
// (ShapesAt), so an external runtime's supervisors and CLIs are provable.
func GroupOwnershipAt(root string, pgid int64, tag string) GroupOwnershipOutcome {
	return groupOwnership(pgid, tag, groupOwnershipDependencies{
		Shapes:    ShapesAt(root),
		Processes: identity.KernelProcessTable{},
		Reader:    identity.KernelProber{},
	})
}

type groupOwnershipDependencies struct {
	// Shapes are the positional shapes a member must match; nil is
	// DefaultShapes.
	Shapes []Shape
	// Processes is the process table the members are read from: the
	// kernel's in production, a test's own rows in a test.
	Processes identity.ProcessTable
	Reader    identity.VerificationReader
}

func groupOwnership(pgid int64, tag string, dependencies groupOwnershipDependencies) GroupOwnershipOutcome {
	if pgid < 2 || tag == "" {
		return GroupNotOwned
	}
	if dependencies.Processes == nil {
		dependencies.Processes = identity.KernelProcessTable{}
	}
	pids, err := dependencies.Processes.Pids()
	if err != nil {
		return GroupIndeterminate
	}
	shapes := dependencies.Shapes
	if shapes == nil {
		shapes = DefaultShapes()
	}
	uncertainMembership := false
	var verifications []identity.Verification
	for _, pid := range pids {
		group, err := dependencies.Processes.Group(pid)
		if err != nil {
			if !errors.Is(err, unix.ESRCH) {
				uncertainMembership = true
			}
			continue
		}
		if int64(group) != pgid {
			continue
		}
		verifications = append(verifications, identity.VerifyProcess(dependencies.Reader, pid, func(argv []string) bool {
			_, ok := MatchShape(shapes, argv, tag)
			return ok
		}))
	}
	return groupOwnershipFromVerifications(verifications, uncertainMembership)
}

func groupOwnershipFromVerifications(verifications []identity.Verification, uncertain bool) GroupOwnershipOutcome {
	knownNonMatch := false
	for _, verification := range verifications {
		switch verification.Outcome {
		case identity.VerificationVerified:
			return GroupOwned
		case identity.VerificationIndeterminate:
			uncertain = true
		case identity.VerificationNotOurs:
			knownNonMatch = true
		}
	}
	if uncertain {
		return GroupIndeterminate
	}
	if knownNonMatch {
		return GroupNotOwned
	}
	// An EMPTY scan proves nothing: a group mid-reap (members leaving
	// between the group probe and the member walk, zombies with no argv)
	// must never read as PROVABLY foreign — that false proof made the
	// wind-down abandon its own dying groups on Linux (the VM sweep's
	// winddown-zombie-ownership-linux finding). NOT-OWNED requires at
	// least one positive not-ours observation.
	return GroupIndeterminate
}
