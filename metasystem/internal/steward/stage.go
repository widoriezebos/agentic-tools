package steward

// Staging the launch inputs at mint time: the intent records digests
// of the exact bytes that will run — the role contract, the brief,
// the permissions preset — so what was authorized is what launches,
// immune to configuration drift between mint and dispatch.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// continuationRole is the one role this machinery ever launches.
const continuationRole = "steward-continuation"

// continuationPermissions is the standing preset for continuations.
const continuationPermissions = "workspace"

// BriefPath is where a staged brief lives, keyed by the intent.
func BriefPath(repoRoot, nonce string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "briefs", nonce+".md")
}

func digestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// StageIntent assembles the full intent for one revival: it writes
// the brief naming the goal and the incident, digests the role
// contract and permissions preset as they exist RIGHT NOW, and
// returns the intent ready for PrepareIntent.
func StageIntent(repoRoot, nonce, goal, jobId, runtime, model, reason string) (Intent, error) {
	rolePath := filepath.Join(repoRoot, "scripts", "agents", "roles", continuationRole+".md")
	roleDigest, err := digestFile(rolePath)
	if err != nil {
		return Intent{}, fmt.Errorf("the continuation role contract is unreadable: %w", err)
	}
	reqDigest, err := digestFile(filepath.Join(repoRoot, "scripts", "agents", "roles", continuationRole+".requirements.json"))
	if err != nil {
		return Intent{}, fmt.Errorf("the continuation requirements are unreadable: %w", err)
	}
	schemaDigest, err := digestFile(filepath.Join(repoRoot, "scripts", "agents", "schemas", continuationRole+".schema.json"))
	if err != nil {
		return Intent{}, fmt.Errorf("the continuation return schema is unreadable: %w", err)
	}
	permsPath := filepath.Join(repoRoot, "scripts", "agents", "permissions", continuationPermissions+".json")
	permsDigest, err := digestFile(permsPath)
	if err != nil {
		return Intent{}, fmt.Errorf("the continuation permissions preset is unreadable: %w", err)
	}
	briefPath := BriefPath(repoRoot, nonce)
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		return Intent{}, err
	}
	brief := fmt.Sprintf(`# Steward continuation

Working Mode: build

The idle watchdog launched you: %s

Serve the open goal %q under the steward-continuation role contract.
Orient from the memory handoff and the goal ledger before touching
anything; yield if a live worker shows fresh progress.
`, reason, goal)
	if err := os.WriteFile(briefPath, []byte(brief), 0o644); err != nil {
		return Intent{}, err
	}
	briefDigest, err := digestFile(briefPath)
	if err != nil {
		return Intent{}, err
	}
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return Intent{}, err
	}
	id, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		return Intent{}, fmt.Errorf("staging requires the armed installation identity: %w", err)
	}
	return Intent{
		Nonce: nonce, Goal: goal, JobId: jobId,
		RepoIdentity: id.RepoIdentity, InstallGen: id.Generation,
		Role: continuationRole, Permissions: continuationPermissions,
		Runtime: runtime, Model: model,
		RoleDigest: roleDigest, BriefDigest: briefDigest, PermsDigest: permsDigest,
		ReqDigest: reqDigest, SchemaDigest: schemaDigest,
	}, nil
}

// VerifyStagedDigests re-checks the staged bytes immediately before
// launch: any drift between mint and dispatch refuses by field.
func VerifyStagedDigests(repoRoot string, it Intent) error {
	rolePath := filepath.Join(repoRoot, "scripts", "agents", "roles", it.Role+".md")
	if got, err := digestFile(rolePath); err != nil || got != it.RoleDigest {
		return fmt.Errorf("role contract drifted since the authorization was minted (%s)", it.Role)
	}
	if got, err := digestFile(BriefPath(repoRoot, it.Nonce)); err != nil || got != it.BriefDigest {
		return fmt.Errorf("staged brief drifted since the authorization was minted")
	}
	permsPath := filepath.Join(repoRoot, "scripts", "agents", "permissions", it.Permissions+".json")
	if got, err := digestFile(permsPath); err != nil || got != it.PermsDigest {
		return fmt.Errorf("permissions preset drifted since the authorization was minted (%s)", it.Permissions)
	}
	reqPath := filepath.Join(repoRoot, "scripts", "agents", "roles", it.Role+".requirements.json")
	if got, err := digestFile(reqPath); err != nil || got != it.ReqDigest {
		return fmt.Errorf("role requirements drifted since the authorization was minted (%s)", it.Role)
	}
	schemaPath := filepath.Join(repoRoot, "scripts", "agents", "schemas", it.Role+".schema.json")
	if got, err := digestFile(schemaPath); err != nil || got != it.SchemaDigest {
		return fmt.Errorf("return schema drifted since the authorization was minted (%s)", it.Role)
	}
	return nil
}
