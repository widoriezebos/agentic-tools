// Package fixtureauth owns fixture-identity authorization:
// the ONE reader of the fake-process fixture table,
// gated by the root's configuration — the reserved-cap predicate,
// metasystem.runtimes=fake — and issued to consumers as MINIMAL
// per-authority capability values. internal/identity stays a
// configuration-free foundation: it defines the neutral probe interface
// and this package implements it. Root authorization is NECESSARY but
// never SUFFICIENT: every consumer keeps its own guards (kernel death
// vetoes fixtures, ancestor keeps runtime=="fake", mission-process
// keeps kernel-argv-first). The boundary kills accidental and
// environmental bypass; in-process forgery by hostile Go code is out of
// scope and the doctrine says so.
package fixtureauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fixtureEnv is the fixture table's environment variable — read ONLY
// here.
const fixtureEnv = "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"

// Authorization is the root-checked fixture authority. A nil
// *Authorization is valid everywhere and refuses every fixture read —
// the fail-closed default for callers with no root.
type Authorization struct {
	tablePath string // "" = no identity fixture in play (env unset)
	// fixtureMode records that the ROOT authorizes fixtures at all —
	// independent of the identity-table env, because the
	// mission-process and process-table sources ride their OWN
	// variables.
	fixtureMode bool
}

// New constructs the authorization from a checkout root. Outcomes:
//   - env unset: an authorization whose probes all answer "no entry"
//     (fixtures simply are not in play);
//   - env set and the root's conf declares metasystem.runtimes=fake:
//     probes serve the table;
//   - env set and the conf is anything else or unreadable: a LOUD
//     error — the reserved-cap fence's rule (reservedcap.go), applied
//     at construction so a leaked fixture never rides an unfenced
//     entry point.
func New(root string) (*Authorization, error) {
	mode := FixtureModeRoot(root)
	path := os.Getenv(fixtureEnv)
	if path == "" {
		return &Authorization{fixtureMode: mode}, nil
	}
	if !mode {
		return nil, fmt.Errorf("%s is set but metasystem.runtimes is not fake in %s", fixtureEnv, root)
	}
	return &Authorization{tablePath: path, fixtureMode: true}, nil
}

// entryFor is the ONE fixture-table read. pidStartedAt is the
// canonical key; the legacy "started" spelling still parses during
// the transition.
func (a *Authorization) entryFor(pid int64) (identity.FixtureEntry, bool) {
	if a == nil || a.tablePath == "" {
		return identity.FixtureEntry{}, false
	}
	data, err := os.ReadFile(a.tablePath)
	if err != nil {
		return identity.FixtureEntry{}, false
	}
	var table map[string]struct {
		PidStartedAt *int64  `json:"pidStartedAt"`
		Started      *int64  `json:"started"`
		Command      *string `json:"command"`
		Pgid         *int64  `json:"pgid"`
		Terminal     *bool   `json:"terminal"`
	}
	if json.Unmarshal(data, &table) != nil {
		return identity.FixtureEntry{}, false
	}
	raw, present := table[strconv.FormatInt(pid, 10)]
	if !present {
		return identity.FixtureEntry{}, false
	}
	entry := identity.FixtureEntry{}
	if raw.PidStartedAt != nil {
		entry.StartedAt, entry.HasStartedAt = *raw.PidStartedAt, true
	} else if raw.Started != nil {
		entry.StartedAt, entry.HasStartedAt = *raw.Started, true
	}
	if raw.Command != nil {
		entry.Command, entry.HasCommand = *raw.Command, true
	}
	if raw.Pgid != nil {
		entry.Pgid, entry.HasPgid = *raw.Pgid, true
	}
	if raw.Terminal != nil {
		entry.Terminal, entry.HasTerminal = *raw.Terminal, true
	}
	return entry, true
}

// IdentityProbe serves identity reads (census verbs, lease
// classification, custodian verdicts). It implements
// identity.FixtureProbe.
type IdentityProbe struct{ a *Authorization }

func (a *Authorization) Identity() IdentityProbe { return IdentityProbe{a} }

func (p IdentityProbe) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	return p.a.entryFor(pid)
}

// CommandProbe serves fixture command lookup only (mission-runner
// host-start verification) — never group ownership.
type CommandProbe struct{ a *Authorization }

