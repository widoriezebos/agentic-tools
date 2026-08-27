package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigTailorCarriesAppetiteGrace(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		set  string
		want string
	}{
		{name: "missing gets built-in", want: "appetite.overrun-grace-percent=25"},
		{name: "existing survives", line: "appetite.overrun-grace-percent=10\n", want: "appetite.overrun-grace-percent=10"},
		{name: "explicit set wins", set: "appetite.overrun-grace-percent=40", want: "appetite.overrun-grace-percent=40"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			body := "metasystem.runtimes=fake\nrole.default.runtime=fake\n" + tc.line
			if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--conf", conf, "--runtimes", "fake"}
			if tc.set != "" {
				args = append(args, "--set", tc.set)
			}
			if code := runConfigTailor(args); code != 0 {
				t.Fatalf("config tailor exited %d", code)
			}
			data, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(data), tc.want) != 1 {
				t.Fatalf("tailored configuration does not carry exactly %q:\n%s", tc.want, data)
			}
		})
	}
}

func TestConfigValidatePrintsOneAppetiteDefaultInfoLine(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "development", "metasystem-design.md"), []byte("template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(repo, "metasystem.conf")
	body := "metasystem.runtimes=fake\n" +
		"evidence.root=" + t.TempDir() + "\n" +
		"role.default.runtime=fake\n" +
		"role.default.model.fake=fake-model\n" +
		"model.tier.1=fake:fake-model\n"
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prior := os.Stdout
	os.Stdout = write
	code := runConfigValidate([]string{"--conf", conf, "--repo", repo})
	os.Stdout = prior
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("config validate exited %d", code)
	}
	line := "INFO: appetite.overrun-grace-percent is absent; using the built-in 25 percent grace band"
	if strings.Count(string(out), line) != 1 {
		t.Fatalf("validation did not print exactly one appetite default information line: %q", out)
	}
}
