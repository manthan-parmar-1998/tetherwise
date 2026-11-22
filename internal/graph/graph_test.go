package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func res(t *testing.T, id, team string) model.Resource {
	t.Helper()
	r, err := model.ResourceFromID(id)
	if err != nil {
		t.Fatal(err)
	}
	r.Team = team
	return r
}

func dep(from, to, kind, source string, conf model.Confidence) model.Dependency {
	return model.Dependency{
		From: from, To: to, Kind: kind, Source: source,
		Sources: []string{source}, Confidence: conf,
	}
}

func TestMergeDedupeAndShared(t *testing.T) {
	t.Parallel()
	g := New()
	domain := res(t, "aws:opensearch:domain/search-lab", "Platform")
	domain.Attrs = map[string]string{"endpoint": "search-lab.example"}
	indexer := res(t, "k8s:deployment/atlas/atlas-indexer", "Atlas")
	query := res(t, "k8s:deployment/keel/keel-query", "Keel")
	if err := g.Merge(
		[]model.Resource{domain, indexer, query},
		[]model.Dependency{
			dep(indexer.ID, domain.ID, model.KindWritesLogsTo, model.SourceK8sEnv, model.ConfidenceMedium),
			dep(indexer.ID, domain.ID, model.KindWritesLogsTo, model.SourceFluentBit, model.ConfidenceHigh),
			dep(query.ID, domain.ID, model.KindReadsFrom, model.SourceEdgesFile, model.ConfidenceHigh),
		},
	); err != nil {
		t.Fatal(err)
	}
	deps := g.Outgoing(indexer.ID)
	if len(deps) != 1 {
		t.Fatalf("dedupe failed: %#v", deps)
	}
	if deps[0].Confidence != model.ConfidenceHigh || deps[0].Source != model.SourceFluentBit {
		t.Fatalf("kept %#v", deps[0])
	}
	if strings.Join(deps[0].Sources, ",") != "fluentbit,k8s-env" {
		t.Fatalf("sources = %v", deps[0].Sources)
	}
	shared := g.SharedResources()
	if len(shared) != 1 || shared[0].ID != domain.ID {
		t.Fatalf("shared = %#v", shared)
	}
	if got := strings.Join(TeamsOf(g, domain.ID), ","); got != "Atlas,Keel" {
		t.Fatalf("teams = %s", got)
	}
	if down := g.Downstream(domain.ID); len(down) != 2 {
		t.Fatalf("downstream = %v", down)
	}
	if up := g.Upstream(indexer.ID); len(up) != 1 || up[0] != domain.ID {
		t.Fatalf("upstream = %v", up)
	}
	if _, ok := g.Resource(domain.ID); !ok {
		t.Fatal("missing domain")
	}
}

func TestFirstValueWins(t *testing.T) {
	t.Parallel()
	g := New()
	first := res(t, "aws:rds:instance/payments", "Payments")
	first.Owner = "payments-oncall"
	first.Attrs = map[string]string{"endpoint": "one.example"}
	second := res(t, "aws:rds:instance/payments", "Other")
	second.Owner = "other"
	second.Attrs = map[string]string{"endpoint": "two.example", "engine": "postgres"}
	if err := g.Merge([]model.Resource{first}, nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Merge([]model.Resource{second}, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := g.Resource(first.ID)
	if got.Team != "Payments" || got.Owner != "payments-oncall" {
		t.Fatalf("identity overwritten: %#v", got)
	}
	if got.Attrs["endpoint"] != "one.example" || got.Attrs["engine"] != "postgres" {
		t.Fatalf("attrs = %v", got.Attrs)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	g := New()
	a := res(t, "k8s:deployment/atlas/atlas-indexer", "Atlas")
	b := res(t, "aws:opensearch:domain/search-lab", "")
	if err := g.Merge([]model.Resource{a}, []model.Dependency{
		dep(a.ID, b.ID, model.KindConnectsTo, model.SourceTerraform, model.ConfidenceLow),
	}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := g.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"schema_version": "v0.1"`) {
		t.Fatalf("snapshot:\n%s", buf.String())
	}
	loaded, err := ReadJSON(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Resources()) != 2 || len(loaded.Dependencies()) != 1 {
		t.Fatalf("round trip resources=%d deps=%d", len(loaded.Resources()), len(loaded.Dependencies()))
	}
	stub, ok := loaded.Resource(b.ID)
	if !ok || stub.Kind != "opensearch:domain" {
		t.Fatalf("stub = %#v", stub)
	}
}

func TestSnapshotRejects(t *testing.T) {
	t.Parallel()
	if _, err := ReadJSON(strings.NewReader(`{"schema_version":"v9","resources":[],"dependencies":[]}`)); err == nil {
		t.Fatal("wrong version")
	}
	if _, err := ReadJSON(strings.NewReader(`{`)); err == nil {
		t.Fatal("bad json")
	}
}

func TestCycle(t *testing.T) {
	t.Parallel()
	g := New()
	a := res(t, "k8s:deployment/atlas/a", "Atlas")
	b := res(t, "k8s:deployment/keel/b", "Keel")
	if err := g.Merge([]model.Resource{a, b}, []model.Dependency{
		dep(a.ID, b.ID, model.KindConnectsTo, model.SourceTerraform, model.ConfidenceHigh),
		dep(b.ID, a.ID, model.KindConnectsTo, model.SourceTerraform, model.ConfidenceHigh),
	}); err != nil {
		t.Fatal(err)
	}
	up := g.Upstream(a.ID)
	if len(up) != 1 || up[0] != b.ID {
		t.Fatalf("upstream = %v", up)
	}
}

func TestMergeRejects(t *testing.T) {
	t.Parallel()
	g := New()
	if err := g.Merge([]model.Resource{{ID: "nope"}}, nil); err == nil {
		t.Fatal("bad resource")
	}
	if err := g.Merge(nil, []model.Dependency{{From: "nope", To: "nope"}}); err == nil {
		t.Fatal("bad dependency")
	}
}

func TestTieBreakSource(t *testing.T) {
	t.Parallel()
	g := New()
	from := res(t, "k8s:deployment/atlas/atlas-indexer", "Atlas")
	to := res(t, "aws:opensearch:domain/search-lab", "Platform")
	if err := g.Merge([]model.Resource{from, to}, []model.Dependency{
		dep(from.ID, to.ID, model.KindReadsFrom, model.SourceK8sEnv, model.ConfidenceHigh),
		dep(from.ID, to.ID, model.KindReadsFrom, model.SourceEdgesFile, model.ConfidenceHigh),
	}); err != nil {
		t.Fatal(err)
	}
	got := g.Incoming(to.ID)
	if len(got) != 1 || got[0].Source != model.SourceEdgesFile {
		t.Fatalf("tie break = %#v", got)
	}
}
