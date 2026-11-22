package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/manthan-parmar-1998/tetherwise/internal/check"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func sampleResult(t *testing.T) check.Result {
	t.Helper()
	limit, err := model.NewLimit("aws:opensearch:domain/search-lab", "JVMMemoryPressure", 85, 92, "Percent", model.DirectionLowerBetter)
	if err != nil {
		t.Fatal(err)
	}
	storage, err := model.NewLimit("aws:opensearch:domain/search-lab", "FreeStorageSpace", 20480, 6000, "Megabytes", model.DirectionHigherBetter)
	if err != nil {
		t.Fatal(err)
	}
	return check.Result{
		SchemaVersion: check.SchemaVersion,
		Verdict:       check.VerdictHigh,
		Reasons: []string{
			"update aws:logs:subscription-filter/eks-logs changes filter_pattern.",
			"aws:opensearch:domain/search-lab is shared by Atlas, Keel.",
			"Verdict is high-exposure because a shared resource is at or past a limit, or is being deleted or replaced.",
		},
		Changes: []model.Change{{
			Resource: "aws:logs:subscription-filter/eks-logs", Action: model.ActionUpdate,
			ChangedAttrs: []string{"filter_pattern"},
		}},
		Affected: []check.Affected{{
			Resource: "k8s:deployment/keel/keel-query", Name: "keel/keel-query", Kind: "deployment",
			Team: "Keel", Owner: "keel-oncall",
			SharedResource: "aws:opensearch:domain/search-lab",
			DependencyKind: model.KindReadsFrom, Confidence: model.ConfidenceHigh,
			Sources:  []string{model.SourceEdgesFile},
			Headroom: []model.Limit{storage, limit},
		}},
		Shared: []check.Shared{{
			Resource: "aws:opensearch:domain/search-lab", Kind: "opensearch:domain",
			Teams: []string{"Atlas", "Keel"}, Limits: []model.Limit{limit, storage}, Tight: true,
		}},
		Teams:  []string{"Keel"},
		Owners: []string{"keel-oncall"},
		Coverage: check.Coverage{
			SourcesLoaded:         []string{"edges-file", "terraform"},
			EdgeSources:           []string{"edges-file", "terraform"},
			ResourcesWithoutOwner: []string{"aws:logs:subscription-filter/eks-logs"},
			LowConfidenceEdges:    []string{},
		},
	}
}

func TestMarkdownGolden(t *testing.T) {
	t.Parallel()
	got := Markdown(sampleResult(t))
	for _, want := range []string{
		"**Verdict:** high-exposure",
		"keel/keel-query",
		"FreeStorageSpace -14480",
		"JVMMemoryPressure -7",
		"Resources with no owner: aws:logs:subscription-filter/eks-logs",
		"Low-confidence edges: none",
		"Who else is exposed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q\n%s", want, got)
		}
	}
	path := filepath.Join("testdata", "comment.md")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
	}
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestJSONGolden(t *testing.T) {
	t.Parallel()
	buf, err := JSON(sampleResult(t))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != check.SchemaVersion {
		t.Fatalf("schema = %v", decoded["schema_version"])
	}
	path := filepath.Join("testdata", "report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, buf)
	}
	if !bytes.Equal(buf, want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf, want)
	}
}

func TestEmptyReport(t *testing.T) {
	t.Parallel()
	got := Markdown(check.Result{Verdict: check.VerdictInfo})
	if !strings.Contains(got, "No resource changes") || !strings.Contains(got, "No other service") {
		t.Fatalf("%s", got)
	}
	buf, err := JSON(check.Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf, []byte(`"schema_version": "v0.1"`)) {
		t.Fatalf("%s", buf)
	}
}

func TestCellEscapes(t *testing.T) {
	t.Parallel()
	if cell("a|b\nc") != `a\|b c` {
		t.Fatal(cell("a|b\nc"))
	}
}
