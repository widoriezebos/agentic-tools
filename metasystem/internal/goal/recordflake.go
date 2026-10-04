package goal

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// FlakeRecordArgs describes a failed check followed by a passing repeat on one tree.
type FlakeRecordArgs struct {
	Unit, Batch, Commit, Tree, Attempt, LogPath, LogDigest string
	Tests, Surfaces                                        []string
	Load                                                   float64
	Repeat                                                 string
	Rerun                                                  TrunkRedRerun
}

// FlakeRecordResult names the fix and the number of sightings in the register.
type FlakeRecordResult struct {
	FixGoal   string
	Action    string // opened | reopened | extended
	Sightings int
	Publish   PublishResult
}

// RecordFlake records the failed check and routes its fix in one publication.
func RecordFlake(r VerbRequest, args FlakeRecordArgs) (FlakeRecordResult, error) {
	return recordFlake(r, args, Publish)
}

func recordFlake(r VerbRequest, args FlakeRecordArgs, publish func(Endpoint, PublishRequest) (PublishResult, error)) (FlakeRecordResult, error) {
	req, err := flakeRecordRequest(r, args)
	if err != nil {
		return FlakeRecordResult{}, err
	}
	pub, err := publish(r.Endpoint, req)
	result := FlakeRecordResult{Publish: pub}
	if err != nil {
		return result, err
	}
	if pub.Outcome != OutcomeConfirmed {
		return result, fmt.Errorf("the flaky test was not recorded in the shared goal list (%s): %s", pub.Outcome, pub.Detail)
	}
	// Read the confirmed tree so a replay returns the same routing information.
	tree, err := loadTreeFor(r.Endpoint, pub.Tip)
	if err != nil {
		return result, err
	}
	for _, entry := range tree.TrunkRed {
		if entry.Identity != "flaky:"+args.Unit || !trunkRedSightingExists([]TrunkRedEntry{entry}, entry.Identity, r.opid()) {
			continue
		}
		result.FixGoal, result.Sightings, result.Action = entry.FixGoal, len(entry.Sightings), "extended"
		f := tree.Live[entry.FixGoal]
		if f == nil {
			f, _ = tree.Archived(entry.FixGoal)
		}
		if f == nil {
			return result, fmt.Errorf("the recorded flaky test has no fix goal %s", entry.FixGoal)
		}
		for _, h := range f.History {
			if h.Opid == r.opid() && h.Verb == "open" {
				result.Action = "opened"
			}
			if h.Opid == r.opid() && h.Verb == "reopen" {
				result.Action = "reopened"
			}
		}
		return result, nil
	}
	return result, fmt.Errorf("the shared goal list did not retain the flaky test sighting for %s", args.Unit)
}

