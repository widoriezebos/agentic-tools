package stopreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ResponseSchemaVersion = 1

type ResponseReportReference struct {
	Installation string `json:"installation"`
	ID           string `json:"id"`
	Alias        string `json:"alias"`
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
}
type Response struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Runtime       string                  `json:"runtime"`
	ShouldBlock   bool                    `json:"shouldBlock"`
	VisibleField  string                  `json:"visibleField"`
	PayloadSHA256 string                  `json:"payloadSha256"`
	Report        ResponseReportReference `json:"report"`
}
type ResolvedResponse struct {
	Response Response
	Visible  string
	Report   []byte
}

func PayloadSHA256(payload []byte) string {
	digest := sha256.Sum256(trimASCIIWhitespace(payload))
	return hex.EncodeToString(digest[:])
}
func ResponsePath(root string, payload []byte) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", "responses", PayloadSHA256(payload)+".json")
}

func WriteResponse(root string, payload []byte, response Response) error {
	wantDigest := PayloadSHA256(payload)
	if err := validateResponse(response, root, wantDigest); err != nil {
		return fmt.Errorf("write Stop response: %w", err)
	}
	dir, err := responseDir(root, true)
	if err != nil {
		return fmt.Errorf("write Stop response: %w", err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("write Stop response: %w", err)
	}
	encoded, path := append(encoded, '\n'), filepath.Join(dir, wantDigest+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("write Stop response: existing response is not a regular file")
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("write Stop response: read existing response: %w", readErr)
		}
		if bytes.Equal(existing, encoded) {
			return nil
		}
		return fmt.Errorf("write Stop response: existing response has different content")
	}
	if err != nil {
		return fmt.Errorf("write Stop response: %w", err)
	}
	removeIncomplete := true
	defer func() {
		if removeIncomplete {
			_ = os.Remove(path)
		}
	}()
	_, writeErr := file.Write(encoded)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write Stop response: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write Stop response: %w", closeErr)
	}
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("write Stop response: %w", err)
	}
	removeIncomplete = false
	return nil
}

func ReadResponse(root string, payload []byte) (Response, error) {
	digest := PayloadSHA256(payload)
	dir, err := responseDir(root, false)
	if err != nil {
		return Response{}, unreadableResponse("response directory is unavailable: %v", err)
	}
	path := filepath.Join(dir, digest+".json")
	response, err := readResponseFile(path, root, digest)
	if err != nil {
		return Response{}, unreadableResponse("read payloadSha256 %s: %v", digest, err)
	}
	return response, nil
}

func ResolveResponse(root string, payload []byte, runtime, session string) (ResolvedResponse, error) {
	response, err := ReadResponse(root, payload)
	if err != nil {
		return ResolvedResponse{}, err
	}
	visible, err := responseVisibleText(response, payload)
	if err != nil {
		return ResolvedResponse{}, unreadableResponse("%v", err)
	}
	reportBytes, identity, resolution, err := Read(response.Report.Installation, response.Report.Alias)
	if err != nil {
		return ResolvedResponse{}, fmt.Errorf("stop response report cannot be read: %w", err)
	}
	digest := sha256.Sum256(reportBytes)
	wantPath := filepath.Join(response.Report.Installation, "artifacts", "agents", "supervision", "stop-verdicts", response.Report.ID+".md")
	if resolution.ID != response.Report.ID || resolution.Alias != response.Report.Alias ||
		resolution.Path != response.Report.Path || response.Report.Path != wantPath ||
		hex.EncodeToString(digest[:]) != response.Report.SHA256 ||
		identity.Installation != response.Report.Installation ||
		identity.SessionKey+"-"+identity.Attempt != response.Report.ID ||
		identity.Runtime != response.Runtime {
		return ResolvedResponse{}, fmt.Errorf("stop response report identity, alias, path, digest, installation, or runtime does not match")
	}
	if runtime != "" && identity.Runtime != runtime {
		return ResolvedResponse{}, fmt.Errorf("stop response report runtime does not match %q", runtime)
	}
	if session != "" && identity.Session != session {
		return ResolvedResponse{}, fmt.Errorf("stop response report session does not match %q", session)
	}
	if !bytes.Contains(reportBytes, []byte("## Console text\n\n```text\n"+visible+"\n```\n")) {
		return ResolvedResponse{}, fmt.Errorf("stop response visible text does not match the report's console text")
	}
	return ResolvedResponse{Response: response, Visible: visible, Report: reportBytes}, nil
}

