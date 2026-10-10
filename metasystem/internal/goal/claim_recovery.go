package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Read every registered checkout so the take-over warning includes a holding
// session in another worktree. A live or unknown process cannot veto a person.
func localHolderWarning(r VerbRequest, claim *ClaimRecord) (string, error) {
	if claim.Machine != r.Actor.Machine {
		return "", nil
	}
	path, err := registry.DefaultPath()
	if err != nil {
		return "", err
	}
	checkouts, err := registry.HostCheckouts(path)
	if err != nil {
		return "", err
	}
	roots := []string{r.Endpoint.Root}
	for _, checkout := range checkouts {
		if !slices.Contains(roots, checkout.Path) {
			roots = append(roots, checkout.Path)
		}
	}
	prober := r.claimHolderProber
	if prober == nil {
		prober = identity.KernelProber{}
	}
	for _, root := range roots {
		dir := filepath.Join(root, "artifacts", "agents", "mains")
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("session custody cannot be read: %w", err)
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return "", err
			}
			var record struct {
				MainID       string `json:"mainId"`
				OwnerLineage string `json:"ownerLineage"`
				activityProcessRef
			}
			if err := json.Unmarshal(data, &record); err != nil {
				return "", fmt.Errorf("session custody %s cannot be read: %w", entry.Name(), err)
			}
			lineage := record.OwnerLineage
			if lineage == "" {
				lineage = record.MainID
			}
			if lineage != claim.Lineage {
				continue
			}
			status := ""
			switch identity.LiveRef(prober, record.identityRef()) {
			case identity.Alive:
				status = "is live"
			case identity.Unknown:
				status = "cannot be proven dead"
			default:
				continue
			}
			return fmt.Sprintf("the holding session %s on %s %s (pid %d, start %d): its further writes to the goal are refused by the claim check", claim.Lineage, claim.Machine, status, record.Pid, record.PidStartedAt), nil
		}
	}
	return "", nil
}
