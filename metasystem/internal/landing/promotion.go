package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const promotionRecordPath = "scripts/agents/landing-promotion.json"

var errPromotionBaseUnreadable = errors.New("landing promotion base tree is unreadable")

type promotionRecord struct {
	SchemaVersion int      `json:"schemaVersion"`
	RefuseCodes   []string `json:"refuseCodes"`
}

// applyPromotion consumes policy from the landing base. A candidate cannot
// weaken the decision that judges it; changes to promotion policy become
// effective only after they land through the implementation-chain bar.
func applyPromotion(params ObserveParams, observation Observation) Observation {
	refuseCodes, present, err := loadPromotion(params.RepoRoot)
	if err != nil {
		code := "promotion-record-malformed"
		if errors.Is(err, errPromotionBaseUnreadable) {
			code = "promotion-base-unreadable"
		}
		refusal := wouldRefuse(code, observation.Provenance)
		refusal.Mode = "refuse"
		return refusal
	}
	if present && observation.Verdict == "would-refuse" && refuseCodes[observation.Code] {
		observation.Mode = "refuse"
	}
	return observation
}

func loadPromotion(root string) (map[string]bool, bool, error) {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return nil, true, fmt.Errorf("%w: %v", errPromotionBaseUnreadable, err)
	}
	data, present, err := workspace.FileAt(baseTree, promotionRecordPath)
	if err != nil {
		return nil, true, fmt.Errorf("landing promotion record is unreadable: %w", err)
	}
	if !present {
		return nil, false, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record promotionRecord
	if err := decoder.Decode(&record); err != nil {
		return nil, true, fmt.Errorf("landing promotion record is malformed: %w", err)
	}
	if err := rejectPromotionTrailingJSON(decoder); err != nil {
		return nil, true, err
	}
	if record.SchemaVersion != 1 || record.RefuseCodes == nil {
		return nil, true, fmt.Errorf("landing promotion record is malformed")
	}

	refuseCodes := make(map[string]bool, len(record.RefuseCodes))
	for _, code := range record.RefuseCodes {
		if !knownRefusalCode(code) || refuseCodes[code] {
			return nil, true, fmt.Errorf("landing promotion record contains an unknown or duplicate verdict code")
		}
		refuseCodes[code] = true
	}
	return refuseCodes, true, nil
}

func rejectPromotionTrailingJSON(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("landing promotion record contains trailing JSON")
		}
		return fmt.Errorf("landing promotion record is malformed: %w", err)
	}
	return nil
}

func knownRefusalCode(code string) bool {
	switch code {
	case "malformed-candidate-tree",
		"candidate-tree-unreadable",
		"missing-declaration",
		"conflicting-declarations",
		"malformed-chain-id",
		"chain-record-unreadable",
		"chain-record-malformed",
		"chain-not-implementation",
		"chain-not-design-bearing",
		"chain-open",
		"chain-output-unreadable",
		"chain-output-mismatch",
		"chain-has-uncarried-paths",
		"register-carriage-policy-unreadable",
		"register-carriage-path-refused",
		"register-carriage-not-append-only",
		"malformed-revert-commit",
		"direct-fix-policy-unreadable",
		"not-exact-revert",
		"direct-fix-floor-refused",
		"unknown-direct-fix-class":
		return true
	default:
		return false
	}
}
