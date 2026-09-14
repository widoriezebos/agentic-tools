package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

func hookDeliveryFixture(t *testing.T, root, sessionKey, attempt, healthSection, extra string, blocked bool) (string, HookDeliveryReference) {
	t.Helper()
	id := sessionKey + "-" + attempt
	dir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reservation, err := stopreport.ReserveShortestAlias(root, id)
	if err != nil {
		t.Fatal(err)
	}
	command := "metasystem report stop-status --id " + reservation.Alias
	visible := "Just completed: unknown for this turn.\nNo task in flight; Stop allowed; Read, then continue lawful work before stopping: " + command
	payload := fmt.Sprintf(`{"systemMessage":%q}`, visible)
	if blocked {
		visible = "Just completed: unknown for this turn.\nTask: test; Stop blocked; Read: " + command
		payload = fmt.Sprintf(`{"decision":"block","reason":%q}`, visible)
	}
	report := fmt.Sprintf("# Task: test; Stop blocked\n\n<!-- metasystem-stop-report-v1 {\"installation\":%q,\"runtime\":\"claude\",\"session\":\"s\",\"sessionKey\":%q,\"attempt\":%q,\"mainId\":\"m\",\"machine\":\"bed\",\"lineage\":\"lineage\",\"observedAt\":\"2026-09-13T12:00:00Z\",\"claimEpoch\":1} -->\n\n## Console text\n\n```text\n%s\n```\n\n## Health\n\n```json\n{\"line\":%q}\n```\n\n%s", root, sessionKey, attempt, visible, healthSection, extra)
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(report))
	digestText := hex.EncodeToString(digest[:])
	if err := stopreport.PublishAlias(reservation, digestText); err != nil {
		t.Fatal(err)
	}
	return payload, HookDeliveryReference{Installation: root, ID: id, Alias: reservation.Alias, Path: path, SHA256: digestText}
}

func TestCompleteHookAttemptBindsPayloadToExactReportHealthSection(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 71, StartedAtSec: 100, StartTicks: 901, BootID: "boot-delivery"}
	sessionKey := stopreport.SessionKey("claude", "s")
	health := "HEALTH healthy — runner=alive"
	payload, delivery := hookDeliveryFixture(t, root, sessionKey, strings.Repeat("b", 32), health, "", true)
	attempt, err := BeginHookAttempt(root, process, "delivery-one", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteHookAttemptWithDelivery(root, attempt.Generation, attempt.AttemptSeq, ComponentOK, "EMITTED", health, payload, delivery, nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	payload, delivery = hookDeliveryFixture(t, root, sessionKey, strings.Repeat("c", 32), "HEALTH unknown", "arbitrary copy: "+health, false)
	attempt, err = BeginHookAttempt(root, process, "delivery-two", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteHookAttemptWithDelivery(root, attempt.Generation, attempt.AttemptSeq, ComponentOK, "EMITTED", health, payload, delivery, nil, now.Add(3*time.Second)); err == nil || !strings.Contains(err.Error(), "health snapshot") {
		t.Fatalf("an arbitrary health string outside the typed section proved delivery: %v", err)
	}
}

func TestCompleteHookAttemptAcceptsInstallationWithAncestorSymlink(t *testing.T) {
	physicalParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physicalRoot := filepath.Join(physicalParent, "installation")
	if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(t.TempDir(), "linked-parent")
	if err := os.Symlink(physicalParent, linkedParent); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(linkedParent, "installation")
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 72, StartedAtSec: 101, StartTicks: 902, BootID: "boot-linked-delivery"}
	sessionKey := stopreport.SessionKey("claude", "s")
	health := "HEALTH healthy — runner=alive"
	payload, delivery := hookDeliveryFixture(t, root, sessionKey, strings.Repeat("d", 32), health, "", false)
	attempt, err := BeginHookAttempt(root, process, "linked-delivery", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteHookAttemptWithDelivery(root, attempt.Generation, attempt.AttemptSeq, ComponentOK, "EMITTED", health, payload, delivery, nil, now.Add(time.Second)); err != nil {
		t.Fatalf("installation spelling with an ancestor symlink was refused: %v", err)
	}
}
