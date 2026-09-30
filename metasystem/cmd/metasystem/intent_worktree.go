package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"crypto/rand"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"time"
)

// intentConnectionOwners are the goal-branch owners the connected public
// work calls: endpoint and claim resolution, the worktree commit token,
// branch commit and branch push. Production uses the goal branch verbs'
// own owners; tests give each invocation its own claim, token and
// transport while keeping the real commit and push owners.
type intentConnectionOwners struct {
	endpoint    func(root string) (goal.Endpoint, error)
	endpointTip func(root string, endpoint goal.Endpoint) (string, error)
	claimCheck  func(root, goalID string, endpoint goal.Endpoint) func() error
	commitToken func(root string, commit func() error) error
	// section is the checkout mutation section a manual submission stages
	// and commits in; its withToken mints the commit token without another
	// lock.
	section     func(root string, body func(withToken func(func() error) error) error) error
	transport   branch.PushTransport
	commit      func(branch.CommitRequest) (string, error)
	push        func(branch.PushRequest) (branch.PushResult, error)
	operationID func() (string, error)
	// isolate prepares the adapters' declared local configuration files
	// from the selected checkout in a new goal worktree through the
	// session-isolation owner; configuration outside that manifest (such as
	// metasystem.conf.local) is never copied.
	isolate func(source, destination string) error
}

// goalWorktreeLockReason marks a goal worktree this composition registered
// detached for a remote adoption it has not finished. Git's own worktree
// lock is the only ownership record; it is removed once the worktree is on
// goal/G.
func goalWorktreeLockReason(goalID string) string {
	return "metasystem goal worktree adoption goal/" + goalID
}

func (inv *intentInvocation) connection() intentConnectionOwners {
	owners := inv.owners.connection
	if owners.endpoint == nil {
		owners.endpoint = branch.MainEndpoint
	}
	if owners.endpointTip == nil {
		owners.endpointTip = goalBranchEndpointTip
	}
	if owners.claimCheck == nil {
		owners.claimCheck = goalBranchClaimCheck
	}
	if owners.commitToken == nil {
		owners.commitToken = func(root string, commit func() error) error {
			return withGoalBranchCommitTokenAt(root, goalBranchHolderRoot(root), commit)
		}
	}
	if owners.section == nil {
		owners.section = func(root string, body func(withToken func(func() error) error) error) error {
			return goalBranchCheckoutSection(root, goalBranchHolderRoot(root), body)
		}
	}
	if owners.transport == nil {
		owners.transport = branch.GitPushTransport{}
	}
	if owners.commit == nil {
		owners.commit = branch.CommitStaged
	}
	if owners.push == nil {
		owners.push = branch.Push
	}
	if owners.operationID == nil {
		owners.operationID = branchOperationID
	}
	if owners.isolate == nil {
		owners.isolate = inv.isolateAdapterConfiguration
	}
	return owners
}

// goalWorktreeInstallation is the selected installation's place inside a
// goal worktree of the same repository.
func (inv *intentInvocation) goalWorktreeInstallation(worktree string) string {
	relative, err := filepath.Rel(inv.layout.GitRoot, inv.layout.InstallationRoot)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return worktree
	}
	return filepath.Join(worktree, relative)
}

// registeredWorktrees are Git's registered worktrees of this repository:
// path to its checked-out branch ref ("" when detached).
type registeredWorktree struct{ ref, lock string }

func (inv *intentInvocation) registeredWorktrees() (map[string]registeredWorktree, error) {
	output, err := inv.work().git(inv.layout.GitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	registered := map[string]registeredWorktree{}
	for _, block := range strings.Split(string(output), "\n\n") {
		path, entry := "", registeredWorktree{}
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				path = strings.TrimPrefix(line, "worktree ")
			}
			if strings.HasPrefix(line, "branch ") {
				entry.ref = strings.TrimPrefix(line, "branch ")
			}
			if line == "locked" || strings.HasPrefix(line, "locked ") {
				entry.lock = strings.TrimPrefix(strings.TrimPrefix(line, "locked"), " ")
			}
		}
		if path != "" {
			registered[filepath.Clean(path)] = entry
		}
	}
	return registered, nil
}

