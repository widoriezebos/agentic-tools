// Package stopreporttest publishes complete Stop response fixtures for tests
// at package boundaries that consume the production record.
package stopreporttest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"os"
	"path/filepath"
	"testing"
)

const ChangedWording = "Just completed: unknown for this turn.\nTask: x; halting is fine here; see metasystem report stop-status --id %s"

type Options struct {
	Root, Runtime, Session, Attempt string
	ShouldBlock                     bool
	HealthLine, ExtraReport         string
	HumanLine                       func(alias string) string
}
type Published struct {
	Payload, Report []byte
	Response        stopreport.Response
	ResponsePath    string
	Identity        stopreport.Identity
}

func ChangedHumanLine(alias string) string { return fmt.Sprintf(ChangedWording, alias) }
func Publish(t testing.TB, options Options) Published {
	t.Helper()
	identity := stopreport.Identity{
		Installation: options.Root,
		Runtime:      options.Runtime,
		Session:      options.Session,
		SessionKey:   stopreport.SessionKey(options.Runtime, options.Session),
		Attempt:      options.Attempt,
		MainId:       "main-1",
		Machine:      "fixture",
		Lineage:      "lineage-1",
		ObservedAt:   "2026-09-17T12:00:00Z",
		ClaimEpoch:   1,
	}
	id := identity.SessionKey + "-" + identity.Attempt
	reservation, err := stopreport.ReserveShortestAlias(options.Root, id)
	if err != nil {
		t.Fatal(err)
	}
	humanLine := defaultHumanLine(reservation.Alias, options.ShouldBlock)
	if options.HumanLine != nil {
		humanLine = options.HumanLine(reservation.Alias)
	}
	identityJSON, _ := json.Marshal(identity)
	reportBytes := []byte(fmt.Sprintf("# Stop fixture\n\n<!-- metasystem-stop-report-v1 %s -->\n\n## Console text\n\n```text\n%s\n```\n\n## Health\n\n```json\n{\"line\":%q}\n```\n\n%s", identityJSON, humanLine, options.HealthLine, options.ExtraReport))
	dir := filepath.Join(options.Root, "artifacts", "agents", "supervision", "stop-verdicts")
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, reportBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	reportDigest := fmt.Sprintf("%x", sha256.Sum256(reportBytes))
	if err := stopreport.PublishAlias(reservation, reportDigest); err != nil {
		t.Fatal(err)
	}
	payloadObject := map[string]any{"systemMessage": humanLine}
	visibleField := "systemMessage"
	if options.ShouldBlock {
		payloadObject = map[string]any{"decision": "block", "reason": humanLine}
		visibleField = "reason"
	}
	payload, _ := json.Marshal(payloadObject)
	response := stopreport.Response{
		SchemaVersion: stopreport.ResponseSchemaVersion,
		Runtime:       options.Runtime,
		ShouldBlock:   options.ShouldBlock,
		VisibleField:  visibleField,
		PayloadSHA256: stopreport.PayloadSHA256(payload),
		Report: stopreport.ResponseReportReference{
			Installation: options.Root,
			ID:           id,
			Alias:        reservation.Alias,
			Path:         path,
			SHA256:       reportDigest,
		},
	}
	if err := stopreport.WriteResponse(options.Root, payload, response); err != nil {
		t.Fatal(err)
	}
	return Published{
		Payload: payload, Response: response, ResponsePath: stopreport.ResponsePath(options.Root, payload),
		Report: reportBytes, Identity: identity,
	}
}
func defaultHumanLine(alias string, blocked bool) string {
	if blocked {
		return "Just completed: unknown for this turn.\nTask: test; Stop blocked; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id " + alias
	}
	return "Just completed: unknown for this turn.\nNo task in flight; Stop allowed; Report: metasystem report stop-status --id " + alias
}
