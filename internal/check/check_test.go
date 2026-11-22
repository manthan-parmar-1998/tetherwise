package check

import (
	"testing"

	"github.com/manthan-parmar-1998/tetherwise/internal/graph"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func mustRes(t *testing.T, id, team, owner string) model.Resource {
	t.Helper()
	res, err := model.ResourceFromID(id)
	if err != nil {
		t.Fatal(err)
	}
	res.Team = team
	res.Owner = owner
	return res
}

func mustDep(from, to, kind, source string, conf model.Confidence) model.Dependency {
	return model.Dependency{
		From: from, To: to, Kind: kind, Source: source,
		Sources: []string{source}, Confidence: conf,
	}
}

func mustLimit(t *testing.T, metric string, threshold, current float64, dir model.Direction) model.Limit {
	t.Helper()
	limit, err := model.NewLimit("aws:opensearch:domain/search-lab", metric, threshold, current, "Percent", dir)
	if err != nil {
		t.Fatal(err)
	}
	return limit
}

type fixture struct {
	g       *graph.Graph
	domain  string
	indexer string
	query   string
	filter  string
	group   string
}

func build(t *testing.T) fixture {
	t.Helper()
	g := graph.New()
	domain := mustRes(t, "aws:opensearch:domain/search-lab", "Platform", "platform-oncall")
	indexer := mustRes(t, "k8s:deployment/atlas/atlas-indexer", "Atlas", "atlas-oncall")
	query := mustRes(t, "k8s:deployment/keel/keel-query", "Keel", "keel-oncall")
	filter := mustRes(t, "aws:logs:subscription-filter/eks-logs", "", "")
	group := mustRes(t, "aws:logs:log-group/eks-lab", "", "")
	err := g.Merge([]model.Resource{domain, indexer, query, filter, group}, []model.Dependency{
		mustDep(filter.ID, domain.ID, model.KindWritesLogsTo, model.SourceTerraform, model.ConfidenceHigh),
		mustDep(filter.ID, group.ID, model.KindReadsFrom, model.SourceTerraform, model.ConfidenceHigh),
		mustDep(indexer.ID, domain.ID, model.KindWritesLogsTo, model.SourceFluentBit, model.ConfidenceHigh),
		mustDep(query.ID, domain.ID, model.KindReadsFrom, model.SourceEdgesFile, model.ConfidenceLow),
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{g: g, domain: domain.ID, indexer: indexer.ID, query: query.ID, filter: filter.ID, group: group.ID}
}

func TestVerdictTable(t *testing.T) {
	t.Parallel()
	tight, err := model.NewLimit("aws:opensearch:domain/search-lab", "JVMMemoryPressure", 85, 92, "Percent", model.DirectionLowerBetter)
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := model.NewLimit("aws:opensearch:domain/search-lab", "CPUUtilization", 80, 10, "Percent", model.DirectionLowerBetter)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		changes []model.Change
		limits  []model.Limit
		want    Verdict
	}{
		{
			name: "tight shared",
			changes: []model.Change{{
				Resource: "aws:logs:subscription-filter/eks-logs", Action: model.ActionUpdate,
				ChangedAttrs: []string{"filter_pattern"},
			}},
			limits: []model.Limit{tight},
			want:   VerdictHigh,
		},
		{
			name: "healthy shared",
			changes: []model.Change{{
				Resource: "aws:logs:subscription-filter/eks-logs", Action: model.ActionUpdate,
				ChangedAttrs: []string{"filter_pattern"},
			}},
			limits: []model.Limit{healthy},
			want:   VerdictWarn,
		},
		{
			name: "unknown headroom",
			changes: []model.Change{{
				Resource: "aws:logs:log-group/eks-lab", Action: model.ActionUpdate,
				ChangedAttrs: []string{"retention_in_days"},
			}},
			want: VerdictWarn,
		},
		{
			name: "delete shared",
			changes: []model.Change{{
				Resource: "aws:opensearch:domain/search-lab", Action: model.ActionDelete,
			}},
			limits: []model.Limit{healthy},
			want:   VerdictHigh,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fix := build(t)
			result, err := Run(fix.g, tt.changes, tt.limits, []string{"terraform", "kubernetes"}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if result.Verdict != tt.want {
				t.Fatalf("verdict = %s, reasons = %v", result.Verdict, result.Reasons)
			}
			if result.SchemaVersion != SchemaVersion {
				t.Fatal(result.SchemaVersion)
			}
			if tt.want == VerdictHigh && len(result.Affected) == 0 && tt.changes[0].Resource != fix.domain {
				t.Fatal("expected affected services")
			}
		})
	}
}

func TestInfoAndSingleTeam(t *testing.T) {
	t.Parallel()
	g := graph.New()
	only := mustRes(t, "k8s:deployment/atlas/atlas-indexer", "Atlas", "atlas-oncall")
	domain := mustRes(t, "aws:opensearch:domain/search-lab", "Platform", "")
	filter := mustRes(t, "aws:logs:subscription-filter/eks-logs", "", "")
	if err := g.Merge([]model.Resource{only, domain, filter}, []model.Dependency{
		mustDep(only.ID, domain.ID, model.KindConnectsTo, model.SourceTerraform, model.ConfidenceHigh),
		mustDep(filter.ID, domain.ID, model.KindWritesLogsTo, model.SourceTerraform, model.ConfidenceHigh),
	}); err != nil {
		t.Fatal(err)
	}
	result, err := Run(g, []model.Change{{
		Resource: filter.ID, Action: model.ActionUpdate, ChangedAttrs: []string{"filter_pattern"},
	}}, nil, []string{"terraform"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictWarn {
		t.Fatalf("single team verdict = %s (%v)", result.Verdict, result.Reasons)
	}

	lonely := graph.New()
	box := mustRes(t, "aws:ec2:security-group/solo", "Atlas", "atlas-oncall")
	if err := lonely.Merge([]model.Resource{box}, nil); err != nil {
		t.Fatal(err)
	}
	info, err := Run(lonely, []model.Change{{Resource: box.ID, Action: model.ActionUpdate, ChangedAttrs: []string{"description"}}}, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Verdict != VerdictInfo {
		t.Fatalf("info = %s %v", info.Verdict, info.Reasons)
	}
	if len(info.Coverage.ResourcesWithoutOwner) != 0 {
		t.Fatal(info.Coverage.ResourcesWithoutOwner)
	}
}

func TestCoverageAndTightFraction(t *testing.T) {
	t.Parallel()
	fix := build(t)
	near, err := model.NewLimit(fix.domain, "CPUUtilization", 100, 90, "Percent", model.DirectionLowerBetter)
	if err != nil {
		t.Fatal(err)
	}
	if !Tight(near, 0.15) {
		t.Fatal("90/100 should be tight at 0.15")
	}
	if Tight(near, 0.05) {
		t.Fatal("90/100 is not within 5 percent")
	}
	result, err := Run(fix.g, []model.Change{{
		Resource: fix.filter, Action: model.ActionUpdate, ChangedAttrs: []string{"filter_pattern"},
	}}, []model.Limit{near}, []string{"terraform", "edges-file"}, Options{TightFraction: 0.05})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictWarn {
		t.Fatalf("fraction override verdict = %s", result.Verdict)
	}
	if len(result.Coverage.LowConfidenceEdges) != 1 {
		t.Fatalf("low edges = %v", result.Coverage.LowConfidenceEdges)
	}
	if len(result.Teams) != 2 {
		t.Fatalf("teams = %v", result.Teams)
	}
	found := false
	for _, id := range result.Coverage.ResourcesWithoutOwner {
		if id == fix.filter {
			found = true
		}
	}
	if !found {
		t.Fatalf("unowned = %v", result.Coverage.ResourcesWithoutOwner)
	}
}

func TestRunRejects(t *testing.T) {
	t.Parallel()
	if _, err := Run(nil, nil, nil, nil, Options{}); err == nil {
		t.Fatal("nil graph")
	}
	g := graph.New()
	if _, err := Run(g, []model.Change{{Resource: "nope", Action: model.ActionUpdate}}, nil, nil, Options{}); err == nil {
		t.Fatal("bad change")
	}
	bad := mustLimit(t, "CPUUtilization", 80, 1, model.DirectionLowerBetter)
	bad.Headroom = 0
	fix := build(t)
	if _, err := Run(fix.g, []model.Change{{
		Resource: fix.filter, Action: model.ActionUpdate, ChangedAttrs: []string{"filter_pattern"},
	}}, []model.Limit{bad}, nil, Options{}); err == nil {
		t.Fatal("bad limit")
	}
}

func TestReplaceIsHigh(t *testing.T) {
	t.Parallel()
	fix := build(t)
	result, err := Run(fix.g, []model.Change{{Resource: fix.domain, Action: model.ActionReplace}}, nil, []string{"terraform"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictHigh {
		t.Fatalf("verdict = %s", result.Verdict)
	}
	if result.Shared[0].Tight != true {
		t.Fatal("replace should mark the shared resource tight")
	}
}
