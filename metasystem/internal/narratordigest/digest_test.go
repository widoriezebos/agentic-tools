package narratordigest

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func rawPayloadBody(t *testing.T, data []byte, source string) []byte {
	t.Helper()
	marker := []byte(" — RAW-PAYLOAD (source: " + source + ") bytes=")
	start := bytes.Index(data, marker)
	if start < 0 {
		t.Fatalf("raw payload source marker %q is absent from %q", source, data)
	}
	headerEnd := bytes.IndexByte(data[start:], '\n')
	if headerEnd < 0 {
		t.Fatalf("raw payload header has no newline: %q", data[start:])
	}
	headerEnd += start
	sizeStart := start + len(marker)
	size, err := strconv.Atoi(string(data[sizeStart:headerEnd]))
	if err != nil || headerEnd+1+size > len(data) {
		t.Fatalf("raw payload header has an invalid byte count: %q", data[start:headerEnd])
	}
	return data[headerEnd+1 : headerEnd+1+size]
}

func TestRawPayloadPreservesRendererBytesAndDeduplicatesItsSource(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	rendered := []byte("Counselor heading.\n\nWarning:  spaces stay.\n")
	payload := Payload{Kind: "lowlight", Body: rendered, SourceType: "counselor-brief", SourceID: "period-1"}
	if err := AppendPayload(root, payload, now); err != nil {
		t.Fatal(err)
	}
	retry := payload
	retry.Body = []byte("a retry rendered different bytes\n")
	if err := AppendPayload(root, retry, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := rawPayloadBody(t, data, "counselor-brief period-1"); !bytes.Equal(got, rendered) {
		t.Fatalf("digest softened the rendered bytes:\n got %q\nwant %q", got, rendered)
	}
	if count := bytes.Count(data, []byte("RAW-PAYLOAD")); count != 1 {
		t.Fatalf("one source produced %d payload frames", count)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(pending.Message, string(rendered)) {
		t.Fatalf("pending delivery changed the payload suffix: %q", pending.Message)
	}
}

func TestPendingCursorLeavesEventsAppendedDuringDelivery(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	first := Entry{Kind: "highlight", Text: "The first landing shipped.", SourceType: "commit", SourceID: "abc"}
	if err := Append(root, []Entry{first, first}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil || strings.Count(pending.Message, "The first landing shipped") != 1 {
		t.Fatalf("first pending digest was not deduplicated: %+v %v", pending, err)
	}
	if err := Append(root, []Entry{{
		Kind: "lowlight", Text: "A later check found a breach.", SourceType: "episode", SourceID: "stop-2",
	}}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}
	next, err := Pending(root)
	if err != nil || strings.Contains(next.Message, "first landing") || !strings.Contains(next.Message, "later check") {
		t.Fatalf("cursor did not preserve the event appended during delivery: %+v %v", next, err)
	}
}

func TestPendingRefusesAChangedDeliveredPrefix(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	if err := Append(root, []Entry{{
		Kind: "highlight", Text: "The landing shipped.", SourceType: "commit", SourceID: "abc",
	}}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	data[0] = '9'
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Pending(root); err == nil || !strings.Contains(err.Error(), "changed before the last check-in cursor") {
		t.Fatalf("changed delivered prefix was accepted: %v", err)
	}
}

func TestAdvanceRefusesCursorEdgesThatWereNotEmitted(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	if err := Append(root, []Entry{{
		Kind: "lowlight", Text: "The check found a breach.", SourceType: "episode", SourceID: "stop-1",
	}}, now); err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, pending.Cursor, pending.PrefixSHA256); err != nil {
		t.Fatal(err)
	}

	for name, cursor := range map[string]struct {
		cursor int64
		prefix string
	}{
		"backward cursor": {pending.Cursor - 1, pending.PrefixSHA256},
		"past log edge":   {pending.Cursor + 1, pending.PrefixSHA256},
		"wrong prefix":    {pending.Cursor, strings.Repeat("0", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := Advance(root, cursor.cursor, cursor.prefix); err == nil || !strings.Contains(err.Error(), "does not name the emitted prefix") {
				t.Fatalf("unemitted cursor edge was accepted: %v", err)
			}
		})
	}
}
