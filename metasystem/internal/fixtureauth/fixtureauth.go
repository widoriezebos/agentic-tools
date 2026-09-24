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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fixtureEnv is the fixture table's environment variable — read ONLY
// here.
const fixtureEnv = "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"

const (
	goalNowEnv       = "METASYSTEM_GOAL_NOW"
	goalBootIDEnv    = "METASYSTEM_GOAL_BOOT_ID"
	goalBootNanosEnv = "METASYSTEM_GOAL_BOOT_NANOS"
)

// Authorization is the root-checked fixture authority. A nil
// *Authorization is valid everywhere and refuses every fixture read —
// the fail-closed default for callers with no root.
type Authorization struct {
	tablePath string // "" = no identity fixture in play (env unset)
	root      string // canonical checkout root this authorization was issued for
	// fixtureMode records that the ROOT authorizes fixtures at all —
	// independent of the identity-table env, because the
	// mission-process and process-table sources ride their OWN
	// variables.
	fixtureMode bool
}

// New constructs the authorization from a checkout root. Outcomes:
//   - fixture table env unset: an authorization whose identity probes answer
//     "no entry";
//   - fixture table env set and layered config declares
//     metasystem.runtimes=fake: probes serve the table;
//   - fixture table env set but the config is non-fake or invalid: an error;
//   - fixture clock requested with invalid runtime config: an error, except
//     for an absent config, which remains a production root and ignores the
//     clock environment variable.
func New(root string) (*Authorization, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	absRoot = filepath.Clean(absRoot)
	mode, modeErr := fixtureModeRoot(root)
	path := os.Getenv(fixtureEnv)
	clockRequest := os.Getenv(goalNowEnv) != "" || os.Getenv(goalBootIDEnv) != "" || os.Getenv(goalBootNanosEnv) != ""
	if modeErr != nil && (path != "" || clockRequest && !errors.Is(modeErr, os.ErrNotExist)) {
		request := fixtureEnv
		if path == "" {
			request = "fixture clock"
		}
		return nil, fmt.Errorf("%s is set but fixture runtime configuration is invalid: %w", request, modeErr)
	}
	if path == "" {
		return &Authorization{root: absRoot, fixtureMode: mode}, nil
	}
	if !mode {
		return nil, fmt.Errorf("%s is set but metasystem.runtimes is not fake in %s", fixtureEnv, root)
	}
	return &Authorization{tablePath: path, root: absRoot, fixtureMode: true}, nil
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
		PidStartedAt           *int64  `json:"pidStartedAt"`
		Started                *int64  `json:"started"`
		PidStartedAtExactMicro *int64  `json:"pidStartedAtExactMicro"`
		PidStartTicks          *int64  `json:"pidStartTicks"`
		BootID                 *string `json:"bootId"`
		Command                *string `json:"command"`
		Pgid                   *int64  `json:"pgid"`
		Terminal               *bool   `json:"terminal"`
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
	if raw.PidStartedAtExactMicro != nil {
		entry.StartedAtExactMicro, entry.HasStartedAtExactMicro = *raw.PidStartedAtExactMicro, true
	}
	if raw.PidStartTicks != nil {
		entry.StartTicks, entry.HasStartTicks = *raw.PidStartTicks, true
	}
	if raw.BootID != nil {
		entry.BootID, entry.HasBootID = *raw.BootID, true
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

// AllowsRecordedGroupProof lets a fake checkout use the exact launch proof in
// a job record when the kernel cannot enumerate the group's current argv.
func (g GroupOwnershipGrant) AllowsRecordedGroupProof() bool {
	return g.a != nil && g.a.fixtureMode
}

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

// ClockProbe is the fixture-only goal clock authority. Production roots
// ignore the environment variable completely, including malformed values.
type ClockProbe struct{ a *Authorization }

func (a *Authorization) Clock() ClockProbe { return ClockProbe{a} }

// GoalNow returns the configured fixture instant when this root explicitly
// runs the fake runtime. The boolean is false when the wall clock remains
// authoritative.
func (p ClockProbe) GoalNow() (time.Time, bool, error) {
	if p.a == nil || !p.a.fixtureMode {
		return time.Time{}, false, nil
	}
	raw := os.Getenv(goalNowEnv)
	if raw == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s must be an RFC3339 timestamp: %v", goalNowEnv, err)
	}
	return parsed.UTC(), true, nil
}

// GoalBootClock returns a fixture boot identity and monotonic elapsed value as
// one indivisible clock sample. Both values must be present together.
func (p ClockProbe) GoalBootClock() (string, time.Duration, bool, error) {
	if p.a == nil || !p.a.fixtureMode {
		return "", 0, false, nil
	}
	bootID, rawNanos := os.Getenv(goalBootIDEnv), os.Getenv(goalBootNanosEnv)
	if bootID == "" && rawNanos == "" {
		return "", 0, false, nil
	}
	if bootID == "" || rawNanos == "" {
		return "", 0, false, fmt.Errorf("%s and %s must be set together", goalBootIDEnv, goalBootNanosEnv)
	}
	nanos, err := strconv.ParseInt(rawNanos, 10, 64)
	if err != nil || nanos < 0 {
		return "", 0, false, fmt.Errorf("%s must be a non-negative duration in nanoseconds", goalBootNanosEnv)
	}
	return bootID, time.Duration(nanos), true, nil
}

// GoalHumanAuthorityProbe is the fixture-only grant for a human-reserved goal
// mutation. It is deliberately independent of the fixture clock: a fixture
// approval must not acquire a wall-clock or governance-horizon dependency.
type GoalHumanAuthorityProbe struct{ a *Authorization }

func (a *Authorization) GoalHumanAuthority() GoalHumanAuthorityProbe {
	return GoalHumanAuthorityProbe{a}
}

// Allows binds the grant to the exact root that authorized fake-runtime
// fixtures. A grant issued for one fixture checkout cannot authorize another.
func (p GoalHumanAuthorityProbe) Allows(root string) bool {
	if p.a == nil || !p.a.fixtureMode {
		return false
	}
	absRoot, err := filepath.Abs(root)
	return err == nil && filepath.Clean(absRoot) == p.a.root
}

// MissionHolderProbe serves dispatch.ValidateMission: command plus
// process-group facts, the two authorities mission-join needs.
type MissionHolderProbe struct{ a *Authorization }

func (a *Authorization) MissionHolder() MissionHolderProbe { return MissionHolderProbe{a} }

func (p MissionHolderProbe) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	return p.a.entryFor(pid)
}

// FixtureModeRoot is the ONE fixture-mode predicate (the reserved-cap
// rule): the root's committed or local config declares metasystem.runtimes=fake.
// Environment overrides cannot grant fixture mode. The
// env-independent gates — the census process-table file and the
// synthetic ancestor pin, which ride their own environment variables —
// consult this instead of open-coding the conf read.
func FixtureModeRoot(root string) bool {
	mode, _ := fixtureModeRoot(root)
	return mode
}

func fixtureModeRoot(root string) (bool, error) {
	value, _, err := config.Get(config.GetParams{
		Key: "metasystem.runtimes", ConfPath: filepath.Join(root, "metasystem.conf"),
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	return err == nil && value == "fake", err
}
