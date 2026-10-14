package goal

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// TransferCoverage names the inherited findings and retained source changes
// explicitly covered by a validated clean destination attestation.
type TransferCoverage struct {
	TargetUnit, ReadID, Commit string
	Findings, SourceCommits    []string
}

func TransferBrief(goal string, obligation ReviewObligation, estimate int64, requirement, reads, dispositions, history []byte) string {
	return fmt.Sprintf("Build unit %s for goal %s.\nRequired source: %s at %s.\nRetain its complete change and resolve every inherited finding.\n\n| Unit | Purpose | Estimated changed lines |\n| --- | --- | --- |\n| %s | Complete the retained source requirement | %d |\n\nOriginal requirement:\n%s\n\nRead the retained source change with:\n`git show --format= --binary %s`\n\nFindings:\n%s\n\nDecisions:\n%s\n\nStop history:\n%s\n", obligation.TargetUnit, goal, obligation.SourceUnit, obligation.SourceCommit, obligation.TargetUnit, estimate, requirement, obligation.SourceCommit, reads, dispositions, history)
}

func validateTransfer(o ReviewObligation) error {
	if item := o.DesignItem; item != nil {
		if item.Exit == "" || item.DesignID == "" || len(item.BodySHA256) != 64 || item.Unit == "" || item.Decision == "" || len(item.Tests) == 0 || o.Fixture == "" || o.TargetUnit != "" {
			return fmt.Errorf("a design item needs its acceptance, unit, Decision and public tests")
		}
	}
	if o.TargetUnit == "" {
		if o.SourceUnit != "" || o.OriginalRead != "" || o.OriginalFinding != "" || o.StopReference != "" || o.TransferredOnce || o.SourceCommit != "" || o.CoverageRead != "" || o.CoverageCommit != "" || o.OriginalEvidence != (readsubject.Finding{}) {
			return fmt.Errorf("transfer metadata needs its destination unit")
		}
		return nil
	}
	for _, value := range []string{o.SourceUnit, o.TargetUnit, o.OriginalRead, o.OriginalFinding, o.StopReference} {
		if !bareReviewID(value) {
			return fmt.Errorf("a transfer needs its units, original read and finding, and recorded stop")
		}
	}
	if o.Finding != o.OriginalFinding || !strings.HasPrefix(o.OriginalFinding, o.OriginalRead+":") {
		return fmt.Errorf("a transferred finding must keep its original read-qualified identity")
	}
	if o.OriginalEvidence.ID != o.OriginalFinding || !o.OriginalEvidence.Material {
		return fmt.Errorf("a transfer must retain its original material finding evidence")
	}
	data, err := json.Marshal(struct {
		Findings []readsubject.Finding `json:"findings"`
		Count    int                   `json:"verdictMaterialCount"`
	}{[]readsubject.Finding{o.OriginalEvidence}, 1})
	if err != nil {
		return err
	}
	if _, err := readsubject.Collect(o.OriginalRead, readsubject.ReadSubject{}, "", "", "", data, ""); err != nil {
		return fmt.Errorf("transferred finding evidence: %w", err)
	}
	if o.SourceUnit == o.TargetUnit {
		return fmt.Errorf("a transfer destination cannot be its source unit")
	}
	if !o.TransferredOnce {
		return fmt.Errorf("a transfer must retain its transferred-once marker")
	}
	return nil
}

func validateTransferGraph(existing, incoming []ReviewObligation) error {
	edges := map[string][]string{}
	originals := map[string]ReviewObligation{}
	for _, set := range [][]ReviewObligation{existing, incoming} {
		for _, o := range set {
			if o.TargetUnit == "" {
				continue
			}
			key := o.OriginalRead + ":" + o.OriginalFinding
			if prior, found := originals[key]; found && (prior.SourceUnit != o.SourceUnit || prior.TargetUnit != o.TargetUnit || prior.StopReference != o.StopReference || prior.SourceCommit != o.SourceCommit || prior.OriginalEvidence != o.OriginalEvidence) {
				return fmt.Errorf("finding %s has already been transferred; a second stop requires a person's act", o.OriginalFinding)
			}
			originals[key] = o
			edges[o.SourceUnit] = append(edges[o.SourceUnit], o.TargetUnit)
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(unit string) bool {
		if visiting[unit] {
			return false
		}
		if done[unit] {
			return true
		}
		visiting[unit] = true
		for _, destination := range edges[unit] {
			if !visit(destination) {
				return false
			}
		}
		delete(visiting, unit)
		done[unit] = true
		return true
	}
	for unit := range edges {
		if !visit(unit) {
			return fmt.Errorf("review transfers must not form a cycle")
		}
	}
	return nil
}

func proveTransferCoverage(o ReviewObligation, coverage *TransferCoverage) error {
	if coverage == nil || coverage.TargetUnit != o.TargetUnit || coverage.ReadID == "" || coverage.Commit == "" || !slices.Contains(coverage.Findings, o.OriginalFinding) || o.SourceCommit != "" && !slices.Contains(coverage.SourceCommits, o.SourceCommit) {
		return fmt.Errorf("finding %s needs destination %s coverage of its identity and source change", o.OriginalFinding, o.TargetUnit)
	}
	return nil
}

// CompleteTransfers records one clean destination's coverage for every inherited
// finding in one goal publication. Missing evidence leaves the entire set open.
func CompleteTransfers(r VerbRequest, id, target string, coverage TransferCoverage) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: Intent{Verb: "complete-transfers", Targets: []string{id}}, Message: "goal complete-transfers " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			file := tree.Live[id]
			if file == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(file, r) {
				return nil, AlreadyApplied{}
			}
			if !ownPair(file.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s transfer completion requires the human or owning pair", id)
			}
			matched, changed := false, false
			for _, obligation := range file.ReviewObligations {
				if obligation.TargetUnit != target {
					continue
				}
				matched = true
				if obligation.State != "discharged" || obligation.CoverageRead != coverage.ReadID || obligation.CoverageCommit != coverage.Commit {
					changed = true
				}
				if err := proveTransferCoverage(obligation, &coverage); err != nil {
					return nil, err
				}
			}
			if !matched {
				return nil, AlreadyHolds{Reason: fmt.Sprintf("unit %s has no inherited review findings", target)}
			}
			if !changed {
				return nil, AlreadyHolds{Reason: fmt.Sprintf("unit %s inherited findings already have this clean coverage", target)}
			}
			for index, obligation := range file.ReviewObligations {
				if obligation.TargetUnit != target {
					continue
				}
				file.ReviewObligations[index].State = "discharged"
				file.ReviewObligations[index].CoverageRead = coverage.ReadID
				file.ReviewObligations[index].CoverageCommit = coverage.Commit
				file.ReviewObligations[index].Test = "covered: " + coverage.ReadID + " commit=" + coverage.Commit
			}
			touch(file, r, "complete-transfers", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(file)}}, nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) }})
}
