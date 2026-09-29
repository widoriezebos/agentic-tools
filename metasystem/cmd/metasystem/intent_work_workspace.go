package main

// work workspace (design engine-owns-disk-lifetimes Part B, 3.6, U6b): a
// registered place to work, owned by a goal, instead of a copy of the
// checkout under /tmp or a clone beside it. A plain workspace is an empty
// directory; --copy-of REV makes it a linked worktree of this checkout at
// REV. --release ends it, whatever the goal's state; the goal's landing and
// its conclusion end it too.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func workspaceIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "work", action: "workspace", audience: "both", summary: "a registered place to work on a goal: an empty directory or a worktree at a revision, released when done",
		usage: []string{
			"metasystem work workspace G [--name N] [--copy-of REV]",
			"metasystem work workspace G --release [--name N]",
			"metasystem work workspace G --release --discard --name N --reason TEXT",
		},
		details: []string{
			"Use it instead of copying the checkout to /tmp or cloning it: the workspace is registered to the goal before it receives a byte, and the engine removes it when it ends. Without --copy-of it is an empty directory; with --copy-of REV it is a linked worktree of this checkout at REV on branch workspace/goal-G/N, whose commits live in this checkout's object store.",
			"It prints the path and the environment to work in: TMPDIR inside the workspace (beside a worktree) and the machine's Go and staticcheck caches, so nothing lands in a private cache or the system temporary directory.",
			"The same goal and name again return the same workspace and write nothing; the same name at another revision is refused with the two ways out. --release ends it at any time: a worktree must be clean, its branch tip is archived under refs/archive/goal-G/N/ before the branch goes, and a workspace a running process uses is kept, naming it. A release of a workspace already gone is success.",
			"--discard drops a worktree's uncommitted changes (committed work is still archived) and is a person's act: " + humanauthority.PersonActRemedy("the release with --discard") + ".",
			"Output is the path and the environment; --verbose adds the record, layout and cap.",
		},
		flags: []intentFlag{
			{name: "name", value: "N", usage: "the workspace's name under the goal (default: default)"},
			{name: "copy-of", value: "REV", usage: "make a linked worktree of this checkout at REV"},
			{name: "release", usage: "end the workspace: archive, check and remove it"},
			{name: "discard", usage: "a person's act: with --release, drop a worktree's uncommitted changes"},
			{name: "reason", value: "TEXT", usage: "why the changes may be dropped (with --discard)"},
			intentVerboseFlag,
		},
		maxArgs:  1,
		accepts:  []string{refGoal},
		examples: []string{"metasystem work workspace verbs-match-intent", "metasystem work workspace verbs-match-intent --name bed --copy-of HEAD", "metasystem work workspace verbs-match-intent --release --name bed"},
		run:      runIntentWorkWorkspace,
	}}
}

func runIntentWorkWorkspace(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "work workspace needs the goal it belongs to; nothing was done",
			Decision: "metasystem work workspace G [--name N] [--copy-of REV]"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk.withDefaults()
	owner := diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: id}
	name := inv.input.text("name")
	if name == "" {
		name = diskstore.DefaultWorkspaceName
	}
	if !diskstore.ValidWorkspaceName(name) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: fmt.Sprintf("%q is not a workspace name: letters, digits, dot, underscore and dash, not starting with a dot or dash, not ending with a dot or .lock; nothing was done", name)})
	}
	registry := diskstore.CheckoutRegistry(inv.stateRoot)
	if inv.input.has("release") {
		return releaseWorkWorkspace(inv, owners, registry, owner, name)
	}
	for _, flag := range []string{"discard", "reason"} {
		if inv.input.has(flag) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "--" + flag + " goes with --release; nothing was done"})
		}
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	if file, where := goalRecord(projection, id); file == nil {
		return unknownGoal(inv, id)
	} else if where != "live" {
		return inv.render(intentResult{Outcome: intentRefused, Targets: inv.targets(id), code: 1,
			Summary: fmt.Sprintf("goal %s is %s; a workspace belongs to an open goal, and none was made", id, where)})
	}
	copyOf := ""
	if inv.input.has("copy-of") {
		out, err := owners.git(context.Background(), inv.layout.GitRoot, "rev-parse", "--verify", "-q", inv.input.text("copy-of")+"^{commit}")
		copyOf = strings.TrimSpace(string(out))
		if err != nil || copyOf == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: fmt.Sprintf("--copy-of %s names no commit of this checkout; nothing was made", inv.input.text("copy-of"))})
		}
	}
	settings, _ := diskstore.LoadSettings(filepath.Join(inv.layout.InstallationRoot, "metasystem.conf"), nil)
	workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: registry, Control: inv.stateRoot,
		GitRoot: inv.layout.GitRoot, Owner: owner, Name: name, CopyOf: copyOf, CapBytes: settings.Bytes(config.DiskWorkspaceKey),
		Now: owners.now().UTC(), Entropy: rand.Reader, Git: owners.git})
	var conflict *diskstore.WorkspaceConflict
	var unproven *diskstore.WorkspaceUnproven
	switch {
	case errors.As(err, &conflict):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: err.Error() + "; nothing was made",
			next: inv.publicArgv("work", "workspace", id, "--release", "--name", name), nextReason: "release the existing workspace first"})
	case errors.As(err, &unproven):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id), Summary: "no workspace was made: " + err.Error(),
			Decision: "metasystem disk show"})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: inv.targets(id), Summary: err.Error(),
			Decision: "metasystem disk show names what is at the path; the same command makes the workspace once the cause is settled"})
	}
	environment := append([]string{"TMPDIR=" + workspace.Tmp}, workspaceCaches(inv.layout.InstallationRoot)...)
	lines := []string{"path: " + workspace.Record.Path}
	for _, entry := range environment {
		lines = append(lines, "export "+entry)
	}
	if inv.input.switched("verbose") {
		lines = append(lines, "record: "+workspace.Record.ID, "layout: "+workspace.Record.Layout,
			fmt.Sprintf("cap: %d GiB (a target: over it the steward reports, it never removes live work)", workspace.Record.CapBytes>>30))
		if workspace.Record.CopyOf != "" {
			lines = append(lines, "copy of: "+workspace.Record.CopyOf+" on branch "+diskstore.WorkspaceBranch(owner, name))
		}
	}
	data := map[string]any{"path": workspace.Record.Path, "tmp": workspace.Tmp, "environment": environment, "record": workspace.Record}
	what := "workspace " + name + " of " + id
	if workspace.Record.Layout == diskstore.LayoutCopy {
		what += " (a worktree at " + shortSHA(workspace.Record.CopyOf) + ")"
	}
	if !workspace.Created {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(id), Data: data, text: lines,
			Summary: what + " already exists at " + workspace.Record.Path})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), Data: data, text: lines,
		Summary: what + " is ready at " + workspace.Record.Path})
}

