// Package protocol is the delegated-agent protocol compiled into the engine:
// what one delegated agent receives (role instructions, capability
// requirements, return schemas, permission presets, the role-packet recipe
// table and the brief templates) and the mission host's turn prompt and
// schema. The bytes are engine source: an edit takes effect only through an
// engine built from the edited tree, and no installation file shadows them.
// The one adopter extension point is dispatch.permissions.<role>, which may
// name an envelope file instead of a preset.
package protocol

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

//go:embed role-packets.json roles schemas permissions templates
var files embed.FS

// ReferencePrefix marks a role-packet source, recipe or record field that
// names protocol bytes compiled into the engine rather than an installation
// path.
const ReferencePrefix = "protocol:"

// RolePacketRecipe is the recipe identity a composition record carries for
// role: the compiled-in table and the role's entry in it. The record's
// recipe digest names the exact table bytes.
func RolePacketRecipe(role string) string {
	return ReferencePrefix + "role-packets.json#" + role
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// Files is the whole compiled-in protocol, rooted at the package directory
// (role-packets.json, roles/, schemas/, permissions/, templates/).
func Files() fs.FS { return files }

// RolePackets is the role-packet recipe table.
func RolePackets() []byte {
	data, err := files.ReadFile("role-packets.json")
	if err != nil {
		panic("protocol: role-packets.json is not embedded: " + err.Error())
	}
	return data
}

func read(name string) ([]byte, error) {
	data, err := files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("engine protocol has no %s", name)
	}
	return data, nil
}

func named(kind, name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid protocol %s name %q", kind, name)
	}
	return nil
}

// RoleInstructions is roles/<role>.md.
func RoleInstructions(role string) ([]byte, error) {
	if err := named("role", role); err != nil {
		return nil, err
	}
	return read("roles/" + role + ".md")
}

// RoleRequirements is roles/<role>.requirements.json.
func RoleRequirements(role string) ([]byte, error) {
	if err := named("role", role); err != nil {
		return nil, err
	}
	return read("roles/" + role + ".requirements.json")
}

// RoleSchema is schemas/<role>.schema.json.
func RoleSchema(role string) ([]byte, error) {
	if err := named("role", role); err != nil {
		return nil, err
	}
	return read("schemas/" + role + ".schema.json")
}

// Dispatchable reports whether role carries both instructions and
// capability requirements, the pair dispatch requires.
func Dispatchable(role string) bool {
	if _, err := RoleInstructions(role); err != nil {
		return false
	}
	_, err := RoleRequirements(role)
	return err == nil
}

// Permissions is the shipped permission preset permissions/<preset>.json.
func Permissions(preset string) ([]byte, error) {
	if err := named("permission preset", preset); err != nil {
		return nil, err
	}
	return read("permissions/" + preset + ".json")
}

// IsPreset reports whether name is a shipped permission preset.
func IsPreset(name string) bool {
	_, err := Permissions(name)
	return err == nil
}

// Template is templates/<name>, name including its extension.
func Template(name string) ([]byte, error) {
	if !nameRe.MatchString(strings.TrimSuffix(name, ".md")) || !strings.HasSuffix(name, ".md") {
		return nil, fmt.Errorf("invalid protocol template name %q", name)
	}
	return read("templates/" + name)
}

// Templates is the brief templates directory as a file system.
func Templates() fs.FS {
	templates, err := fs.Sub(files, "templates")
	if err != nil {
		panic("protocol: templates are not embedded: " + err.Error())
	}
	return templates
}

// IsReference reports whether source names compiled-in protocol bytes.
func IsReference(source string) bool {
	return strings.HasPrefix(source, ReferencePrefix)
}

// Source resolves a role-packet source. ok is false for an installation path
// (the caller reads it from the installation); for a protocol reference ok is
// true and err refuses an unknown or escaping name.
func Source(source string) (data []byte, ok bool, err error) {
	if !IsReference(source) {
		return nil, false, nil
	}
	name := strings.TrimPrefix(source, ReferencePrefix)
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "../") || name == ".." || strings.HasPrefix(name, "/") {
		return nil, true, fmt.Errorf("invalid protocol reference %q", source)
	}
	data, err = read(name)
	return data, true, err
}
