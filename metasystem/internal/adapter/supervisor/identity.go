package supervisor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

// cliVersion runs `CLI --version` and parses the first dotted version token.
func (d Deps) cliVersion(cli string) (string, error) {
	path, err := d.LookPath(cli)
	if err != nil {
		return "", fmt.Errorf("%s CLI is not installed", cli)
	}
	command := exec.Command(path, "--version")
	command.Env = d.Environ
	output, _ := command.Output()
	return adapter.ParseCLIVersion(bytes.NewReader(output))
}

// projectRoot is the git top level of the installation, the project whose
// runtime settings a session launched here merges.
func (d Deps) projectRoot() (string, error) {
	top, ok := d.Git(d.Root, "rev-parse", "--show-toplevel")
	if !ok {
		return "", errors.New("the installation is not inside a git work tree")
	}
	return top, nil
}

// configIdentity is a runtime's canonical configuration identity: its CLI
// version and the hash of the settings sources it merges.
func (d Deps) configIdentity(runtime, version string, sources []string) (string, error) {
	identity, err := config.BuildConfigIdentity(runtime, version, sources)
	if err != nil {
		return "", err
	}
	return config.CanonicalConfigJSON(identity)
}

// identityFields reads the version, hash, and per-key hashes out of an
// identity document.
func identityFields(identity string) (version, hash, keyHashes string, err error) {
	var ok bool
	if version, ok = jsonedit.Get([]byte(identity), "cliVersion", nil); !ok {
		return "", "", "", errors.New("configuration identity lacks cliVersion")
	}
	if hash, ok = jsonedit.Get([]byte(identity), "configHash", nil); !ok {
		return "", "", "", errors.New("configuration identity lacks configHash")
	}
	if keyHashes, ok = jsonedit.Get([]byte(identity), "configKeyHashes", nil); !ok {
		return "", "", "", errors.New("configuration identity lacks configKeyHashes")
	}
	return version, hash, keyHashes, nil
}

// writeCapabilitySnapshot writes a probed runtime's snapshot into the
// installation's capabilities directory.
func (d Deps) writeCapabilitySnapshot(runtime, version, hash, transports, capabilities, permissions, enforcement, keyHashes string) error {
	dir := filepath.Join(d.agents(), "capabilities")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path, err := adapter.WriteCapabilitySnapshot(dir, runtime, version, hash, transports, capabilities, permissions, enforcement, keyHashes)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.Stdout, path)
	return nil
}

// contractSnapshot is the no-auth, no-provider contract emission: the REAL
// snapshot construction path runs against deterministic dummy facts in a
// throwaway directory, and the constructed bytes are returned. No schema
// field is invented; the shape the suite decodes IS the shape probes write.
func contractSnapshot(runtime, enforcement string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "metasystem-contract.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path, err := adapter.WriteCapabilitySnapshot(dir, runtime, "0.0.0-contract", "contract0", "[]",
		`{"sessionEstablishedTimeoutSec":1}`, `{"unverified":[]}`, enforcement, "{}")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// configValue resolves one metasystem.conf key for this installation.
func (d Deps) configValue(key, fallback string) (string, error) {
	value, _, err := config.Get(config.GetParams{
		Key: key, Default: fallback, DefaultSet: true,
		ConfPath: filepath.Join(d.Root, "metasystem.conf"),
		LookupEnv: func(name string) (string, bool) {
			for _, entry := range d.Environ {
				if found, value, ok := strings.Cut(entry, "="); ok && found == name {
					return value, true
				}
			}
			return "", false
		},
	})
	return value, err
}
