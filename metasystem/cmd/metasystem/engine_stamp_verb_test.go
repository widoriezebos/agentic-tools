package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

func TestUtilEngineStampPrintsTheRecordAndRefusesAStamplessFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stamped := filepath.Join(dir, "stamped")
	record := "\x00prefix" + enginebuild.StampRecord("witness-abcdef012345") + "\x00suffix"
	if err := os.WriteFile(stamped, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := engineStamp([]string{"--file", stamped}, &stdout, &stderr); code != 0 || stdout.String() != "witness-abcdef012345\n" || stderr.Len() != 0 {
		t.Fatalf("stamped: exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}

	stampless := filepath.Join(dir, "stampless")
	if err := os.WriteFile(stampless, []byte("no record here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if code := engineStamp([]string{"--file", stampless}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "util engine-stamp: no stamp in "+stampless+"\n" {
		t.Fatalf("stampless: exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}

	stderr.Reset()
	if code := engineStamp(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("no --file: exit %d, want 2", code)
	}
}
