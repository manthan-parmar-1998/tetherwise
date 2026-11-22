package edges

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func TestParse(t *testing.T) {
	t.Parallel()
	buf := []byte(`apiVersion: tetherwise.io/v1
kind: Edges
edges:
  - from: k8s:deployment/keel/keel-query
    to: aws:opensearch:domain/search-lab
    kind: reads-from
`)
	deps, err := Parse(buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Confidence != model.ConfidenceHigh || deps[0].Source != model.SourceEdgesFile {
		t.Fatalf("%#v", deps)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "tetherwise.edges.yaml")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		"apiVersion: no\nkind: Edges\n",
		"apiVersion: tetherwise.io/v1\nkind: Graph\n",
		"apiVersion: tetherwise.io/v1\nkind: Edges\nedges:\n  - from: bad\n    to: also\n    kind: reads-from\n",
		": [\n",
	}
	for _, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Fatalf("accepted %q", doc)
		}
	}
}
