package testimpact

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func BuildRequest(ctx context.Context, start, base, mode string) (Request, error) {
	rootBytes, err := git(ctx, start, "rev-parse", "--show-toplevel")
	if err != nil {
		return Request{}, err
	}
	root := strings.TrimSpace(string(rootBytes))
	if root, err = filepath.Abs(root); err != nil {
		return Request{}, err
	}
	if base == "" {
		base = "HEAD"
	}
	baseBytes, err := git(ctx, root, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return Request{}, fmt.Errorf("resolve base %q: %w", base, err)
	}
	resolved := strings.TrimSpace(string(baseBytes))
	conflicts, err := git(ctx, root, "ls-files", "-u", "-z")
	if err != nil {
		return Request{}, err
	}
	if len(conflicts) != 0 {
		return Request{}, fmt.Errorf("the working tree has unresolved conflicts")
	}
	tracked, err := git(ctx, root, "diff", "--name-only", "-z", "--no-renames", resolved, "--")
	if err != nil {
		return Request{}, err
	}
	untracked, err := git(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return Request{}, err
	}
	paths, err := changedPaths(append(tracked, untracked...))
	if err != nil {
		return Request{}, err
	}
	binding, err := bindChanges(root, resolved, paths)
	if err != nil {
		return Request{}, err
	}
	request := Request{SchemaVersion: SchemaVersion, Mode: mode, Base: resolved, RepositoryRoot: root, ChangedPaths: paths, Binding: binding}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}
func changedPaths(data []byte) ([]string, error) {
	seen := map[string]bool{}
	for _, raw := range strings.Split(string(data), "\x00") {
		if raw == "" {
			continue
		}
		if !utf8.ValidString(raw) || !relativePath(raw) {
			return nil, fmt.Errorf("changed path is not a safe UTF-8 repository path: %q", raw)
		}
		seen[raw] = true
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}
func bindChanges(root, base string, paths []string) (string, error) {
	hash := sha256.New()
	writeBound(hash.Write, []byte(base))
	for _, path := range paths {
		writeBound(hash.Write, []byte(path))
		absolute := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(absolute)
		if os.IsNotExist(err) {
			writeBound(hash.Write, []byte("deleted"))
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect changed path %s: %w", path, err)
		}
		var mode string
		var content []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			mode = "120000"
			target, readErr := os.Readlink(absolute)
			if readErr != nil {
				return "", fmt.Errorf("read changed symlink %s: %w", path, readErr)
			}
			content = []byte(target)
		case info.Mode().IsRegular():
			mode = "100644"
			if info.Mode().Perm()&0o111 != 0 {
				mode = "100755"
			}
			content, err = os.ReadFile(absolute)
			if err != nil {
				return "", fmt.Errorf("read changed path %s: %w", path, err)
			}
		default:
			return "", fmt.Errorf("changed path %s is not a file or symlink", path)
		}
		contentHash := sha256.Sum256(content)
		writeBound(hash.Write, []byte(mode))
		writeBound(hash.Write, contentHash[:])
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func writeBound(write func([]byte) (int, error), value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = write(length[:])
	_, _ = write(value)
}
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
