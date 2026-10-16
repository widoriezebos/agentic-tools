// Package enginepin observes the engine bytes executed by a recorded process.
package enginepin

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Execution retains facts from the executable itself. An empty digest means
// UNKNOWN; Cause explains why the process could not be authenticated.
type Execution struct {
	Digest     string `json:"digest,omitempty"`
	Generation int    `json:"generation,omitempty"`
	Source     string `json:"source,omitempty"`
	Cause      string `json:"cause,omitempty"`
}

// Read joins the process identity to kernel executable evidence and verifies
// its pin name against the same open file that supplies the source stamp.
func Read(root string, ref identity.Ref, prober identity.Prober) Execution {
	unknown := func(err error) Execution { return Execution{Cause: err.Error()} }
	if prober == nil {
		prober = identity.KernelProber{}
	}
	exact, state, err := prober.Probe(ref.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, ref) || exact.Zombie || exact.Exiting || !exact.ExeKnown {
		return unknown(fmt.Errorf("process %d executable identity is unknown (%s): %v", ref.Pid, state, err))
	}
	directory, err := filepath.EvalSymlinks(filepath.Join(root, "artifacts", "agents", "steward", "engine-pins"))
	if err != nil {
		return unknown(err)
	}
	executableDir, pathErr := filepath.EvalSymlinks(filepath.Dir(exact.Exe))
	name := strings.TrimPrefix(filepath.Base(exact.Exe), "generation-")
	generationText, digestName, ok := strings.Cut(name, "-")
	generation, parseErr := strconv.Atoi(generationText)
	if pathErr != nil || executableDir != directory || !strings.HasPrefix(filepath.Base(exact.Exe), "generation-") || !ok || parseErr != nil || generation < 1 || len(digestName) != 64 {
		return unknown(fmt.Errorf("process %d executable is not an engine pin: %s", ref.Pid, exact.Exe))
	}
	file, err := os.Open(exact.Exe)
	if err != nil {
		return unknown(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return unknown(fmt.Errorf("engine pin is not a readable executable regular file: %v", err))
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return unknown(err)
	}
	digest := fmt.Sprintf("%x", hash.Sum(nil))
	if digest != digestName {
		return unknown(fmt.Errorf("engine pin filename/bytes mismatch: named %s, bytes %s", digestName, digest))
	}
	source, err := enginebuild.ReadStamp(file)
	if err != nil {
		return unknown(fmt.Errorf("read engine pin source stamp: %w", err))
	}
	pathInfo, err := os.Stat(exact.Exe)
	after, afterState, probeErr := prober.Probe(ref.Pid)
	if err != nil || !os.SameFile(info, pathInfo) || probeErr != nil || afterState != identity.Alive || !identity.SameIdentity(after, ref) || after.Zombie || after.Exiting || !after.ExeKnown || after.Exe != exact.Exe {
		return unknown(fmt.Errorf("process %d executable changed or became uninspectable during observation", ref.Pid))
	}
	return Execution{Digest: "sha256:" + digest, Generation: generation, Source: source}
}
