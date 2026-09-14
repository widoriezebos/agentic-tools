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

func writeExclusiveBrief(path, body string) error {
	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(body); err != nil {
		_ = handle.Close()
		_ = os.Remove(path)
		return err
	}
	if err := handle.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func stagedDigests(repoRoot string) (role, req, schema, perms string, id InstallIdentity, err error) {
	rolePath := filepath.Join(repoRoot, "scripts", "agents", "roles", continuationRole+".md")
	role, err = digestFile(rolePath)
	if err != nil {
		err = fmt.Errorf("the continuation role contract is unreadable: %w", err)
		return
	}
	req, err = digestFile(filepath.Join(repoRoot, "scripts", "agents", "roles", continuationRole+".requirements.json"))
	if err != nil {
		err = fmt.Errorf("the continuation requirements are unreadable: %w", err)
		return
	}
	schema, err = digestFile(filepath.Join(repoRoot, "scripts", "agents", "schemas", continuationRole+".schema.json"))
	if err != nil {
		err = fmt.Errorf("the continuation return schema is unreadable: %w", err)
		return
	}
	permsPath := filepath.Join(repoRoot, "scripts", "agents", "permissions", continuationPermissions+".json")
	perms, err = digestFile(permsPath)
	if err != nil {
		err = fmt.Errorf("the continuation permissions preset is unreadable: %w", err)
		return
	}
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", "", "", InstallIdentity{}, err
	}
	id, err = VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		err = fmt.Errorf("staging requires the armed installation identity: %w", err)
	}
	return
}

// StageIntent assembles the full intent for one revival: it writes
// the brief naming the goal and the incident, digests the role
// contract and permissions preset as they exist RIGHT NOW, and
// returns the intent ready for PrepareIntent.
func StageIntent(repoRoot, nonce, goal, jobId, runtime, model, reason string) (Intent, error) {
	roleDigest, reqDigest, schemaDigest, permsDigest, id, err := stagedDigests(repoRoot)
	if err != nil {
		return Intent{}, err
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
	return Intent{
		Nonce: nonce, Goal: goal, JobId: jobId, Reason: reason,
		RepoIdentity: id.RepoIdentity, InstallGen: id.Generation,
		Role: continuationRole, Permissions: continuationPermissions,
		Runtime: runtime, Model: model,
		RoleDigest: roleDigest, BriefDigest: briefDigest, PermsDigest: permsDigest,
		ReqDigest: reqDigest, SchemaDigest: schemaDigest,
	}, nil
}

// StageHandoffIntent binds the continuation contract and brief to a verified
// immutable handoff snapshot before the intent can be minted.
func StageHandoffIntent(repoRoot, nonce, goalID, jobId, runtime, model string, binding HandoffBinding) (Intent, error) {
	if _, err := verifyBoundHandoffState(repoRoot, nonce, goalID, binding); err != nil {
		return Intent{}, fmt.Errorf("handoff state cannot authorize staging: %w", err)
	}
	roleDigest, reqDigest, schemaDigest, permsDigest, id, err := stagedDigests(repoRoot)
	if err != nil {
		return Intent{}, err
	}
	briefPath := BriefPath(repoRoot, nonce)
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		return Intent{}, err
	}
	top, err := filepath.Abs(repoRoot)
	if err != nil {
		return Intent{}, err
	}
	canonicalRoot, err := canonicalExistingPath(repoRoot)
	if err != nil {
		return Intent{}, err
	}
	statePath, err := filepath.Rel(canonicalRoot, binding.StatePath)
	if err != nil {
		return Intent{}, err
	}
	brief := fmt.Sprintf(`# Steward continuation

Working Mode: build

The seat handed off under handoff %s.

Read %s first in bounded views. Before using it, run
`+"`metasystem context verify --root %s --nonce %s`"+` and stop on anything
but ok (its sha256 is %s). Hold the goal %q under the
steward-continuation role contract. The goal ledger and the job records are
the authority wherever the state snapshot disagrees.
`, nonce, filepath.ToSlash(statePath), top, nonce, binding.StateDigest, goalID)
	if err := writeExclusiveBrief(briefPath, brief); err != nil {
		return Intent{}, err
	}
	briefDigest, err := digestFile(briefPath)
	if err != nil {
		return Intent{}, err
	}
	return Intent{
		Nonce: nonce, Goal: goalID, JobId: jobId, Reason: seatHandoffReason,
		RepoIdentity: id.RepoIdentity, InstallGen: id.Generation,
		Role: continuationRole, Permissions: continuationPermissions,
		Runtime: runtime, Model: model,
		RoleDigest: roleDigest, BriefDigest: briefDigest, PermsDigest: permsDigest,
		ReqDigest: reqDigest, SchemaDigest: schemaDigest, Handoff: &binding,
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
	if it.Reason == seatHandoffReason && it.Handoff == nil {
		return fmt.Errorf("seatHandoff intent %s carries no handoff binding", it.Nonce)
	}
	if it.Handoff != nil {
		if it.Reason != seatHandoffReason {
			return fmt.Errorf("intent %s carries a handoff binding for reason %q", it.Nonce, it.Reason)
		}
		if _, err := verifyBoundHandoffState(repoRoot, it.Nonce, it.Goal, *it.Handoff); err != nil {
			return fmt.Errorf("handoff state drifted since the authorization was minted: %w", err)
		}
	}
	return nil
}
