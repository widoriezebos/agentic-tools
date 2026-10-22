package goal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// FlakeRecordsOnly reports whether before and after differ only as the lane's
// observe-only flake records change them: new flake entries and appended
// sightings in the incident register, and on each live fix goal those
// sightings link, one exact "Flaky:" sentence, one record-flake history line
// and one revision per sighting. Both maps hold every changed path,
// installation-relative; a path missing on either side is not such a change.
func FlakeRecordsOnly(before, after map[string][]byte) bool {
	if len(after) == 0 || len(before) != len(after) || after[trunkRedPath] == nil {
		return false
	}
	added, ok := flakeSightingsOnly(before[trunkRedPath], after[trunkRedPath])
	if !ok {
		return false
	}
	for name, data := range after {
		old, present := before[name]
		id, live := strings.CutPrefix(name, goalsPrefix)
		id, page := strings.CutSuffix(id, ".md")
		if name != trunkRedPath && (!present || !live || !page || strings.Contains(id, "/") || !flakeSentencesOnly(old, data, id, added)) {
			return false
		}
	}
	return true
}

// FlakeRecordPath reports whether name is the register or a live goal page.
func FlakeRecordPath(name string) bool {
	id, live := strings.CutPrefix(name, goalsPrefix)
	id, page := strings.CutSuffix(id, ".md")
	return name == trunkRedPath || live && page && !strings.Contains(id, "/")
}

// flakeSighting is one sighting a register change added, with its incident.
type flakeSighting struct {
	entry    TrunkRedEntry
	sighting TrunkRedSighting
	seen     int
}

// flakeSightingsOnly returns the sightings after adds to before when every
// register change is a new open flake entry or appended flake sightings, and
// nothing else changed.
func flakeSightingsOnly(before, after []byte) ([]flakeSighting, bool) {
	old, oldProblems := ParseTrunkRed(before)
	entries, problems := ParseTrunkRed(after)
	oldCadence, oldClaim := trunkRedCadenceFields(before)
	cadence, claim := trunkRedCadenceFields(after)
	if len(oldProblems) != 0 || len(problems) != 0 || !sameJSON(oldCadence, cadence) || !sameJSON(oldClaim, claim) {
		return nil, false
	}
	priors := map[string]TrunkRedEntry{}
	for _, prior := range old {
		priors[prior.ID] = prior
	}
	var added []flakeSighting
	for _, entry := range entries {
		prior, existed := priors[entry.ID]
		delete(priors, entry.ID)
		if existed && sameJSON(prior, entry) {
			continue
		}
		kept := entry
		kept.Sightings = entry.Sightings[:min(len(prior.Sightings), len(entry.Sightings))]
		kept.FixGoal, kept.FixOpenedAt = prior.FixGoal, prior.FixOpenedAt
		if entry.EntryClass() != TrunkRedClassFlake || entry.Closed != nil || len(entry.Sightings) <= len(prior.Sightings) || existed && !sameJSON(prior, kept) {
			return nil, false
		}
		for _, sighting := range entry.Sightings[len(prior.Sightings):] {
			added = append(added, flakeSighting{entry: entry, sighting: sighting})
		}
	}
	if len(priors) != 0 {
		return nil, false
	}
	for i := range added {
		for _, fact := range FlakeFacts(entries) {
			if fact.TestUnit == added[i].entry.TestUnit && fact.TestName == added[i].entry.TestName {
				added[i].seen = 1 + slices.IndexFunc(fact.Sightings, func(s TrunkRedSighting) bool { return s.Opid == added[i].sighting.Opid })
			}
		}
	}
	return added, true
}

// flakeSentencesOnly reports whether after is before with, for each history
// line it adds, the record-flake line and the exact sentence of the sighting
// of that operation whose incident links goal id, and one revision each.
func flakeSentencesOnly(before, after []byte, id string, added []flakeSighting) bool {
	old, problems := ParseFile(before)
	if len(problems) != 0 {
		return false
	}
	got, problems := ParseFile(after)
	if len(problems) != 0 || len(got.History) <= len(old.History) || got.Revision != old.Revision+uint64(len(got.History)-len(old.History)) {
		return false
	}
	rest, kept := strings.CutPrefix(got.NextStep, strings.TrimSpace(old.NextStep))
	for _, line := range got.History[len(old.History):] {
		rest = strings.TrimPrefix(rest, " ")
		match := slices.IndexFunc(added, func(a flakeSighting) bool {
			sentence := a.sentence()
			return a.sighting.Opid == line.Opid && a.entry.FixGoal == id && a.seen > 0 && strings.HasPrefix(rest, sentence) && (len(rest) == len(sentence) || rest[len(sentence)] == ' ')
		})
		if !kept || match < 0 || !reflect.DeepEqual(line, HistoryLine{At: added[match].sighting.SeenAt, Opid: line.Opid, Verb: "record-flake", Actor: line.Actor, Targets: []string{id}, Keep: -1}) {
			return false
		}
		rest = rest[len(added[match].sentence()):]
		added = slices.Delete(slices.Clone(added), match, match+1)
	}
	if rest != "" {
		return false
	}
	want := *old
	want.Revision, want.NextStep = got.Revision, got.NextStep
	want.History = append(slices.Clone(old.History), got.History[len(old.History):]...)
	return bytes.Equal(bytes.TrimSpace(RenderFile(&want)), bytes.TrimSpace(after))
}

// sentence is the next-step sentence record-flake writes for this sighting.
func (a flakeSighting) sentence() string {
	return fmt.Sprintf("Flaky: %s %s; attempt %s; log %s; load %g; seen %d times.", a.entry.TestUnit, a.entry.TestName, a.sighting.Attempt, a.sighting.LogPath, a.sighting.Load, a.seen)
}

func sameJSON(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}
