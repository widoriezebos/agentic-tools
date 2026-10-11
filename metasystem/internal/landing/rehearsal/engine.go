package rehearsal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// reuseEngine reads the build record without running the candidate binary.
// A missing, unreadable, or differently stamped build is rebuilt once.
func (s *Scratch) reuseEngine(module string) (bool, error) {
	start := time.Now()
	source := filepath.Join(module, "bin", "metasystem")
	file, err := os.Open(source)
	if err != nil {
		return false, nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return false, nil
	}
	stamp, err := enginebuild.ReadStamp(file)
	if err != nil || !s.matchesBuildTree(module, stamp) {
		return false, nil
	}
	target, err := os.OpenFile(s.Engine, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return false, err
	}
	_, copyErr := io.Copy(target, file)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		return false, fmt.Errorf("copy candidate engine: %v; close: %v", copyErr, closeErr)
	}
	s.Steps = append(s.Steps, Step{Command: "copy candidate bin/metasystem", WallMS: time.Since(start).Milliseconds(), Exit: 0, Output: "build tree " + s.Tree})
	s.engineStamp, s.engineTree = stamp, s.Tree
	return true, nil
}

func (s *Scratch) matchesBuildTree(module, stamp string) bool {
	if stamp == s.Tree || stamp == s.Candidate {
		return true
	}
	// Older builds record a commit; different commits can carry the same tree.
	if len(stamp) != 40 && len(stamp) != 64 {
		return false
	}
	tree, err := s.Git(module, "rev-parse", "--verify", stamp+"^{tree}")
	return err == nil && tree == s.Tree
}

// ChildScope confines the scripted person and agent to an accepted rehearsal
// store, its own candidate executable, lane installation and private registry.
func ChildScope(scratch, module, executable, registry string) bool {
	if !filepath.IsAbs(scratch) || realpath.Resolve(scratch) != scratch ||
		realpath.Resolve(module) != filepath.Join(scratch, "lane", "metasystem") ||
		realpath.Resolve(executable) != filepath.Join(scratch, "bin", "metasystem") ||
		realpath.Resolve(registry) != filepath.Join(scratch, "registry") {
		return false
	}
	base := filepath.Dir(filepath.Dir(scratch))
	records, unreadable := diskstore.CheckoutRegistry(base).Inventory()
	if len(unreadable) != 0 {
		return false
	}
	for _, record := range records {
		if record.Path == scratch && record.Class == "rehearsal" && record.State == diskstore.StateAccepted &&
			record.Owner == (diskstore.Owner{Kind: diskstore.OwnerWorkspace, Ref: filepath.Base(base)}) {
			return diskstore.Revalidate(record) == nil
		}
	}
	return false
}

// PublishCheckoutRecord is the public rehearsal command's opt-in to keeping
// its candidate evidence in the checkout. Isolated registries never publish.
func (s *Scratch) PublishCheckoutRecord(module string) {
	if !s.isolatedCaller {
		s.recordModule = realpath.Resolve(module)
	}
}
