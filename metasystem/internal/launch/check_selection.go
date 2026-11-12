package launch

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
