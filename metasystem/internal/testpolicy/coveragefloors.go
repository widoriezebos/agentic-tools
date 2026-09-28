package testpolicy

// The coverage floors are this repository's own development test policy:
// the measured per-package statement coverage its full gate must not fall
// below, one file per measured platform, beside testing.json. Lowering a
// floor is refused against the landing's base.
const (
	coverageFloorsFile      = "testing-coverage-floors.json"
	coverageFloorsLinuxFile = "testing-coverage-floors-linux.json"
)

// legacyCoverageFloors is where a base tree from before the floors moved
// beside testing.json keeps each file; the floor protection reads it only
// when the base lacks the file, which is true of the landing that moves it.
var legacyCoverageFloors = map[string]string{
	coverageFloorsFile:      "scripts/agents/coverage-ratchet.json",
	coverageFloorsLinuxFile: "scripts/agents/coverage-ratchet-linux.json",
}

// CoverageFloorsFile is the installation-relative floors file for goos.
func CoverageFloorsFile(goos string) string {
	if goos == "linux" {
		return coverageFloorsLinuxFile
	}
	return coverageFloorsFile
}

// CoverageFloorsFiles are both platforms' floors files.
func CoverageFloorsFiles() []string {
	return []string{coverageFloorsFile, coverageFloorsLinuxFile}
}

// LegacyCoverageFloorsFile is the pre-move base path of a floors file.
func LegacyCoverageFloorsFile(file string) string {
	return legacyCoverageFloors[file]
}