func (a *Authorization) Command() CommandProbe { return CommandProbe{a} }

func (p CommandProbe) FixtureCommand(pid int64) (string, bool) {
	entry, ok := p.a.entryFor(pid)
	if !ok || !entry.HasCommand {
		return "", false
	}
	return entry.Command, true
}

// GroupOwnershipGrant is the SIGNAL-authorizing authority (the
// runner's group-ownership proof); granted only to the runner's
// signal path and never bundled with command lookup.
type GroupOwnershipGrant struct{ a *Authorization }

func (a *Authorization) GroupOwnership() GroupOwnershipGrant { return GroupOwnershipGrant{a} }

// FixtureGroup returns the fixture's recorded (pgid, command) for the
// ownership proof — and only while the fixture leader is KERNEL-LIVE
// at its recorded start and still in that group (a stale row must
// never authorize signaling a recycled group).
func (g GroupOwnershipGrant) FixtureGroup(pid int64) (pgid int64, command string, ok bool) {
	entry, present := g.a.entryFor(pid)
	if !present || !entry.HasPgid || !entry.HasCommand {
		return 0, "", false
	}
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		return 0, "", false
	}
	if entry.HasStartedAt && exact.StartedAt.Unix() != entry.StartedAt {
		return 0, "", false
	}
	if kernelPgid, pgErr := unix.Getpgid(int(pid)); pgErr != nil || int64(kernelPgid) != entry.Pgid {
		return 0, "", false
	}
	return entry.Pgid, entry.Command, true
}

// AncestorProbe serves the synthetic census ancestor; the consumer
// keeps its runtime=="fake" guard on top.
type AncestorProbe struct{ a *Authorization }

func (a *Authorization) Ancestor() AncestorProbe { return AncestorProbe{a} }

// Allows reports whether fixture ancestry may be honored at all —
// root fixture mode, its own env var read by the consumer.
func (p AncestorProbe) Allows() bool { return p.a != nil && p.a.fixtureMode }

// MissionProcessProbe serves the mission-process identity fallback;
// the consumer keeps its kernel-argv-first order.
type MissionProcessProbe struct{ a *Authorization }

func (a *Authorization) MissionProcess() MissionProcessProbe { return MissionProcessProbe{a} }

// Allows gates the mission-process source (its OWN env var, read by
// the consumer): the ROOT's fixture mode authorizes it, not the
// identity table's presence.
func (p MissionProcessProbe) Allows() bool { return p.a != nil && p.a.fixtureMode }

// PublicationGrant authorizes the mission runner's fixture WRITES —
// its own grant, never implied by any read capability.
type PublicationGrant struct{ a *Authorization }

func (a *Authorization) Publication() PublicationGrant { return PublicationGrant{a} }

// TablePath returns the writable fixture path, or ok=false when
// publication is not authorized.
func (g PublicationGrant) TablePath() (string, bool) {
	if g.a == nil || g.a.tablePath == "" {
		return "", false
	}
	return g.a.tablePath, true
}

// ProcessTableProbe authorizes the census process-table fixture
// source (METASYSTEM_CENSUS_PROCESS_FILE selection and parsing).
type ProcessTableProbe struct{ a *Authorization }

func (a *Authorization) ProcessTable() ProcessTableProbe { return ProcessTableProbe{a} }

func (p ProcessTableProbe) Allows() bool { return p.a != nil && p.a.fixtureMode }

// MissionHolderProbe serves dispatch.ValidateMission: command plus
// process-group facts, the two authorities mission-join needs.
type MissionHolderProbe struct{ a *Authorization }

func (a *Authorization) MissionHolder() MissionHolderProbe { return MissionHolderProbe{a} }

func (p MissionHolderProbe) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	return p.a.entryFor(pid)
}

// FixtureModeRoot is the ONE fixture-mode predicate (the reserved-cap
// rule): the root's conf declares metasystem.runtimes=fake. The
// env-independent gates — the census process-table file and the
// synthetic ancestor pin, which ride their own environment variables —
// consult this instead of open-coding the conf read.
func FixtureModeRoot(root string) bool {
	return config.ConfValue(filepath.Join(root, "metasystem.conf"), "metasystem.runtimes", "") == "fake"
}
