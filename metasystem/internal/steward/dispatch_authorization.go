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
// generation. The caller has already proved it classifies STEWARD; the
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
	if it.RepoIdentity != installed.RepoIdentity || it.InstallGen != installed.Generation {
		return DispatchAuthorization{}, fmt.Errorf("the authorization was minted under installation generation %d of %q; the current installation is generation %d of %q — a superseded authorization launches nothing",
			it.InstallGen, it.RepoIdentity, installed.Generation, installed.RepoIdentity)
	}
	return DispatchAuthorization{
		Goal: it.Goal, JobId: it.JobId, Runtime: it.Runtime, Model: it.Model,
		Role: it.Role, Permissions: it.Permissions, Brief: BriefPath(repo, it.Nonce),
	}, nil
}
