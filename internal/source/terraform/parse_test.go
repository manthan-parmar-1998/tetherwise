package terraform

import (
	"testing"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func TestParseState(t *testing.T) {
	t.Parallel()
	resources, deps, err := ParseStateFile("testdata/state.json")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]model.Resource{}
	for _, res := range resources {
		byID[res.ID] = res
	}
	db, ok := byID["aws:rds:instance/payments"]
	if !ok {
		t.Fatalf("missing rds instance: %v", ids(resources))
	}
	if db.Team != "Payments" || db.Owner != "payments-oncall" {
		t.Fatalf("owner = %s/%s", db.Team, db.Owner)
	}
	if db.Attrs["endpoint"] != "payments.lab.example.local" {
		t.Fatalf("endpoint = %q", db.Attrs["endpoint"])
	}
	if _, ok := db.Attrs["password"]; ok {
		t.Fatal("password attribute leaked")
	}
	if _, ok := byID["aws:msk:cluster/events"]; !ok {
		t.Fatal("child module cluster missing")
	}
	if !hasDep(deps, "aws:rds:instance/payments", "aws:ec2:security-group/db", model.KindConnectsTo, model.ConfidenceHigh) {
		t.Fatalf("security group edge missing: %#v", deps)
	}
	if !hasDep(deps, "aws:ssm:parameter/db-host", "aws:rds:instance/payments", model.KindConnectsTo, model.ConfidenceMedium) {
		t.Fatalf("endpoint edge missing: %#v", deps)
	}
}

func TestParsePlan(t *testing.T) {
	t.Parallel()
	resources, deps, changes, err := ParsePlanFile("testdata/plan.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findResource(resources, "aws:opensearch:domain/search-lab"); !ok {
		t.Fatalf("domain missing: %v", ids(resources))
	}
	if findResourceID(resources, "aws:region") {
		t.Fatal("data source was included")
	}
	if !hasDep(deps, "aws:logs:subscription-filter/eks-logs", "aws:opensearch:domain/search-lab", model.KindWritesLogsTo, model.ConfidenceHigh) {
		t.Fatalf("log edge missing: %#v", deps)
	}
	if !hasDep(deps, "aws:logs:subscription-filter/eks-logs", "aws:logs:log-group/eks-lab", model.KindReadsFrom, model.ConfidenceHigh) {
		t.Fatalf("log group edge missing: %#v", deps)
	}
	if len(changes) != 2 {
		t.Fatalf("changes = %#v", changes)
	}
	byRes := map[string]model.Change{}
	for _, change := range changes {
		byRes[change.Resource] = change
	}
	group := byRes["aws:logs:log-group/eks-lab"]
	if group.Action != model.ActionUpdate || len(group.ChangedAttrs) != 1 || group.ChangedAttrs[0] != "retention_in_days" {
		t.Fatalf("log group change = %#v", group)
	}
	filter := byRes["aws:logs:subscription-filter/eks-logs"]
	if filter.Action != model.ActionReplace {
		t.Fatalf("filter action = %s", filter.Action)
	}
	if containsAttr(filter.ChangedAttrs, "password") {
		t.Fatal("sensitive attr reported")
	}
	if !containsAttr(filter.ChangedAttrs, "filter_pattern") {
		t.Fatalf("filter attrs = %v", filter.ChangedAttrs)
	}
}

func TestParsePlanRejects(t *testing.T) {
	t.Parallel()
	if _, _, _, err := ParsePlanFile("testdata/missing.json"); err == nil {
		t.Fatal("missing file")
	}
	if _, _, _, err := ParsePlanFile("testdata/bad.json"); err == nil {
		t.Fatal("invalid plan")
	}
	if _, _, err := ParseStateFile("testdata/bad.json"); err == nil {
		t.Fatal("invalid state")
	}
}

func ids(resources []model.Resource) []string {
	out := make([]string, len(resources))
	for i, res := range resources {
		out[i] = res.ID
	}
	return out
}

func findResource(resources []model.Resource, id string) (model.Resource, bool) {
	for _, res := range resources {
		if res.ID == id {
			return res, true
		}
	}
	return model.Resource{}, false
}

func findResourceID(resources []model.Resource, prefix string) bool {
	for _, res := range resources {
		if len(res.ID) >= len(prefix) && res.ID[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func hasDep(deps []model.Dependency, from, to, kind string, conf model.Confidence) bool {
	for _, dep := range deps {
		if dep.From == from && dep.To == to && dep.Kind == kind && dep.Confidence == conf && dep.Source == model.SourceTerraform {
			return true
		}
	}
	return false
}

func containsAttr(attrs []string, want string) bool {
	for _, attr := range attrs {
		if attr == want {
			return true
		}
	}
	return false
}
