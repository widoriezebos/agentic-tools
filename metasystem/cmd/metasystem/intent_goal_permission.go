package main

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// goal allow G PERMISSION and goal disallow G PERMISSION record and withdraw
// a goal permission (internal/goal Permissions). Both run through the goal's
// edit transaction, so the record is resealed, its revision bumped and its
// History says which way the permission moved and why. Allowing is a person's
// act under the proof a lowering takes; disallowing is anyone's; a repeat of
// what already holds is success with no record (R-129-ui).

// permissionWord reads the PERMISSION after G and looks it up; a missing or
// unknown name is refused with the names that exist.
func (inv *intentInvocation) permissionWord(id, act string) (goal.Permission, int, bool) {
	known := "the permissions are: " + strings.Join(goal.PermissionNames(), ", ")
	if len(inv.input.args) < 2 || strings.TrimSpace(inv.input.args[1]) == "" {
		return goal.Permission{}, inv.refuse(id, fmt.Sprintf("needs the permission: metasystem goal %s G PERMISSION; nothing was done", act), known), false
	}
	permission, err := goal.LookupPermission(inv.input.args[1])
	if err != nil {
		return goal.Permission{}, inv.refuse(id, err.Error()+"; nothing was done", ""), false
	}
	return permission, 0, true
}

func runIntentAllow(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	permission, code, ok := inv.permissionWord(id, "allow")
	if !ok {
		return code
	}
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	command := goal.AllowCommand(id, permission.Name)
	if strings.TrimSpace(reason) == "" {
		return inv.refuse(id, fmt.Sprintf("allowing %s says why; nothing was done", permission.Words), "run "+command)
	}
	personAct := func(detail string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary:  fmt.Sprintf("allowing %s is a person's act%s; nothing was done", permission.Words, detail),
			Decision: "a person runs " + command + " at the enrolled terminal (naming themself with --by NAME where the shell carries a session lineage)"})
	}
	// An agent session names itself by its lineage; it cannot allow, and
	// the refusal names the command the person runs.
	agent := inv.input.has("lineage") || inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != ""
	if agent && inv.input.text("by") == "" {
		return personAct(" and this command runs as an agent session")
	}
	actor, proof, refused := inv.actingAs("allow", id, actorHuman)
	if refused != nil {
		return personAct(" and no enrolled person was proven here (" + refused.Summary + ")")
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	change := goal.PermissionChange{Name: permission.Name, Allowed: true}
	return inv.render(inv.goalAct(id, "allow", inv.syncOwner("edit", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Edit(req, f.id, goal.EditFields{Permission: &change, Why: reason, Proof: proof})
	}, "id")))
}

func runIntentDisallow(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	permission, code, ok := inv.permissionWord(id, "disallow")
	if !ok {
		return code
	}
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	actor, proof, refused := inv.actingAs("disallow", id, actorEither)
	if refused != nil {
		return inv.render(*refused)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	change := goal.PermissionChange{Name: permission.Name, Allowed: false}
	return inv.render(inv.goalAct(id, "disallow", inv.syncOwner("edit", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Edit(req, f.id, goal.EditFields{Permission: &change, Why: reason, Proof: proof})
	}, "id")))
}
