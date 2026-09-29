package testpolicy

var cadenceCatchGroupIDs = []string{
	"section/go-engine-gate",
	"shipped-installation-standard",
	"supervision-bed-standard",
	"section/dispatcher-adapter-and-mission-runner-fixtures",
	"section/adoption-fixtures",
	"section/witness-gate-fixtures",
}

func CadenceCatchGroupIDs() []string { return append([]string(nil), cadenceCatchGroupIDs...) }
