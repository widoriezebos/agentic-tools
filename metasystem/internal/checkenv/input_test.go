package checkenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestAdmission(t *testing.T) {
	t.Parallel()
	for _, ambient := range []bool{true, false} {
		input, err := Admit([]string{"PATH=/first", "PATH=/last", "GOENV=", "GOWORK=/work", "XDG_CONFIG_HOME=/config"}, []string{"CUSTOM="}, ambient)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(input.Environment(), []string{"CUSTOM=", "GOENV=", "GOWORK=/work", "PATH=/last", "XDG_CONFIG_HOME=/config"}) {
			t.Fatal("canonical entries differ")
		}
		if err := input.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, environment := range [][]string{nil, {}} {
		input, err := Admit(environment, nil, true)
		if err != nil || input.Environment() == nil || len(input.Entries) != 0 {
			t.Fatal("empty input must stay explicit")
		}
	}
	a, err := Admit([]string{"AMBIENT=synthetic-one", "GOOGLE_API_KEY=synthetic-two"}, nil, true)
	if err != nil || !slices.Equal(a.Dropped, []string{"AMBIENT", "GOOGLE_API_KEY"}) {
		t.Fatal("ambient diagnostics differ")
	}
	data, _ := json.Marshal(a)
	if strings.Contains(string(data), "synthetic-") {
		t.Fatal("ambient value persisted")
	}
	b, _ := Admit([]string{"OTHER=synthetic-three"}, nil, true)
	if !reflect.DeepEqual(a.Stable(), b.Stable()) {
		t.Fatal("diagnostics affected identity")
	}
	if _, err := Admit([]string{"CUSTOM=synthetic-value"}, nil, false); err == nil || strings.Contains(err.Error(), "synthetic-value") || !strings.Contains(err.Error(), "CUSTOM") {
		t.Fatal("legacy custom input must be refused names-only")
	}
}
func TestUnsupportedInputs(t *testing.T) {
	t.Parallel()
	for _, assignment := range []string{"GOPROXY=https://user:synthetic@host", "GOFLAGS=-proxy=https://user:synthetic@host", "GONOSUMDB=https://user:synthetic@host", "HTTPS_PROXY=https://user:synthetic@host", "GOAUTH=synthetic-command"} {
		for _, declared := range []bool{false, true} {
			var err error
			if declared {
				_, err = Admit(nil, []string{assignment}, false)
			} else {
				_, err = Admit([]string{assignment}, nil, true)
			}
			name, _, _ := strings.Cut(assignment, "=")
			if err == nil || strings.Contains(err.Error(), "synthetic") || !strings.Contains(err.Error(), name) {
				t.Fatal("unsupported value was admitted or disclosed")
			}
		}
	}
	for _, value := range []string{"off", "netrc"} {
		if _, err := Admit([]string{"GOAUTH=" + value}, nil, false); err != nil {
			t.Fatal(err)
		}
	}
}
func TestApprovedExecutableResolution(t *testing.T) {
	directory := t.TempDir()
	program := filepath.Join(directory, "tool")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	for _, environment := range [][]string{nil, {}, {"PATH=" + t.TempDir()}} {
		if _, err := Resolve("tool", directory, environment); err == nil {
			t.Fatal("live PATH supplied the executable")
		}
	}
	if resolved, err := Resolve("tool", directory, []string{"PATH=" + directory}); err != nil || resolved != program {
		t.Fatal("approved PATH not used")
	}
	if resolved, err := Resolve(program, directory, nil); err != nil || resolved != program {
		t.Fatal("absolute executable refused")
	}
}

func TestRetainedInputIsValidated(t *testing.T) {
	t.Parallel()
	for _, input := range []*Input{
		{Version: Version + 1},
		{Version: Version, Entries: []Entry{{Name: "CUSTOM", Value: "synthetic", Source: "host"}}},
		{Version: Version, Entries: []Entry{{Name: "GOAUTH", Value: "unsupported-synthetic", Source: "declared"}}},
		{Version: Version, Entries: []Entry{{Name: "GOPROXY", Value: "https://user:synthetic@host", Source: "host"}}},
		{Version: Version, Entries: []Entry{{Name: "PATH", Source: "host"}, {Name: "PATH", Source: "host"}}},
		{Version: Version, Dropped: []string{"NOT_A_NAME=synthetic"}},
	} {
		if err := input.Validate(); err == nil || strings.Contains(err.Error(), "synthetic") {
			t.Fatal("retained input was accepted or disclosed a value")
		}
	}
}

func TestEmptyStableInputDoesNotDependOnEncoding(t *testing.T) {
	t.Parallel()
	a := (&Input{Version: Version}).Stable()
	b := (&Input{Version: Version, Entries: []Entry{}, Dropped: []string{"IGNORED"}}).Stable()
	if !reflect.DeepEqual(a, b) || a.Environment() == nil {
		t.Fatal("empty encoding changed stable identity or enabled inheritance")
	}
}
