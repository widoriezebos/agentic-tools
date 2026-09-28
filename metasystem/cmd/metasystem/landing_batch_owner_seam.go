package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

var _ batch.FlakeLedgerOwner = ledgerTrunkRedOwner{}

// OpenByClass returns the open register entries of the named classes.
func (owner ledgerTrunkRedOwner) OpenByClass(classes ...string) ([]batch.OpenEntry, error) {
	open, err := owner.Open()
	return slices.DeleteFunc(open, func(entry batch.OpenEntry) bool { return !slices.Contains(classes, entry.Class) }), err
}

// RecordPending opens or sights a pending flake from a tip red and its passing rerun.
func (owner ledgerTrunkRedOwner) RecordPending(opid string, sighting batch.FlakeSighting) ([]batch.EntryRef, error) {
	args := flakeRecordArgs(sighting)
	args.Class, args.Where, args.Tree = goal.TrunkRedClassPendingFlake, "tip", sighting.TipTree
	return owner.publishRecord(opid, func(request goal.VerbRequest) (goal.PublishResult, error) { return goal.RecordTrunkRed(request, args) })
}

// Promote records main's red-then-green: a known flake opens or a pending one is promoted.
func (owner ledgerTrunkRedOwner) Promote(opid string, promotion batch.Promotion) ([]batch.EntryRef, error) {
	args := flakeRecordArgs(promotion.FlakeSighting)
	if promotion.TipTree != "" {
		args.Where, args.Tree = "tip", promotion.TipTree
	}
	args.Approver, args.OwnerMachine = promotion.Owner, promotion.OwnerMachine
	return owner.publishRecord(opid, func(request goal.VerbRequest) (goal.PublishResult, error) {
		return goal.PromoteKnownFlake(request, args, promotion.Location)
	})
}

// RecordHang opens or sights a hang entry with the watchdog's evidence.
func (owner ledgerTrunkRedOwner) RecordHang(opid string, hang batch.HangSighting) ([]batch.EntryRef, error) {
	evidence := hang.Evidence
	group := hang.Group
	args := goal.TrunkRedRecordArgs{Class: goal.TrunkRedClassHang, Batch: hang.BatchID, Attempt: hang.AttemptID, BaseCommit: hang.BaseCommit,
		BaseTree: hang.BaseTree, SeenAt: trunkRedStamp(hang.SeenAt), Sample: loadSampleText(hang.Sample),
		Groups: []goal.TrunkRedRecordGroup{{Identity: batch.TrunkRedID(group), Group: group.ID, Status: group.Status, NotRunReason: group.NotRunReason,
			LogPath: group.LogPath, LogDigest: group.LogDigest, Failures: []goal.TrunkRedFailure{},
			Hang: &goal.TrunkRedHang{EvidenceDir: evidence.EvidenceDir, Dump: evidence.Dump, Section: evidence.Section, LastStartedTest: evidence.LastStartedTest,
				LongestSilentSeconds: evidence.LongestSilentSeconds, LongestZeroCPUSeconds: evidence.LongestZeroCPUSeconds}}}}
	if hang.TipTree != "" {
		args.Where, args.Tree = "tip", hang.TipTree
	}
	return owner.publishRecord(opid, func(request goal.VerbRequest) (goal.PublishResult, error) { return goal.RecordTrunkRed(request, args) })
}

// flakeRecordArgs keys each identified failing test as its own identity.
func flakeRecordArgs(sighting batch.FlakeSighting) goal.TrunkRedRecordArgs {
	args := goal.TrunkRedRecordArgs{Batch: sighting.BatchID, Attempt: sighting.RedAttempt, BaseCommit: sighting.BaseCommit,
		BaseTree: sighting.BaseTree, SeenAt: trunkRedStamp(sighting.SeenAt), Sample: loadSampleText(sighting.RedSample), Groups: []goal.TrunkRedRecordGroup{}}
	if sighting.GreenAttempt != "" {
		args.Rerun = &goal.TrunkRedRerun{Attempt: sighting.GreenAttempt, LogPath: sighting.GreenLogPath, LogDigest: sighting.GreenLogDigest,
			Sample: loadSampleText(sighting.GreenSample)}
	}
	for _, group := range sighting.Groups {
		for _, failure := range group.Failures {
			if failure.Status != "failed" {
				continue
			}
			args.Groups = append(args.Groups, goal.TrunkRedRecordGroup{Identity: batch.FlakeID(group.ID, failure), Group: group.ID,
				Status: group.Status, LogPath: group.LogPath, LogDigest: group.LogDigest,
				Failures: []goal.TrunkRedFailure{{Report: failure.Report, Classname: failure.Classname, Name: failure.Name, Status: failure.Status, Reason: failure.Reason}}})
		}
	}
	return args
}

func (owner ledgerTrunkRedOwner) publishRecord(opid string, publish func(goal.VerbRequest) (goal.PublishResult, error)) ([]batch.EntryRef, error) {
	request, err := owner.request(opid)
	if err != nil {
		return nil, err
	}
	result, err := owner.runJournaled(opid, func() (goal.PublishResult, error) { return publish(request) })
	if err != nil {
		return nil, err
	}
	projection, err := goal.ProjectAtEndpoint(owner.endpoint, result.Tip, request.Now)
	if err != nil {
		return nil, err
	}
	var refs []batch.EntryRef
	for _, ref := range goal.TrunkRedRefs(projection.Tree, opid) {
		refs = append(refs, batch.EntryRef{ID: ref.ID, Group: ref.Group})
	}
	return refs, nil
}

func trunkRedStamp(at time.Time) string { return at.UTC().Truncate(time.Second).Format(time.RFC3339) }

func loadSampleText(sample proofrun.LoadSample) string {
	return fmt.Sprintf("at=%s load1m=%.2f cores=%d available=%t overlapping-host=%d", sample.At, sample.Load1m, sample.Cores, sample.Available, sample.OverlappingHost)
}
