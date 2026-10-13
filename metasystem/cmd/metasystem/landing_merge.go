package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// laneMergeWork uses the lane's pending merge instead of a goal branch workspace.
func laneMergeWork(inv *intentInvocation) (int, bool) {
	if !strings.HasPrefix(inv.input.text("work"), "lane-merge-") || len(inv.input.args) != 1 {
		return 0, false
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem), true
	}
	root, checkout := inv.layout.InstallationRoot.Path(), inv.layout.GitRoot
	fix, err := plain.ReadFix(root)
	if err != nil {
		return inv.render(landingLaneFailure(nil, err.Error(), nil)), true
	}
	if fix == nil || fix.Goal != inv.input.args[0] || fix.Units[0] != inv.input.text("work") {
		fmt.Fprintln(inv.stderr, "No active lane merge matches this work; continuing with the normal goal build")
		return 0, false
	}
	_, home, registered, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem), true
	}
	if registered.Install != root || registered.Root != checkout || !landingFixActor(root, int64(os.Getpid())) {
		return inv.render(landingLaneFailure(nil, "only the lane's current session builds its merge", nil)), true
	}
	if err := plain.RefreshMerge(root, checkout, fix, plain.ProveSeams{Git: inv.landing().plainResolve.Git}); err != nil {
		return inv.render(landingLaneFailure(nil, err.Error(), nil)), true
	}
	if fix.State == "abandoned" {
		return inv.render(intentResult{Outcome: intentUnchanged, Data: fix, Summary: fix.Reason, next: inv.publicArgv("landing", "resolve")}), true
	}
	runner := inv.unitRunner()
	result := intentResult{Outcome: intentConfirmed, Data: fix, Summary: "Resolving conflicts of " + fix.Goal, next: inv.sameCommand(), nextReason: "collects this same job"}
	if inv.command.action == "build" {
		if !inv.input.has("brief") || len(inv.input.values["check"]) != 1 || inv.input.text("check") != "metasystem test impact" {
			return inv.render(landingLaneFailure(nil, "a lane merge requires a brief and --check 'metasystem test impact'", nil)), true
		}
		var job launch.Record
		if fix.Job == "" {
			id := fmt.Sprintf("lane-merge-%x", sha256.Sum256([]byte(fix.Attempt)))[:63]
			if fix.Launch != nil && fix.Launch.Job != "" {
				id = fix.Launch.Job
			}
			job, err = runner.Manager.Status(id)
			if err == nil {
				if fix.Launch == nil {
					fix.Launch = &plain.LaunchRecord{}
				}
				fix.Launch.Job = id
				err = plain.LaunchResolution(root, fix, inv.landing().now(), "landing", func() (string, error) { return job.ID, nil })
			} else if errors.Is(err, os.ErrNotExist) {
				fix.Launch = &plain.LaunchRecord{Attempt: fix.Attempt, Job: id}
				fix.Brief = inv.callerPath(inv.input.text("brief"))
				brief, readErr := os.ReadFile(fix.Brief)
				if readErr != nil {
					return inv.render(landingLaneFailure(nil, readErr.Error(), nil)), true
				}
				fix.Brief = filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".build.md")
				brief = append(brief, []byte("\nBefore returning, stage the resolution and run: "+shellCommand([]string{"env", "LANDING_PROOF_BASE=" + fix.Commit, "metasystem", "test", "impact", "--check"})+"\n")...)
				if err = os.WriteFile(fix.Brief, brief, 0600); err == nil {
					err = plain.LaunchResolution(root, fix, inv.landing().now(), "landing", func() (string, error) {
						var launchErr error
						job, launchErr = runner.Manager.Start(launch.StartSpec{ID: id, Kind: "build", Goal: fix.Goal, Tag: fix.Units[0], WorkingDirectory: checkout, Brief: fix.Brief, UnitsPage: fix.Brief, Units: fix.Units})
						return job.ID, launchErr
					})
				}
			}
		} else {
			job, err = runner.Manager.Status(fix.Job)
		}
		if err == nil && job.State.Terminal() && fix.State == "resolving" {
			if job.State != launch.Completed {
				err = errors.New("resolution job " + fix.Job + " cannot resolve: " + job.Reason)
			} else {
				err = plain.CompleteMerge(home, root, checkout, fix, "metasystem test impact --check", inv.landing().plainResolve)
			}
		}
		if fix.State == "reviewing" || fix.State == "resolved" {
			result.Summary = "Committed the resolved merge of " + fix.Goal
			result.next = inv.publicArgv("work", "review", fix.Goal, "--work", fix.Units[0])
		}
	} else {
		if fix.State != "reviewing" && fix.State != "resolved" {
			err = errors.New("the merge build has not completed")
		} else {
			readEffects := inv.laneBatchSeams(home, registered, inv.landing().plainProve)
			readEffects.Now = inv.landing().now
			returned, expireErr := plain.ExpireResolutionRead(root, func(ref string) (bool, error) {
				read, err := runner.StopRead(ref)
				if err == nil && len(read.Uncertain) > 0 {
					err = fmt.Errorf("resolution read jobs could not be proved stopped: %s", strings.Join(read.Uncertain, ", "))
				}
				return !read.Complete && len(read.Uncertain) == 0, err
			}, readEffects)
			if expireErr != nil {
				return inv.render(landingLaneFailure(nil, "the resolution read could not be returned", expireErr)), true
			}
			if returned != nil {
				return inv.render(intentResult{Outcome: intentConfirmed, Data: returned, Summary: returned.Reason, Details: inv.writeReturnedCard(fix.Goal), next: inv.publicArgv("landing", "status")}), true
			}
			current, readErr := plain.FixForAttempt(root, fix.Attempt)
			if readErr != nil {
				return inv.render(landingLaneFailure(nil, "the resolution read could not be read", readErr)), true
			}
			if current != nil {
				fix = current
			}
			patch := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".resolution.patch")
			context := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".context.json")
			var diff []byte
			var fallbackParent int
			var paths []string
			diff, fallbackParent, err = inv.mergeResolutionDiff(checkout, fix, &paths)
			if err == nil && fallbackParent == 0 && !resolutionNeedsRead(paths) {
				fix.State = "done"
				fix.Reason = fmt.Sprintf("resolution of %s reviewed by its check only: %d test/records files, no production code", fix.Goal, len(paths))
				if err = plain.WriteFix(root, fix); err != nil {
					return inv.render(landingLaneFailure(nil, "the resolution check could not be recorded", err)), true
				}
				return inv.render(intentResult{Outcome: intentConfirmed, Data: fix, Summary: fix.Reason, next: inv.publicArgv("landing", "prove")}), true
			}
			if err == nil {
				err = os.WriteFile(patch, diff, 0o600)
			}
			if err == nil {
				var detail []byte
				detail, err = json.Marshal(struct {
					Paths        any     `json:"paths"`
					CheckResult  string  `json:"check_result"`
					CheckMinutes float64 `json:"check_minutes"`
				}{fix.Paths, fix.CheckResult, fix.CheckMinutes})
				if err == nil {
					err = os.WriteFile(context, detail, 0o600)
				}
			}
			if err == nil {
				var read launch.ReadResult
				readBrief := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".read.md")
				var brief []byte
				brief, err = os.ReadFile(fix.Brief)
				if err != nil {
					err = fmt.Errorf("read the merge brief: %w", err)
				}
				if err == nil {
					scope := fmt.Sprintf("\nRead only the conflict resolution of merge %s. The goal's units were read on their branch; the rest of the goal is out of scope. CommitDiff contains the resolution diff from the automatic merge tree, excluding the record's generated paths. INPUT %s records the conflicted paths, their classes and the check result. It is a reference file, never applied to the checkout.\n", fix.Commit, filepath.Base(context))
					if fallbackParent == 1 {
						scope += "The resolution kept merge-tree's side; read the conflicted paths' first-parent diff.\n"
					} else if fallbackParent == 2 {
						scope += "The resolution kept main's side; this diff shows what the goal's side lost.\n"
					}
					brief = append(brief, []byte(scope)...)
					err = os.WriteFile(readBrief, brief, 0o600)
				}
				if err == nil {
					request := launch.ReadRequest{Directory: checkout, Base: fix.Commit, CommitDiff: patch, Brief: readBrief, Goal: fix.Goal, Inputs: []string{context}}
					if fix.State == "resolved" && fix.Read != "" {
						previous, readErr := runner.InspectRead(fix.Read)
						if readErr != nil {
							err = readErr
						} else if previous.Attempt.State != "running" && !previous.Complete {
							request.Retry = previous.Attempt.Number
						}
					}
					if err == nil {
						if fix.ReadStartedAt == "" {
							fix.ReadStartedAt = inv.landing().now().UTC().Format(time.RFC3339Nano)
							fix.ReadLaneOwned = true
							err = plain.WriteFix(root, fix)
						}
						if err == nil {
							read, err = runner.StartRead(request)
						}
						fix.UpdateReadMinutes(inv.landing().now())
					}
				}
				if read.Ref != "" {
					fix.Read = read.Ref
				}
				if err == nil && read.Attempt.Outcome == "read-failed" {
					reason := "the reader did not finish successfully"
					for _, step := range read.Attempt.Round.Steps {
						if step.State == launch.StepFailed && step.Reason != "" {
							reason = step.Reason
							break
						}
					}
					err = fmt.Errorf("merge read %s failed: %s", read.Ref, reason)
				}
				result = inv.diagnosticReadResult(read, err, patch)
				if err == nil && read.Empty {
					err = errors.New("the merge read has no reviewable diff")
				}
				if err == nil && read.Complete {
					fix.State = "done"
					result.Summary = "Reviewed the resolved merge of " + fix.Goal
					result.next = inv.publicArgv("landing", "prove")
				}
				if err == nil {
					if read.Complete || fix.State != "reviewing" {
						fix.Reason = ""
					}
					if !read.Complete {
						fix.State = "reviewing"
						if fix.Reason != "" {
							result.Details = append(result.Details, fix.Reason)
						}
					}
					err = plain.WriteFix(root, fix)
				}
			}
			if err != nil {
				fix.State, fix.Reason = "resolved", "the merge read could not complete: "+err.Error()
				err = errors.Join(err, plain.WriteFix(root, fix))
			}
		}
	}
	if err != nil {
		result.Outcome, result.code, result.Summary = intentFailed, 1, err.Error()
		result.next = inv.publicArgv("landing", "resolve")
		if fix.State == "resolved" {
			result.next = inv.publicArgv("work", "review", fix.Goal, "--work", fix.Units[0])
		}
	}
	return inv.render(result), true
}

