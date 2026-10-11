package rehearsal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

func TestRehearsalChildScopeRequiresItsAcceptedStoreAndPrivatePaths(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, err := Prepare(ctx, t.TempDir(), os.Environ())
	if s == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("scratch: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(s.base) })
	module, executable, registry := filepath.Join(s.Scratch, "lane/metasystem"), filepath.Join(s.Scratch, "bin/metasystem"), filepath.Dir(s.Registry)
	if !ChildScope(s.Scratch, module, executable, registry) {
		t.Fatal("private accepted rehearsal was refused")
	}
	for _, mutation := range []string{"module", "engine", "registry", "store", "alias"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			path, mod, exe, home := s.Scratch, module, executable, registry
			switch mutation {
			case "module":
				mod = t.TempDir()
			case "engine":
				exe = filepath.Join(t.TempDir(), "metasystem")
			case "registry":
				home = t.TempDir()
			case "store":
				path = t.TempDir()
				mod, exe, home = filepath.Join(path, "lane/metasystem"), filepath.Join(path, "bin/metasystem"), filepath.Join(path, "registry")
			case "alias":
				path = filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(s.Scratch, path); err != nil {
					t.Fatal(err)
				}
			}
			if ChildScope(path, mod, exe, home) {
				t.Fatalf("%s escaped private rehearsal scope", mutation)
			}
		})
	}
}

func TestRehearsalPrivateRecordNeverWritesTheInvokingRepository(t *testing.T) {
	t.Parallel()
	for _, isolated := range []bool{false, true} {
		t.Run(map[bool]string{false: "private clone", true: "test registry"}[isolated], func(t *testing.T) {
			t.Parallel()
			module, path := t.TempDir(), recordScratch(t)
			engine := filepath.Join(path, "engine")
			if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("tree")), 0644); err != nil {
				t.Fatal(err)
			}
			s := &Scratch{Record: Record{Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path)), isolatedCaller: isolated}
			if err := s.Finish(module, time.Now(), true, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(module, "records")); !os.IsNotExist(err) {
				t.Fatalf("private run wrote invoking repository records: %v", err)
			}
			if _, err := os.Stat(filepath.Join(path, "records/rehearsals/tree.json")); err != nil {
				t.Fatalf("scratch evidence missing: %v", err)
			}
		})
	}
}

func TestRehearsalRecordPublicationRequiresPublicCommandAndNormalRegistry(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"public command", "test registry", "private driver", "another checkout"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			module, path := t.TempDir(), recordScratch(t)
			engine := filepath.Join(path, "engine")
			if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("tree")), 0644); err != nil {
				t.Fatal(err)
			}
			s := &Scratch{Record: Record{Tree: "tree", Engine: engine, Scratch: path}, isolatedCaller: mutation == "test registry"}
			if mutation != "private driver" {
				s.PublishCheckoutRecord(module)
			}
			if mutation == "another checkout" {
				module = t.TempDir()
			}
			if err := s.Finish(module, time.Now(), true, nil); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(filepath.Join(module, "records/rehearsals/tree.json"))
			if got := err == nil; got != (mutation == "public command") {
				t.Fatalf("%s: checkout record=%t: %v", mutation, got, err)
			}
			if mutation != "public command" {
				if _, err := os.Stat(filepath.Join(path, "records/rehearsals/tree.json")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