// prepareGoalWorktree returns the worktree that has goal/G checked out,
// creating it at the deterministic sibling <checkout>-<goal> when none
// exists. Creation happens only after the goal claim and checkout lease are
// verified. Its base is a validated local goal branch, else the remote goal
// branch adopted through the branch push owner, else the freshly resolved
// endpoint tip; divergent local and remote histories are refused. A remote
// adoption registers its worktree detached and locked with this goal's
// reason, so an interrupted adoption is resumed (before or after the owner
// moved the branch ref) while any other detached worktree at the path is
// refused. The current checkout, its files and any occupied path are left
// as they are: there is no force, reset or move.
func (inv *intentInvocation) prepareGoalWorktree(id string) (string, *intentResult) {
	// An engine verb that enters the goal's registered worktree holds its
	// record lock shared before it resolves the path, for the verb's whole
	// life (Part B 3.1 "Entrants"); a worktree being released is gone.
	if problem := inv.enterGoalWorktree(id, ""); problem != nil {
		return "", problem
	}
	path, problem := inv.goalWorktreeEntry(id)
	if problem != nil {
		return path, problem
	}
	if problem := inv.enterGoalWorktree(id, path); problem != nil {
		return "", problem
	}
	// Every worktree handed to a build, new, reused, resumed or created by
	// a concurrent call, first has the adapters' declared local
	// configuration completed; the owner never overwrites existing files.
	// What the copy places is recorded as the engine's (Round D3 F-1), so
	// the worktree's release does not count it as work.
	manifest, _ := supervisor.LocalConfigManifest(supervisor.Deps{Root: inv.layout.InstallationRoot})
	before := diskstore.PresentPaths(path, manifest)
	if err := inv.connection().isolate(inv.layout.GitRoot, path); err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "goal", ID: id}},
			Summary: fmt.Sprintf("goal worktree %s is kept, but the adapters' local configuration is not complete in it: %v; nothing was built", path, err),
			next:    inv.sameCommand(), nextReason: "the same command completes the configuration and continues"}
	}
	if problem := inv.recordGoalWorktreeContent(id, path, diskstore.Placed(path, manifest, before)); problem != nil {
		return "", problem
	}
	return path, nil
}

