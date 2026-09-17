package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

// MapStopOutput serializes one already-decided presentation into the sole
// human-visible field supported by the runtime's candidate Stop contract. It
// never judges or spends a refusal.
func MapStopOutput(runtime, inputPath, outputPath string) error {
	declaration, ok := runtimes.Lookup(runtime)
	if !ok || declaration.ExpectedStopDelivery == "" {
		return fmt.Errorf("runtime %q has no declared Stop output mapping", runtime)
	}
	if declaration.ExpectedStopDelivery != "shared-reason-v1" {
		return fmt.Errorf("runtime %q declares unsupported Stop output mapping %q", runtime, declaration.ExpectedStopDelivery)
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read Stop presentation: %w", err)
	}
	if err := validateStopPresentationResultWire(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var presentation report.StopPresentationResult
	if err := decoder.Decode(&presentation); err != nil {
		return fmt.Errorf("decode Stop presentation: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode Stop presentation: trailing JSON")
	}
	if presentation.SchemaVersion != report.StopPresentationSchemaVersion || presentation.Identity.Runtime != runtime {
		return fmt.Errorf("stop presentation schema or runtime identity does not match")
	}
	if err := report.ValidateStopHumanLine(presentation.HumanLine); err != nil {
		return err
	}
	if presentation.Control.ShouldBlock != (presentation.Control.BlockSource != nil) {
		return fmt.Errorf("stop presentation block source does not match its decision")
	}
	wantCommand := "metasystem report stop-status --id " + presentation.Report.Alias
	if presentation.Report.Id == "" || presentation.Report.Alias == "" || presentation.Report.ReadCommand != wantCommand ||
		presentation.Report.Path == "" || presentation.Report.SHA256 == "" {
		return fmt.Errorf("stop presentation report reference is inconsistent")
	}
	reportBytes, identity, err := report.ReadStopStatus(presentation.Identity.Installation, presentation.Report.Alias)
	if err != nil {
		return fmt.Errorf("verify Stop presentation report: %w", err)
	}
	digest := sha256.Sum256(reportBytes)
	if identity != presentation.Identity || hex.EncodeToString(digest[:]) != presentation.Report.SHA256 || presentation.Report.Path != filepath.Join(presentation.Identity.Installation, "artifacts", "agents", "supervision", "stop-verdicts", presentation.Report.Id+".md") {
		return fmt.Errorf("stop presentation report identity, path, or digest is inconsistent")
	}
	if !bytes.Contains(reportBytes, []byte("## Console text\n\n```text\n"+presentation.HumanLine+"\n```\n")) {
		return fmt.Errorf("stop presentation line does not match its immutable report")
	}
	payload := map[string]any{}
	if presentation.Control.ShouldBlock {
		payload["decision"] = "block"
		payload["reason"] = presentation.HumanLine
	} else {
		payload["systemMessage"] = presentation.HumanLine
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return fmt.Errorf("stop output already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect Stop output: %w", err)
	}
	visibleField := "systemMessage"
	if presentation.Control.ShouldBlock {
		visibleField = "reason"
	}
	response := stopreport.Response{
		SchemaVersion: stopreport.ResponseSchemaVersion,
		Runtime:       runtime,
		ShouldBlock:   presentation.Control.ShouldBlock,
		VisibleField:  visibleField,
		PayloadSHA256: stopreport.PayloadSHA256(encoded),
		Report: stopreport.ResponseReportReference{
			Installation: presentation.Identity.Installation,
			ID:           presentation.Report.Id,
			Alias:        presentation.Report.Alias,
			Path:         presentation.Report.Path,
			SHA256:       presentation.Report.SHA256,
		},
	}
	if err := stopreport.WriteResponse(presentation.Identity.Installation, encoded, response); err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(outputPath, string(encoded)+"\n", "")
	if err != nil {
		return fmt.Errorf("write Stop output: %w", err)
	}
	if !durable {
		return fmt.Errorf("write Stop output: crash durability is unknown")
	}
	return nil
}

func validateStopPresentationResultWire(data []byte) error {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("decode Stop presentation: %w", err)
	}
	requireObject := func(field string) (map[string]any, error) {
		object, ok := raw[field].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("decode Stop presentation: required object %s is missing or has the wrong type", field)
		}
		return object, nil
	}
	control, err := requireObject("control")
	if err != nil {
		return err
	}
	if _, ok := control["shouldBlock"].(bool); !ok {
		return fmt.Errorf("decode Stop presentation: required boolean shouldBlock is missing or has the wrong type")
	}
	if _, ok := control["judgmentAvailable"].(bool); !ok {
		return fmt.Errorf("decode Stop presentation: required boolean judgmentAvailable is missing or has the wrong type")
	}
	for _, field := range []string{"needsYourDecision", "needsSupervisionRepair"} {
		if _, ok := raw[field].(bool); !ok {
			return fmt.Errorf("decode Stop presentation: required boolean %s is missing or has the wrong type", field)
		}
	}
	for _, field := range []string{"identity", "report"} {
		if _, err := requireObject(field); err != nil {
			return err
		}
	}
	return nil
}
