// Package behaviorsurface owns the versioned definition of which repository
// bytes each gate law judges. Callers name a projection; none may recreate a
// closure from path tests of its own.
package behaviorsurface

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SupportedVersion = 2

// Projection names one deliberately bounded byte claim.
type Projection string

const (
	Engine  Projection = "ENGINE"
	Landing Projection = "LANDING"
	Payload Projection = "PAYLOAD"
)

// Class is the policy-owned explanation for paths with special treatment.
type Class string

const (
	Standard        Class = "STANDARD"
	Coordination    Class = "COORDINATION"
	OperationalData Class = "OPERATIONAL_DATA"
	Tailored        Class = "TAILORED"
	NonRepository   Class = "NON_REPOSITORY"
)

// SkipScope names the equality claim that may authorize an omission.
// WITNESS is ENGINE plus toolchain equality; DELIVERY adds PAYLOAD equality.
type SkipScope string

const (
	WitnessScope  SkipScope = "WITNESS"
	DeliveryScope SkipScope = "DELIVERY"
)

// Policy is the machine-readable behavior-surface declaration. Pattern
// grammar is intentionally small: an exact toplevel-relative path or a
// directory prefix ending in /**.
type Policy struct {
	Version           int      `json:"version"`
	EnginePaths       []string `json:"enginePaths"`
	PayloadRoots      []string `json:"payloadRoots"`
	CoordinationPaths []string `json:"coordinationPaths"`
	// RepositoryOperationalDataPaths stays in Git-toplevel path space so a
	// nested metasystem cannot hide operational data behind prefix mapping.
	RepositoryOperationalDataPaths []string            `json:"repositoryOperationalDataPaths"`
	TailoredPaths                  map[string][]string `json:"tailoredPaths"`
	NonRepositoryPaths             []string            `json:"nonRepositoryPaths"`
	WitnessSkips                   []string            `json:"witnessSkips"`
	DeliveryContractSkips          []string            `json:"deliveryContractSkips"`
}

//go:embed policy.v2.json
var policyBytes []byte

// Load returns the policy embedded in this engine build. This makes a
// prospective policy edit effective only through the engine built from the
// prospective bytes.
func Load() (Policy, error) {
	return loadPolicy(policyBytes)
}

