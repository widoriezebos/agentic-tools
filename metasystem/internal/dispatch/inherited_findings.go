package dispatch

import (
	"encoding/json"
	"fmt"
	"strings"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// CritiqueInheritFindings freezes published transfer evidence on an existing
// destination critic. Fresh dispatch freezes the same evidence before launch.
func CritiqueInheritFindings(repoRoot, rootJob string, obligations []goal.ReviewObligation) error {
	if len(obligations) == 0 {
		return nil
	}
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		err := withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			existing, err := inheritedObligations(root["inheritedFindings"])
			if err != nil {
				return err
			}
			for _, o := range obligations {
				if err := validateInheritedFinding(o); err != nil {
					return err
				}
				matched := false
				for _, prior := range existing {
					if prior.OriginalFinding == o.OriginalFinding {
						matched = true
						if prior.OriginalEvidence != o.OriginalEvidence || prior.SourceCommit != o.SourceCommit || prior.TargetUnit != o.TargetUnit || prior.OriginalRead != o.OriginalRead || prior.StopReference != o.StopReference {
							return fmt.Errorf("inherited finding %s differs from its frozen published evidence", o.OriginalFinding)
						}
					}
				}
				if !matched {
					existing = append(existing, o)
				}
			}
			root["inheritedFindings"] = existing
			return writeRecord(path, root)
		})
		return "", err
	})
	return err
}

func inheritedObligations(value any) ([]goal.ReviewObligation, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var obligations []goal.ReviewObligation
	if err := json.Unmarshal(data, &obligations); err != nil {
		return nil, fmt.Errorf("inherited findings are unreadable: %w", err)
	}
	return obligations, nil
}

func validateInheritedFinding(o goal.ReviewObligation) error {
	if !o.TransferredOnce || o.SourceUnit == "" || o.TargetUnit == "" || o.SourceUnit == o.TargetUnit || o.StopReference == "" || !gitObjectIDRe.MatchString(o.SourceCommit) || o.OriginalEvidence.ID != o.OriginalFinding || !strings.HasPrefix(o.OriginalFinding, o.OriginalRead+":") || !o.OriginalEvidence.Material {
		return fmt.Errorf("inherited finding %s has unavailable original transfer evidence", o.OriginalFinding)
	}
	data := canonicalJSON(map[string]any{"findings": []readsubject.Finding{o.OriginalEvidence}, "verdictMaterialCount": 1})
	if _, err := readsubject.Collect(o.OriginalRead, readsubject.ReadSubject{}, "", "", "", data, ""); err != nil {
		return fmt.Errorf("inherited finding %s: %w", o.OriginalFinding, err)
	}
	return nil
}

// admitInheritedResolutions imports only originals explicitly referenced by
// this read. Other completion obligations never become unrelated open rows.
func admitInheritedResolutions(register []registerFinding, provenance any, findings []any) ([]registerFinding, error) {
	obligations, err := inheritedObligations(provenance)
	if err != nil {
		return nil, err
	}
	byID := map[string]bool{}
	for _, f := range register {
		byID[f.FindingID] = true
	}
	result := append([]registerFinding(nil), register...)
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := asString(finding["resolves"])
		if id == "" || byID[id] {
			continue
		}
		for _, o := range obligations {
			if o.OriginalFinding != id {
				continue
			}
			if err := validateInheritedFinding(o); err != nil {
				return nil, err
			}
			f := o.OriginalEvidence
			result = append(result, registerFinding{FindingID: f.ID, Critic: o.OriginalRead, RigorClass: critiqueModel.Unproven, Grain: "invariant", Class: f.Class, Where: f.Where, Change: f.Change, Relation: f.Relation, Artifact: f.Where, Title: f.Claim, Status: "open", Evidence: f.Evidence, EvidenceDigest: digestJSON(f.Evidence), FactsDigest: digestJSON(nil), Multiplicity: 1})
			byID[id] = true
			break
		}
	}
	return result, nil
}
