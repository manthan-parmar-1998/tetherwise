package main

import (
	"bytes"
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
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE",
		"AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"KUBECONFIG",
	} {
		_ = os.Unsetenv(key)
	}
	_ = os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func execute(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	a := newApp()
	cmd := a.command()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err != nil {
		return stdout.String(), stderr.String(), 1
	}
	if a.exitCode != 0 {
		return stdout.String(), stderr.String(), a.exitCode
	}
	return stdout.String(), stderr.String(), 0
}

func TestVersion(t *testing.T) {
	stdout, stderr, code := execute(t, "version")
	if code != 0 {
		t.Fatalf("version code %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "tetherwise "+version) {
		t.Fatalf("stdout = %q, want version %q", stdout, version)
	}
	if !strings.Contains(stdout, commit) || !strings.Contains(stdout, date) {
		t.Fatalf("stdout = %q, want commit %q and date %q", stdout, commit, date)
	}
}

func TestHelpListsCommands(t *testing.T) {
	stdout, stderr, code := execute(t, "--help")
	if code != 0 {
		t.Fatalf("help code %d: %s", code, stderr)
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
	if _, stderr, code := execute(t, "--config", path, "version"); code != 0 {
		t.Fatalf("valid config code %d: %s", code, stderr)
	}

	missing := filepath.Join(dir, "missing.yaml")
	if _, _, code := execute(t, "--config", missing, "version"); code == 0 {
		t.Fatal("missing --config file succeeded")
	}

	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("a: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := execute(t, "--config", bad, "version"); code == 0 {
		t.Fatal("malformed config succeeded")
	}
}

func TestGraphRequiresInput(t *testing.T) {
	_, stderr, code := execute(t, "graph")
	if code == 0 {
		t.Fatal("graph with no sources succeeded")
	}
	if !strings.Contains(stderr, "--tf-state") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func scenarioDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "lab", "scenarios", "opensearch-shared-overload")
}

func TestScenarioCheck(t *testing.T) {
	dir := scenarioDir(t)
	stdout, stderr, code := execute(t, "check", filepath.Join(dir, "plan.json"),
		"--k8s", filepath.Join(dir, "k8s"),
		"--edges", filepath.Join(dir, "tetherwise.edges.yaml"),
		"--metrics", "fixture:"+filepath.Join(dir, "metrics.json"),
		"--format", "markdown",
	)
	if code != 0 {
		t.Fatalf("check code %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "**Verdict:** high-exposure") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	for _, want := range []string{"atlas/atlas-indexer", "keel/keel-query", "keel/keel-checkout", "Atlas", "Keel", "JVMMemoryPressure", "FreeStorageSpace"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q\n%s", want, stdout)
		}
	}
	path := filepath.Join(dir, "expected-comment.md")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(stdout), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, stdout)
	}
	if stdout != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout, want)
	}

	jsonOut, stderr, code := execute(t, "check", filepath.Join(dir, "plan.json"),
		"--k8s", filepath.Join(dir, "k8s"),
		"--edges", filepath.Join(dir, "tetherwise.edges.yaml"),
		"--metrics", "fixture:"+filepath.Join(dir, "metrics.json"),
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("json code %d\n%s", code, stderr)
	}
	if !strings.Contains(jsonOut, `"schema_version": "v0.1"`) || !strings.Contains(jsonOut, `"verdict": "high-exposure"`) {
		t.Fatalf("json:\n%s", jsonOut)
	}

	_, stderr, code = execute(t, "check", filepath.Join(dir, "plan.json"),
		"--k8s", filepath.Join(dir, "k8s"),
		"--edges", filepath.Join(dir, "tetherwise.edges.yaml"),
		"--metrics", "fixture:"+filepath.Join(dir, "metrics.json"),
		"--fail-on", "high",
	)
	if code != 2 {
		t.Fatalf("fail-on code = %d\n%s", code, stderr)
	}
}

func TestGraphSnapshotFeedsCheck(t *testing.T) {
	dir := t.TempDir()
	scenario := scenarioDir(t)
	snapshot := filepath.Join(dir, "snapshot.json")
	_, stderr, code := execute(t, "graph",
		"--tf-plan", filepath.Join(scenario, "plan.json"),
		"--k8s", filepath.Join(scenario, "k8s"),
		"--edges", filepath.Join(scenario, "tetherwise.edges.yaml"),
		"-o", snapshot,
	)
	if code != 0 {
		t.Fatalf("graph code %d\n%s", code, stderr)
	}
	stdout, stderr, code := execute(t, "check", filepath.Join(scenario, "plan.json"),
		"--snapshot", snapshot,
		"--metrics", "fixture:"+filepath.Join(scenario, "metrics.json"),
		"--format", "markdown",
	)
	if code != 0 {
		t.Fatalf("check code %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "high-exposure") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestBadFlags(t *testing.T) {
	scenario := scenarioDir(t)
	if _, _, code := execute(t, "check", filepath.Join(scenario, "plan.json"), "--format", "html"); code == 0 {
		t.Fatal("bad format")
	}
	if _, _, code := execute(t, "check", filepath.Join(scenario, "plan.json"), "--fail-on", "low"); code == 0 {
		t.Fatal("bad fail-on")
	}
	if _, _, code := execute(t, "check", filepath.Join(scenario, "plan.json"), "--metrics", "nope"); code == 0 {
		t.Fatal("bad metrics")
	}
}
