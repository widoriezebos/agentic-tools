package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

func declaredOutputPaths(record Record) ([]string, error) {
	var declared []string
	if len(record.AdapterData["declaredOutputs"]) > 0 {
		if err := json.Unmarshal(record.AdapterData["declaredOutputs"], &declared); err != nil {
			return nil, fmt.Errorf("declared outputs: %w", err)
		}
	}
	seen := map[string]bool{}
	var paths []string
	for _, path := range declared {
		if path == "" {
			return nil, fmt.Errorf("declared output path is empty")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(record.WorkingDirectory, path)
		}
		path = filepath.Clean(path)
		if seen[path] {
			return nil, fmt.Errorf("declared output path is repeated: %s", path)
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, nil
}

// prepareDeclaredOutputs holds one lock per output path until the child has
// exited and its outputs have been collected. A prior file is preserved under
// this launch's state before removal, so a child must create its own result.
func (m *Manager) prepareDeclaredOutputs(record Record, stateDir string) (func(), error) {
	paths, err := declaredOutputPaths(record)
	if err != nil || len(paths) == 0 {
		return func() {}, err
	}
	root, err := m.Store.root()
	if err != nil {
		return nil, err
	}
	locksDir := filepath.Join(root, "declared-output-locks")
	if err := os.MkdirAll(locksDir, 0o700); err != nil {
		return nil, err
	}
	var locks []*os.File
	release := func() {
		for index := len(locks) - 1; index >= 0; index-- {
			_ = unix.Flock(int(locks[index].Fd()), unix.LOCK_UN)
			_ = locks[index].Close()
		}
	}
	lockPaths := append([]string(nil), paths...)
	sort.Strings(lockPaths)
	for _, path := range lockPaths {
		digest := sha256.Sum256([]byte(path))
		lock, openErr := os.OpenFile(filepath.Join(locksDir, fmt.Sprintf("%x.flock", digest)), os.O_CREATE|os.O_RDWR, 0o600)
		if openErr != nil {
			release()
			return nil, openErr
		}
		if lockErr := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); lockErr != nil {
			_ = lock.Close()
			release()
			if errors.Is(lockErr, unix.EWOULDBLOCK) || errors.Is(lockErr, unix.EAGAIN) {
				return nil, fmt.Errorf("declared-output-busy: %s", path)
			}
			return nil, lockErr
		}
		locks = append(locks, lock)
	}
	if err := m.checkEarlierOutputOwners(record, paths); err != nil {
		release()
		return nil, err
	}
	for index, path := range paths {
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			release()
			return nil, fmt.Errorf("inspect declared output %s: %w", path, statErr)
		}
		if !info.Mode().IsRegular() {
			release()
			return nil, fmt.Errorf("declared output is not a regular file: %s", path)
		}
		archive := filepath.Join(stateDir, "previous-outputs", fmt.Sprintf("%d-%s", index, filepath.Base(path)))
		durable, err := atomicfile.CopyFile(path, archive, stateDir)
		if err != nil {
			release()
			return nil, fmt.Errorf("preserve declared output %s: %w", path, err)
		}
		if !durable {
			release()
			return nil, fmt.Errorf("preserve declared output %s: archive durability is unknown", path)
		}
		if err := os.Remove(path); err != nil {
			release()
			return nil, fmt.Errorf("clear declared output %s: %w", path, err)
		}
	}
	return release, nil
}

// A lock disappears when its supervisor exits. The launch record retains
// ownership until the previous process group is proved dead, including when
// the supervisor died between spawning a child and recording its PID.
func (m *Manager) checkEarlierOutputOwners(record Record, paths []string) error {
	records, err := m.Store.List()
	if err != nil {
		return err
	}
	claimed := make(map[string]bool, len(paths))
	for _, path := range paths {
		claimed[path] = true
	}
	for _, prior := range records {
		if prior.ID == record.ID || !prior.OutputOwnerUnproven {
			continue
		}
		priorPaths, err := declaredOutputPaths(prior)
		if err != nil {
			return fmt.Errorf("declared-output-owner-unproven: inspect launch %s: %w", prior.ID, err)
		}
		overlaps := false
		for _, path := range priorPaths {
			if claimed[path] {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue
		}
		if prior.ProcessGroup == nil {
			return fmt.Errorf("declared-output-owner-unproven: launch %s may have spawned an unrecorded writer", prior.ID)
		}
		alive, err := m.Processes.GroupAlive(prior.ProcessGroup.Pid)
		if err != nil || alive {
			return fmt.Errorf("declared-output-owner-unproven: launch %s process group %d has not been proved dead: %v", prior.ID, prior.ProcessGroup.Pid, err)
		}
	}
	return nil
}

func copyDeclaredOutputs(record Record, stateDir string) ([]Output, error) {
	paths, err := declaredOutputPaths(record)
	if err != nil {
		return nil, err
	}
	var outputs []Output
	for _, source := range paths {
		info, err := os.Lstat(source)
		if err != nil {
			return outputs, fmt.Errorf("declared output %s: %w", source, err)
		}
		if !info.Mode().IsRegular() {
			return outputs, fmt.Errorf("declared output is not a regular file: %s", source)
		}
		target := filepath.Join(stateDir, "outputs", filepath.Base(source))
		if _, err := atomicfile.CopyFile(source, target, stateDir); err != nil {
			return outputs, err
		}
		outputs = append(outputs, Output{Path: target, Bytes: info.Size()})
	}
	return outputs, nil
}
