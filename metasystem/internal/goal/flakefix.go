package goal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// FlakeFix carries the steward's proposed fix and the evidence it answers.
type FlakeFix struct {
	Identity, ID, Intent, Basis, Next string
	Tier, Priority                    uint8
}

func flakeFixRequest(r VerbRequest, args FlakeRecordArgs) (PublishRequest, error) {
	fix := *args.Fix
	if fix.Tier != 3 || fix.Priority != 1 || fix.Identity == "" || fix.Basis == "" {
		return PublishRequest{}, fmt.Errorf("a lane flake fix needs its test identity, sightings, tier 3 and top priority")
	}
	if len(fix.ID) > 100 {
		fix.ID = strings.TrimRight(fix.ID[:91], "-") + fmt.Sprintf("-%x", sha256.Sum256([]byte(fix.Identity)))[:9]
	}
	args.Fix = &fix
	encoded, _ := json.Marshal(args)
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:   Intent{Verb: "record-flake", Args: map[string]string{"flake": string(encoded), "at": r.stamp()}},
		Message:  "open flaky test fix " + fix.ID,
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			for _, fact := range FlakeFacts(tree.TrunkRed) {
				if fact.Identity != fix.Identity {
					continue
				}
				if tree.Live[fact.FixGoal] != nil {
					return nil, AlreadyApplied{}
				}
				if fact.Closed != nil || fact.EntryClass() != TrunkRedClassFlake || len(fact.Sightings) < 2 {
					return nil, fmt.Errorf("the test has fewer than two current confirmed sightings")
				}
				risk := &RiskRecord{Severity: 3, Novelty: 1, Exposure: 3, Accumulation: 3, Basis: fix.Basis}
				request, err := openRequest(r, fix.ID, fix.Intent, OriginMain, fix.Next, nil, nil, fix.Tier, nil, risk, "", false, false, nil)
				if tree.Exists(fix.ID) {
					request, err = reopenRequest(r, fix.ID), nil
				}
				if err != nil {
					return nil, err
				}
				changes, err := request.Mutate(tip)
				if err != nil {
					return nil, err
				}
				for i := range changes {
					if changes[i].Path != livePath(fix.ID) {
						continue
					}
					file, problems := ParseFile(changes[i].Content)
					if len(problems) != 0 {
						return nil, fmt.Errorf("the flake fix record is invalid: %v", problems)
					}
					file.Priority, file.Sequence = fix.Priority, uint64(len(priorityIDs(tree.Live, fix.Priority, fix.ID))+1)
					file.NextStep = fix.Next
					changes[i].Content = RenderFile(file)
				}
				for i := range tree.TrunkRed {
					for _, source := range fact.Sources {
						if tree.TrunkRed[i].ID == source.ID {
							tree.TrunkRed[i].FixGoal, tree.TrunkRed[i].FixOpenedAt = fix.ID, r.stamp()
						}
					}
				}
				return append(changes, Change{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}), nil
			}
			return nil, fmt.Errorf("the confirmed flaky test is absent from the register")
		},
	}, nil
}