func (inv *intentInvocation) goalWorktreeEntry(id string) (string, *intentResult) {
	refused := func(format string, args ...any) (string, *intentResult) {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "goal", ID: id}},
			Summary: fmt.Sprintf(format, args...) + "; nothing was built"}
	}
	ref, reason := "refs/heads/goal/"+id, goalWorktreeLockReason(id)
	git := inv.work().git
	find := func() (string, map[string]registeredWorktree, error) {
		registered, err := inv.registeredWorktrees()
		if err != nil {
			return "", nil, err
		}
		for path, entry := range registered {
			if entry.ref == ref {
				if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
					if entry.lock == reason {
						if _, err := git(inv.layout.GitRoot, "worktree", "unlock", path); err != nil {
							return "", nil, err
						}
					}
					return path, registered, nil
				}
			}
		}
		return "", registered, nil
	}
	existing, registered, err := find()
	if err != nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot list the repository's worktrees: " + err.Error()}
	}
	if existing != "" {
		return existing, nil
	}
	conn := inv.connection()
	install := inv.layout.InstallationRoot
	endpoint, err := conn.endpoint(install)
	if err != nil {
		return refused("the goal branch endpoint is unavailable: %v", err)
	}
	check := conn.claimCheck(install, id, endpoint)
	if err := branch.CheckHolder(check); err != nil {
		return refused("goal/%s is prepared only under this session's claim: %v", id, err)
	}
	target := filepath.Join(filepath.Dir(inv.layout.GitRoot), filepath.Base(inv.layout.GitRoot)+"-"+id)
	if parent, err := filepath.EvalSymlinks(filepath.Dir(target)); err == nil {
		target = filepath.Join(parent, filepath.Base(target))
	}
	_, localErr := git(inv.layout.GitRoot, "rev-parse", "--verify", "-q", ref+"^{commit}")
	local := localErr == nil
	remoteTip, remote, err := conn.transport.RemoteTip(install, endpoint.Remote, ref)
	if err != nil {
		return refused("cannot read %s's goal/%s: %v", endpoint.Remote, id, err)
	}
	entry, taken := registered[filepath.Clean(target)]
	owned := taken && entry.ref == "" && entry.lock == reason
	if taken && !owned {
		return refused("path %s is a worktree of %s that this goal's preparation did not create, not goal/%s", target, chooseUnitValue(entry.ref, "a detached HEAD"), id)
	}
	if !taken {
		if entries, statErr := os.ReadDir(target); statErr == nil && len(entries) > 0 {
			return refused("path %s is occupied by files that are not the goal/%s worktree", target, id)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			if _, fileErr := os.Stat(target); fileErr == nil {
				return refused("path %s is occupied and is not the goal/%s worktree", target, id)
			}
		}
	}
	endpointTip, err := conn.endpointTip(install, endpoint)
	if err != nil {
		return refused("cannot resolve the landing endpoint's tip: %v", err)
	}
	adopt, switchOwned := false, false
	var add []string
	switch {
	case owned && !local && remote:
		adopt = true
	case owned && local:
		// The push owner moved the branch ref but did not switch the
		// worktree it created; only a clean worktree is switched.
		switchOwned = true
	case owned:
		return refused("worktree %s was created for goal/%s's adoption, but %s no longer has that branch", target, id, endpoint.Remote)
	case !local && !remote:
		add = []string{"worktree", "add", "-b", "goal/" + id, target, endpointTip}
	case !local:
		// The push owner adopts a remote-only goal branch by switching the
		// checkout it runs in; it runs in the new worktree, detached at
		// the endpoint and locked as this goal's, so the caller's checkout
		// never moves.
		add = []string{"worktree", "add", "--lock", "--reason", reason, "--detach", target, endpointTip}
		adopt = true
	default:
		if remote {
			localTip, _ := git(inv.layout.GitRoot, "rev-parse", ref)
			tip := strings.TrimSpace(string(localTip))
			if tip != remoteTip {
				if _, known := git(inv.layout.GitRoot, "cat-file", "-e", remoteTip+"^{commit}"); known != nil {
					if _, fetchErr := git(inv.layout.GitRoot, "fetch", "--no-tags", "--refmap=", endpoint.Remote, ref); fetchErr != nil {
						return refused("cannot fetch %s's goal/%s to compare histories: %v", endpoint.Remote, id, fetchErr)
					}
				}
				_, behind := git(inv.layout.GitRoot, "merge-base", "--is-ancestor", tip, remoteTip)
				_, ahead := git(inv.layout.GitRoot, "merge-base", "--is-ancestor", remoteTip, tip)
				if behind != nil && ahead != nil {
					return refused("local goal/%s (%.12s) and %s's (%.12s) have diverged", id, tip, endpoint.Remote, remoteTip)
				}
			}
		}
		add = []string{"worktree", "add", target, "goal/" + id}
	}
	if add != nil {
		// Registered before git writes a byte of it (Part B 3.1), with no
		// file inside the tree.
		var reserved diskstore.Record
		if filepath.IsAbs(inv.goalWorktreeControl()) {
			var err error
			reserved, err = diskstore.ReserveLinkedWorktree(inv.goalWorktreeRegistry(), target, diskstore.GoalWorktreeClass,
				diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: id}, inv.goalWorktreeControl(), "", time.Now().UTC(), rand.Reader)
			if err != nil {
				return refused("goal/%s's worktree %s cannot be registered: %v", id, target, err)
			}
		}
		if _, addErr := git(inv.layout.GitRoot, add...); addErr != nil {
			// A concurrent creation of the same worktree is reused; any
			// other failure is reported with what exists now so a retry
			// reconciles.
			if again, _, findErr := find(); findErr == nil && again != "" {
				return again, nil
			}
			if reserved.ID != "" {
				_ = diskstore.AbandonLinkedWorktree(inv.goalWorktreeRegistry(), reserved.ID, addErr.Error())
			}
			_, branchErr := git(inv.layout.GitRoot, "rev-parse", "--verify", "-q", ref+"^{commit}")
			return refused("git worktree add for goal/%s at %s failed: %v (local goal branch present: %v)", id, target, addErr, branchErr == nil)
		}
	}
	if adopt {
		opid, err := conn.operationID()
		if err == nil {
			_, err = conn.push(branch.PushRequest{Repo: inv.goalWorktreeInstallation(target), Remote: endpoint.Remote, EndpointTip: endpointTip,
				GoalID: id, OpID: opid, CheckClaim: check, Transport: conn.transport})
		}
		if err != nil {
			return refused("worktree %s is created detached, but the branch push owner did not adopt %s's goal/%s: %v; the same build resumes the adoption", target, endpoint.Remote, id, err)
		}
	}
	if switchOwned {
		status, err := git(target, "status", "--porcelain")
		if err != nil || len(status) != 0 {
			return refused("worktree %s created for goal/%s has changes; it is left as it is (%v)", target, id, err)
		}
		if _, err := git(target, "switch", "goal/"+id); err != nil {
			return refused("worktree %s could not switch to goal/%s: %v", target, id, err)
		}
	}
	if again, _, findErr := find(); findErr == nil && again != "" {
		return again, nil
	}
	return refused("git registered no goal/%s worktree at %s", id, target)
}

// goalWorktreeRegistry is the checkout registry goal worktrees are recorded
// in.
func (inv *intentInvocation) goalWorktreeRegistry() diskstore.Registry {
	return diskstore.CheckoutRegistry(inv.goalWorktreeControl())
}

// goalWorktreeControl is the control root whose registry records the goal
// worktrees: the selected state root, else the installation.
func (inv *intentInvocation) goalWorktreeControl() string {
	if inv.stateRoot != "" {
		return inv.stateRoot
	}
	return inv.layout.InstallationRoot
}

