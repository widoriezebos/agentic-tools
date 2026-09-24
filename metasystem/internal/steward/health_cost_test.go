package steward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
)

func TestHookPreviewWarmCostAtSyntheticVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("synthetic hook-preview volume is intentionally excluded from short tests")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := strings.Join([]string{
		"metasystem.runtimes=fake",
		"spend.mode=alert",
		"spend.currency=USD",
		"spend.ceiling.day.tokens=250000000",
		"spend.ceiling.day.money=750",
		"spend.ceiling.goal.tokens=125000000",
		"spend.ceiling.goal.money=300",
		"spend.price.fake.fixture-model.input=1",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	jobPadding := strings.Repeat("x", 10*1024)
	for index := 0; index < 700; index++ {
		record := fmt.Sprintf(`{"jobId":"synthetic-%03d","goalId":"synthetic-volume","machineId":"fixture-m1","status":"completed","runtime":"fake","canonicalModelKey":"fixture-model","startedAt":"2026-09-06T10:00:00Z","usage":{"inputTokens":1},"padding":%q}`, index, jobPadding)
		if err := os.WriteFile(filepath.Join(jobs, fmt.Sprintf("synthetic-%03d.json", index)), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	projects := filepath.Join(home, ".claude", "projects")
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	var warmTranscript string
	for index := 0; index < 5; index++ {
		path := filepath.Join(projects, slug, fmt.Sprintf("seat-%02d.jsonl", index))
		writeSyntheticTranscript(t, path, root, fmt.Sprintf("seat-request-%02d", index), 5*1024*1024)
		if index == 0 {
			warmTranscript = path
		}
	}
	foreignRoot := filepath.Join(filepath.Dir(root), "foreign-checkout")
	if err := os.MkdirAll(filepath.Join(foreignRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 35; index++ {
		path := filepath.Join(projects, fmt.Sprintf("%s-foreign-%02d", slug, index), fmt.Sprintf("foreign-%02d.jsonl", index))
		writeSyntheticTranscript(t, path, foreignRoot, fmt.Sprintf("foreign-request-%02d", index), 5*1024*1024)
	}

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	started := time.Now()
	first, coldWork := previewHealthAtWithWork(root, now)
	cold := time.Since(started)
	appended := syntheticTranscriptLine(t, root, "seat-request-appended")
	file, err := os.OpenFile(warmTranscript, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(appended); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	cpuBefore := processCPUTime()
	second, warmWork := previewHealthAtWithWork(root, now.Add(time.Minute))
	warm := time.Since(started)
	warmCPU := processCPUTime() - cpuBefore
	fmt.Printf("hook preview synthetic volume: cold=%s warm=%s warm-cpu=%s cold-work=%+v warm-work=%+v\n", cold, warm, warmCPU, coldWork, warmWork)
	if len(first.Roles) != len(healthRoleOrder) || len(second.Roles) != len(healthRoleOrder) {
		t.Fatalf("synthetic health preview omitted roles: cold=%d warm=%d", len(first.Roles), len(second.Roles))
	}
	if coldWork.JobRecordsRead != 700 || coldWork.TranscriptLinesParsed != 45 || coldWork.CacheWrites != 41 {
		t.Fatalf("cold preview work = %+v, want 700 job reads, 45 parsed lines, and 41 cache writes", coldWork)
	}
	if coldWork.TranscriptBytesRead < 25*1024*1024 || coldWork.TranscriptBytesRead >= 40*5*1024*1024 {
		t.Fatalf("cold preview transcript bytes = %d, want all five seat files but bounded foreign-file probes", coldWork.TranscriptBytesRead)
	}
	if warmWork.JobRecordsRead != 0 || warmWork.TranscriptBytesRead != int64(len(appended)) || warmWork.TranscriptLinesParsed != 1 || warmWork.CacheWrites != 1 {
		t.Fatalf("warm preview did not resume only the appended transcript suffix: work=%+v appended=%d", warmWork, len(appended))
	}
}

func previewHealthAtWithWork(root string, now time.Time) (HealthVerdict, spend.MeasureWork) {
	var work spend.MeasureWork
	measure := func(repoRoot, machine string, observedAt time.Time) (spend.Ledger, error) {
		ledger, measured, err := spend.MeasureWithWork(repoRoot, machine, observedAt)
		work = measured
		return ledger, err
	}
	return previewHealthAtWithMeasure(root, root, now, healthProbe{}, measure), work
}

func writeSyntheticTranscript(t *testing.T, path, cwd, requestID string, size int) {
	t.Helper()
	line := syntheticTranscriptLine(t, cwd, requestID)
	if len(line)+1 > size {
		t.Fatalf("synthetic transcript line is larger than its %d-byte file", size)
	}
	content := make([]byte, size)
	copy(content, line)
	copy(content[len(line):], bytes.Repeat([]byte{' '}, size-len(line)))
	content[len(content)-1] = '\n'
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func syntheticTranscriptLine(t *testing.T, cwd, requestID string) []byte {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"type": "assistant", "sessionId": requestID, "requestId": requestID,
		"cwd": cwd, "timestamp": "2026-09-06T10:00:00Z",
		"message": map[string]any{
			"model": "claude-sonnet-4-20250514",
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(line, '\n')
}
