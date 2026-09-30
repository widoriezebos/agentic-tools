package goal

import (
	"fmt"
	"strings"
)

// A goal permission is a standing allowance a person records on one goal:
// goal allow G NAME grants it, goal disallow G NAME withdraws it. Each name
// is stored in the sealed record by a field of its own, so the table maps the
// word a person types to the field a reader checks. Allowing is a person's
// act under the proof a lowering takes; disallowing is anyone's.
type Permission struct {
	// Name is the word goal allow and goal disallow take.
	Name string
	// Words is the permission in plain words, as goal show says it.
	Words string
	// Holds reports whether the goal record carries the permission.
	Holds func(*GoalFile) bool
	// Set records or clears it.
	Set func(*GoalFile, bool)
}

// PermissionStopTestChanges allows a landing to move or change a test
// assertion or fixture on the Stop decision surface; the sealed record keeps
// it as the line "- StopSurface: moves", which the Stop surface audit reads.
const PermissionStopTestChanges = "stop-test-changes"

// Permissions is the table of every goal permission, in the order goal show
// lists them. A new permission is one row here and one field in the record.
var Permissions = []Permission{
	{
		Name:  PermissionStopTestChanges,
		Words: "stop-test changes",
		Holds: func(f *GoalFile) bool { return f.StopSurfaceMoves },
		Set:   func(f *GoalFile, allowed bool) { f.StopSurfaceMoves = allowed },
	},
}

// PermissionNames lists the names goal allow and goal disallow take.
func PermissionNames() []string {
	names := make([]string, 0, len(Permissions))
	for _, permission := range Permissions {
		names = append(names, permission.Name)
	}
	return names
}

// LookupPermission finds a permission by the name a person typed; an unknown
// name is refused with the names that exist.
func LookupPermission(name string) (Permission, error) {
	for _, permission := range Permissions {
		if permission.Name == name {
			return permission, nil
		}
	}
	return Permission{}, fmt.Errorf("%q is not a goal permission; the permissions are: %s", name, strings.Join(PermissionNames(), ", "))
}

// AllowedWords is every permission the goal holds, in plain words.
func AllowedWords(f *GoalFile) []string {
	var words []string
	for _, permission := range Permissions {
		if permission.Holds(f) {
			words = append(words, permission.Words)
		}
	}
	return words
}

// PermissionChange is one allow or disallow carried by the goal's edit
// transaction.
type PermissionChange struct {
	Name    string
	Allowed bool
}

// permissionDelta is the journal's word for a change: NAME=allowed or
// NAME=disallowed.
func permissionDelta(change PermissionChange) string {
	if change.Allowed {
		return change.Name + "=allowed"
	}
	return change.Name + "=disallowed"
}

func parsePermissionDelta(value string) (PermissionChange, error) {
	name, state, found := strings.Cut(value, "=")
	if !found || (state != "allowed" && state != "disallowed") {
		return PermissionChange{}, fmt.Errorf("the recorded permission change %q isn't <permission>=allowed or <permission>=disallowed", value)
	}
	if _, err := LookupPermission(name); err != nil {
		return PermissionChange{}, err
	}
	return PermissionChange{Name: name, Allowed: state == "allowed"}, nil
}

// permissionReason is the History reason of an edit that changes a
// permission: which way it moved and, when given, why.
func permissionReason(change PermissionChange, why string) string {
	reason := "Disallowed: " + change.Name
	if change.Allowed {
		reason = "Allowed: " + change.Name
	}
	if why = strings.TrimSpace(why); why != "" {
		reason += " why=" + why
	}
	return reason
}

// AllowCommand is the public command a person runs to allow a permission.
func AllowCommand(goalID, name string) string {
	return "metasystem goal allow " + goalID + " " + name + " --reason TEXT"
}
