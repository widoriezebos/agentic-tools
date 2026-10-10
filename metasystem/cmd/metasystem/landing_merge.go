package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			fix.Job, fix.Brief = fmt.Sprintf("lane-merge-%x", sha256.Sum256([]byte(fix.Attempt)))[:63], inv.callerPath(inv.input.text("brief"))
			if err = plain.WriteFix(root, fix); err == nil {
				job, err = runner.Manager.Start(launch.StartSpec{ID: fix.Job, Kind: "build", Goal: fix.Goal, Tag: fix.Units[0], WorkingDirectory: checkout, Brief: fix.Brief, UnitsPage: fix.Brief, Units: fix.Units})
			}
		} else {
			job, err = runner.Manager.Status(fix.Job)
		}
		if err == nil && job.State.Terminal() && fix.State == "resolving" {
			if job.State != launch.Completed {
				err = errors.New("resolution job " + fix.Job + " cannot resolve: " + job.Reason)
			} else {
				err = plain.CompleteMerge(home, root, checkout, fix, "metasystem test impact", inv.landing().plainResolve)
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
			patch := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".patch")
			context := filepath.Join(plain.Dir(root), "fixes", fix.Attempt+".context.patch")
			var diff []byte
			diff, err = inv.work().git(checkout, "diff", "--binary", fix.Commit+"^", fix.Commit)
			if err == nil {
				err = os.WriteFile(context, diff, 0o600)
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
					brief = append(brief, []byte(fmt.Sprintf("\nRead the resolved merge %s in the checkout. INPUT %s is the hand resolution against AUTO_MERGE; INPUT %s is the complete first-parent diff. Both are reference files; neither is applied to the checkout.\n", fix.Commit, filepath.Base(patch), filepath.Base(context)))...)
					err = os.WriteFile(readBrief, brief, 0o600)
				}
				if err == nil {
					request := launch.ReadRequest{Directory: checkout, Base: fix.Commit, Brief: readBrief, Goal: fix.Goal, Inputs: []string{patch, context}}
					if fix.State == "resolved" && fix.Read != "" {
						previous, readErr := runner.InspectRead(fix.Read)
						if readErr != nil {
							err = readErr
						} else if previous.Attempt.State != "running" && !previous.Complete {
							request.Retry = previous.Attempt.Number
						}
					}
					if err == nil {
						read, err = runner.StartRead(request)
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
				if read.Complete {
					fix.State = "done"
					result.Summary = "Reviewed the resolved merge of " + fix.Goal
					result.next = inv.publicArgv("landing", "prove")
				}
				if err == nil {
					fix.Reason = ""
					if !read.Complete {
						fix.State = "reviewing"
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
