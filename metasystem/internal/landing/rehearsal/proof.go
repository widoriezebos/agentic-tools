package rehearsal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

const fixtureCheck = "go vet ./internal/rehearsalgreen/... && go test -count=1 -timeout 30m ./internal/rehearsalgreen/..."

// Only the scratch static group changes. Other groups and canaries retain
// their real declarations so the engine measures the real selection shapes.
func (s *Scratch) fixtureProofContract() error {
	path := filepath.Join(s.Module, "testing.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var contract map[string]json.RawMessage
	var groups []map[string]json.RawMessage
	if err = json.Unmarshal(data, &contract); err != nil {
		return err
	}
	if err = json.Unmarshal(contract["groups"], &groups); err != nil {
		return err
	}
	found := false
	for _, group := range groups {
		if string(group["id"]) == `"fast-static-build"` {
			group["argv"], _ = json.Marshal([]string{"sh", "-c", fixtureCheck})
			// The fixture check is independent of the engine's source audits.
			delete(group, "requires")
			found = true
		}
	}
	if !found {
		return fmt.Errorf("scratch contract has no fast-static-build group")
	}
	contract["groups"], _ = json.Marshal(groups)
	data, err = json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, append(data, '\n'), 0644, s.Module)
	return err
}
