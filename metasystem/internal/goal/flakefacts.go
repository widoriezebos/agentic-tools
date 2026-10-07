package goal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
)

// FlakeIdentity preserves the exact unit and test, including subtest names.
func FlakeIdentity(unit, test string) string {
	data, _ := json.Marshal([2]string{unit, test})
	return fmt.Sprintf("flake:%x", sha256.Sum256(data))
}

// FlakeFact keeps the current test state and every source's evidence and authority.
// Its joined identity is a read projection, never a register entry to publish.
type FlakeFact struct {
	TrunkRedEntry
	Sources []TrunkRedEntry
}

// FlakeFacts joins explicit legacy test names with ordinary test observations.
// Callers validate the register first; unreadable history is never an empty fact.
func FlakeFacts(entries []TrunkRedEntry) []FlakeFact {
	facts := map[string]FlakeFact{}
	latest := map[string]string{}
	for _, entry := range entries {
		class := entry.EntryClass()
		if class != TrunkRedClassFlake && class != TrunkRedClassKnownFlake && (class != TrunkRedClassPendingFlake || entry.Identity != "flaky:"+entry.Group) {
			continue
		}
		for _, failure := range entry.Failures {
			if failure.Name == "" {
				continue
			}
			unit, test := entry.Group, failure.Name
			if class == TrunkRedClassFlake {
				unit, test = entry.TestUnit, entry.TestName
			}
			key := FlakeIdentity(unit, test)
			fact := facts[key]
			sightings := append([]TrunkRedSighting(nil), fact.Sightings...)
			at := entry.Opened
			for _, sighting := range entry.Sightings {
				if sighting.SeenAt > at {
					at = sighting.SeenAt
				}
				if !slices.ContainsFunc(sightings, func(prior TrunkRedSighting) bool {
					return prior.Opid == sighting.Opid || prior.Attempt == sighting.Attempt
				}) {
					sightings = append(sightings, sighting)
				}
			}
			if entry.Closed != nil && entry.Closed.At >= at {
				at = entry.Closed.At
			}
			if at > latest[key] || at == latest[key] && (entry.Closed != nil || class == TrunkRedClassFlake && fact.Class != TrunkRedClassFlake) {
				fact.TrunkRedEntry, latest[key] = entry, at
				fact.Identity, fact.TestUnit, fact.TestName = key, unit, test
				fact.Failures = []TrunkRedFailure{failure}
			}
			sort.SliceStable(sightings, func(i, j int) bool { return sightings[i].SeenAt < sightings[j].SeenAt })
			fact.Sources = append(fact.Sources, entry)
			fact.Sightings = sightings
			facts[key] = fact
		}
	}
	out := make([]FlakeFact, 0, len(facts))
	for _, fact := range facts {
		sort.Slice(fact.Sources, func(i, j int) bool { return fact.Sources[i].ID < fact.Sources[j].ID })
		out = append(out, fact)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identity < out[j].Identity })
	return out
}
