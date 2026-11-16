package fileclone

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestCopy(t *testing.T) {
	t.Parallel()
	for _, fault := range []error{nil, unix.ENOTSUP, unix.EXDEV} {
		name := "native"
		if fault != nil {
			name = fault.Error()
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path, destination := filepath.Join(directory, "source"), filepath.Join(directory, "copy")
			contents := bytes.Repeat([]byte("authenticated contents\n"), 4096)
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			source, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			// Both the clone and fallback must use the opened inode, from its start.
			if err := os.Rename(path, path+".old"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := source.Seek(17, 0); err != nil {
				t.Fatal(err)
			}
			cloned := false
			attempt := func(open *os.File, target string) (*os.File, error) {
				if open != source {
					t.Fatal("copy replaced the authenticated descriptor")
				}
				if fault != nil {
					return nil, fault
				}
				file, err := clone(open, target)
				if err == nil {
					cloned = true
				}
				return file, err
			}
			if err := copyWith(source, destination, 0o500, attempt); err != nil {
				t.Fatal(err)
			}
			original, err := source.Stat()
			if err != nil {
				t.Fatal(err)
			}
			copied, err := os.Stat(destination)
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(original, copied) || copied.Mode().Perm() != 0o500 {
				t.Fatalf("copy shares inode or wrong mode: %v", copied)
			}
			data, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, contents) {
				t.Fatalf("copy differs: %v", err)
			}
			if err := copyWith(source, destination, 0o700, attempt); !errors.Is(err, os.ErrExist) {
				t.Fatalf("existing destination: %v", err)
			}
			data, err = os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, contents) {
				t.Fatalf("refusal changed destination: %v", err)
			}
			if err := os.Chmod(destination, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, []byte("changed clone"), 0o600); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path + ".old")
			if err != nil || !bytes.Equal(data, contents) {
				t.Fatalf("write changed source: %v", err)
			}
			if fault == nil {
				t.Logf("native clone supported: %t", cloned)
			}
		})
	}
}