// workspaceCaches is the engine's Go and staticcheck caches for this
// installation's domain (Part A), so a build in the workspace never makes
// a private cache; an unresolvable domain prints none and says nothing
// else.
func workspaceCaches(installation string) []string {
	resolution, err := cachedomain.Resolve(os.Environ(), installation)
	if err != nil {
		return nil
	}
	return gocache.Environment(resolution.Paths)
}

func releaseWorkWorkspace(inv *intentInvocation, owners diskOwners, registry diskstore.Registry, owner diskstore.Owner, name string) int {
	targets := inv.targets(owner.Ref)
	if inv.input.has("copy-of") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--copy-of makes a workspace; --release ends one; nothing was done"})
	}
	record, found, err := diskstore.FindWorkspace(registry, owner, name)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the workspace record cannot be read: " + err.Error(),
			Decision: "metasystem disk show"})
	}
	if !found {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: fmt.Sprintf("workspace %s of %s is already gone; nothing to release", name, owner.Ref)})
	}
	var discard *diskstore.Discard
	if inv.input.switched("discard") {
		reason := strings.TrimSpace(inv.input.text("reason"))
		if reason == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--discard needs --reason TEXT: why the uncommitted changes may be dropped; nothing was done"})
		}
		by, err := owners.person(inv.stateRoot)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 3, Targets: targets,
				Summary:  "--discard drops uncommitted work and is a person's act; this shell was not proven to be one; nothing was done",
				Decision: humanauthority.PersonActRemedy("metasystem work workspace " + owner.Ref + " --release --discard --name " + name + " --reason TEXT")})
		}
		discard = &diskstore.Discard{By: by, At: owners.now().UTC(), Reason: reason}
	}
	outcome, err := diskstore.ReleaseWorkspace(context.Background(), diskstore.WorkspaceReleaseRequest{Registry: registry, GitRoot: inv.layout.GitRoot,
		ID: record.ID, Git: owners.git, TakeCensus: owners.census, Discard: discard, By: "work workspace --release", Now: owners.now().UTC(),
		IgnoredReleaseBytes: workspaceIgnoredBytes(inv.layout.InstallationRoot)})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the release stopped: " + err.Error(), Decision: "metasystem disk show"})
	}
	data := map[string]any{"release": outcome, "record": record.ID}
	var lines []string
	if outcome.Archive != "" {
		lines = append(lines, fmt.Sprintf("archived: %s (%d commit(s) nothing else contains)", outcome.Archive, outcome.Unique))
	}
	switch {
	case outcome.Done && outcome.Already:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: fmt.Sprintf("workspace %s of %s is already released", name, owner.Ref)})
	case outcome.Done:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: lines,
			Summary: fmt.Sprintf("released workspace %s of %s at %s", name, owner.Ref, outcome.Path)})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, text: lines,
		Summary:  fmt.Sprintf("workspace %s of %s is kept: %s; nothing was removed", name, owner.Ref, outcome.Reason),
		Decision: outcome.Command})
}
