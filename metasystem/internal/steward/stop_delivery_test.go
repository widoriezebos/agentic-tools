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
)

func hookDeliveryFixture(t *testing.T, root, sessionKey, attempt, healthSection, extra string) (string, HookDeliveryReference) {
	t.Helper()
	id := sessionKey + "-" + attempt
	dir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	report := fmt.Sprintf("# Task: test; Stop blocked\n\n<!-- metasystem-stop-report-v1 {\"installation\":%q,\"runtime\":\"claude\",\"session\":\"s\",\"sessionKey\":%q,\"attempt\":%q,\"mainId\":\"m\",\"machine\":\"bed\",\"lineage\":\"lineage\",\"observedAt\":\"2026-09-13T12:00:00Z\",\"claimEpoch\":1} -->\n\n## Health\n\n```json\n{\"line\":%q}\n```\n\n%s", root, sessionKey, attempt, healthSection, extra)
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(report))
	return id, HookDeliveryReference{Installation: root, ID: id, Path: path, SHA256: hex.EncodeToString(digest[:])}
}

func TestCompleteHookAttemptBindsPayloadToExactReportHealthSection(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 71, StartedAtSec: 100, StartTicks: 901, BootID: "boot-delivery"}
	sessionKey := strings.Repeat("a", 64)
	health := "HEALTH healthy — runner=alive"
	id, delivery := hookDeliveryFixture(t, root, sessionKey, strings.Repeat("b", 32), health, "")
	attempt, err := BeginHookAttempt(root, process, "delivery-one", now)
	if err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"decision":"block","reason":"Task: test; Stop blocked; status: metasystem report stop-status --id %s"}`, id)
	if _, err := CompleteHookAttemptWithDelivery(root, attempt.Generation, attempt.AttemptSeq, ComponentOK, "EMITTED", health, payload, delivery, nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	id, delivery = hookDeliveryFixture(t, root, sessionKey, strings.Repeat("c", 32), "HEALTH unknown", "arbitrary copy: "+health)
	attempt, err = BeginHookAttempt(root, process, "delivery-two", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	payload = fmt.Sprintf(`{"systemMessage":"No task in flight; Stop allowed; status: metasystem report stop-status --id %s"}`, id)
	if _, err := CompleteHookAttemptWithDelivery(root, attempt.Generation, attempt.AttemptSeq, ComponentOK, "EMITTED", health, payload, delivery, nil, now.Add(3*time.Second)); err == nil || !strings.Contains(err.Error(), "health snapshot") {
		t.Fatalf("an arbitrary health string outside the typed section proved delivery: %v", err)
	}
}