func RemoveResponsesForReport(root, reportID string) error {
	dir, err := responseDir(root, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	removed := false
	for _, path := range paths {
		name := filepath.Base(path)
		digest := strings.TrimSuffix(name, ".json")
		if !validSHA256(digest) {
			return fmt.Errorf("stop response filename %q is malformed", name)
		}
		response, err := readResponseFile(path, root, digest)
		if err != nil {
			return unreadableResponse("%v", err)
		}
		if response.Report.ID != reportID {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed = true
	}
	if removed {
		return syncDirectory(dir)
	}
	return nil
}
func responseDir(root string, create bool) (string, error) {
	base, err := reportDir(root, create)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "responses")
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("stop response directory is not a directory")
	}
	if _, err := resolveInstallationDirectory(root, dir); err != nil {
		return "", fmt.Errorf("stop response directory must not contain symlinks")
	}
	return dir, nil
}
func readResponseFile(path, root, digest string) (Response, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Response{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Response{}, fmt.Errorf("response record is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Response{}, err
	}
	return decodeResponse(data, root, digest)
}
func decodeResponse(data []byte, root, digest string) (Response, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return Response{}, fmt.Errorf("record is not one JSON object: %w", err)
	}
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return Response{}, fmt.Errorf("record is not one JSON object")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Response{}, fmt.Errorf("record is not one JSON object")
	}
	for _, field := range []string{"schemaVersion", "runtime", "shouldBlock", "visibleField", "payloadSha256", "report"} {
		if _, ok := fields[field]; !ok {
			return Response{}, fmt.Errorf("required field %s is missing", field)
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return Response{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Response{}, fmt.Errorf("record is not one JSON object")
	}
	if err := validateResponse(response, root, digest); err != nil {
		return Response{}, err
	}
	return response, nil
}
func validateResponse(response Response, root, digest string) error {
	if response.SchemaVersion != ResponseSchemaVersion {
		return fmt.Errorf("field schemaVersion has unsupported value %d", response.SchemaVersion)
	}
	if response.Runtime == "" {
		return fmt.Errorf("required field runtime is empty")
	}
	wantVisible := "systemMessage"
	if response.ShouldBlock {
		wantVisible = "reason"
	}
	if response.VisibleField != wantVisible {
		return fmt.Errorf("field visibleField must be %q when shouldBlock is %t", wantVisible, response.ShouldBlock)
	}
	if !validSHA256(response.PayloadSHA256) || response.PayloadSHA256 != digest {
		return fmt.Errorf("field payloadSha256 is missing, malformed, or does not match the payload")
	}
	if response.Report.Installation == "" || !filepath.IsAbs(response.Report.Installation) {
		return fmt.Errorf("required field report.installation is empty or malformed")
	}
	if response.Report.Installation != root {
		return fmt.Errorf("field report.installation does not match the response installation")
	}
	if err := ValidateFullID(response.Report.ID); err != nil {
		return fmt.Errorf("required field report.id is empty or malformed")
	}
	if !aliasPattern.MatchString(response.Report.Alias) {
		return fmt.Errorf("required field report.alias is empty or malformed")
	}
	wantPath := filepath.Join(response.Report.Installation, "artifacts", "agents", "supervision", "stop-verdicts", response.Report.ID+".md")
	if response.Report.Path == "" || response.Report.Path != wantPath {
		return fmt.Errorf("required field report.path is empty or malformed")
	}
	if !validSHA256(response.Report.SHA256) {
		return fmt.Errorf("required field report.sha256 is empty or malformed")
	}
	return nil
}
func responseVisibleText(response Response, payload []byte) (string, error) {
	if err := rejectDuplicateKeys(trimASCIIWhitespace(payload)); err != nil {
		return "", fmt.Errorf("payload is not one JSON object: %w", err)
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(trimASCIIWhitespace(payload)))
	if err := decoder.Decode(&object); err != nil || object == nil || decoder.Decode(&struct{}{}) != io.EOF {
		return "", fmt.Errorf("payload is not one JSON object")
	}
	if response.ShouldBlock {
		if len(object) != 2 || object["decision"] == nil || object["reason"] == nil {
			return "", fmt.Errorf("payload shape disagrees with shouldBlock and visibleField")
		}
		var decision, visible string
		if json.Unmarshal(object["decision"], &decision) != nil || decision != "block" || json.Unmarshal(object["reason"], &visible) != nil {
			return "", fmt.Errorf("payload shape disagrees with shouldBlock and visibleField")
		}
		return visible, nil
	}
	if len(object) != 1 || object["systemMessage"] == nil {
		return "", fmt.Errorf("payload shape disagrees with shouldBlock and visibleField")
	}
	var visible string
	if json.Unmarshal(object["systemMessage"], &visible) != nil {
		return "", fmt.Errorf("payload shape disagrees with shouldBlock and visibleField")
	}
	return visible, nil
}
func trimASCIIWhitespace(data []byte) []byte {
	return bytes.Trim(data, " \t\n\v\f\r")
}
func unreadableResponse(format string, args ...any) error {
	return fmt.Errorf("Stop response is unreadable: "+format, args...)
}
