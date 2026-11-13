package launch

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

const CheckEnvironmentVariable = "METASYSTEM_CHECK_ENVIRONMENT_SHA256"

// CheckEnvironmentDigest binds evidence to the frozen environment before a shell or host lease adds variables.
func CheckEnvironmentDigest(environment []string, base string) string {
	environment = slices.DeleteFunc(slices.Clone(environment), func(entry string) bool {
		return strings.HasPrefix(entry, "METASYSTEM_CHECK_SELECTION=") || strings.HasPrefix(entry, CheckEnvironmentVariable+"=") || strings.HasPrefix(entry, "LANDING_ONLY=") || strings.HasPrefix(entry, "LANDING_PROOF_BASE=")
	})
	slices.Sort(environment)
	environment = append(environment, "LANDING_ONLY=", "LANDING_PROOF_BASE="+base)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(environment, "\x00"))))
}

// CheckSelection records a completed selection, without granting proof authority.
type CheckSelection struct {
	Version, NotRunnable, Workers, VerdictPolicyVersion                            int
	Base, BaseTree, BaseReason                                                     string
	Groups                                                                         []string
	GroupDigests                                                                   map[string]string
	PackageTests                                                                   map[string][]string
	Mode, ContractDigest, EngineDigest, GoVersion, GOOS, GOARCH, EnvironmentSHA256 string
	BuildTags                                                                      map[string][]string
	Settings                                                                       map[string]string
}

func (selection CheckSelection) Write(path string) error { return writeUnitJSON(path, selection, "") }
