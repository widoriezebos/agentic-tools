package config

import (
	"path/filepath"
	"strings"
)

// TemplateModeKey declares, in the committed metasystem.conf of the metasystem
// template repository, that this checkout is the template itself. It is the
// one template-mode signal: no document's presence decides it, so renaming a
// design never changes engine behaviour. Adoption never ships it.
const TemplateModeKey = "metasystem.template"

// TemplateMode reports whether the installation at installationRoot is the
// template repository's own. It reads the committed file alone: neither the
// uncommitted .local file nor the environment can make a checkout the
// template.
func TemplateMode(installationRoot string) bool {
	value, found, err := ConfLookup(filepath.Join(installationRoot, "metasystem.conf"), TemplateModeKey)
	return err == nil && found && strings.TrimSpace(value) == "true"
}
