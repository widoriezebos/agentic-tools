package launch

import (
	"os"
	"reflect"
	"slices"
	"strings"
)

type CheckNeed struct {
	Tree, Base, Strength string
	Check                UnitCheck
	Context              CheckSelection
	Requirements         []string
}

type CheckReuse struct{ Source, Path, Why string }
type CheckDecision struct {
	Reuse *CheckReuse
	Field string
}

// ApplicableCheck reports the first mismatch in publication identity order.
// Reading evidence belongs to the consumer; unreadable records grant no proof.
func ApplicableCheck(record *CheckEvidence, path string, readErr error, need CheckNeed) CheckDecision {
	if os.IsNotExist(readErr) || record == nil && readErr == nil {
		return CheckDecision{Field: "missing evidence"}
	}
	if readErr != nil {
		return CheckDecision{Field: "unreadable evidence"}
	}
	if record.Version == 0 {
		return CheckDecision{Field: "version (legacy evidence)"}
	}
	selected, current := record.Selection, need.Context
	declared := record.Declaration == nil && need.Check.Declaration == nil || record.Declaration != nil && need.Check.Declaration != nil && record.Declaration.Values == need.Check.Declaration.Values && record.Declaration.SourceSHA256 == need.Check.Declaration.SourceSHA256
	audited := record.Strength.Audited && !record.Manual && record.Declaration != nil && record.Commands[1] == record.Declaration.Values[1] && strings.TrimSpace(record.Commands[1]) != "true" && !strings.Contains(record.Commands[1], "--publication-only")
	required := !slices.ContainsFunc(need.Requirements, func(id string) bool {
		return !slices.Contains(selected.Groups, id)
	})
	for _, field := range []struct {
		name string
		same bool
	}{
		{"version", record.Version == 1},
		{"completeness", record.Strength.Complete && record.Source != "" && selected.Version == 1 && record.Content.Before != "" && record.Content.Before == record.Content.After},
		{"strength", need.Strength == "audited" && audited || need.Strength == "cheap" && record.Strength.Cheap},
		{"tree", record.Content.Before == need.Tree},
		{"base", selected.Base == need.Base && (current.BaseTree == "" || selected.BaseTree == current.BaseTree)},
		{"declaration", declared && record.Manual == (need.Check.Declaration == nil)},
		{"commands", record.Commands == [2]string{need.Check.Cheap, need.Check.Audits}},
		{"engine", selected.EngineDigest == current.EngineDigest},
		{"toolchain", selected.GoVersion == current.GoVersion},
		{"platform", selected.GOOS == current.GOOS && selected.GOARCH == current.GOARCH && reflect.DeepEqual(selected.BuildTags, current.BuildTags)},
		{"environment", selected.EnvironmentSHA256 == current.EnvironmentSHA256},
		{"settings", reflect.DeepEqual(selected.Settings, current.Settings) && selected.Workers == current.Workers},
		{"requirements", required},
		{"contract digest", selected.ContractDigest == current.ContractDigest && selected.VerdictPolicyVersion == current.VerdictPolicyVersion},
	} {
		if !field.same {
			return CheckDecision{Field: field.name}
		}
	}
	return CheckDecision{Reuse: &CheckReuse{record.Source, path, "the completed check covers this publication"}}
}
