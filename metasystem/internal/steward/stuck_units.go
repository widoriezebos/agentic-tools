package steward

import (
	"fmt"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// StuckUnits reports only units of the goals this machine claims. Empty
// roots and nil dependencies select the production stores and ledger.
type StuckUnits struct {
	UnitRoot string
	Store    launch.Store
	Claims   func(repoRoot string, now time.Time) (machine string, goals map[string]bool, err error)
	Deliver  func(repoRoot, message string) error
}

// StuckUnitLimits reads the serving seat's positive bounds for alerts and cards.
func StuckUnitLimits(repoRoot string) (launch.UnitStuckLimits, error) {
	var limits launch.UnitStuckLimits
	for key, target := range map[string]*int{
		"steward.stuck.build-min": &limits.BuildMin, "steward.stuck.proof-min": &limits.ProofMin,
		"steward.stuck.read-min": &limits.ReadMin, "steward.stuck.rounds": &limits.Rounds,
	} {
		value, err := boundedConfig(repoRoot, key, config.MustIntDefault(key), 1)
		if err != nil {
			return limits, err
		}
		*target = value
	}
	return limits, nil
}

func stuckClaims(repoRoot string, now time.Time) (string, map[string]bool, error) {
	snapshot, err := readClaimedDeliverySnapshot(repoRoot, newHealthLedger(repoRoot, now))
	if err != nil {
		return "", nil, err
	}
	if !snapshot.converted || snapshot.machine == "" || snapshot.projection.Tree == nil {
		return "", nil, fmt.Errorf("the claimed-goal ledger or machine identity is unreadable")
	}
	claims := map[string]bool{}
	for id, file := range snapshot.projection.Tree.Live {
		if file != nil && file.State == goal.StateClaimed && file.Claimed != nil && file.Claimed.Machine == snapshot.machine {
			claims[id] = true
		}
	}
	return snapshot.machine, claims, nil
}

// Run shares the per-work alert lifecycle without replacing the lane's
// pattern state. A single clean reading also releases a person's dismissal.
func (pass StuckUnits) Run(repoRoot string, now time.Time) error {
	episodes, err := AlertEpisodes(repoRoot)
	if err != nil {
		return err
	}
	observations := map[string]PatternObservation{}
	for _, episode := range episodes {
		if episode.Owner == PatternOwner("stuck-unit") && (!episode.Cleared || episode.Suppressed) {
			observations[episode.ScopeID] = PatternObservation{Work: episode.ScopeID, Kind: ObsClear}
		}
	}
	claims := pass.Claims
	if claims == nil {
		claims = stuckClaims
	}
	machine, goals, readErr := claims(repoRoot, now)
	limits, limitsErr := StuckUnitLimits(repoRoot)
	standings, storeErr := launch.UnitStandings(pass.UnitRoot, pass.Store, limits, now)
	unknown := readErr != nil || machine == "" || limitsErr != nil || storeErr != nil
	for _, standing := range standings {
		if standing.Unreadable != "" && (standing.Goal == "" || standing.Unit == "") {
			unknown = true
		}
	}
	if unknown {
		for work, observation := range observations {
			observation.Kind = ObsUnknown
			observations[work] = observation
		}
	} else {
		for _, standing := range standings {
			work := standing.Goal + "/" + standing.Unit
			if standing.Unreadable != "" {
				observations[work] = PatternObservation{Work: work, Kind: ObsUnknown}
				continue
			}
			if !goals[standing.Goal] {
				continue
			}
			if !standing.Stuck || observations[work].Kind == ObsUnknown {
				continue
			}
			message := fmt.Sprintf("Seat %s is on round %d of unit %s of goal %s (limit %d)", machine, standing.Rounds, standing.Unit, standing.Goal, standing.Limit)
			fact := fmt.Sprintf("rounds %d of %d, limit %d", standing.Rounds, standing.MaxRounds, standing.Limit)
			if standing.Launch != "" {
				message = fmt.Sprintf("Seat %s has run the %s step of unit %s of goal %s for %d minutes (limit %d, round %d of %d): stop it with `metasystem work stop j1:%s`", machine, standing.Kind, standing.Unit, standing.Goal, standing.Minutes, standing.Limit, standing.Rounds, standing.MaxRounds, standing.Launch)
				fact = fmt.Sprintf("%s %s %d min, limit %d", standing.Kind, standing.Step, standing.Minutes, standing.Limit)
			}
			message += ", cut the unit, or land with notes (`metasystem work land --message`)."
			observations[work] = PatternObservation{Work: work, Kind: ObsFinding, Since: standing.StartedAt, Message: message,
				Evidence: []AlertEvidence{{Record: standing.Path, At: now.UTC().Format(time.RFC3339), Fact: fact}}}
		}
	}
	_, err = UpdatePatterns(repoRoot, PatternCycle{Now: now, ClearTicks: 1, Deliver: pass.Deliver,
		Step: func([]byte) ([]byte, []PatternRun, error) {
			var all []PatternObservation
			var works []string
			for work := range observations {
				works = append(works, work)
			}
			sort.Strings(works)
			for _, work := range works {
				all = append(all, observations[work])
			}
			return nil, []PatternRun{{Pattern: "stuck-unit", Observations: all}}, nil
		}})
	return err
}
