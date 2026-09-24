package seat

import (
	"fmt"
	"strings"
)

// A Rung is one way to hold presence at the remote. The publisher climbs the
// ladder by itself so that a host which refuses the first rung never leaves a
// fleet without presence (design section 8).
type Rung int

const (
	// RungMetasystemRef is ordinary git: a ref outside heads and tags, force
	// pushed, each record a parentless commit. Proven on github.com and on
	// every self-hosted server.
	RungMetasystemRef Rung = 1
	// RungBranchForce is a branch per machine, still force pushed and still
	// parentless. A host that admits only branches and tags serves this.
	RungBranchForce Rung = 2
	// RungBranchFastForward is a branch per machine whose records are
	// children of one another, so no push is a force push. It works under any
	// branch policy that allows pushes at all.
	RungBranchFastForward Rung = 3
)

// MetasystemNamespace and BranchNamespace are the two remote ref prefixes
// presence ever occupies; seat.presence-namespace names one of them.
const (
	MetasystemNamespace = "refs/metasystem/presence"
	BranchNamespace     = "refs/heads/presence"
)

// ladder is the order the publisher tries the rungs in.
var ladder = []Rung{RungMetasystemRef, RungBranchForce, RungBranchFastForward}

// Namespace is the remote ref prefix this rung publishes under.
func (r Rung) Namespace() string {
	if r == RungMetasystemRef {
		return MetasystemNamespace
	}
	return BranchNamespace
}

// Ref is the remote ref one machine's presence occupies on this rung.
func (r Rung) Ref(machine string) string { return r.Namespace() + "/" + machine }

// Force reports whether this rung's push replaces the ref without history.
func (r Rung) Force() bool { return r != RungBranchFastForward }

// Description is how a health line says which rung is carrying presence.
func (r Rung) Description() string {
	switch r {
	case RungMetasystemRef:
		return "a metasystem ref"
	case RungBranchForce:
		return "a branch per machine"
	case RungBranchFastForward:
		return "a branch per machine without force"
	default:
		return "an unknown rung"
	}
}

// Valid reports whether the rung is one of the three.
func (r Rung) Valid() bool { return r >= RungMetasystemRef && r <= RungBranchFastForward }

// Ladder is the sequence of rungs a publisher may try, starting at start.
// A pinned namespace confines the ladder to that namespace's rungs: pinning
// the metasystem ref never falls to a branch, and pinning the branch
// namespace still lets a force-push ban drop from rung 2 to rung 3, because
// that fall changes the push, not the namespace.
func Ladder(start Rung, pinned string) []Rung {
	if !start.Valid() {
		start = RungMetasystemRef
	}
	var rungs []Rung
	for _, rung := range ladder {
		if rung < start {
			continue
		}
		if pinned != "" && rung.Namespace() != pinned {
			continue
		}
		rungs = append(rungs, rung)
	}
	if len(rungs) == 0 {
		// A pin that excludes the remembered rung starts the pinned
		// namespace from its own first rung rather than publishing nothing.
		for _, rung := range ladder {
			if pinned == "" || rung.Namespace() == pinned {
				rungs = append(rungs, rung)
			}
		}
	}
	return rungs
}

// ValidatePinnedNamespace accepts only the two prefixes presence knows.
func ValidatePinnedNamespace(value string) error {
	switch strings.TrimSuffix(strings.TrimSpace(value), "/") {
	case "", MetasystemNamespace, BranchNamespace:
		return nil
	}
	return fmt.Errorf("%s must be %s or %s, got %q", PresenceNamespaceKey, MetasystemNamespace, BranchNamespace, value)
}

// PresenceNamespaceKey is the configuration key that pins the ladder. It is
// declared here so the package that reads configuration and the package that
// uses it cannot drift apart.
const PresenceNamespaceKey = "seat.presence-namespace"

// PresenceStaleMinutesKey is the reader's own stale window.
const PresenceStaleMinutesKey = "seat.presence-stale-min"

// RefRefused is a remote's answer that named the ref: git's "funny refname",
// a pre-receive hook, a branch ruleset. It moves the publisher one rung down.
// A transport failure that names no ref is an ordinary error and moves
// nothing, because the rung was not the fault.
type RefRefused struct {
	Ref    string
	Detail string
}

func (e *RefRefused) Error() string {
	return fmt.Sprintf("the remote refused %s: %s", e.Ref, e.Detail)
}

// refusalNamesRef decides, from git's own voice, whether a failed push was
// refused for the ref it named or failed for the transport.
func refusalNamesRef(ref, output string) bool {
	lowered := strings.ToLower(output)
	for _, transport := range []string{
		"could not read username", "authentication failed", "could not resolve host",
		"connection timed out", "operation timed out", "timed out", "connection refused",
		"repository not found", "permission denied (publickey)",
	} {
		if strings.Contains(lowered, transport) {
			return false
		}
	}
	for _, named := range []string{"funny refname", "remote rejected", "pre-receive hook", "protected branch", "ruleset", "cannot lock ref", "non-fast-forward"} {
		if strings.Contains(lowered, named) {
			return true
		}
	}
	return strings.Contains(output, ref)
}
