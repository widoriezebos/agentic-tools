package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// textExtensions name the files whose line endings are normalised before
// hashing, so a checkout made on a machine that rewrites them still digests to
// the value the bundle recorded. Everything else, a font or an image, is
// hashed byte for byte.
var textExtensions = map[string]struct{}{
	".ts": {}, ".tsx": {}, ".mts": {}, ".cts": {},
	".js": {}, ".mjs": {}, ".cjs": {}, ".jsx": {},
	".json": {}, ".css": {}, ".html": {}, ".svg": {},
	".txt": {}, ".md": {},
}

// keptDotFiles are the only names beginning with "." the digest reads. They
// pin the toolchain, so a change to either must invalidate the bundle; the
// rest are editor and tool droppings that never reach a build. Both are text.
var keptDotFiles = map[string]struct{}{".nvmrc": {}, ".npmrc": {}}

// SourceDigest walks dir and reports the digest of the source a bundle was
// built from, with the per-file digests it is made of. The installed
// dependency tree is not part of it: the lockfile pins what npm installs, and
// the tree itself is not repository content.
func SourceDigest(dir string) (string, []FileDigest, error) {
	var files []FileDigest
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "node_modules" || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if _, kept := keptDotFiles[name]; strings.HasPrefix(name, ".") && !kept {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		// A link's target could change the build without changing the digest,
		// and neither implementation of this rule follows one.
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link; the interface source tree holds none", relative)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s is neither a regular file nor a directory", relative)
		}
		if !isASCII(relative) {
			return fmt.Errorf("%s is not an ASCII path; the interface source tree holds none", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isTextFile(name) {
			content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
		}
		sum := sha256.Sum256(content)
		files = append(files, FileDigest{Path: relative, SHA256: hex.EncodeToString(sum[:])})
		return nil
	}
	if err := filepath.WalkDir(dir, walk); err != nil {
		return "", nil, err
	}
	slices.SortFunc(files, func(a, b FileDigest) int { return strings.Compare(a.Path, b.Path) })
	return DigestOf(files), files, nil
}

// DigestOf renders the sorted per-file digests as the text the source digest is
// taken over: one "<hex>  <path>" line each, in path order.
func DigestOf(files []FileDigest) string {
	var text strings.Builder
	for _, file := range files {
		text.WriteString(file.SHA256)
		text.WriteString("  ")
		text.WriteString(file.Path)
		text.WriteString("\n")
	}
	sum := sha256.Sum256([]byte(text.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func isTextFile(name string) bool {
	if _, kept := keptDotFiles[name]; kept {
		return true
	}
	_, text := textExtensions[strings.ToLower(filepath.Ext(name))]
	return text
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 {
			return false
		}
	}
	return true
}