// enterGoalWorktree holds goal id's registered worktree shared for the rest
// of the verb and re-reads its record. With path empty it enters the record
// the goal already has, before the path is resolved; with the resolved
// path it registers a linked worktree no record names yet (one made before
// registration existed, or by this verb), accepts a reservation git has
// completed, and enters it. A main checkout on goal/<id> is never a store.
// A record being released or released is gone: the verb refuses as input.
func (inv *intentInvocation) enterGoalWorktree(id, path string) *intentResult {
	if !filepath.IsAbs(inv.goalWorktreeControl()) {
		return nil
	}
	registry := inv.goalWorktreeRegistry()
	owner := diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: id}
	gone := &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "goal", ID: id}},
		Summary: fmt.Sprintf("goal %s's worktree is being released because the goal concluded; nothing was built", id),
		next:    inv.publicArgv("goal", "show", id), nextReason: "see the goal's state"}
	records, err := diskstore.FindLinkedWorktrees(registry, diskstore.GoalWorktreeClass, owner)
	if err != nil {
		return &intentResult{Outcome: intentFailed, code: 1, Summary: "the store registry cannot be read: " + err.Error() + "; nothing was built",
			Decision: "metasystem disk show names what cannot be read"}
	}
	if path != "" {
		if _, err := diskstore.ReadGitIdentity(path); err != nil || filepath.Clean(path) == filepath.Clean(inv.layout.GitRoot) {
			return nil
		}
		matched := false
		for _, record := range records {
			matched = matched || record.Path == path
		}
		if !matched {
			record, err := diskstore.ReserveLinkedWorktree(registry, path, diskstore.GoalWorktreeClass, owner, inv.goalWorktreeControl(), "", time.Now().UTC(), rand.Reader)
			if err != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("goal/%s's worktree %s cannot be registered: %v; nothing was built", id, path, err)}
			}
			records = append(records, record)
		}
	}
	for _, record := range records {
		if path != "" && record.Path != path {
			continue
		}
		if record.State == diskstore.StateReserved && path != "" {
			if _, err := diskstore.AcceptLinkedWorktree(registry, record.ID); err != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("goal/%s's worktree %s cannot be recorded: %v; nothing was built", id, path, err)}
			}
		}
		if inv.entered(record.ID) {
			continue
		}
		entrant, err := registry.Enter(record.ID)
		if errors.Is(err, diskstore.ErrStoreGone) {
			return gone
		}
		if err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("goal/%s's worktree record cannot be held: %v; nothing was built", id, err)}
		}
		inv.entrants = append(inv.entrants, entrant)
	}
	return nil
}

// recordGoalWorktreeContent records the files the engine just placed in
// goal id's registered worktree at path, and the worktree installation's
// state root, which only the engine writes.
func (inv *intentInvocation) recordGoalWorktreeContent(id, path string, placed []string) *intentResult {
	installation, err := filepath.Rel(path, inv.goalWorktreeInstallation(path))
	if err != nil || strings.HasPrefix(installation, "..") {
		installation = "."
	}
	for _, entrant := range inv.entrants {
		if entrant.Record.Path != path {
			continue
		}
		if err := inv.goalWorktreeRegistry().RecordEngineContent(entrant.Record.ID, path, placed, []diskstore.EngineDir{diskstore.EngineStateDir(installation)}); err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("the local configuration placed in goal/%s's worktree cannot be recorded: %v; nothing was built", id, err)}
		}
	}
	return nil
}

// entered reports a store this verb already holds.
func (inv *intentInvocation) entered(id string) bool {
	for _, entrant := range inv.entrants {
		if entrant.Record.ID == id {
			return true
		}
	}
	return false
}

// isolateAdapterConfiguration prepares the files each adapter declares as
// its local configuration (its local-config-paths manifest) in a new goal
// worktree, through the same session-isolation owner second sessions use.
func (inv *intentInvocation) isolateAdapterConfiguration(source, destination string) error {
	paths, err := supervisor.LocalConfigManifest(supervisor.Deps{Root: inv.layout.InstallationRoot})
	if err != nil {
		return err
	}
	var manifest strings.Builder
	for _, path := range paths {
		manifest.WriteString(path + "\n")
	}
	if manifest.Len() == 0 {
		return nil
	}
	file, done, err := diskstore.ScratchFile("metasystem-local-config-paths.")
	if err != nil {
		return err
	}
	defer done()
	if _, err := file.WriteString(manifest.String()); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = validate.SessionIsolation(source, destination, file.Name(), inv.layout.InstallationRoot)
	return err
}
