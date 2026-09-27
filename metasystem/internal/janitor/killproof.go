// Package janitor implements the machine-wide sweep that closes dead claims
// and stops only the surviving processes whose ownership it can prove. Owners
// are stopped before survivors because a surviving owner may relaunch them.
package janitor

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"golang.org/x/sys/unix"
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
		{Name: "shell-watcher", Includes: []string{"watch-background-jobs.sh"}, TagFlag: "--instance-tag"},
		{Name: "shell-reaper", Includes: []string{"dispatch.sh", "reap"}, TagFlag: "--instance-tag"},
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
	return groupOwnership(pgid, tag, groupOwnershipDependencies{
		PIDs: identity.AllPids,
		PGID: func(pid int64) (int64, error) {
			group, err := unix.Getpgid(int(pid))
			return int64(group), err
		},
		Reader: identity.KernelProber{},
	})
}

type groupOwnershipDependencies struct {
	PIDs   func() ([]int64, error)
	PGID   func(pid int64) (int64, error)
	Reader identity.VerificationReader
}

func groupOwnership(pgid int64, tag string, dependencies groupOwnershipDependencies) GroupOwnershipOutcome {
	if pgid < 2 || tag == "" {
		return GroupNotOwned
	}
	pids, err := dependencies.PIDs()
	if err != nil {
		return GroupIndeterminate
	}
	uncertainMembership := false
	var verifications []identity.Verification
	for _, pid := range pids {
		group, err := dependencies.PGID(pid)
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
			_, ok := MatchShape(DefaultShapes(), argv, tag)
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

// Killable applies the three-part signal proof to one live observation: pid,
// start identity, and claim-consistent argv captured immediately before
// signalling. recorded may be nil for an establishment orphan whose identity
// was never recorded; then the positioned shape match is the ownership proof.
func Killable(observed identity.Exact, recorded *registry.ProcessRef, shapes []Shape, claimTags []string) (string, bool) {
	if recorded != nil {
		if observed.Pid != recorded.Pid || observed.StartedAt.Unix() != recorded.PidStartedAt {
			return "", false // The pid now names a different process.
		}
	}
	if !observed.ArgvKnown {
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
