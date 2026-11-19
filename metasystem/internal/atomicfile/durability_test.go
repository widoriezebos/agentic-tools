package atomicfile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

// earlySyncEnv asks a re-run of this binary to make one atomicfile sync, of
// the named kind, in a directory the parent made, before testenv.Main opts the
// process in to kernel. The level then shows whether that sync went through
// internal/durable: only a sync through the adapter freezes the process at full.
const earlySyncEnv = "ATOMICFILE_EARLY_SYNC"

func earlySyncBeforeMain() error {
	kind, dir, ok := strings.Cut(os.Getenv(earlySyncEnv), ":")
	if !ok {
		return nil
	}
	switch kind {
	case "file":
		name, err := writeTemp(dir, "early", func(f *os.File) error {
			_, err := f.WriteString("x")
			return err
		}, true)
		if err != nil {
			return err
		}
		return os.Remove(name)
	case "dir":
		if !Confirm(dir, "") {
			return errors.New("confirm failed")
		}
		return nil
	}
	return errors.New("unknown early sync kind")
}

func TestAtomicfileLevelUnderMain(t *testing.T) {
	t.Parallel()
	want := "kernel"
	if os.Getenv(earlySyncEnv) != "" {
		want = "full"
	}
	if got := durable.Current(); got != want {
		t.Fatalf("level = %q, want %q", got, want)
	}
}

func TestAtomicfileSyncsThroughDurableAdapter(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"file", "dir"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "target")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestAtomicfileLevelUnderMain$", "-test.count=1", "-test.v")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, earlySyncEnv+"=") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, earlySyncEnv+"="+kind+":"+dir)
			output, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(output), "--- PASS: TestAtomicfileLevelUnderMain") {
				t.Fatalf("%s sync before Main did not go through internal/durable: %v\n%s", kind, err, output)
			}
		})
	}
}