func loadPolicy(data []byte) (Policy, error) {
	var policy Policy
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, fmt.Errorf("behavior-surface policy: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Policy{}, fmt.Errorf("behavior-surface policy has trailing JSON content")
	}
	if policy.Version != SupportedVersion {
		return Policy{}, fmt.Errorf("behavior-surface policy version %d is unsupported (engine supports %d)", policy.Version, SupportedVersion)
	}
	if err := policy.validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

// Bytes returns a defensive copy of the versioned declaration for the CLI.
func Bytes() []byte { return append([]byte(nil), policyBytes...) }

func (p Policy) validate() error {
	groups := map[string][]string{
		"enginePaths": p.EnginePaths, "payloadRoots": p.PayloadRoots,
		"coordinationPaths": p.CoordinationPaths, "nonRepositoryPaths": p.NonRepositoryPaths,
		"repositoryOperationalDataPaths": p.RepositoryOperationalDataPaths,
	}
	for name, paths := range p.TailoredPaths {
		groups["tailoredPaths."+name] = paths
	}
	for name, paths := range groups {
		seen := make(map[string]bool)
		for _, pattern := range paths {
			if err := validPattern(pattern); err != nil {
				return fmt.Errorf("behavior-surface %s: %w", name, err)
			}
			if seen[pattern] {
				return fmt.Errorf("behavior-surface %s repeats %q", name, pattern)
			}
			seen[pattern] = true
		}
	}
	seenScopes := make(map[string]string)
	for _, skipSet := range []struct {
		name     string
		families []string
	}{
		{name: "witnessSkips", families: p.WitnessSkips},
		{name: "deliveryContractSkips", families: p.DeliveryContractSkips},
	} {
		seenSkips := make(map[string]bool)
		for _, family := range skipSet.families {
			if family == "" || strings.ContainsRune(family, '\x00') || seenSkips[family] {
				return fmt.Errorf("behavior-surface %s has invalid or duplicate family %q", skipSet.name, family)
			}
			if priorScope, exists := seenScopes[family]; exists {
				return fmt.Errorf("behavior-surface skip family %q is declared in both %s and %s", family, priorScope, skipSet.name)
			}
			seenSkips[family] = true
			seenScopes[family] = skipSet.name
		}
	}
	return nil
}

func validPattern(pattern string) error {
	if pattern == "" || strings.HasPrefix(pattern, "/") || strings.ContainsRune(pattern, '\x00') {
		return fmt.Errorf("invalid toplevel-relative pattern %q", pattern)
	}
	base := strings.TrimSuffix(pattern, "/**")
	if strings.Contains(base, "*") || filepath.ToSlash(filepath.Clean(filepath.FromSlash(base))) != base || base == "." {
		return fmt.Errorf("invalid pattern %q", pattern)
	}
	return nil
}

// ParseProjection validates a public projection name.
func ParseProjection(value string) (Projection, error) {
	projection := Projection(strings.ToUpper(value))
	switch projection {
	case Engine, Landing, Payload:
		return projection, nil
	default:
		return "", fmt.Errorf("unknown behavior-surface projection %q", value)
	}
}

// ParseSkipScope validates the proof scope named by a skip consumer.
func ParseSkipScope(value string) (SkipScope, error) {
	scope := SkipScope(strings.ToUpper(value))
	switch scope {
	case WitnessScope, DeliveryScope:
		return scope, nil
	default:
		return "", fmt.Errorf("unknown behavior-surface skip scope %q", value)
	}
}

// NormalizePath maps a Git-toplevel-relative path into the policy owner's
// path space. prefix is Git's slash-terminated --show-prefix result and may
// be empty when the metasystem is itself the repository root.
func NormalizePath(name, prefix string) (string, error) {
	if strings.ContainsRune(name, '\x00') || strings.ContainsRune(prefix, '\x00') {
		return "", fmt.Errorf("behavior-surface paths cannot contain NUL")
	}
	name = filepath.ToSlash(name)
	// Git enumerations name a nested checkout or directory with a trailing
	// slash; the marker is notation, not part of the path being classified.
	// A bare slash names the filesystem root, not a directory marker, and
	// must keep its refusal.
	if trimmed := strings.TrimSuffix(name, "/"); trimmed != "" {
		name = trimmed
	}
	prefix = strings.Trim(filepath.ToSlash(prefix), "/")
	if prefix != "" {
		if name == prefix {
			name = ""
		} else if strings.HasPrefix(name, prefix+"/") {
			name = strings.TrimPrefix(name, prefix+"/")
		} else {
			return "", nil
		}
	}
	if name == "" {
		return "", nil
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean != name {
		return "", fmt.Errorf("behavior-surface path is not clean and toplevel-relative: %q", name)
	}
	return clean, nil
}

func matches(pattern, name string) bool {
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		return name == base || strings.HasPrefix(name, base+"/")
	}
	return name == pattern
}

func matchesAny(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if matches(pattern, name) {
			return true
		}
	}
	return false
}

func (p Policy) tailored(name string) bool {
	for _, patterns := range p.TailoredPaths {
		if matchesAny(patterns, name) {
			return true
		}
	}
	return false
}

