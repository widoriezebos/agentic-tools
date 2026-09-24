package slack

import (
	"testing"
	"time"
)

func TestParseTimestampPreservesProtocolPrecision(t *testing.T) {
	t.Parallel()
	want := time.Date(2030, 1, 2, 3, 4, 5, 123457000, time.UTC)
	got, ok := parseTimestamp("1893553445.123457")
	if !ok || !got.Equal(want) {
		t.Fatalf("parsed timestamp = %s, valid=%t, want %s", got, ok, want)
	}
}

func TestParseTimestampRejectsMalformedProtocolValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", ".123456", "-1893553445.123456", "1893553445.", "1893553445.-1", "1893553445.+1",
		"1893553445.1234567890", "1893553445.12.34", "1893553445.abc",
	} {
		if got, ok := parseTimestamp(value); ok || !got.IsZero() {
			t.Fatalf("parseTimestamp(%q) = %s, %t; want zero, false", value, got, ok)
		}
	}
}
