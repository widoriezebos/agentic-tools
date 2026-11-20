package goal

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
)

func platformIdentity(platform, unit, test string) string {
	id := "platform:" + platform + ":" + unit
	if test != "" {
		id += ":" + test
	}
	return id
}

func validatePlatformEntry(e *TrunkRedEntry) error {
	c := e.Closed
	if e.Class != TrunkRedClassPlatform {
		if e.Platform != "" || strings.HasPrefix(e.Identity, "platform:") || c != nil && (c.Platform != "" || c.LogPath != "" || c.LogDigest != "") {
			return errors.New("platform identity and evidence belong to a platform incident")
		}
		return nil
	}
	if !config.ValidPlatform(e.Platform) || e.TestUnit != e.Group || e.Identity != platformIdentity(e.Platform, e.Group, e.TestName) || e.Status != "failed" || len(e.Holds) > 0 || e.FixProof != nil {
		return errors.New("platform incident needs its exact platform, failing unit and test, with no holds or flake passes")
	}
	if e.TestName == "" && len(e.Failures) != 0 || e.TestName != "" && (len(e.Failures) != 1 || e.Failures[0].Classname != e.Group || e.Failures[0].Name != e.TestName) {
		return errors.New("platform incident failure differs from its identity")
	}
	for _, s := range e.Sightings {
		if !validCadenceObjectID(s.BaseCommit) || s.LogPath == "" || !validCadenceDigest(s.LogDigest) {
			return errors.New("platform sighting needs its failing commit, log path and checksum")
		}
	}
	if c != nil {
		if c.How == "green" && (c.Platform != e.Platform || !validCadenceObjectID(c.BaseCommit) || c.LogPath == "" || strings.ContainsAny(c.LogPath, "\r\n") || !validCadenceDigest(c.LogDigest)) {
			return errors.New("platform closure needs a matching check from its platform")
		}
		if c.How == "hand" && (c.Platform != "" || c.LogPath != "" || c.LogDigest != "") {
			return errors.New("a hand closure cannot claim a platform check")
		}
	}
	return nil
}

func platformFilingAllowed(e Endpoint, tip, platform string) error {
	files, err := e.repository().Files(tip, "metasystem.conf")
	if err != nil {
		return err
	}
	value, _, err := config.CommittedContentLookup(string(files["metasystem.conf"]), config.DecidingPlatformsKey)
	if err != nil {
		return err
	}
	if err := config.SettingValueProblem(config.DecidingPlatformsKey, value); err != nil {
		return err
	}
	deciding := strings.Fields(value)
	if len(deciding) == 0 {
		return fmt.Errorf("declare the deciding platforms in %s after every engine reading main supports platform incidents", config.DecidingPlatformsKey)
	}
	if slices.Contains(deciding, platform) {
		return fmt.Errorf("%s decides landings; its reds are trunk reds", platform)
	}
	return nil
}

// RecordPlatformProof imports one complete native run. A named incident asks
// for closure; otherwise each reported failure is a separate sighting.
func RecordPlatformProof(r VerbRequest, path, id string) (PublishResult, error) {
	proof, err := repoproof.ReadPlatformProof(path)
	if err != nil {
		return PublishResult{}, err
	}
	if id != "" {
		return closePlatformProof(r, id, proof)
	}
	if proof.Verdict != "red" {
		return PublishResult{}, errors.New("nothing failed in the supplied check")
	}
	args := TrunkRedRecordArgs{Class: TrunkRedClassPlatform, Platform: proof.Platform, Attempt: proof.Digest, BaseCommit: proof.Commit, SeenAt: r.stamp()}
	for unit, tests := range proof.Failed {
		if len(tests) == 0 {
			tests = []string{""}
		}
		for _, test := range tests {
			group := TrunkRedRecordGroup{Group: unit, Identity: platformIdentity(proof.Platform, unit, test), Status: "failed", LogPath: proof.Path, LogDigest: proof.Digest}
			if test != "" {
				group.Failures = []TrunkRedFailure{{Report: proof.Path, Classname: unit, Name: test, Status: "failed"}}
			}
			args.Groups = append(args.Groups, group)
		}
	}
	sort.Slice(args.Groups, func(i, j int) bool { return args.Groups[i].Identity < args.Groups[j].Identity })
	return Publish(r.Endpoint, trunkRedRecordRequest(r, args))
}

type platformClear struct {
	Entry, Unit, Test, Platform, Commit, LogPath, LogDigest, At string
}

func closePlatformProof(r VerbRequest, id string, p repoproof.PlatformProof) (PublishResult, error) {
	if p.Verdict != "green" {
		return PublishResult{}, errors.New("closing a platform incident needs a passing check")
	}
	tip, present, err := r.Endpoint.repository().Accepted()
	if err != nil {
		return PublishResult{}, err
	}
	if !present {
		return PublishResult{}, errors.New("no accepted incident register")
	}
	tree, err := loadTreeFor(r.Endpoint, tip)
	if err != nil {
		return PublishResult{}, err
	}
	entry := trunkRedByID(tree.TrunkRed, id)
	if entry == nil || entry.Class != TrunkRedClassPlatform {
		return PublishResult{}, fmt.Errorf("%s is not a platform incident", id)
	}
	if entry.Platform != p.Platform {
		return PublishResult{}, fmt.Errorf("platform %s is not %s", p.Platform, entry.Platform)
	}
	passed, ran := p.Passed[entry.Group]
	if !ran || entry.TestName != "" && !slices.Contains(passed, entry.TestName) {
		return PublishResult{}, fmt.Errorf("the check did not pass %s %s", entry.Group, entry.TestName)
	}
	args := platformClear{Entry: id, Unit: entry.Group, Test: entry.TestName, Platform: p.Platform, Commit: p.Commit, LogPath: p.Path, LogDigest: p.Digest, At: r.stamp()}
	return Publish(r.Endpoint, platformClearRequest(r, args))
}

func platformClearRequest(r VerbRequest, a platformClear) PublishRequest {
	encoded, _ := json.Marshal(a)
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "platform-clear", Targets: []string{a.Entry}, Args: map[string]string{"proof": string(encoded)}}, Message: "record closing proof for " + a.Entry,
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			entry := trunkRedByID(tree.TrunkRed, a.Entry)
			if entry == nil || entry.Class != TrunkRedClassPlatform || entry.Platform != a.Platform || entry.Group != a.Unit || entry.TestName != a.Test {
				return nil, errors.New("the incident differs from the recorded check")
			}
			if entry.Closed != nil {
				if entry.Closed.Opid != r.opid() {
					return nil, AlreadyHolds{Reason: "this platform incident is already closed"}
				}
				return nil, AlreadyApplied{}
			}
			for _, s := range entry.Sightings {
				ok, err := r.Endpoint.repository().IsAncestor(s.BaseCommit, a.Commit)
				if err != nil {
					return nil, err
				}
				if !ok || s.BaseCommit == a.Commit {
					return nil, fmt.Errorf("the closing commit must be a later descendant of %s", s.BaseCommit)
				}
			}
			entry.Closed = &TrunkRedClosure{At: a.At, Opid: r.opid(), How: "green", Attempt: a.LogDigest, BaseCommit: a.Commit, Platform: a.Platform, LogPath: a.LogPath, LogDigest: a.LogDigest}
			return trunkRedChanges(tree, nil)
		}}
}
