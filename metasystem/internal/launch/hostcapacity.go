package launch

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostcapacity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// CapacityBuilds reconciles the global store and exposes active build reservations.
func (m *Manager) CapacityBuilds() ([]hostcapacity.Build, error) {
	records, err := m.List()
	var builds []hostcapacity.Build
	for _, record := range records {
		if record.Kind == "build" && (record.State == Starting || record.State == Running) {
			builds = append(builds, hostcapacity.Build{ID: record.ID, Goal: record.Goal, Runtime: record.Adapter,
				State: string(record.State), WorkingDirectory: record.WorkingDirectory})
		}
	}
	return builds, err
}

// HostWork observes active builds and unit checks without changing their records.
func (m *Manager) HostWork() (builds, checks int, err error) {
	root, err := m.Store.root()
	if err == nil {
		_, err = os.Stat(root)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("the launch store %s cannot be read: %w", root, err)
	}
	records, err := m.EffectiveList()
	if err != nil {
		return 0, 0, fmt.Errorf("the launch store %s cannot be read: %w", root, err)
	}
	for _, record := range records {
		if record.State != Starting && record.State != Running {
			continue
		}
		if record.Kind == "build" {
			builds++
		}
		if record.Kind == "proof" {
			checks++
		}
	}
	return builds, checks, err
}

// HostLoad is the one-minute load, or "unknown", and the host.load-max this
// manager admits builds against.
func (m *Manager) HostLoad() (string, string) {
	params := m.BuildPolicy
	params.Key, params.Policy = "host.load-max", nil
	raw, _, _ := config.Get(params)
	sample := hostcapacity.Read(m.CapacityHome, m, m.Now(), m.CapacitySources).Load
	if !sample.Available {
		return "unknown", raw
	}
	return fmt.Sprintf("%g", sample.Load1m), raw
}

// createAdmitted keeps the current observation and Starting reservation under
// one host-wide lock. The lock is released before a supervisor can start.
func (m *Manager) createAdmitted(spec StartSpec, record Record) error {
	if spec.Kind != "build" && !spec.HostGated {
		return m.Store.Create(record)
	}
	buildKind := spec.Kind == "build"
	root, err := m.Store.root()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(root, "build-admission.lock"), 0600, lock.TryExclusive)
	if err != nil {
		if spec.Actor == "person" { // A person's act waits for the reservation writer.
			held, err = lock.File(filepath.Join(root, "build-admission.lock"), 0600, lock.Exclusive)
		}
		if err != nil {
			return coded("LAUNCH_BUILD_CAPACITY", "", fmt.Errorf("host build admission is unavailable: %w; host.builds and host.load-max remain in force", err))
		}
	}
	defer held.Release()
	if spec.Actor == "person" {
		return m.Store.Create(record)
	}
	if m.BuildPolicyError != nil {
		return coded("LAUNCH_BUILD_CAPACITY", "", m.BuildPolicyError)
	}
	params := m.BuildPolicy
	params.Key = "host.builds"
	var policy config.PolicyResolution
	if buildKind {
		policy, err = config.ResolvePolicy(params)
	}
	if buildKind && err != nil {
		// Seat policy state is advisory; configuration errors still hold agents.
		params.Policy = &config.PolicyContext{Readers: config.PolicyReaders{Helm: func(string) helm.State { return helm.State{} }}}
		policy.Value, _, err = config.Get(params)
	}
	if err != nil {
		return coded("LAUNCH_BUILD_CAPACITY", "", err)
	}
	params.Key, params.Policy = "host.load-max", nil
	raw, _, err := config.Get(params)
	limit, parseErr := strconv.ParseFloat(raw, 64)
	snapshot := hostcapacity.Read(m.CapacityHome, m, m.Now(), m.CapacitySources)
	facts := fmt.Sprintf("load=%g limit=%s host.builds=%s host.load-max=%s", snapshot.Load.Load1m, raw, policy.Value, raw)
	if !buildKind {
		facts = fmt.Sprintf("load=%g limit=%s host.load-max=%s", snapshot.Load.Load1m, raw, raw)
	}
	refuseBuild := func(code, cause, why string) error {
		facts := facts + " cause=" + cause
		remedy := humanauthority.PersonActRemedy("the original work build command")
		if spec.LaneMerge {
			remedy = laneRemedy(cause, raw, snapshot.Errors)
		}
		return coded(code, facts, fmt.Errorf("%s (%s); %s", why, facts, remedy))
	}
	if buildKind && policy.Value == "person" {
		return refuseBuild("LAUNCH_BUILD_PERSON", "person-policy", "host.builds=person requires the person's exact build act; ask through a question")
	}
	if err != nil || parseErr != nil || math.IsNaN(limit) || math.IsInf(limit, 0) || limit <= 0 {
		return refuseBuild("LAUNCH_BUILD_CAPACITY", "invalid-limit", "host.load-max must declare a positive finite load limit")
	}
	if !snapshot.OwnerKnown || !snapshot.BuildsKnown || !snapshot.Load.Available || math.IsNaN(snapshot.Load.Load1m) || math.IsInf(snapshot.Load.Load1m, 0) || snapshot.Load.Load1m < 0 {
		return refuseBuild("LAUNCH_BUILD_CAPACITY", "unknown-capacity", "current host capacity is unknown: "+strings.Join(snapshot.Errors, "; "))
	}
	if snapshot.Load.Load1m >= limit {
		return refuseBuild("LAUNCH_BUILD_CAPACITY", "overload", "current host load meets or exceeds host.load-max")
	}
	if buildKind && policy.Value != "auto" {
		goals := map[string]bool{}
		for _, build := range snapshot.Builds {
			key := "goal:" + build.Goal
			if build.Goal == "" {
				key = "launch:" + build.ID
			}
			goals[key] = true
		}
		key := "goal:" + spec.Goal
		if spec.Goal == "" {
			key = "launch:" + record.ID
		}
		goals[key] = true
		cap, _ := new(big.Int).SetString(policy.Value, 10)
		if cap.Cmp(big.NewInt(int64(len(goals)))) < 0 {
			return refuseBuild("LAUNCH_BUILD_CAPACITY", "build-cap", "distinct active build goals reach the host.builds cap")
		}
	}
	return m.Store.Create(record)
}

func laneRemedy(cause, limit string, errs []string) string {
	switch cause {
	case "overload":
		return "the lane waits until the host's one-minute load is below " + limit + " and retries this same command"
	case "build-cap":
		return "the lane waits for a build place and retries this same command"
	case "unknown-capacity":
		return "the lane retries when host capacity can be read: " + strings.Join(errs, "; ")
	case "person-policy":
		return "host.builds=person: " + humanauthority.PersonActRemedy("this same work build command")
	case "invalid-limit":
		return "a person declares a positive host.load-max: " + humanauthority.PersonActRemedy("metasystem settings set host.load-max N")
	default:
		return humanauthority.PersonActRemedy("the original work build command")
	}
}
