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

// createAdmitted keeps the current observation and Starting reservation under
// one host-wide lock. The lock is released before a supervisor can start.
func (m *Manager) createAdmitted(spec StartSpec, record Record) error {
	if spec.Kind != "build" {
		return m.Store.Create(record)
	}
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
	policy, err := config.ResolvePolicy(params)
	if err != nil {
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
	refuse := func(code, why string) error {
		return coded(code, facts, fmt.Errorf("%s (%s); %s", why, facts, humanauthority.PersonActRemedy("the original work build command")))
	}
	if policy.Value == "person" {
		return refuse("LAUNCH_BUILD_PERSON", "host.builds=person requires the person's exact build act; ask through a question")
	}
	if err != nil || parseErr != nil || math.IsNaN(limit) || math.IsInf(limit, 0) || limit <= 0 {
		return refuse("LAUNCH_BUILD_CAPACITY", "host.load-max must declare a positive finite load limit")
	}
	if !snapshot.OwnerKnown || !snapshot.BuildsKnown || !snapshot.Load.Available || math.IsNaN(snapshot.Load.Load1m) || math.IsInf(snapshot.Load.Load1m, 0) || snapshot.Load.Load1m < 0 {
		return refuse("LAUNCH_BUILD_CAPACITY", "current host capacity is unknown: "+strings.Join(snapshot.Errors, "; "))
	}
	if snapshot.Load.Load1m >= limit {
		return refuse("LAUNCH_BUILD_CAPACITY", "current host load meets or exceeds host.load-max")
	}
	if policy.Value != "auto" {
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
			return refuse("LAUNCH_BUILD_CAPACITY", "distinct active build goals reach the host.builds cap")
		}
	}
	return m.Store.Create(record)
}