func flakeRecordRequest(r VerbRequest, args FlakeRecordArgs) (PublishRequest, error) {
	if strings.TrimSpace(args.Unit) == "" || len(args.Tests) == 0 || r.Now.IsZero() {
		return PublishRequest{}, fmt.Errorf("recording a flaky test needs its unit, failed tests, and the time it was seen")
	}
	if args.Repeat != "alone" && args.Repeat != "whole" {
		return PublishRequest{}, fmt.Errorf("a flaky test repeat must have run alone or as a whole check")
	}
	if math.IsNaN(args.Load) || math.IsInf(args.Load, 0) || args.Load < 0 {
		return PublishRequest{}, fmt.Errorf("a flaky test sighting needs a finite, non-negative load")
	}
	failures := make([]TrunkRedFailure, 0, len(args.Tests))
	for _, name := range sortedUnique(args.Tests) {
		failures = append(failures, TrunkRedFailure{Report: args.LogPath, Classname: args.Unit, Name: name, Status: "failed"})
	}
	red := TrunkRedRecordArgs{Batch: args.Batch, Attempt: args.Attempt, BaseCommit: args.Commit, BaseTree: args.Tree,
		SeenAt: r.stamp(), Class: TrunkRedClassPendingFlake, Where: "tip", Tree: args.Tree, Rerun: &args.Rerun,
		Groups: []TrunkRedRecordGroup{{Identity: "flaky:" + args.Unit, Group: args.Unit, Status: "failed", LogPath: args.LogPath, LogDigest: args.LogDigest, Failures: failures}}}
	if err := validateTrunkRedRecordClass(red); err != nil {
		return PublishRequest{}, err
	}
	encoded, _ := json.Marshal(args)
	req := trunkRedRecordRequest(r, red)
	record := req.Mutate
	req.Intent = Intent{Verb: "record-flake", Args: map[string]string{"flake": string(encoded), "at": r.stamp()}}
	req.Message = "record flaky test " + args.Unit
	req.Mutate = func(tip string) ([]Change, error) {
		tree, err := loadTreeFor(r.Endpoint, tip)
		if err != nil {
			return nil, err
		}
		changes, err := record(tip)
		if err != nil {
			return nil, err
		}
		entries, problems := ParseTrunkRed(changes[0].Content)
		if len(problems) != 0 {
			return nil, fmt.Errorf("the flaky test register could not be read: %v", problems)
		}
		entry := openTrunkRedByIdentity(entries, red.Groups[0].Identity)
		for _, failure := range failures {
			found := false
			for _, prior := range entry.Failures {
				if prior.Name == failure.Name {
					found = true
					break
				}
			}
			if !found {
				entry.Failures = append(entry.Failures, failure)
			}
		}
		sighting := &entry.Sightings[len(entry.Sightings)-1]
		sighting.Load, sighting.Surfaces, sighting.Repeat = args.Load, args.Surfaces, args.Repeat
		seen := fmt.Sprintf("seen %d times.", len(entry.Sightings))
		if len(entry.Sightings) == 1 {
			seen = "seen once."
		}
		line := fmt.Sprintf("Flaky: %s (%s) failed in the lane's check of %s on %s, passed when repeated (%s); load %g; log %s; %s",
			args.Unit, strings.Join(sortedUnique(args.Tests), " "), args.Commit, r.Now.In(time.Local).Format("2006-01-02"), args.Repeat, args.Load, args.LogPath, seen)
		id := entry.FixGoal
		if id == "" {
			id = "fix-flaky-" + strings.ReplaceAll(args.Unit, "/", "-")
		}
		f := tree.Live[id]
		if f == nil {
			opened := entry.FixGoal == "" && !tree.Exists(id)
			goalReq := reopenRequest(r, id)
			if opened {
				// The lane may open one unapproved fix for a flaky unit.
				goalReq, err = openRequest(r, id, "Fix intermittent test failures in "+args.Unit+".", OriginMain, line, nil, nil, 1, nil, nil, "", false, false, nil)
				if err != nil {
					return nil, err
				}
			}
			goalChanges, err := goalReq.Mutate(tip)
			if err != nil {
				return nil, err
			}
			for i := range goalChanges {
				if goalChanges[i].Path != livePath(id) {
					continue
				}
				f, problems = ParseFile(goalChanges[i].Content)
				if len(problems) != 0 {
					return nil, fmt.Errorf("the flaky test's fix goal could not be read: %v", problems)
				}
				if opened {
					f.Priority, f.Sequence = 1, uint64(len(priorityIDs(tree.Live, 1, id))+1)
				} else {
					f.NextStep = appendNextStep(f.NextStep, line)
				}
				goalChanges[i].Content = RenderFile(f)
			}
			changes = append(changes, goalChanges...)
		} else {
			f.NextStep = appendNextStep(f.NextStep, line)
			touch(f, r, "record-flake", []string{id})
			changes = append(changes, Change{Path: livePath(id), Content: RenderFile(f)})
		}
		entry.FixGoal = id
		changes[0].Content = renderTrunkRedState(entries, tree.Cadence, tree.CadenceClaim)
		return changes, nil
	}
	return req, nil
}
