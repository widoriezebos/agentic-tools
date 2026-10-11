package rehearsal

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func TestRehearsalArchivePreservesFilesModesAndSymlinks(t *testing.T) {
	t.Parallel()
	archive := filepath.Join(t.TempDir(), "candidate.tar")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	for _, header := range []*tar.Header{
		{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "candidate-commit"}},
		{Name: "bin/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "bin/engine", Typeflag: tar.TypeReg, Mode: 0755, Size: 7},
		{Name: "engine", Typeflag: tar.TypeSymlink, Linkname: "bin/engine", Mode: 0777},
	} {
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte("payload")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot := t.TempDir()
	if err := unpackArchive(archive, snapshot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(snapshot, "engine"))
	if err != nil || string(data) != "payload" {
		t.Fatalf("archive content: %q: %v", data, err)
	}
	info, err := os.Stat(filepath.Join(snapshot, "bin/engine"))
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("executable mode: %v: %v", info, err)
	}
	directoryInfo, err := os.Stat(filepath.Join(snapshot, "bin"))
	if err != nil || directoryInfo.Mode().Perm() != 0755 {
		t.Fatalf("directory mode: %v: %v", directoryInfo, err)
	}
	link, err := os.Readlink(filepath.Join(snapshot, "engine"))
	if err != nil || link != "bin/engine" {
		t.Fatalf("symlink: %q: %v", link, err)
	}
}

func TestRehearsalArchiveRefusesTruncationAndEscape(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"../escape", "link/escape", "truncated"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			archive := filepath.Join(t.TempDir(), "candidate.tar")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := tar.NewWriter(file)
			if name == "link/escape" {
				if err := writer.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: t.TempDir(), Mode: 0777}); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0644, Size: 10}); err != nil {
				t.Fatal(err)
			}
			if name != "truncated" {
				_, err = writer.Write([]byte("0123456789"))
				if err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := unpackArchive(archive, t.TempDir()); err == nil {
				t.Fatal("invalid archive accepted")
			}
		})
	}
}
