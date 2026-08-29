package proofrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManifestRecordsSymlinkExecutableAndRawSort(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "z-plain"), []byte("plain"), 0644)
	writeTestFile(t, filepath.Join(root, "internal", "a-exec"), []byte("exec"), 0755)
	writeTestFile(t, filepath.Join(root, "internal", "b-group-other-exec"), []byte("not owner executable"), 0611)
	if err := os.Symlink("z-plain", filepath.Join(root, "internal", "m-link")); err != nil {
		t.Fatal(err)
	}
	rawName := "~raw"
	writeTestFile(t, filepath.Join(root, "internal", rawName), []byte("raw"), 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(m.entries))
	for i, item := range m.entries {
		paths[i] = item.path
	}
	wantOrder := []string{"internal/a-exec", "internal/b-group-other-exec", "internal/m-link", "internal/z-plain", "internal/" + rawName}
	if !reflect.DeepEqual(paths, wantOrder) {
		t.Fatalf("raw path order = %q, want %q", paths, wantOrder)
	}
	if !m.entries[0].executable || m.entries[0].kind != 'f' {
		t.Fatalf("executable file record lost its kind or executable bit: %+v", m.entries[0])
	}
	if m.entries[1].executable {
		t.Fatalf("group/other-only execute permissions set the normative executable bit: %+v", m.entries[1])
	}
	if m.entries[3].executable {
		t.Fatalf("plain file record acquired an executable bit: %+v", m.entries[3])
	}
	if m.entries[2].kind != 'l' || !bytes.Equal(m.entries[2].target, []byte("z-plain")) {
		t.Fatalf("symlink record = %+v, want raw target z-plain", m.entries[2])
	}
	if err := os.Chmod(filepath.Join(root, "internal", "b-group-other-exec"), 0600); err != nil {
		t.Fatal(err)
	}
	withoutGroupOtherExec, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if withoutGroupOtherExec.digest != m.digest {
		t.Fatalf("group/other-only execute permissions changed the manifest digest: %s != %s", withoutGroupOtherExec.digest, m.digest)
	}
}

func TestManifestLengthFramingAndDigest(t *testing.T) {
	root := t.TempDir()
	content := []byte("framed content")
	writeTestFile(t, filepath.Join(root, "internal", "one"), content, 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.records) < recordLengthBytes {
		t.Fatalf("framed manifest is only %d bytes", len(m.records))
	}
	bodyLength := binary.BigEndian.Uint64(m.records[:recordLengthBytes])
	if int(bodyLength) != len(m.records)-recordLengthBytes {
		t.Fatalf("record length prefix = %d, body has %d bytes", bodyLength, len(m.records)-recordLengthBytes)
	}
	fileHash := sha256.Sum256(content)
	wantBody := append([]byte("internal/one\x00f\x00"), fileHash[:]...)
	if got := m.records[recordLengthBytes:]; !bytes.Equal(got, wantBody) {
		t.Fatalf("record body = %x, want %x", got, wantBody)
	}
	wantDigest := sha256.Sum256(m.records)
	if m.digest != stringHex(wantDigest[:]) {
		t.Fatalf("manifest digest = %s, want %x", m.digest, wantDigest)
	}
}

func TestManifestExcludesOnlyDeclaredRootClosures(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"artifacts/runtime", "bin/metasystem", ".git/index"} {
		writeTestFile(t, filepath.Join(root, path), []byte(path), 0644)
	}
	writeTestFile(t, filepath.Join(root, "internal/artifacts/input"), []byte("included"), 0644)

	m, err := readManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(m.entries))
	for i, item := range m.entries {
		paths[i] = item.path
	}
	// Directories are pruned only at the declared runtime roots; a
	// nested artifacts name inside a projection member is ordinary
	// content, and directories themselves are no longer entries.
	want := []string{"internal/artifacts/input"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("manifest paths = %q, want %q", paths, want)
	}
}

func TestFreezeExportsIdenticalProjection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal/dir/plain"), []byte("plain"), 0644)
	writeTestFile(t, filepath.Join(root, "cmd/tool"), []byte("tool"), 0755)
	if err := os.Symlink("dir/plain", filepath.Join(root, "internal/link")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "artifacts/ignored"), []byte("runtime"), 0644)

	frozen, err := Freeze(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(frozen.Root)) })
	if _, err := Verify(frozen.Root, frozen.Digest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(frozen.Root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("excluded artifacts closure was exported: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(frozen.Root, "internal/link")); err != nil || target != "dir/plain" {
		t.Fatalf("exported symlink target = %q, %v", target, err)
	}
}

func writeTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func stringHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2] = digits[b>>4]
		result[i*2+1] = digits[b&0xf]
	}
	return string(result)
}
