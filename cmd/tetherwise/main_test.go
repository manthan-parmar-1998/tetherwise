package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "tetherwise-home")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestVersion(t *testing.T) {
	stdout, stderr, err := execute(t, "version")
	if err != nil {
		t.Fatalf("version: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "tetherwise "+version) {
		t.Fatalf("stdout = %q, want version %q", stdout, version)
	}
	if !strings.Contains(stdout, commit) || !strings.Contains(stdout, date) {
		t.Fatalf("stdout = %q, want commit %q and date %q", stdout, commit, date)
	}
}

func TestPendingCommandsExitNonZero(t *testing.T) {
	for _, name := range []string{"graph", "check"} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := execute(t, name)
			if !errors.Is(err, errNotImplemented) {
				t.Fatalf("err = %v, want %v", err, errNotImplemented)
			}
			if !strings.Contains(stderr, "not implemented yet") {
				t.Fatalf("stderr = %q, want the not-implemented message", stderr)
			}
		})
	}
}

func TestHelpListsCommands(t *testing.T) {
	stdout, stderr, err := execute(t, "--help")
	if err != nil {
		t.Fatalf("help: %v\nstderr: %s", err, stderr)
	}
	for _, name := range []string{"graph", "check", "version"} {
		if !strings.Contains(stdout, name) {
			t.Fatalf("help missing %q:\n%s", name, stdout)
		}
	}
}

func TestExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tetherwise.yaml")
	if err := os.WriteFile(path, []byte("note: synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := execute(t, "--config", path, "version"); err != nil {
		t.Fatalf("valid config: %v\nstderr: %s", err, stderr)
	}

	missing := filepath.Join(dir, "missing.yaml")
	if _, _, err := execute(t, "--config", missing, "version"); err == nil {
		t.Fatal("missing --config file succeeded")
	}

	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("a: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, "--config", bad, "version"); err == nil {
		t.Fatal("malformed config succeeded")
	}
}
