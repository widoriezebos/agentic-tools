package dispatch

import (
	"bytes"
	"strings"
	"testing"
)

func validBriefBoundsRecord() BriefBoundsRecord {
	ceiling := int64(4)
	return BriefBoundsRecord{1, "job-1", "root-1", 2, strings.Repeat("a", 64), 12, []string{"metasystem/internal/"}, &ceiling}
}

func TestBriefBoundsRecordSchema(t *testing.T) {
	valid, err := EncodeBriefBoundsRecord(validBriefBoundsRecord())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func([]byte) []byte
	}{
		{"unknown", func(b []byte) []byte {
			b = bytes.TrimSpace(b)
			return append(append([]byte(nil), b[:len(b)-1]...), []byte(`,"extra":true}`)...)
		}},
		{"missing", func([]byte) []byte { return []byte(`{"schemaVersion":1}`) }},
		{"duplicate", func([]byte) []byte {
			return []byte(`{"schemaVersion":1,"schemaVersion":1,"jobId":"j","rootJob":"r","round":1,"admittedSha256":"` + strings.Repeat("a", 64) + `","admittedBytes":0,"boundary":null,"ceiling":null}`)
		}},
		{"trailing-json", func(b []byte) []byte { return append(b, []byte(`{}`)...) }},
		{"wrong-type", func(b []byte) []byte { return bytes.Replace(b, []byte(`"round": 2`), []byte(`"round": true`), 1) }},
		{"partial-pair", func(b []byte) []byte { return bytes.Replace(b, []byte(`"ceiling": 4`), []byte(`"ceiling": null`), 1) }},
		{"negative-bytes", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"admittedBytes": 12`), []byte(`"admittedBytes": -1`), 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeBriefBoundsRecord(tc.edit(valid)); err == nil {
				t.Fatal("invalid record admitted")
			}
		})
	}
	parsed, err := DecodeBriefBoundsRecord(valid)
	if err != nil || parsed.JobID != "job-1" || *parsed.Ceiling != 4 {
		t.Fatalf("valid record = %+v, %v", parsed, err)
	}
}
