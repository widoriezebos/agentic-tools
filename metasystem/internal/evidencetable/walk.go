package evidencetable

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// The walker is the one way this package touches the tree. Every
// lookup is an openat on a HELD parent descriptor with
// O_NOFOLLOW|O_NONBLOCK, and every verdict is an fstat on the held
// fd: a component can neither be a symlink nor be swapped between
// check and use, and a FIFO cannot hang the walk before fstat
// rejects it. Sequential pathname Lstat is forbidden by design — a
// symlinked ancestor would route it outside the repository.

// ValidateDepPath refuses every path shape openat cannot be trusted
// with: absolute paths ignore the root descriptor, ".." escapes it,
// and empty, ".", NUL, or newline components are never a declared
// dependency. Pure; safe at parse time.
func ValidateDepPath(path string) error {
	if path == "" {
		return errors.New("the path is empty")
	}
	if strings.HasPrefix(path, "/") {
		return errors.New("absolute paths are refused — deps are repository-relative")
	}
	if strings.ContainsAny(path, "\x00\n") {
		return errors.New("NUL and newline bytes are refused")
	}
	for _, comp := range strings.Split(path, "/") {
		switch comp {
		case "":
			return errors.New("empty path components are refused")
		case ".", "..":
			return fmt.Errorf("%q components are refused — deps stay inside the repository", comp)
		}
	}
	return nil
}

// OpenRoot holds the repository root as the walk's anchor descriptor.
func OpenRoot(root string) (int, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return -1, fail("the root %s is a symlink; the walk anchors on the directory itself", root)
		}
		return -1, fail("cannot hold the root %s: %v", root, err)
	}
	return fd, nil
}

// walkOpen opens path component-by-component under rootFD and returns
// the final component's held descriptor. Intermediates must be
// directories; the caller judges the final fd's type and closes it.
func walkOpen(rootFD int, path string) (int, error) {
	if err := ValidateDepPath(path); err != nil {
		return -1, err
	}
	comps := strings.Split(path, "/")
	parent := rootFD
	held := -1
	defer func() {
		if held >= 0 && held != parent {
			unix.Close(held)
		}
	}()
	for i, comp := range comps {
		fd, err := unix.Openat(parent, comp, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if err != nil {
			// The refusal already happened at the open; this lstat on
			// the still-held parent only names the reason (macOS says
			// ENOTDIR where Linux says ELOOP for a symlink under
			// O_NOFOLLOW).
			var st unix.Stat_t
			symlink := unix.Fstatat(parent, comp, &st, unix.AT_SYMLINK_NOFOLLOW) == nil &&
				st.Mode&unix.S_IFMT == unix.S_IFLNK
			if parent != rootFD {
				unix.Close(parent)
				held = -1
			}
			joined := strings.Join(comps[:i+1], "/")
			switch {
			case symlink, errors.Is(err, unix.ELOOP), errors.Is(err, unix.EMLINK):
				return -1, fmt.Errorf("%s is a symlink — the walk follows none, anywhere in the chain", joined)
			case errors.Is(err, unix.ENOTDIR):
				return -1, fmt.Errorf("%s is not a directory on the way to %s", joined, path)
			case errors.Is(err, unix.ENOENT):
				return -1, fmt.Errorf("%s does not exist in the tree", joined)
			default:
				return -1, fmt.Errorf("cannot open %s: %v", joined, err)
			}
		}
		if parent != rootFD {
			unix.Close(parent)
		}
		parent = fd
		held = fd
		// Every component's type verdict comes from fstat ON THE HELD
		// descriptor — intermediates must be directories, and nothing
		// is ever judged by a second path lookup.
		if i < len(comps)-1 {
			var st unix.Stat_t
			if err := unix.Fstat(fd, &st); err != nil {
				unix.Close(fd)
				held = -1
				return -1, fmt.Errorf("cannot stat %s on the held handle: %v", strings.Join(comps[:i+1], "/"), err)
			}
			if st.Mode&unix.S_IFMT != unix.S_IFDIR {
				unix.Close(fd)
				held = -1
				return -1, fmt.Errorf("%s is not a directory on the way to %s", strings.Join(comps[:i+1], "/"), path)
			}
		}
	}
	held = -1
	return parent, nil
}

// CheckDep proves one declared dependency present and safe.
// entrypoint demands a regular file; otherwise a directory passes too.
func CheckDep(rootFD int, path string, entrypoint bool) error {
	fd, err := walkOpen(rootFD, path)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return fmt.Errorf("cannot stat %s on the held handle: %v", path, err)
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		return nil
	case unix.S_IFDIR:
		if entrypoint {
			return fmt.Errorf("%s is a directory, and the first dep is the proof's entrypoint FILE", path)
		}
		return nil
	default:
		return fmt.Errorf("%s is neither a regular file nor a directory", path)
	}
}

// LoadTable walks to the evidence table under rootFD, reads it from
// the held descriptor, and parses it. The table gets the same
// discipline as its dependencies: no component may be a symlink, and
// the bytes judged are the bytes the held handle serves.
func LoadTable(rootFD int, root string) (*Table, error) {
	fd, err := walkOpen(rootFD, TableFilename)
	if err != nil {
		return nil, fail("%s: %v", TableFilename, err)
	}
	file := os.NewFile(uintptr(fd), TableFilename)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fail("cannot stat %s: %v", TableFilename, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fail("%s must be a regular file, not %s", TableFilename, info.Mode().Type())
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fail("cannot read %s: %v", TableFilename, err)
	}
	return Parse(data, TableFilename)
}
