package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
)

func hookDeliveryFixture(t *testing.T, root, sessionKey, attempt, healthSection, extra string, blocked bool) (string, HookDeliveryReference) {
	t.Helper()
	if sessionKey != stopreport.SessionKey("claude", "s") {
		t.Fatalf("fixture session key %q does not name claude/s", sessionKey)
	}
	published := stopreporttest.Publish(t, stopreporttest.Options{
		Root: root, Runtime: "claude", Session: "s", Attempt: attempt, ShouldBlock: blocked,
		HealthLine: healthSection, ExtraReport: extra,
	})
	return string(published.Payload), deliveryReference(published)
}
func deliveryReference(published stopreporttest.Published) HookDeliveryReference {
	return HookDeliveryReference{
		Installation: published.Response.Report.Installation,
		ID:           published.Response.Report.ID,
		Alias:        published.Response.Report.Alias,
		Path:         published.Response.Report.Path,
		SHA256:       published.Response.Report.SHA256,
	}
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

func TestVerifyHookDeliveryRejectsPayloadOutsideReportConsoleText(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	published := stopreporttest.Publish(t, stopreporttest.Options{
		Root: root, Runtime: "claude", Session: "outside", Attempt: strings.Repeat("e", 32), HealthLine: "HEALTH healthy — runner=alive",
	})
	var payload map[string]any
	if err := json.Unmarshal(published.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["systemMessage"] = payload["systemMessage"].(string) + " outside"
	changed, _ := json.Marshal(payload)
	response := published.Response
	response.PayloadSHA256 = stopreport.PayloadSHA256(changed)
	if err := stopreport.WriteResponse(root, changed, response); err != nil {
		t.Fatal(err)
	}
	if err := verifyHookDelivery(root, string(changed), "HEALTH healthy — runner=alive", deliveryReference(published)); err == nil || !strings.Contains(err.Error(), "console text") {
		t.Fatalf("delivery verifier accepted text outside the report: %v", err)
	}
}

func TestVerifyHookDeliveryRejectsPayloadOutcomeMismatch(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		health := "HEALTH healthy — runner=alive"
		payload, delivery := hookDeliveryFixture(t, root, stopreport.SessionKey("claude", "s"), strings.Repeat("f", 32), health, "", blocked)
		resolved, err := stopreport.ReadResponse(root, []byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(payload), &object); err != nil {
			t.Fatal(err)
		}
		visible := object["systemMessage"]
		if blocked {
			visible = object["reason"]
			object = map[string]any{"systemMessage": visible}
		} else {
			object = map[string]any{"decision": "block", "reason": visible}
		}
		changed, _ := json.Marshal(object)
		resolved.PayloadSHA256 = stopreport.PayloadSHA256(changed)
		if err := stopreport.WriteResponse(root, changed, resolved); err != nil {
			t.Fatal(err)
		}
		payload = string(changed)
		if err := verifyHookDelivery(root, payload, health, delivery); err == nil {
			t.Fatalf("delivery verifier accepted blocked=%t with the opposite payload shape", blocked)
		}
	}
}

func TestHookDeliveryResolvesTheReportFromTheResponseRecord(t *testing.T) {
	for _, runtime := range []string{"claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			for _, blocked := range []bool{false, true} {
				t.Run(map[bool]string{false: "allowed", true: "blocked"}[blocked], func(t *testing.T) {
					for _, test := range []struct {
						name      string
						delivery  bool
						humanLine func(string) string
					}{
						{name: "changed wording", delivery: true, humanLine: stopreporttest.ChangedHumanLine},
						{name: "without reference flags", humanLine: stopreporttest.ChangedHumanLine},
					} {
						t.Run(test.name, func(t *testing.T) {
							root, err := filepath.EvalSymlinks(t.TempDir())
							if err != nil {
								t.Fatal(err)
							}
							health := "HEALTH healthy — runner=alive"
							published := stopreporttest.Publish(t, stopreporttest.Options{
								Root: root, Runtime: runtime, Session: "resolved", Attempt: strings.Repeat("a", 32),
								ShouldBlock: blocked, HealthLine: health, HumanLine: test.humanLine,
							})
							delivery := HookDeliveryReference{}
							if test.delivery {
								delivery = deliveryReference(published)
							}
							if err := verifyHookDelivery(root, string(published.Payload), health, delivery); err != nil {
								t.Fatalf("response-record delivery was refused: %v", err)
							}
						})
					}
				})
			}
		})
	}
}
func TestHookDeliveryRefusesAResponseWithoutTheReportField(t *testing.T) {
	type refusal struct {
		name, want string
		mutate     func(*testing.T, string, stopreporttest.Published) ([]byte, HookDeliveryReference)
	}
	cases := []refusal{
		{name: "missing record", want: "unreadable", mutate: func(t *testing.T, _ string, published stopreporttest.Published) ([]byte, HookDeliveryReference) {
			t.Helper()
			if err := os.Remove(published.ResponsePath); err != nil {
				t.Fatal(err)
			}
			return published.Payload, HookDeliveryReference{}
		}},
		{name: "missing report field", want: "report", mutate: func(t *testing.T, _ string, published stopreporttest.Published) ([]byte, HookDeliveryReference) {
			t.Helper()
			var object map[string]any
			data, _ := os.ReadFile(published.ResponsePath)
			if json.Unmarshal(data, &object) != nil {
				t.Fatal("decode response fixture")
			}
			delete(object, "report")
			data, _ = json.Marshal(object)
			if err := os.WriteFile(published.ResponsePath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			return published.Payload, HookDeliveryReference{}
		}},
		{name: "reference flags disagree", want: "alias flag", mutate: func(t *testing.T, _ string, published stopreporttest.Published) ([]byte, HookDeliveryReference) {
			delivery := deliveryReference(published)
			delivery.Alias += "f"
			return published.Payload, delivery
		}},
		{name: "payload shape disagrees", want: "payload shape", mutate: func(t *testing.T, root string, published stopreporttest.Published) ([]byte, HookDeliveryReference) {
			var object map[string]any
			if json.Unmarshal(published.Payload, &object) != nil {
				t.Fatal("decode payload fixture")
			}
			visible := object["systemMessage"]
			if published.Response.ShouldBlock {
				visible = object["reason"]
				object = map[string]any{"systemMessage": visible}
			} else {
				object = map[string]any{"decision": "block", "reason": visible}
			}
			payload, _ := json.Marshal(object)
			response := published.Response
			response.PayloadSHA256 = stopreport.PayloadSHA256(payload)
			if err := stopreport.WriteResponse(root, payload, response); err != nil {
				t.Fatal(err)
			}
			return payload, HookDeliveryReference{}
		}},
		{name: "report digest changed", want: "digest", mutate: func(t *testing.T, _ string, published stopreporttest.Published) ([]byte, HookDeliveryReference) {
			if err := os.WriteFile(published.Response.Report.Path, append(published.Report, []byte("changed\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			return published.Payload, HookDeliveryReference{}
		}},
	}
	for _, runtime := range []string{"claude", "codex"} {
		t.Run(runtime, func(t *testing.T) {
			for _, blocked := range []bool{false, true} {
				t.Run(map[bool]string{false: "allowed", true: "blocked"}[blocked], func(t *testing.T) {
					for _, test := range cases {
						t.Run(test.name, func(t *testing.T) {
							root, err := filepath.EvalSymlinks(t.TempDir())
							if err != nil {
								t.Fatal(err)
							}
							published := stopreporttest.Publish(t, stopreporttest.Options{
								Root: root, Runtime: runtime, Session: "refusal", Attempt: strings.Repeat("b", 32),
								ShouldBlock: blocked, HealthLine: "HEALTH healthy — runner=alive", HumanLine: stopreporttest.ChangedHumanLine,
							})
							payload, delivery := test.mutate(t, root, published)
							if err := verifyHookDelivery(root, string(payload), "HEALTH healthy — runner=alive", delivery); err == nil || !strings.Contains(err.Error(), test.want) {
								t.Fatalf("%s was accepted: %v", test.name, err)
							}
						})
					}
				})
			}
		})
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
