package steward

import (
	"fmt"
	"path/filepath"
)

// DispatchAuthorization is the staged launch tuple a consumed, not yet
// launched intent authorizes. Nothing in it is caller-selectable: the
// unattended continuation dispatches exactly these inputs.
type DispatchAuthorization struct {
	Goal        string `json:"goal"`
	JobId       string `json:"jobId"`
	Runtime     string `json:"runtime"`
	Model       string `json:"model"`
	Role        string `json:"role"`
	Permissions string `json:"permissions"`
	Brief       string `json:"brief"`
}

// AuthorizeDispatch is the steward's half of the continuation gate: the
// consumed intent must exist unstamped, its staged bytes must match their
// digests, and it must have been minted under the current installation
// generation or its preserved human enrollment. The caller has already proved it classifies STEWARD; the
// steward package cannot classify callers itself (lease imports steward).
func AuthorizeDispatch(repo, nonce string) (DispatchAuthorization, error) {
	it, err := ConsumedIntent(repo, nonce)
	if err != nil {
		return DispatchAuthorization{}, err
	}
	if it.LaunchStamped {
		return DispatchAuthorization{}, fmt.Errorf("intent %s already launched; a replay authorizes nothing", nonce)
	}
	if err := VerifyStagedDigests(repo, it); err != nil {
		return DispatchAuthorization{}, err
	}
	top, err := filepath.Abs(repo)
	if err != nil {
		return DispatchAuthorization{}, err
	}
	installed, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		return DispatchAuthorization{}, err
	}
	sameAuthority := it.InstallGen == installed.Generation
	if it.HumanWitnessedGeneration != 0 || it.HumanWitnessedAt != "" {
		sameAuthority = it.HumanWitnessedGeneration > 0 && it.HumanWitnessedAt != "" &&
			it.HumanWitnessedGeneration == installed.HumanWitnessedGeneration && it.HumanWitnessedAt == installed.HumanWitnessedAt &&
			(it.InstallGen == installed.Generation || installed.Generation > it.InstallGen && installed.MintedBy == "machine-rebuild")
	}
	if it.RepoIdentity != installed.RepoIdentity || !sameAuthority {
		return DispatchAuthorization{}, fmt.Errorf("nothing was launched: the engine was reinstalled since this was approved\n(install %d of %q, now %d of %q)",
			it.InstallGen, it.RepoIdentity, installed.Generation, installed.RepoIdentity)
	}
	fence, err := ReadEnrollmentFence(repo)
	if err != nil {
		return DispatchAuthorization{}, err
	}
	if fence != it.FenceAtMint {
		return DispatchAuthorization{}, fmt.Errorf("nothing was launched: a worker enrolled after the reservation")
	}
	return DispatchAuthorization{
		Goal: it.Goal, JobId: it.JobId, Runtime: it.Runtime, Model: it.Model,
		Role: it.Role, Permissions: it.Permissions, Brief: BriefPath(repo, it.Nonce),
	}, nil
}
