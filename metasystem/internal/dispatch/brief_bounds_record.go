package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type BriefBoundsRecord struct {
	SchemaVersion  int      `json:"schemaVersion"`
	JobID          string   `json:"jobId"`
	RootJob        string   `json:"rootJob"`
	Round          int64    `json:"round"`
	AdmittedSHA256 string   `json:"admittedSha256"`
	AdmittedBytes  int64    `json:"admittedBytes"`
	Boundary       []string `json:"boundary"`
	Ceiling        *int64   `json:"ceiling"`
}

type AdmittedBriefMarker struct {
	SchemaVersion int    `json:"schemaVersion"`
	Bounded       bool   `json:"bounded"`
	RecordSHA256  string `json:"recordSha256"`
}

func (r BriefBoundsRecord) Bounds() BriefBounds { return BriefBounds{r.Boundary, r.Ceiling} }

func EncodeBriefBoundsRecord(r BriefBoundsRecord) ([]byte, error) {
	if err := validateBriefBoundsRecord(r); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode brief bounds record: %w", err)
	}
	return append(b, '\n'), nil
}

func DecodeBriefBoundsRecord(data []byte) (BriefBoundsRecord, error) {
	f, err := decodeBriefBoundsObject(data)
	if err != nil {
		return BriefBoundsRecord{}, err
	}
	known := map[string]bool{"schemaVersion": true, "jobId": true, "rootJob": true, "round": true, "admittedSha256": true, "admittedBytes": true, "boundary": true, "ceiling": true}
	if len(f) != len(known) {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record has an incomplete or expanded shape")
	}
	for key := range f {
		if !known[key] {
			return BriefBoundsRecord{}, fmt.Errorf("brief bounds record contains undeclared field %q", key)
		}
	}
	var r BriefBoundsRecord
	if err := recordValue(f, "schemaVersion", &r.SchemaVersion); err != nil || r.SchemaVersion != 1 {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires schema version 1")
	}
	if err := recordValue(f, "jobId", &r.JobID); err != nil || r.JobID == "" {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires a jobId")
	}
	if err := recordValue(f, "rootJob", &r.RootJob); err != nil || r.RootJob == "" {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires a rootJob")
	}
	if err := recordValue(f, "round", &r.Round); err != nil || r.Round < 1 {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires a positive round")
	}
	if err := recordValue(f, "admittedSha256", &r.AdmittedSHA256); err != nil || !incarnationRe.MatchString(r.AdmittedSHA256) {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires a lowercase admittedSha256")
	}
	if err := recordValue(f, "admittedBytes", &r.AdmittedBytes); err != nil || r.AdmittedBytes < 0 {
		return BriefBoundsRecord{}, fmt.Errorf("brief bounds record requires a nonnegative admittedBytes")
	}
	if err := recordBounds(f, &r.Boundary, &r.Ceiling); err != nil {
		return BriefBoundsRecord{}, err
	}
	if err := validateBriefBoundsRecord(r); err != nil {
		return BriefBoundsRecord{}, err
	}
	return r, nil
}

func ReadBriefBoundsRecord(path string) (BriefBoundsRecord, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return BriefBoundsRecord{}, nil, err
	}
	r, err := DecodeBriefBoundsRecord(b)
	if err != nil {
		return BriefBoundsRecord{}, nil, err
	}
	return r, b, nil
}

func validateBriefBoundsRecord(r BriefBoundsRecord) error {
	if r.SchemaVersion != 1 || r.JobID == "" || r.RootJob == "" || r.Round < 1 || !incarnationRe.MatchString(r.AdmittedSHA256) || r.AdmittedBytes < 0 {
		return fmt.Errorf("brief bounds record has invalid identity or admitted bytes")
	}
	if r.Boundary == nil != (r.Ceiling == nil) {
		return fmt.Errorf("brief bounds record requires boundary and ceiling together")
	}
	if err := ValidateBriefBounds(r.Bounds()); err != nil {
		return fmt.Errorf("brief bounds record has invalid bounds: %w", err)
	}
	return nil
}

func decodeBriefBoundsObject(data []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("decode brief bounds record: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("brief bounds record must be a JSON object")
	}
	f := map[string]json.RawMessage{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return nil, fmt.Errorf("decode brief bounds key: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("brief bounds key is not a string")
		}
		if _, exists := f[key]; exists {
			return nil, fmt.Errorf("brief bounds record repeats field %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, fmt.Errorf("decode brief bounds field %q: %w", key, err)
		}
		f[key] = raw
	}
	token, err = d.Token()
	if err != nil {
		return nil, fmt.Errorf("close brief bounds record: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '}' {
		return nil, fmt.Errorf("brief bounds record must close its object")
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("brief bounds record has trailing JSON")
		}
		return nil, fmt.Errorf("brief bounds record has trailing data: %w", err)
	}
	return f, nil
}

func recordValue(fields map[string]json.RawMessage, key string, out any) error {
	raw, ok := fields[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("brief bounds field %q is missing or null", key)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("brief bounds field %q has the wrong type", key)
	}
	return nil
}

func recordBounds(fields map[string]json.RawMessage, boundary *[]string, ceiling **int64) error {
	raw, ok := fields["boundary"]
	if !ok {
		return fmt.Errorf("brief bounds field boundary is missing")
	}
	if !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := json.Unmarshal(raw, boundary); err != nil {
			return fmt.Errorf("brief bounds boundary must be an array or null")
		}
	}
	raw, ok = fields["ceiling"]
	if !ok {
		return fmt.Errorf("brief bounds field ceiling is missing")
	}
	if !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("brief bounds ceiling must be an integer or null")
		}
		*ceiling = &value
	}
	return nil
}

func admittedBriefMarker(r BriefBoundsRecord, recordBytes []byte) AdmittedBriefMarker {
	return AdmittedBriefMarker{SchemaVersion: 1, Bounded: r.Boundary != nil, RecordSHA256: digestBytes(recordBytes)}
}

func validateAdmittedBriefMarker(value any) error {
	m, ok := value.(map[string]any)
	if !ok || len(m) != 3 {
		return fmt.Errorf("admittedBrief marker has an invalid shape")
	}
	for key := range m {
		if key != "schemaVersion" && key != "bounded" && key != "recordSha256" {
			return fmt.Errorf("admittedBrief marker contains undeclared field %q", key)
		}
	}
	version, versionOK := numInt(m["schemaVersion"])
	_, boundedOK := m["bounded"].(bool)
	digest, digestOK := m["recordSha256"].(string)
	if !versionOK || version != 1 || !boundedOK || !digestOK || !incarnationRe.MatchString(digest) {
		return fmt.Errorf("admittedBrief marker has invalid version, bounded value, or record digest")
	}
	return nil
}
