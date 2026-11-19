package launch

import (
	"encoding/json"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// UnitDeclarationKeys fixes the order shared by admission and restoration.
// The first four entries retain their positions in older serialized records.
func UnitDeclarationKeys() [6]string {
	return [6]string{"proof.cheap", "proof.audits", "proof.deadline", "proof.full", "proof.check-kind", "proof.check-budget"}
}

// EffectiveValues makes an old four-value declaration mean legacy with the
// default feedback budget, without rewriting its retained bytes or identity.
func (declaration UnitDeclaration) EffectiveValues() [6]string {
	values := declaration.Values
	if values[4] == "" {
		values[4] = "legacy"
	}
	if values[5] == "" {
		values[5] = "6"
	}
	return values
}

// MarshalJSON keeps default legacy declarations byte-compatible with the old
// shape: carried gate digests include this JSON, including its array length.
func (declaration UnitDeclaration) MarshalJSON() ([]byte, error) {
	type wire UnitDeclaration
	values := declaration.Values[:]
	if effective := declaration.EffectiveValues(); effective[4] == "legacy" && effective[5] == "6" {
		values = values[:4]
	}
	return json.Marshal(struct {
		Values []string
		*wire
	}{values, (*wire)(&declaration)})
}

// RequireSupportedKind prevents a declared bounded check from silently running
// the legacy commands on an engine without the bounded executor.
func (check UnitCheck) RequireSupportedKind() error {
	if check.Declaration == nil {
		return nil
	}
	values := check.Declaration.EffectiveValues()
	for i, key := range UnitDeclarationKeys() {
		if i >= 4 {
			if err := config.SettingValueProblem(key, values[i]); err != nil {
				return err
			}
		}
	}
	if values[4] != "legacy" {
		return fmt.Errorf("bounded checks are not available in this engine yet; the tree must declare legacy")
	}
	return nil
}
