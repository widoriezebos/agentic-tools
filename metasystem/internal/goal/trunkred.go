package goal

import "fmt"

const TrunkRedBranchOpen, TrunkRedBranchMerged = "open", "merged"

type TrunkRedFailure struct {
	Report    string `json:"report"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
}
type TrunkRedSighting struct {
	Attempt    string `json:"attempt"`
	Batch      string `json:"batch"`
	BaseCommit string `json:"baseCommit"`
	BaseTree   string `json:"baseTree"`
	LogPath    string `json:"logPath"`
	LogDigest  string `json:"logDigest"`
	SeenAt     string `json:"seenAt"`
	Opid       string `json:"opid"`
}
type TrunkRedOwner struct {
	Machine string `json:"machine"`
	Since   string `json:"since"`
	How     string `json:"how"`
}
type TrunkRedBranch struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
	State  string `json:"state"`
}

func (branch TrunkRedBranch) Validate() error {
	if branch.Name == "" {
		if branch.Commit != "" || branch.State != "" {
			return fmt.Errorf("trunk-red branch without a name")
		}
		return nil
	}
	if branch.State != TrunkRedBranchOpen && branch.State != TrunkRedBranchMerged {
		return fmt.Errorf("trunk-red branch %q has unknown state %q", branch.Name, branch.State)
	}
	return nil
}

type TrunkRedClosure struct {
	At         string `json:"at"`
	Attempt    string `json:"attempt"`
	BaseCommit string `json:"baseCommit"`
	How        string `json:"how"`
	Opid       string `json:"opid"`
}
type TrunkRedEntry struct {
	ID           string             `json:"id"`
	Group        string             `json:"group"`
	Status       string             `json:"status"`
	Failures     []TrunkRedFailure  `json:"failures"`
	NotRunReason string             `json:"notRunReason"`
	Sightings    []TrunkRedSighting `json:"sightings"`
	Owner        TrunkRedOwner      `json:"owner"`
	FixGoal      string             `json:"fixGoal"`
	FixBranch    TrunkRedBranch     `json:"fixBranch"`
	Holds        []string           `json:"holds"`
	Opened       string             `json:"opened"`
	Closed       *TrunkRedClosure   `json:"closed"`
}