// mergeResolutionDiff compares the automatic merge with the committed resolution,
// excluding generated paths. A resolution that kept the automatic tree's side
// is judged on its recorded conflicted paths against the parent whose side
// was not kept. The returned parent number identifies that fallback.
func (inv *intentInvocation) mergeResolutionDiff(checkout string, fix *plain.Fix, paths ...*[]string) ([]byte, int, error) {
	parents, err := inv.work().git(checkout, "rev-list", "--parents", "-n", "1", fix.Commit)
	if err != nil {
		return nil, 0, err
	}
	commits := strings.Fields(string(parents))
	if len(commits) != 3 {
		return nil, 0, errors.New("the resolution must be a two-parent merge commit")
	}
	tree, err := inv.work().git(checkout, "merge-tree", "--write-tree", commits[1], commits[2])
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, 0, err
		}
	}
	automatic, _, _ := strings.Cut(string(tree), "\n")
	if strings.TrimSpace(automatic) == "" || len(fix.Paths) == 0 {
		return nil, 0, errors.New("the automatic merge tree and recorded conflict paths are required")
	}
	prefix := inv.layout.InstallationRel
	var exclusions []string
	for _, set := range fix.Generated {
		for _, path := range set.Paths {
			exclusions = append(exclusions, ":(exclude)"+filepath.ToSlash(filepath.Join(prefix, path)))
		}
	}
	args := append([]string{"diff", "--binary", strings.TrimSpace(automatic), fix.Commit + "^{tree}", "--"}, exclusions...)
	diff, err := inv.work().git(checkout, args...)
	if err != nil || len(bytes.TrimSpace(diff)) > 0 {
		return diff, 0, errors.Join(err, inv.resolutionPaths(checkout, args, paths))
	}
	for parent := 1; parent <= 2; parent++ {
		args = []string{"diff", "--binary", commits[parent], fix.Commit + "^{tree}", "--"}
		for _, path := range fix.Paths {
			args = append(args, ":(literal)"+path.Path)
		}
		args = append(args, exclusions...)
		diff, err = inv.work().git(checkout, args...)
		if err != nil || len(bytes.TrimSpace(diff)) > 0 {
			return diff, parent, errors.Join(err, inv.resolutionPaths(checkout, args, paths))
		}
	}
	return nil, 2, errors.New("the resolved merge has no reviewable diff on its recorded conflict paths")
}

func (inv *intentInvocation) resolutionPaths(checkout string, args []string, paths []*[]string) error {
	if len(paths) == 0 {
		return nil
	}
	args = append([]string{"diff", "--name-only", "-z", "--no-renames"}, args[2:]...)
	raw, err := inv.work().git(checkout, args...)
	if err != nil {
		return err
	}
	for _, path := range strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00") {
		if path != "" {
			path = strings.TrimPrefix(path, strings.TrimSuffix(inv.layout.InstallationRel, "/")+"/")
			*paths[0] = append(*paths[0], path)
		}
	}
	return nil
}

// resolutionNeedsRead excludes tests and records; production Go files always need a read.
func resolutionNeedsRead(paths []string) bool {
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if strings.HasSuffix(path, ".go") {
			return true
		}
		record := false
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			if part == "testdata" || part == "plans" || part == "memory" || part == "records" || part == "skills" {
				record = true
				break
			}
		}
		if !record {
			return true
		}
	}
	return false
}
