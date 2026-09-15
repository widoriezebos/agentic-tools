package dispatch

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

const validBoundsRecord = `{"schemaVersion":1,"jobId":"impl-r2","rootJob":"impl","round":2,"admittedSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","admittedBytes":123,"boundary":["metasystem/internal/"],"ceiling":400}`

func recordVariant(old, replacement string) []byte {
	return []byte(strings.Replace(validBoundsRecord, old, replacement, 1))
}

func TestBriefBoundsRecordSchema(t *testing.T) {
	t.Run("version", func(t *testing.T) {
		r := decodeRecord(t, []byte(validBoundsRecord))
		assertRecordRefused(t, recordVariant(`"schemaVersion":1`, `"schemaVersion":2`))
		r.SchemaVersion = 2
		assertRecordMarshalRefused(t, r)
	})
	t.Run("required", func(t *testing.T) {
		for _, field := range []string{`"schemaVersion":1,`, `"jobId":"impl-r2",`, `"rootJob":"impl",`, `"round":2,`, `"admittedSha256":"` + strings.Repeat("a", 64) + `",`, `"admittedBytes":123,`, `"boundary":["metasystem/internal/"],`, `,"ceiling":400`} {
			assertRecordRefused(t, []byte(strings.Replace(validBoundsRecord, field, "", 1)))
		}
		assertRecordRefused(t, []byte(`{"schemaVersion":1,"jobId":"impl-r2","rootJob":"impl","round":2,"admittedSha256":"`+strings.Repeat("a", 64)+`","admittedBytes":123}`))
	})
	t.Run("unknown", func(t *testing.T) { assertRecordRefused(t, recordVariant(`"round":2`, `"round":2,"extra":true`)) })
	t.Run("duplicate-key", func(t *testing.T) { assertRecordRefused(t, recordVariant(`"round":2`, `"round":2,"round":2`)) })
	t.Run("trailing-json", func(t *testing.T) { assertRecordRefused(t, []byte(validBoundsRecord+` {}`)) })
	t.Run("types", func(t *testing.T) {
		for _, pair := range [][2]string{
			{`"schemaVersion":1`, `"schemaVersion":"1"`}, {`"jobId":"impl-r2"`, `"jobId":1`},
			{`"rootJob":"impl"`, `"rootJob":false`}, {`"round":2`, `"round":"2"`},
			{`"admittedSha256":"` + strings.Repeat("a", 64) + `"`, `"admittedSha256":1`},
			{`"admittedBytes":123`, `"admittedBytes":"123"`}, {`"admittedBytes":123`, `"admittedBytes":null`},
			{`"boundary":["metasystem/internal/"]`, `"boundary":{}`}, {`"ceiling":400`, `"ceiling":"400"`},
		} {
			assertRecordRefused(t, recordVariant(pair[0], pair[1]))
		}
		badBoundary := recordVariant(`"boundary":["metasystem/internal/"],"ceiling":400`, `"boundary":{},"ceiling":null`)
		assertRecordRefused(t, badBoundary)
		badCeiling := recordVariant(`"boundary":["metasystem/internal/"],"ceiling":400`, `"boundary":null,"ceiling":"400"`)
		assertRecordRefused(t, badCeiling)
		assertRecordRefused(t, []byte(`["schemaVersion",1,"jobId","impl-r2","rootJob","impl","round",2,"admittedSha256","`+strings.Repeat("a", 64)+`","admittedBytes",123,"boundary",["metasystem/internal/"],"ceiling",400]`))
	})
	t.Run("identity", func(t *testing.T) {
		for _, pair := range [][2]string{{`"jobId":"impl-r2"`, `"jobId":""`}, {`"rootJob":"impl"`, `"rootJob":""`}, {`"round":2`, `"round":0`}, {`"round":2`, `"round":-1`}, {`"round":2`, `"round":9223372036854775808`}} {
			assertRecordRefused(t, recordVariant(pair[0], pair[1]))
		}
		if r := decodeRecord(t, recordVariant(`"round":2`, `"round":9223372036854775807`)); r.Round != math.MaxInt64 {
			t.Fatalf("round = %d", r.Round)
		}
	})
	t.Run("digest", func(t *testing.T) {
		for _, digest := range []string{strings.Repeat("a", 63), strings.Repeat("A", 64), strings.Repeat("g", 64)} {
			assertRecordRefused(t, recordVariant(strings.Repeat("a", 64), digest))
		}
	})
	t.Run("bytes", func(t *testing.T) {
		assertRecordRefused(t, recordVariant(`"admittedBytes":123`, `"admittedBytes":-1`))
		assertRecordRefused(t, recordVariant(`"admittedBytes":123`, `"admittedBytes":9223372036854775808`))
		for _, count := range []string{"0", "9223372036854775807"} {
			if r := decodeRecord(t, recordVariant(`"admittedBytes":123`, `"admittedBytes":`+count)); r.AdmittedBytes < 0 {
				t.Fatalf("admitted bytes = %d", r.AdmittedBytes)
			}
		}
	})
	t.Run("pair", func(t *testing.T) {
		assertRecordRefused(t, recordVariant(`"boundary":["metasystem/internal/"]`, `"boundary":null`))
		assertRecordRefused(t, recordVariant(`"ceiling":400`, `"ceiling":null`))
	})
	t.Run("null-pair", func(t *testing.T) {
		r := decodeRecord(t, recordVariant(`"boundary":["metasystem/internal/"],"ceiling":400`, `"boundary":null,"ceiling":null`))
		if r.Boundary != nil || r.Ceiling != nil {
			t.Fatalf("null pair = %#v/%v", r.Boundary, r.Ceiling)
		}
		assertRecordRoundTrip(t, r)
	})
	t.Run("empty-boundary", func(t *testing.T) {
		r := decodeRecord(t, recordVariant(`["metasystem/internal/"]`, `[]`))
		if r.Boundary == nil || len(r.Boundary) != 0 {
			t.Fatalf("boundary = %#v", r.Boundary)
		}
		assertRecordRoundTrip(t, r)
	})
	t.Run("ceiling-range", func(t *testing.T) {
		for _, endpoint := range []struct {
			encoded string
			want    int64
		}{{"0", 0}, {"9223372036854775807", math.MaxInt64}} {
			r := decodeRecord(t, recordVariant(`"ceiling":400`, `"ceiling":`+endpoint.encoded))
			if r.Ceiling == nil {
				t.Fatal("ceiling is nil")
			}
			if *r.Ceiling != endpoint.want {
				t.Fatalf("ceiling = %d, want %d", *r.Ceiling, endpoint.want)
			}
		}
		assertRecordRefused(t, recordVariant(`"ceiling":400`, `"ceiling":-1`))
		assertRecordRefused(t, recordVariant(`"ceiling":400`, `"ceiling":9223372036854775808`))
	})
	t.Run("member-values", func(t *testing.T) {
		for _, invalid := range []string{`[""]`, `["/a"]`, `["a//b"]`, `["a/./b"]`, `["a/../b"]`} {
			assertRecordRefused(t, recordVariant(`["metasystem/internal/"]`, invalid))
		}
		for _, literal := range []string{`a[`, `a\\b`} {
			r := decodeRecord(t, recordVariant(`["metasystem/internal/"]`, `["`+literal+`"]`))
			if r.Boundary[0] != strings.ReplaceAll(literal, `\\`, `\`) {
				t.Fatalf("member changed: %#v", r.Boundary)
			}
		}
	})
}

func assertRecordRoundTrip(t *testing.T, want BriefBoundsRecord) {
	t.Helper()
	data, err := MarshalBriefBoundsRecord(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("record lacks trailing newline: %q", data)
	}
	if got := decodeRecord(t, data); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func assertRecordMarshalRefused(t *testing.T, r BriefBoundsRecord) {
	t.Helper()
	if data, err := MarshalBriefBoundsRecord(r); err == nil {
		t.Fatalf("record marshaled: %s", data)
	}
}

func assertRecordRefused(t *testing.T, data []byte) {
	t.Helper()
	if _, err := DecodeBriefBoundsRecord(data); err == nil {
		t.Fatalf("record admitted: %s", data)
	}
}

func decodeRecord(t *testing.T, data []byte) BriefBoundsRecord {
	t.Helper()
	r, err := DecodeBriefBoundsRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
