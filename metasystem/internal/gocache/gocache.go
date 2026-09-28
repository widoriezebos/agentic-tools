// Package gocache resolves the one machine engine cache: Go's build cache
// and staticcheck's cache beside it. The outermost engine process, whose HOME
// is real, resolves once and sets both variables explicitly on every child
// that may compile, so a nested engine under a replaced HOME inherits the
// outer values and never asks os.UserCacheDir (disk-lifetimes rule A1).
package gocache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Paths are the resolved engine cache locations, both absolute.
type Paths struct {
	GoCache          string `json:"goCache"`
	StaticcheckCache string `json:"staticcheckCache"`
}

// Resolve keeps an inherited absolute GOCACHE and STATICCHECK_CACHE from
// environment and computes a missing or relative one beneath the process's
// os.UserCacheDir as go-build and staticcheck.
func Resolve(environment []string) (Paths, error) {
	return ResolveUsing(environment, os.UserCacheDir)
}

// ResolveUsing is Resolve with the user cache directory behind a seam; a nil
// seam is os.UserCacheDir.
func ResolveUsing(environment []string, userCacheDir func() (string, error)) (Paths, error) {
	if userCacheDir == nil {
		userCacheDir = os.UserCacheDir
	}
	paths := Paths{GoCache: inherited(environment, "GOCACHE"), StaticcheckCache: inherited(environment, "STATICCHECK_CACHE")}
	if paths.GoCache != "" && paths.StaticcheckCache != "" {
		return paths, nil
	}
	base, err := userCacheDir()
	if err == nil && !filepath.IsAbs(base) {
		err = errors.New("user cache directory is not absolute")
	}
	if err != nil {
		return paths, errors.New("engine cache: cannot resolve the user cache directory: " + err.Error())
	}
	if paths.GoCache == "" {
		paths.GoCache = filepath.Join(base, "go-build")
	}
	if paths.StaticcheckCache == "" {
		paths.StaticcheckCache = filepath.Join(base, "staticcheck")
	}
	return paths, nil
}

// Environment is the explicit child environment for paths; an empty path is
// left out.
func Environment(paths Paths) []string {
	var result []string
	if paths.GoCache != "" {
		result = append(result, "GOCACHE="+paths.GoCache)
	}
	if paths.StaticcheckCache != "" {
		result = append(result, "STATICCHECK_CACHE="+paths.StaticcheckCache)
	}
	return result
}

// Carry resolves paths from environment and returns environment with both
// variables set explicitly after it. On a resolution error it carries what
// resolved and leaves the rest to the child.
func Carry(environment []string) []string {
	paths, _ := Resolve(environment)
	return append(append([]string(nil), environment...), Environment(paths)...)
}

// inherited is the last value of name when it is absolute; a relative value
// is not inherited.
func inherited(environment []string, name string) string {
	value := ""
	for _, entry := range environment {
		if key, found, ok := strings.Cut(entry, "="); ok && key == name {
			value = found
		}
	}
	if !filepath.IsAbs(value) {
		return ""
	}
	return filepath.Clean(value)
}
