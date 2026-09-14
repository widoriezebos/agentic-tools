package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
)

type stagedHandoffFixture struct {
	binding      HandoffBinding
	liveSource   string
	stagedSource string
	manifest     string
}

func testDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testHandoffReference(t *testing.T, root, nonce, slot, purpose, source string, data []byte) HandoffReference {
	t.Helper()
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	openPath := filepath.Join(HandoffDir(root, nonce), "references", slot+".json")
	writeTestFile(t, openPath, data)
	rel, err := filepath.Rel(canonicalRoot, openPath)
	if err != nil {
		t.Fatal(err)
	}
	return HandoffReference{
		CompositionReference: dispatch.CompositionReference{
			Slot: slot, Purpose: purpose, Path: filepath.ToSlash(rel), OpenPath: openPath,
			Digest: testDigest(data), Bytes: len(data), Lifetime: "immutable",
		},
		Required: true, SourcePath: filepath.ToSlash(source),
	}
}

func writeStagedHandoffFixture(t *testing.T, root, nonce string) stagedHandoffFixture {
	t.Helper()
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := HandoffDir(root, nonce)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	liveSource := filepath.Join(root, "artifacts", "agents", "jobs", "job-9.json")
	writeTestFile(t, liveSource, []byte("{\"status\":\"running\"}\n"))
	jobReference := testHandoffReference(t, root, nonce, "job-9", "open job record", "artifacts/agents/jobs/job-9.json", []byte("{\"status\":\"running\"}\n"))

	manifest := HandoffManifest{
		SchemaVersion: HandoffSchemaVersion,
		OpenJobs: []HandoffOpenJob{{
			ID: "job-9", Role: "implementer", Status: "running", Phase: "work", Record: jobReference,
		}},
		LastLandings: HandoffLastLandings{
			History:  []HandoffLanding{{At: "2026-09-14T18:00:00Z", OpID: "op-1", Verb: "land-ready", Actor: "m1+l1"}},
			Receipts: []HandoffReceipt{{Receipt: "memory/receipts.log:7", Type: "other", Outcome: "shipped", Note: "prior unit"}},
		},
		MessagesOwed: []HandoffMessage{{Nonce: "notice-1", Message: "review the handoff", DeliveryStatus: "pending"}},
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestData = append(manifestData, '\n')
	manifestPath := filepath.Join(dir, "manifest.json")
	writeTestFile(t, manifestPath, manifestData)
	manifestRel, err := filepath.Rel(canonicalRoot, manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestReference := dispatch.CompositionReference{
		Slot: "manifest", Purpose: "handoff overflow", Path: filepath.ToSlash(manifestRel), OpenPath: manifestPath,
		Digest: testDigest(manifestData), Bytes: len(manifestData), Lifetime: "immutable",
	}

	originalSession := "session / original"
	writtenAt := time.Date(2026, 9, 14, 18, 30, 0, 0, time.UTC)
	predecessor := identity.Ref{Pid: 4242, StartedAtSec: 1}
	state := HandoffState{
		SchemaVersion: HandoffSchemaVersion,
		WrittenAt:     writtenAt,
		Seat: HandoffSeat{
			Machine: "m1", Runtime: "claude", Session: originalSession,
			NormalizedSession: goal.NormalizeSession(originalSession), MainID: "main-1",
			Identity: predecessor, Tag: "tag-1", JobID: "job-9",
		},
		HeldGoal: HandoffHeldGoal{
			ID: "fix-it", State: "claimed", AcceptedRevision: 7,
			Claimant: HandoffClaimant{Machine: "m1", Lineage: "l1", At: "2026-09-14T17:00:00Z", Revision: 6},
		},
		NextStep: HandoffNextStep{Text: "continue the B1 implementation"},
		Scratch: []HandoffReference{
			testHandoffReference(t, root, nonce, "scratch-proof", "scratch proof", "artifacts/reports/proof.json", []byte("{\"proof\":true}\n")),
		},
		Manifest:   &manifestReference,
		Disposable: HandoffDisposable,
	}
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	stateData = append(stateData, '\n')
	statePath := filepath.Join(dir, "state.json")
	writeTestFile(t, statePath, stateData)

	return stagedHandoffFixture{
		binding: HandoffBinding{
			StatePath: statePath, StateDigest: testDigest(stateData), Runtime: "claude",
			Session: goal.NormalizeSession(originalSession), MainId: "main-1", Predecessor: predecessor,
			PredecessorTag: "tag-1", PredecessorJob: "job-9", RecordedAt: writtenAt,
		},
		liveSource: liveSource, stagedSource: jobReference.OpenPath, manifest: manifestPath,
	}
}

func rewriteState(t *testing.T, fixture stagedHandoffFixture, mutate func(map[string]any)) stagedHandoffFixture {
	t.Helper()
	data, err := os.ReadFile(fixture.binding.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	mutate(object)
	data, err = json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	writeTestFile(t, fixture.binding.StatePath, data)
	fixture.binding.StateDigest = testDigest(data)
	return fixture
}

func rewriteManifest(t *testing.T, fixture stagedHandoffFixture, mutate func(map[string]any)) stagedHandoffFixture {
	t.Helper()
	data, err := os.ReadFile(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	mutate(object)
	data, err = json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	writeTestFile(t, fixture.manifest, data)
	return rewriteState(t, fixture, func(state map[string]any) {
		manifest := state["manifest"].(map[string]any)
		manifest["digest"] = testDigest(data)
		manifest["bytes"] = len(data)
	})
}

func TestHandoffStateVerifierIsStrictAndBounded(t *testing.T) {
	t.Run("unknown state field", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000001")
		fixture = rewriteState(t, fixture, func(object map[string]any) { object["surprise"] = true })
		if _, err := verifyBoundHandoffState(root, "0000000000000001", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("unknown state field must refuse: %v", err)
		}
	})

	t.Run("unknown manifest field", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000006")
		fixture = rewriteManifest(t, fixture, func(object map[string]any) { object["surprise"] = true })
		if _, err := verifyBoundHandoffState(root, "0000000000000006", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("unknown manifest field must refuse: %v", err)
		}
	})

	t.Run("optional missing scratch is explicit", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000007")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			scratch := object["scratch"].([]any)
			object["scratch"] = append(scratch, map[string]any{
				"purpose": "optional notes", "required": false,
				"sourcePath": "artifacts/reports/absent.md", "status": "missing",
			})
		})
		if _, err := verifyBoundHandoffState(root, "0000000000000007", "fix-it", fixture.binding); err != nil {
			t.Fatalf("explicit optional absence must remain readable: %v", err)
		}
	})

	t.Run("second JSON value", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000002")
		handle, err := os.OpenFile(fixture.binding.StatePath, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := handle.WriteString("{}\n"); err != nil {
			handle.Close()
			t.Fatal(err)
		}
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(fixture.binding.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		fixture.binding.StateDigest = testDigest(data)
		if _, err := verifyBoundHandoffState(root, "0000000000000002", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
			t.Fatalf("second JSON value must refuse: %v", err)
		}
	})

	t.Run("state byte limit", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000003")
		data, err := os.ReadFile(fixture.binding.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, make([]byte, output.MaxInlineBytes-len(data))...)
		writeTestFile(t, fixture.binding.StatePath, data)
		fixture.binding.StateDigest = testDigest(data)
		if _, err := verifyBoundHandoffState(root, "0000000000000003", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "must be smaller than") {
			t.Fatalf("state at the byte limit must refuse: %v", err)
		}
	})

	t.Run("combined list limit", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000004")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			messages := make([]any, 51)
			for i := range messages {
				messages[i] = map[string]any{"nonce": "n", "message": "m", "deliveryStatus": "pending"}
			}
			object["messagesOwed"] = messages
		})
		if _, err := verifyBoundHandoffState(root, "0000000000000004", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "inline messagesOwed exceeds 50") {
			t.Fatalf("oversized list must refuse: %v", err)
		}
	})

	t.Run("manifest preserves overflow", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000008")
		fixture = rewriteManifest(t, fixture, func(object map[string]any) {
			messages := make([]any, 51)
			for i := range messages {
				messages[i] = map[string]any{"nonce": "n", "message": "m", "deliveryStatus": "pending"}
			}
			object["messagesOwed"] = messages
		})
		if _, err := verifyBoundHandoffState(root, "0000000000000008", "fix-it", fixture.binding); err != nil {
			t.Fatalf("manifest overflow must remain complete and verifiable: %v", err)
		}
	})

	t.Run("writtenAt matches binding", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000b")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			object["writtenAt"] = "2026-09-14T18:31:00Z"
		})
		if _, err := verifyBoundHandoffState(root, "000000000000000b", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "writtenAt does not match the recorded binding time") {
			t.Fatalf("state time must match the binding: %v", err)
		}
	})

	t.Run("normalized session is derived from original", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000c")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			seat := object["seat"].(map[string]any)
			seat["session"] = "different / original"
		})
		if _, err := verifyBoundHandoffState(root, "000000000000000c", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "seat does not match its launch binding") {
			t.Fatalf("normalized session must derive from the recorded original: %v", err)
		}
	})

	t.Run("seat identity matches predecessor", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000d")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			seat := object["seat"].(map[string]any)
			predecessor := seat["identity"].(map[string]any)
			predecessor["Pid"] = 4343
		})
		if _, err := verifyBoundHandoffState(root, "000000000000000d", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "seat does not match its launch binding") {
			t.Fatalf("state identity must match the bound predecessor: %v", err)
		}
	})

	t.Run("held goal id matches requested goal", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000e")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			heldGoal := object["heldGoal"].(map[string]any)
			heldGoal["id"] = "different-goal"
		})
		if _, err := verifyBoundHandoffState(root, "000000000000000e", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "heldGoal does not name the exact accepted claim") {
			t.Fatalf("state goal must match the requested goal: %v", err)
		}
	})

	t.Run("disposable declaration is fixed", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000f")
		fixture = rewriteState(t, fixture, func(object map[string]any) {
			object["disposable"] = "the harness memory is required"
		})
		if _, err := verifyBoundHandoffState(root, "000000000000000f", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "disposable declaration is missing or changed") {
			t.Fatalf("disposable declaration must be exact: %v", err)
		}
	})

	t.Run("clean binding path outside owned nonce", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000010")
		data, err := os.ReadFile(fixture.binding.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		wrongPath := filepath.Join(HandoffDir(root, "0000000000000011"), "state.json")
		writeTestFile(t, wrongPath, data)
		fixture.binding.StatePath = wrongPath
		if _, err := verifyBoundHandoffState(root, "0000000000000010", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "canonical state path") {
			t.Fatalf("clean state path outside the owned nonce must refuse: %v", err)
		}
	})

	t.Run("noncanonical binding path", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000005")
		fixture.binding.StatePath = filepath.Dir(fixture.binding.StatePath) + string(filepath.Separator) + ".." + string(filepath.Separator) + "0000000000000005" + string(filepath.Separator) + "state.json"
		if _, err := verifyBoundHandoffState(root, "0000000000000005", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "canonical state path") {
			t.Fatalf("path alias must refuse: %v", err)
		}
	})

	t.Run("symlinked state ancestor", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "0000000000000009")
		ownedDir := HandoffDir(root, "0000000000000009")
		redirectedDir := filepath.Join(t.TempDir(), "redirected-state")
		if err := os.Rename(ownedDir, redirectedDir); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(redirectedDir, ownedDir); err != nil {
			t.Fatal(err)
		}
		if _, err := verifyBoundHandoffState(root, "0000000000000009", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "resolves outside its canonical owned path") {
			t.Fatalf("symlinked ancestor must not redirect the bound state: %v", err)
		}
	})

	t.Run("nonpositive predecessor pid", func(t *testing.T) {
		root := stagedRepo(t)
		fixture := writeStagedHandoffFixture(t, root, "000000000000000a")
		fixture.binding.Predecessor.Pid = 0
		if _, err := verifyBoundHandoffState(root, "000000000000000a", "fix-it", fixture.binding); err == nil || !strings.Contains(err.Error(), "predecessor identity is invalid") {
			t.Fatalf("nonpositive predecessor pid must refuse at the binding: %v", err)
		}
	})
}
