package gopackages

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAffectedGoSelectionOwnsAssetsDeletionAndBuildableInventory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSelectionFile(t, root, "go.mod", "module example.invalid/selection\n\ngo 1.27\n")
	writeSelectionFile(t, root, "a/a.go", "package a\n")
	writeSelectionFile(t, root, "a/assets/value.txt", "one\n")
	writeSelectionFile(t, root, "a/testdata/golden.json", "{}\n")
	writeSelectionFile(t, root, "a/header.h", "/* header */\n")
	writeSelectionFile(t, root, "b/b.go", "package b\n")
	writeSelectionFile(t, root, "b/b_test.go", "package b\nimport _ \"example.invalid/selection/a\"\n")
	writeSelectionFile(t, root, "c/c.go", "package c\n")
	writeSelectionFile(t, root, "outer/keep.txt", "keep\n")
	writeSelectionFile(t, root, "outer/missing/inner/value.go", "package inner\n")
	writeSelectionFile(t, root, "tagonly/tagged.go", "//go:build tools\n\npackage tagonly\n")
	writeSelectionFile(t, root, "featureonly/tagged.go", "//go:build feature\n\npackage featureonly\n")
	writeSelectionFile(t, root, "futureonly/future.go", "//go:build go1.99\n\npackage futureonly\n")
	writeSelectionFile(t, root, "archfeatureonly/feature.go", "//go:build amd64.v99\n\npackage archfeatureonly\n")
	writeSelectionFile(t, root, "compileronly/compiler.go", "//go:build gccgo\n\npackage compileronly\n")
	writeSelectionFile(t, root, "onlytest/only_test.go", "package onlytest\nimport \"testing\"\nfunc TestOnly(t *testing.T) {}\n")
	writeSelectionFile(t, root, "cgoonly/c.go", "package cgoonly\nimport \"C\"\n")
	writeSelectionFile(t, root, "unsatisfied/only.go", "//go:build tools && go1.99\n\npackage unsatisfied\n")
	writeSelectionFile(t, root, "unsatisfiedlegacy/only.go", "// +build tools,go1.99\n\npackage unsatisfiedlegacy\n")
	writeSelectionFile(t, root, "unsatisfiedmultiline/only.go", "// +build tools\n// +build go1.99\n\npackage unsatisfiedmultiline\n")
	otherPlatform := "windows"
	if build.Default.GOOS == otherPlatform {
		otherPlatform = "linux"
	}
	writeSelectionFile(t, root, "platformonly/only_"+otherPlatform+".go", "package platformonly\n")
	writeSelectionFile(t, root, "ignored/_ignored.go", "package ignored\n")
	writeSelectionFile(t, root, "docs/readme.txt", "read me\n")
	check := func(t *testing.T, path string, edit func(string), tags []string, openBase bool) (Selection, *selectionFixture) {
		t.Helper()
		fixture := newSelectionFixture(t, root, path, edit, openBase)
		selection, err := fixture.selectWithTags(tags)
		if err != nil {
			t.Fatal(err)
		}
		return selection, fixture
	}
	for _, asset := range []string{"a/assets/value.txt", "a/testdata/golden.json", "a/header.h"} {
		t.Run(asset, func(t *testing.T) {
			selected, _ := check(t, asset, func(candidate string) { writeSelectionFile(t, candidate, asset, "changed\n") }, nil, false)
			if !slices.Equal(selected.Changed, []string{"./a"}) || !slices.Equal(selected.Packages, []string{"./a", "./b"}) {
				t.Fatalf("asset %s selected changed=%v packages=%v", asset, selected.Changed, selected.Packages)
			}
		})
	}
	baseSource, _ := check(t, "a/a.go", func(candidate string) {
		writeSelectionFile(t, candidate, "a/a.go", "package a\nconst Value = 2\n")
	}, nil, false)
	if !slices.Equal(baseSource.Changed, []string{"./a"}) ||
		!slices.Equal(baseSource.Dependents, []string{"./b"}) ||
		!slices.Equal(baseSource.Packages, []string{"./a", "./b"}) ||
		!slices.Contains(baseSource.InputDirs["./b"], "./a") ||
		!slices.Contains(baseSource.InputDirs["./b"], "./b") {
		t.Fatalf("base source did not retain its test-import consumer and input directories: %+v", baseSource)
	}
	deleted, _ := check(t, "outer/missing/inner/value.go", func(candidate string) {
		if err := os.Remove(filepath.Join(candidate, "outer", "missing", "inner", "value.go")); err != nil {
			t.Fatal(err)
		}
	}, nil, true)
	if !slices.Equal(deleted.Changed, []string{"./..."}) || !slices.Contains(deleted.Packages, "./a") || !slices.Contains(deleted.Packages, "./b") {
		t.Fatalf("deleted leaf under surviving non-Go ancestor = %+v", deleted)
	}
	manifest, manifestFixture := check(t, "go.mod", func(candidate string) {
		writeSelectionFile(t, candidate, "go.mod", "module example.invalid/selection\n\ngo 1.27\n// changed\n")
	}, nil, false)
	if !slices.Equal(manifest.Changed, []string{"./..."}) || slices.Contains(manifest.Packages, "./tagonly") ||
		slices.Contains(manifest.Packages, "./featureonly") || slices.Contains(manifest.Packages, "./ignored") || slices.Contains(manifest.Packages, "./platformonly") ||
		slices.Contains(manifest.Packages, "./unsatisfied") || slices.Contains(manifest.Packages, "./unsatisfiedlegacy") ||
		slices.Contains(manifest.Packages, "./unsatisfiedmultiline") ||
		!slices.Contains(manifest.Packages, "./onlytest") {
		t.Fatalf("untagged manifest inventory included excluded package: %+v", manifest)
	}
	cgoDisabled, err := manifestFixture.selectWithEnvironment(nil, []string{"CGO_ENABLED=0"})
	if err != nil || slices.Contains(cgoDisabled.Packages, "./cgoonly") || !slices.Contains(cgoDisabled.Packages, "./onlytest") {
		t.Fatalf("cgo-disabled/test-only inventory = %+v err=%v", cgoDisabled, err)
	}
	listed := exec.Command("go", "list", "./...")
	listed.Dir = manifestFixture.candidate
	listed.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := listed.CombinedOutput()
	if err != nil {
		t.Fatalf("native cgo-disabled package inventory: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "example.invalid/selection/onlytest") ||
		strings.Contains(string(output), "example.invalid/selection/cgoonly") ||
		strings.Contains(string(output), "example.invalid/selection/unsatisfied") {
		t.Fatalf("native package inventory disagrees with selector: %s", output)
	}
	for _, uncertainToolPackage := range []string{"./futureonly", "./archfeatureonly", "./compileronly"} {
		if !slices.Contains(manifest.Packages, uncertainToolPackage) {
			t.Fatalf("possible cross-tool build tag %s was silently excluded: %+v", uncertainToolPackage, manifest)
		}
	}
	tagged, taggedFixture := check(t, "go.mod", func(candidate string) {
		writeSelectionFile(t, candidate, "go.mod", "module example.invalid/selection\n\ngo 1.27\n// changed\n")
	}, []string{"feature"}, false)
	if !slices.Contains(tagged.Packages, "./featureonly") || slices.Contains(tagged.Packages, "./tagonly") {
		t.Fatalf("feature-tagged inventory = %+v", tagged)
	}
	fromEnvironment, err := taggedFixture.selectWithEnvironment(nil, []string{"GOFLAGS=-tags=feature"})
	if err != nil || !slices.Contains(fromEnvironment.Packages, "./featureonly") || slices.Contains(fromEnvironment.Packages, "./tagonly") {
		t.Fatalf("GOFLAGS-tagged inventory = %+v err=%v", fromEnvironment, err)
	}
	if _, err := taggedFixture.selectWithEnvironment(nil, []string{"GOFLAGS=-overlay=dirty.json"}); err == nil || !strings.Contains(err.Error(), "overlay") {
		t.Fatalf("unresolved Go overlay allowed a possibly stale package inventory: %v", err)
	}
	platform, err := taggedFixture.selectWithEnvironment(nil, []string{"GOOS=" + otherPlatform, "GOARCH=amd64"})
	if err != nil || !slices.Contains(platform.Packages, "./platformonly") {
		t.Fatalf("target-platform inventory = %+v err=%v", platform, err)
	}
	unknown, _ := check(t, "docs/readme.txt", func(candidate string) { writeSelectionFile(t, candidate, "docs/readme.txt", "changed\n") }, nil, true)
	if !slices.Equal(unknown.Changed, []string{"./..."}) || !slices.Contains(unknown.Packages, "./a") {
		t.Fatalf("unowned asset did not select full candidate inventory: %+v", unknown)
	}
}

func TestDeletedLastGoPackageAllowsEmptyCandidateInventory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSelectionFile(t, root, "go.mod", "module example.invalid/last\n\ngo 1.27\n")
	writeSelectionFile(t, root, "leaf/leaf.go", "package leaf\n")
	writeSelectionFile(t, root, "outer/keep.txt", "keep\n")
	fixture := newSelectionFixture(t, root, "leaf/leaf.go", func(candidate string) {
		if err := os.Remove(filepath.Join(candidate, "leaf", "leaf.go")); err != nil {
			t.Fatal(err)
		}
	}, true)
	selected, err := fixture.selectWithTags(nil)
	if err != nil || len(selected.Packages) != 0 || !slices.Equal(selected.Changed, []string{"./..."}) {
		t.Fatalf("last Go package deletion selection=%+v err=%v", selected, err)
	}
}

func writeSelectionFile(t *testing.T, root, path, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
