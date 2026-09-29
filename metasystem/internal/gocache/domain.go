package gocache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Domain is a cache's trust domain (disk-lifetimes 3.1): engine code
// compiles into the engine cache, delegates into the delegate cache, so a
// delegate can plant nothing the seat's own build reuses.
type Domain string

const (
	// DomainEngine is Go's and staticcheck's default cache under the user
	// cache dir: the machine cache every engine build and every person's
	// plain `go build` shares.
	DomainEngine Domain = "engine"
	// DomainDelegate is the one cache every delegate round of every adapter
	// shares, beside the engine's under metasystem- names.
	DomainDelegate Domain = "delegate"
)

// Directory names beneath the user cache dir.
const (
	EngineGoCacheName       = "go-build"
	EngineStaticcheckName   = "staticcheck"
	DelegateGoCacheName     = "metasystem-delegate-go-build"
	DelegateStaticcheckName = "metasystem-delegate-staticcheck"
)

// ParseDomain accepts exactly engine or delegate.
func ParseDomain(value string) (Domain, error) {
	switch Domain(value) {
	case DomainEngine, DomainDelegate:
		return Domain(value), nil
	}
	return "", fmt.Errorf("unknown cache domain %q: want engine or delegate", value)
}

// DomainPaths is the domain's cache pair under os.UserCacheDir.
func DomainPaths(domain Domain) (Paths, error) {
	return DomainPathsUsing(domain, os.UserCacheDir)
}

// DelegatePaths is the delegate cache pair under os.UserCacheDir.
func DelegatePaths() (Paths, error) { return DomainPaths(DomainDelegate) }

// DomainPathsUsing is DomainPaths with the user cache dir behind a seam; a
// nil seam is os.UserCacheDir. The paths are a pure function of that
// directory and never read GOCACHE.
func DomainPathsUsing(domain Domain, userCacheDir func() (string, error)) (Paths, error) {
	if _, err := ParseDomain(string(domain)); err != nil {
		return Paths{}, err
	}
	if userCacheDir == nil {
		userCacheDir = os.UserCacheDir
	}
	base, err := userCacheDir()
	if err == nil && !filepath.IsAbs(base) {
		err = errors.New("user cache directory is not absolute")
	}
	if err != nil {
		return Paths{}, fmt.Errorf("%s cache: cannot resolve the user cache directory: %v", domain, err)
	}
	base = filepath.Clean(base)
	if domain == DomainDelegate {
		return Paths{GoCache: filepath.Join(base, DelegateGoCacheName), StaticcheckCache: filepath.Join(base, DelegateStaticcheckName)}, nil
	}
	return Paths{GoCache: filepath.Join(base, EngineGoCacheName), StaticcheckCache: filepath.Join(base, EngineStaticcheckName)}, nil
}

// Directories lists the pair's nonempty directories, Go's cache first.
func (p Paths) Directories() []string {
	var result []string
	for _, dir := range []string{p.GoCache, p.StaticcheckCache} {
		if dir != "" {
			result = append(result, dir)
		}
	}
	return result
}
