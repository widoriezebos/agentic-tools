package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// BriefBoundsRecord binds structured bounds to the exact brief admitted for
// one job round.
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

func (r BriefBoundsRecord) Bounds() BriefBounds {
	return BriefBounds{Boundary: r.Boundary, Ceiling: r.Ceiling}
}

func validateBriefBoundsRecord(r BriefBoundsRecord) error {
	if r.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema version")
	}
	if r.JobID == "" || r.RootJob == "" || r.Round < 1 {
		return fmt.Errorf("invalid job, root job, or round identity")
	}
	if !incarnationRe.MatchString(r.AdmittedSHA256) {
		return fmt.Errorf("invalid admitted brief digest")
	}
	if r.AdmittedBytes < 0 {
		return fmt.Errorf("invalid admitted brief byte count")
	}
	return ValidateBriefBounds(r.Bounds())
}

// MarshalBriefBoundsRecord renders the closed record for persistence.
func MarshalBriefBoundsRecord(r BriefBoundsRecord) ([]byte, error) {
	if err := validateBriefBoundsRecord(r); err != nil {
		return nil, fmt.Errorf("invalid brief bounds record: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	return append(data, '\n'), err
}

// DecodeBriefBoundsRecord reads the closed schema and rejects duplicate keys
// or any bytes after its single JSON object.
func DecodeBriefBoundsRecord(data []byte) (BriefBoundsRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	fields := map[string]json.RawMessage{}
	fieldNames := []string{
		"schemaVersion", "jobId", "rootJob", "round", "admittedSha256", "admittedBytes", "boundary", "ceiling",
	}
	allowed := make(map[string]bool, len(fieldNames))
	for _, key := range fieldNames {
		allowed[key] = true
	}
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: expected object")
	}
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		key, keyOK := keyToken.(string)
		if keyErr != nil || !keyOK {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: invalid key")
		}
		if !allowed[key] {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: unknown key %s", key)
		}
		if fields[key] != nil {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: duplicate or invalid key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: %w", err)
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: trailing JSON")
	}
	for _, key := range fieldNames {
		if fields[key] == nil {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: missing %s", key)
		}
	}
	var r BriefBoundsRecord
	values := []struct {
		key string
		out any
	}{{"schemaVersion", &r.SchemaVersion}, {"jobId", &r.JobID}, {"rootJob", &r.RootJob}, {"round", &r.Round}, {"admittedSha256", &r.AdmittedSHA256}, {"admittedBytes", &r.AdmittedBytes}}
	for _, value := range values {
		if isJSONNull(fields[value.key]) || json.Unmarshal(fields[value.key], value.out) != nil {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: wrong type for %s", value.key)
		}
	}
	if !isJSONNull(fields["boundary"]) && json.Unmarshal(fields["boundary"], &r.Boundary) != nil {
		return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: wrong type for boundary")
	}
	if !isJSONNull(fields["ceiling"]) {
		var ceiling int64
		if json.Unmarshal(fields["ceiling"], &ceiling) != nil {
			return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: wrong type for ceiling")
		}
		r.Ceiling = &ceiling
	}
	if err := validateBriefBoundsRecord(r); err != nil {
		return BriefBoundsRecord{}, fmt.Errorf("invalid brief bounds record: %w", err)
	}
	return r, nil
}

func isJSONNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}