// Classify explains the path's most specific declared class. Repository
// operational data is classified before prefix mapping. Fresh ledgers are
// TAILORED by name and are independently excluded from LANDING when they also
// match the coordination set.
func (p Policy) Classify(name, prefix string) (Class, error) {
	repositoryName, err := NormalizePath(name, "")
	if err != nil || repositoryName == "" {
		return Standard, err
	}
	if matchesAny(p.RepositoryOperationalDataPaths, repositoryName) {
		return OperationalData, nil
	}
	normalized, err := NormalizePath(name, prefix)
	if err != nil || normalized == "" {
		return Standard, err
	}
	switch {
	case p.tailored(normalized):
		return Tailored, nil
	case matchesAny(p.CoordinationPaths, normalized):
		return Coordination, nil
	case matchesAny(p.NonRepositoryPaths, normalized):
		return NonRepository, nil
	default:
		return Standard, nil
	}
}

// Includes is the only projection membership decision.
func (p Policy) Includes(projection Projection, name, prefix string) (bool, error) {
	cleanPrefix := strings.Trim(filepath.ToSlash(prefix), "/")
	if projection == Landing {
		clean, err := NormalizePath(name, "")
		if err != nil || clean == "" {
			return false, err
		}
		if matchesAny(p.RepositoryOperationalDataPaths, clean) {
			return false, nil
		}
		if cleanPrefix != "" && clean != cleanPrefix && !strings.HasPrefix(clean, cleanPrefix+"/") {
			return true, nil
		}
	}
	normalized, err := NormalizePath(name, prefix)
	if err != nil || normalized == "" {
		return false, err
	}
	switch projection {
	case Engine:
		return matchesAny(p.EnginePaths, normalized), nil
	case Landing:
		return !matchesAny(p.CoordinationPaths, normalized) && !matchesAny(p.NonRepositoryPaths, normalized), nil
	case Payload:
		return matchesAny(p.PayloadRoots, normalized) && !p.tailored(normalized), nil
	default:
		return false, fmt.Errorf("unknown behavior-surface projection %q", projection)
	}
}

// SkipAllowed reports whether the named equality claim may authorize omission
// of this exact validation family. Callers cannot borrow a stronger scope by
// omitting the distinction.
func (p Policy) SkipAllowed(scope SkipScope, family string) bool {
	var declaredFamilies []string
	switch scope {
	case WitnessScope:
		declaredFamilies = p.WitnessSkips
	case DeliveryScope:
		declaredFamilies = p.DeliveryContractSkips
	default:
		return false
	}
	for _, declared := range declaredFamilies {
		if family == declared {
			return true
		}
	}
	return false
}

// Digest is a policy-versioned, projection-scoped, mode-independent digest.
// WalkDir never follows directory symlinks; a symlink contributes its target
// text, and a regular file contributes its content digest. Path records are
// sorted as raw UTF-8 strings and separated with NUL, so whitespace and
// newlines are data.
func (p Policy) Digest(root string, projection Projection) (string, error) {
	return p.DigestWithPrefix(root, projection, "")
}

