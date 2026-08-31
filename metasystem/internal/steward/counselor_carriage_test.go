package steward

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

func writeCounselorConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func counselorPayloads(t *testing.T, data []byte) [][]byte {
	t.Helper()
	var payloads [][]byte
	for searchAt := 0; ; {
		relative := bytes.Index(data[searchAt:], []byte(" — RAW-PAYLOAD (source: counselor-brief "))
		if relative < 0 {
			return payloads
		}
		start := searchAt + relative
		headerEndRelative := bytes.IndexByte(data[start:], '\n')
		if headerEndRelative < 0 {
			t.Fatalf("counselor payload header has no newline: %q", data[start:])
		}
		headerEnd := start + headerEndRelative
		sizeMarker := bytes.LastIndex(data[start:headerEnd], []byte(" bytes="))
		if sizeMarker < 0 {
			t.Fatalf("counselor payload header has no byte count: %q", data[start:headerEnd])
		}
		sizeStart := start + sizeMarker + len(" bytes=")
		size, err := strconv.Atoi(string(data[sizeStart:headerEnd]))
		bodyStart := headerEnd + 1
		if err != nil || size < 1 || bodyStart+size > len(data) {
			t.Fatalf("counselor payload byte count is invalid: %q", data[start:headerEnd])
		}
		payloads = append(payloads, append([]byte(nil), data[bodyStart:bodyStart+size]...))
		searchAt = bodyStart + size
	}
}

func renderedCounselorBrief(t *testing.T, root string, now time.Time) []byte {
	t.Helper()
	brief := counselor.Build(counselor.Options{Root: root, Now: func() time.Time { return now }})
	var rendered bytes.Buffer
	if err := counselor.Render(&rendered, brief); err != nil {
		t.Fatal(err)
	}
	return rendered.Bytes()
}

func TestCounselorBriefCarriageDeliversOncePerConfiguredPeriod(t *testing.T) {
	root := t.TempDir()
	writeCounselorConfig(t, root, counselorBriefCadenceKey+"=12\n")
	first := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	if err := sweepCounselorBrief(root, first); err != nil {
		t.Fatal(err)
	}
	if err := sweepCounselorBrief(root, first.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if payloads := counselorPayloads(t, data); len(payloads) != 1 {
		t.Fatalf("one twelve-hour period produced %d counselor briefs", len(payloads))
	}
	cursor, err := loadCounselorBriefCursor(root)
	if err != nil || cursor.Status != counselorBriefDelivered || cursor.CadenceHours != 12 {
		t.Fatalf("the delivered period cursor is incomplete: %+v %v", cursor, err)
	}

	if err := sweepCounselorBrief(root, first.Add(12*time.Hour)); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if payloads := counselorPayloads(t, data); len(payloads) != 2 {
		t.Fatalf("two twelve-hour periods produced %d counselor briefs", len(payloads))
	}
}

func TestCounselorBriefCarriagePreservesRendererBytes(t *testing.T) {
	root := t.TempDir()
	writeCounselorConfig(t, root, "")
	now := time.Date(2026, 8, 30, 13, 15, 0, 0, time.UTC)
	expected := renderedCounselorBrief(t, root, now)
	if hours, err := counselorBriefCadenceHours(root); err != nil || hours != defaultCounselorBriefCadenceHours {
		t.Fatalf("absent counselor cadence did not resolve to the twenty-four-hour default: hours=%d err=%v", hours, err)
	}
	if err := sweepCounselorBrief(root, now); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	payloads := counselorPayloads(t, data)
	if len(payloads) != 1 || !bytes.Equal(payloads[0], expected) {
		t.Fatalf("digest payload differs from renderer output:\n got %q\nwant %q", payloads, expected)
	}
	pending, err := narratordigest.Pending(root)
	if err != nil || !strings.HasSuffix(pending.Message, string(expected)) {
		t.Fatalf("pending human delivery softened the rendered suffix: err=%v message=%q", err, pending.Message)
	}
}

type failingCounselorWriter struct{}

func (failingCounselorWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture renderer closed")
}

func TestCounselorRenderFailureSurfacesCounselorNamedHealth(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 30, 13, 15, 0, 0, time.UTC)
	period := counselorBriefPeriodStart(now, defaultCounselorBriefCadenceHours)
	renderErr := counselor.Render(failingCounselorWriter{}, counselor.Brief{})
	if renderErr == nil {
		t.Fatal("the fixture did not produce a real counselor render failure")
	}
	cursor := counselorBriefCursor{Schema: 1}
	if err := recordCounselorRenderFailure(root, cursor, period, defaultCounselorBriefCadenceHours, now, renderErr); err != nil {
		t.Fatal(err)
	}
	recorded, err := loadCounselorBriefCursor(root)
	if err != nil || recorded.Status != counselorBriefRenderFailed || !strings.Contains(recorded.Failure, "fixture renderer closed") {
		t.Fatalf("render failure cursor is incomplete: %+v %v", recorded, err)
	}
	digest, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	healthLine := "HEALTH unhealthy — counselor=dead"
	if !bytes.Contains(digest, []byte(healthLine)) || !bytes.Contains(digest, []byte("source: counselor-brief-health")) {
		t.Fatalf("render failure did not reach the counselor-named digest health line: %s", digest)
	}
	narration, err := os.ReadFile(NarrationPath(root))
	if err != nil || !bytes.Contains(narration, []byte(healthLine)) {
		t.Fatalf("render failure did not reach health narration: %q %v", narration, err)
	}
	if err := recordCounselorRenderFailure(root, recorded, period, defaultCounselorBriefCadenceHours, now.Add(time.Minute), renderErr); err != nil {
		t.Fatal(err)
	}
	afterRetry, err := os.ReadFile(narratordigest.Path(root))
	if err != nil || bytes.Count(afterRetry, []byte(healthLine)) != 1 {
		t.Fatalf("the same failed period repeated its health line: %q %v", afterRetry, err)
	}
}
