package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// landRecords uses the manual capture and staging owners for a plan commit.
// Matching bytes rejoin the branch tip; publication is outside the checkout
// lock and hand-in reads the published branch through the ordinary route.
func (inv *intentInvocation) landRecords(id string) intentResult {
	targets := inv.targets(id)
	refused := func(err error) intentResult {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: oneLine(err.Error()) + "; nothing was committed", next: inv.sameCommand()}
	}
	conn, git := inv.connection(), inv.work().git
	captureChanges := conn.captureChanges
	if captureChanges == nil {
		captureChanges = captureCheckoutChanges
	}
	recordsCheck := conn.recordsCheck
	if recordsCheck == nil {
		recordsCheck = checkRecordFiles
	}
	classes, err := pathclass.Load()
	if err != nil {
		return refused(err)
	}
	prefix, err := filepath.Rel(inv.layout.GitRoot, inv.layout.InstallationRoot.Path())
	if err != nil {
		return refused(err)
	}
	capturedTree := ""
	checkPaths := func(paths []string) *intentResult {
		for _, path := range paths {
			owner, mode, err := inv.owners.resolver.OwnerForInstallation(inv.layout.InstallationRoot.Path(), path)
			if err != nil {
				returnResult := refused(err)
				return &returnResult
			}
			class := classes.ResolveRepositoryPath(pathclass.Mode(mode), owner, filepath.ToSlash(prefix), path).Class
			if filepath.IsAbs(path) || branch.PathClass(path) != branch.ClassPlan || class != pathclass.Record {
				return &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: path + " is not a record; a build carries it",
					next: inv.publicArgv("work", "review", id, "--changes")}
			}
			reason := ""
			full := filepath.Join(inv.layout.GitRoot, path)
			if capturedTree != "" {
				entry, err := git(inv.layout.GitRoot, "--literal-pathspecs", "ls-tree", "-z", capturedTree, "--", path)
				if err != nil {
					result := refused(err)
					return &result
				}
				if len(entry) == 0 || (!strings.HasPrefix(string(entry), "100644 blob ") && !strings.HasPrefix(string(entry), "100755 blob ")) {
					reason = "is not a regular file in the captured tree"
				}
			} else {
				info, err := os.Lstat(full)
				switch {
				case strings.ContainsAny(path, "*?["):
					reason = "is a glob"
				case os.IsNotExist(err):
					reason = "is missing"
				case err != nil:
					result := refused(err)
					return &result
				case info.IsDir():
					reason = "is a directory"
				case !info.Mode().IsRegular():
					reason = "is not a regular file"
				}
			}
			if reason == "is a directory" || reason == "is a glob" {
				roots, err := filepath.Glob(full)
				files := []string{}
				for _, root := range roots {
					err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
						if err == nil && !entry.IsDir() {
							rel, _ := filepath.Rel(inv.layout.GitRoot, file)
							files = append(files, filepath.ToSlash(rel))
						}
						return err
					})
					if err != nil {
						break
					}
				}
				if err != nil {
					result := refused(err)
					return &result
				}
				slices.Sort(files)
				where := "under it"
				if reason == "is a glob" {
					where = "it matches"
				}
				reason += fmt.Sprintf(" (%d files %s: %s)", len(files), where, strings.Join(files, ", "))
			}
			if reason != "" {
				return &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
					Summary: "--records takes record files; " + path + " " + reason + "; name the files", next: inv.sameCommand()}
			}
		}
		return nil
	}
	paths := slices.Clone(inv.input.values["records"])
	if problem := checkPaths(paths); problem != nil {
		return *problem
	}
	endpoint, err := conn.endpoint(inv.layout.InstallationRoot.Path())
	if err != nil {
		return refused(err)
	}
	check := conn.claimCheck(inv.layout.InstallationRoot.Path(), id, endpoint)
	if err := branch.CheckCommitAccess(id, check); err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("this session must hold goal %s to hand in its records", id), next: inv.publicArgv("goal", "claim", id), Details: []string{err.Error()}}
	}
	base, err := conn.endpointTip(inv.layout.InstallationRoot.Path(), endpoint)
	if err != nil {
		return refused(err)
	}
	line := func(dir string, args ...string) (string, error) {
		out, err := git(dir, args...)
		return strings.TrimSpace(string(out)), err
	}
	top, err := line(inv.cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return refused(err)
	}
	head, err := line(top, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return refused(err)
	}
	capture, err := captureChanges(top, head, "", paths...)
	if err != nil {
		return refused(err)
	}
	capturedTree = capture.tree
	if problem := checkPaths(paths); problem != nil {
		return *problem
	}
	if problem := checkPaths(capture.paths); problem != nil {
		return *problem
	}
	matchingPaths := func(tip string) ([]string, error) {
		out, err := git(top, append([]string{"--literal-pathspecs", "diff", "--name-only", "-z", "--no-renames", tip, capture.tree, "--"}, paths...)...)
		return splitNUL(out), err
	}
	_, localErr := line(top, "rev-parse", "--verify", "--quiet", "refs/heads/goal/"+id+"^{commit}")
	_, remote, err := conn.transport.RemoteTip(inv.layout.InstallationRoot.Path(), endpoint.Remote, "refs/heads/goal/"+id)
	if err != nil {
		return refused(err)
	}
	if localErr != nil && !remote {
		changed, err := matchingPaths(base)
		if err != nil {
			return refused(err)
		}
		if len(changed) == 0 {
			return intentResult{Targets: targets, Outcome: intentUnchanged, Summary: strings.Join(paths, ", ") + " already says this on main"}
		}
	}
	worktree, problem := inv.prepareGoalWorktree(id)
	if problem != nil {
		return *problem
	}
	install := inv.goalWorktreeInstallation(worktree)
	var commit string
	var failure *intentResult
	err = conn.section(install, func(withToken func(func() error) error) error {
		tip, err := line(worktree, "rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			return err
		}
		if _, err := branch.ValidateRangeWithGit(install, base, tip, id, git); err != nil {
			return err
		}
		matching, err := matchingPaths(tip)
		if err != nil {
			return err
		}
		if len(matching) == 0 {
			if err := recordsCheck(install); err != nil {
				return err
			}
			commit = tip
			return nil
		}
		capture.head, capture.paths = tip, matching
		capture.patch, err = git(top, append([]string{"--literal-pathspecs", "diff", "--binary", "--full-index", "--no-renames", tip, capture.tree, "--"}, paths...)...)
		if err != nil {
			return err
		}
		if problem := checkPaths(capture.paths); problem != nil {
			failure = problem
			return nil
		}
		staged, err := stageManual(git, worktree, sameDirectory(top, worktree), capture, conn.applyIndex)
		if err != nil {
			return err
		}
		undo := func(cause error) error {
			if err := staged.undo(); err != nil {
				result := intentResult{Targets: targets, Outcome: intentPartial, code: 1, Summary: "the records were not committed, and earlier staging could not be restored", next: []string{"git", "-C", worktree, "status"}, Details: []string{cause.Error(), err.Error()}}
				failure = &result
				return nil
			}
			return cause
		}
		if err := recordsCheck(install); err != nil {
			return undo(err)
		}
		digest := sha256.Sum256(append([]byte(tip), capture.patch...))
		err = withToken(func() error {
			var err error
			commit, err = conn.commit(branch.CommitRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: id, Kind: branch.Plan,
				OpID: fmt.Sprintf("records-%x", digest[:16]), CheckClaim: check, Transport: conn.transport})
			return err
		})
		if err != nil {
			after, readErr := line(worktree, "rev-parse", "--verify", "HEAD^{commit}")
			if readErr != nil || after != tip {
				result := intentResult{Targets: targets, Outcome: intentPartial, code: 1, Summary: "the goal worktree moved while committing its records; its staging was left as is", next: inv.sameCommand(), Details: []string{err.Error()}}
				failure = &result
				return nil
			}
			return undo(err)
		}
		return nil
	})
	if err != nil {
		return refused(err)
	}
	if failure != nil {
		return *failure
	}
	_, err = conn.push(branch.PushRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: id, OpID: "records-" + commit + "-push", CheckClaim: check, Transport: conn.transport})
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Summary: fmt.Sprintf("goal %s's records are committed as %s but not published: %s", id, shortSHA(commit), oneLine(err.Error())),
			next: inv.sameCommand(), nextReason: "the same command publishes this commit; it never makes another"}
	}
	return inv.landGoal(id, "")
}

// checkRecordFiles is the static gate's project reader over the staged
// goal worktree, so invalid heads refuse before the plan is committed.
func checkRecordFiles(installation string) error {
	roots, err := project.ResolveRoots(installation)
	if err != nil {
		return err
	}
	read, err := project.Read(roots)
	if err != nil {
		return err
	}
	problems := []string{}
	for _, problem := range read.Problems {
		problems = append(problems, problem.String())
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}