// DigestWithPrefix hashes a projection from the Git toplevel while mapping a
// nested metasystem through prefix. LANDING remains repository-wide: content
// outside a nested metasystem prefix is included under its toplevel path.
func (p Policy) DigestWithPrefix(root string, projection Projection, prefix string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	type record struct{ path, kind, value string }
	var records []record
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() && rel == ".git" {
			return fs.SkipDir
		}
		if entry.IsDir() && projection != Landing {
			base := strings.Trim(filepath.ToSlash(prefix), "/")
			if base != "" && rel != base && !strings.HasPrefix(rel, base+"/") && !strings.HasPrefix(base, rel+"/") {
				return fs.SkipDir
			}
		}
		included, err := p.Includes(projection, rel, prefix)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !included {
			return nil
		}
		recordPath := rel
		if projection != Landing {
			var normalizeErr error
			recordPath, normalizeErr = NormalizePath(rel, prefix)
			if normalizeErr != nil {
				return normalizeErr
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			records = append(records, record{recordPath, "symlink", target})
		case info.Mode().IsRegular():
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			records = append(records, record{recordPath, "file", hex.EncodeToString(hash.Sum(nil))})
		default:
			return fmt.Errorf("behavior-surface %s contains unsupported file kind at %q", projection, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })
	hash := projectionHash(p.Version, projection)
	for _, item := range records {
		for _, field := range []string{item.path, item.kind, item.value} {
			_, _ = hash.Write([]byte(field))
			_, _ = hash.Write([]byte{0})
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ListPaths returns the existing path set for a projection. It is used to
// carry the source PAYLOAD allowlist to an adopted endpoint: project-owned
// extras beneath a shared directory are not payload bytes, while every source
// path must still exist and match.
func (p Policy) ListPaths(root string, projection Projection) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		included, err := p.Includes(projection, rel, "")
		if err != nil {
			return err
		}
		if included {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// DigestListed hashes exactly the named members of a projection. Every path
// is checked by an lstat component walk, so a target cannot replace a source
// directory with a symlink and smuggle outside bytes into equality.
func (p Policy) DigestListed(root string, projection Projection, paths []string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	hash := projectionHash(p.Version, projection)
	prior := ""
	for _, name := range paths {
		if name == prior {
			return "", fmt.Errorf("behavior-surface path manifest repeats %q", name)
		}
		prior = name
		included, err := p.Includes(projection, name, "")
		if err != nil {
			return "", err
		}
		if !included {
			return "", fmt.Errorf("behavior-surface path manifest names %q outside %s", name, projection)
		}
		clean, err := NormalizePath(name, "")
		if err != nil || clean == "" {
			return "", fmt.Errorf("behavior-surface path manifest has invalid path %q: %w", name, err)
		}
		parts := strings.Split(clean, "/")
		current := root
		var info fs.FileInfo
		for index, part := range parts {
			current = filepath.Join(current, filepath.FromSlash(part))
			info, err = os.Lstat(current)
			if err != nil {
				return "", fmt.Errorf("behavior-surface %s endpoint lacks %q: %w", projection, clean, err)
			}
			if index < len(parts)-1 && info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("behavior-surface %s endpoint has symlink ancestor at %q", projection, strings.Join(parts[:index+1], "/"))
			}
		}
		kind, value := "", ""
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			kind = "symlink"
			value, err = os.Readlink(current)
		case info.Mode().IsRegular():
			kind = "file"
			var file *os.File
			file, err = os.Open(current)
			if err == nil {
				contentHash := sha256.New()
				_, err = io.Copy(contentHash, file)
				closeErr := file.Close()
				if err == nil {
					err = closeErr
				}
				value = hex.EncodeToString(contentHash.Sum(nil))
			}
		default:
			err = fmt.Errorf("unsupported file kind")
		}
		if err != nil {
			return "", fmt.Errorf("behavior-surface %s cannot read %q: %w", projection, clean, err)
		}
		for _, field := range []string{clean, kind, value} {
			_, _ = hash.Write([]byte(field))
			_, _ = hash.Write([]byte{0})
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func projectionHash(version int, projection Projection) hash.Hash {
	h := sha256.New()
	for _, field := range []string{"behavior-surface", strconv.Itoa(version), string(projection)} {
		_, _ = h.Write([]byte(field))
		_, _ = h.Write([]byte{0})
	}
	return h
}

// Change is one side of a path change. Renames are represented by callers as
// one removal and one addition so crossing a policy class cannot disappear.
type Change struct {
	Path string
	Kind string
}

// ClassifyChanges preserves each change side and attaches policy facts.
func (p Policy) ClassifyChanges(projection Projection, prefix string, changes []Change) ([]ClassifiedChange, error) {
	result := make([]ClassifiedChange, 0, len(changes))
	for _, change := range changes {
		class, err := p.Classify(change.Path, prefix)
		if err != nil {
			return nil, err
		}
		included, err := p.Includes(projection, change.Path, prefix)
		if err != nil {
			return nil, err
		}
		result = append(result, ClassifiedChange{Change: change, Class: class, Included: included})
	}
	return result, nil
}

type ClassifiedChange struct {
	Change
	Class    Class
	Included bool
}
