package testpolicy

import "sort"

var cadenceCatchGroupIDs = []string{
	"section/go-engine-gate",
	"section/static-contract-audits",
	"section/supervision-and-census-fixtures",
	"section/dispatcher-adapter-and-mission-runner-fixtures",
	"section/adoption-fixtures",
	"section/witness-gate-fixtures",
}

func CadenceCatchGroupIDs() []string { return append([]string(nil), cadenceCatchGroupIDs...) }

func GroupIDs(contract Contract) []string {
	result := make([]string, 0, len(contract.Groups))
	for _, group := range contract.Groups {
		result = append(result, group.ID)
	}
	sort.Strings(result)
	return result
}
